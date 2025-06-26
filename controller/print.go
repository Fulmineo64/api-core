package controller

import (
	"bytes"
	"net/http"
	"os"
	"path"
	"reflect"

	"api_core/interfaces"
	"api_core/message"
	"api_core/model"
	"api_core/permissions"
	"api_core/request"
	"api_core/utils"

	"github.com/Datosystem/gofpdf"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func PrintHandler(printFunc func(*gin.Context) *gofpdf.Fpdf, fileFunc func(*gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		pdf := printFunc(c)
		pdfFile := fileFunc(c)
		if request.AbortIfError(c, pdf.Error()) {
			return
		}
		fileBuffer := new(bytes.Buffer)
		pdf.Output(fileBuffer)
		c.DataFromReader(http.StatusOK, int64(fileBuffer.Len()), "application/pdf", fileBuffer, map[string]string{
			"Content-Description":       "File Transfer",
			"Content-Transfer-Encoding": "binary",
			"Content-Disposition":       `attachment; filename="` + pdfFile + `"`,
		})
	}
}

func PrintReadWriteHandler(printFunc func(*gin.Context) *gofpdf.Fpdf, pathFunc, fileFunc func(*gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		pdfPath := pathFunc(c)
		pdfFile := fileFunc(c)
		pathGetter := func(c *gin.Context) string { return pdfPath }
		fileGetter := func(c *gin.Context) string { return pdfFile }

		if !c.GetBool("SkipPrintWriteHandler") {
			_, err := os.Stat(path.Join(pdfPath, pdfFile))
			if os.IsNotExist(err) {
				PrintWriteHandler(printFunc, pathGetter, fileGetter)(c)
				if c.IsAborted() {
					return
				}
			}
		}
		PrintReadHandler(pathGetter, fileGetter)(c)
	}
}

func PrintReadHandler(pathFunc, fileFunc func(*gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		pdfPath := pathFunc(c)
		pdfFile := fileFunc(c)

		c.Header("Content-Disposition", `filename="`+pdfFile+`"`)
		c.File(path.Join(pdfPath, pdfFile))
	}
}

func PrintWriteHandler(printFunc func(*gin.Context) *gofpdf.Fpdf, pathFunc, fileFunc func(*gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := printFunc(c)
		if p != nil {
			if request.AbortIfError(c, p.Error()) {
				return
			}
			writePdf(c, p, pathFunc(c), fileFunc(c))
		}
	}
}

func writePdf(c *gin.Context, p *gofpdf.Fpdf, folder, file string) {
	if p.Error() != nil {
		request.AbortWithError(c, p.Error())
		return
	}
	if _, err := os.Stat(folder); os.IsNotExist(err) {
		os.MkdirAll(folder, os.ModePerm)
	}
	err := p.OutputFileAndClose(path.Join(folder, file))
	if err != nil {
		request.AbortWithError(c, p.Error())
	}
}

func Print(pattern string, printFunc func(*gin.Context) *gofpdf.Fpdf, pathFunc, fileFunc func(*gin.Context) string, perms FileSystemPermissions) []interfaces.Route {
	return []interfaces.Route{
		{Method: http.MethodGet, Pattern: pattern, Permissions: permissions.Merge(perms.Get, perms.Conditions), Handler: PrintReadWriteHandler(printFunc, pathFunc, fileFunc)},
		{Method: http.MethodPost, Pattern: pattern, Permissions: permissions.Merge(perms.Post, perms.Conditions), Handler: PrintWriteHandler(printFunc, pathFunc, fileFunc)},
	}
}

func DefaultPrintPermissions(ctrl interfaces.Modeler) FileSystemPermissions {
	mdl := ctrl.Model()
	printPermissions := FileSystemPermissions{
		Get:  permissions.Get(mdl),
		Post: permissions.Post(mdl),
	}
	if condMdl, ok := mdl.(model.ConditionsModel); ok {
		printPermissions.Conditions = func(c *gin.Context) error {
			db := c.MustGet("db").(*gorm.DB)
			primaries := map[string]interface{}{}
			msg := GetPathParamsMsg(c, mdl, utils.GetPrimaryFields(reflect.TypeOf(mdl)), &primaries)
			if msg != nil {
				return msg
			}

			tx := db.Model(mdl).Where(primaries)
			table := mdl.(model.TableModel).TableName()
			query, args := condMdl.DefaultConditions(db, table)
			if query != "" {
				tx = tx.Where("("+query+")", args...)
			}

			var count int64
			tx.Count(&count)

			if count == 0 {
				return message.ItemNotFound(c)
			}
			return nil
		}
	}
	return printPermissions
}
