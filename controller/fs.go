package controller

import (
	"api_core/interfaces"
	"api_core/message"
	"api_core/model"
	"api_core/permissions"
	"api_core/request"
	"api_core/utils"
	"encoding/base64"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func RemoveParentPaths(path string) string {
	return filepath.Clean(strings.ReplaceAll(path, "..", ""))
}

func Folder(pathFunc func(*gin.Context) string) func(*gin.Context) {
	return func(c *gin.Context) {
		type fileInfo struct {
			NAME          string
			PATH          string
			DATE_MODIFIED time.Time
			SIZE          string
		}

		filesDetailed := []fileInfo{}
		files := []string{}

		detailed := c.Query("detailed")

		basePath := pathFunc(c)
		err := filepath.Walk(basePath, func(path string, info fs.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				path = strings.ReplaceAll(path, "\\", "/")
				trimmedPath := strings.TrimPrefix(path, basePath+"/")
				trimmedPathRight := strings.TrimRight(path, trimmedPath)

				size := info.Size()

				unit := "B"

				if size >= 1<<30 {
					size /= 1 << 30
					unit = "GB"
				} else if size >= 1<<20 {
					size /= 1 << 20
					unit = "MB"
				} else if size >= 1<<10 {
					size /= 1 << 10
					unit = "KB"
				}

				if detailed != "" {
					filesDetailed = append(filesDetailed, fileInfo{
						NAME:          trimmedPath,
						PATH:          trimmedPathRight,
						DATE_MODIFIED: info.ModTime(),
						SIZE:          fmt.Sprintf("%d %s", size, unit),
					})
				} else {
					files = append(files, trimmedPath)
				}
			}
			return nil
		})

		if err != nil {
			log.Printf("Error walking the path %q: %v\n", basePath, err)
		}
		if detailed != "" {
			c.JSON(http.StatusOK, gin.H{"files": filesDetailed})
		} else {
			c.JSON(http.StatusOK, gin.H{"files": files})
		}
	}
}

func GetFile(pathFunc, nameFunc func(*gin.Context) string) func(*gin.Context) {
	return func(c *gin.Context) {
		path := RemoveParentPaths(pathFunc(c))
		name := nameFunc(c)
		file := filepath.Join(path, name)
		if !strings.HasPrefix(filepath.Clean(file), path) {
			message.Forbidden(c).Abort(c)
			return
		}

		if c.Query("download") == "" && c.Query("base64") == "" {
			c.Header("Content-Disposition", "inline; filename="+name)
		} else if c.Query("base64") != "" {
			bytebuffer, err := os.ReadFile(file)
			if err != nil {
				message.Forbidden(c).Abort(c)
				return
			}
			content := base64.StdEncoding.EncodeToString(bytebuffer)
			c.Data(http.StatusOK, "application/octet-stream", []byte(content))
			return
		} else {
			c.Header("Content-Description", "File Transfer")
			c.Header("Content-Transfer-Encoding", "binary")
			c.Header("Content-Disposition", "attachment; filename="+name)
			c.Header("Content-Type", "application/octet-stream")
		}
		c.File(file)
	}
}

func GetFileOrFolder(pathFunc, nameFunc func(*gin.Context) string) func(*gin.Context) {
	return func(c *gin.Context) {
		path := RemoveParentPaths(pathFunc(c))
		name := nameFunc(c)
		file := filepath.Join(path, name)
		if !strings.HasPrefix(filepath.Clean(file), path) {
			message.Forbidden(c).Abort(c)
			return
		}

		fileInfo, err := os.Stat(file)
		if err != nil {
			message.FileNotFound(c).Abort(c)
			return
		}

		if fileInfo.IsDir() {
			folderFunc := func(c *gin.Context) string {
				return file
			}
			Folder(folderFunc)(c)
		} else {
			filePathFunc := func(c *gin.Context) string {
				return path
			}
			fileNameFunc := func(c *gin.Context) string {
				return name
			}
			GetFile(filePathFunc, fileNameFunc)(c)
		}
	}
}

func PostFile(pathFunc func(*gin.Context) string) func(*gin.Context) {
	return func(c *gin.Context) {
		path := RemoveParentPaths(pathFunc(c))
		numeroFiles := c.Query("numeroFiles")

		if len(numeroFiles) > 0 {
			fileLength, err := strconv.Atoi(numeroFiles)
			if err != nil {
				// ... handle error
				panic(err)
			}
			for i := 0; i < fileLength; i++ {
				file, err := c.FormFile("file" + strconv.Itoa(i))
				if err != nil {
					message.BadRequest(c).Text(err.Error()).Abort(c)
					log.Println(err)
					return
				}
				newFileName := file.Filename
				err = c.SaveUploadedFile(file, filepath.Join(path, newFileName))
				if err != nil {
					message.InternalServerError(c).Text("Unable to save the file").Write(c)
					return
				}
			}
		} else {
			file, err := c.FormFile("file")
			if err != nil {
				message.BadRequest(c).Text(err.Error()).Write(c)
				log.Println(err)
				return
			}
			newFileName := file.Filename
			err = c.SaveUploadedFile(file, filepath.Join(path, newFileName))
			if err != nil {
				message.InternalServerError(c).Text("Unable to save the file").Write(c)
				return
			}
		}
		message.Ok(c).JSON(c)
	}
}

func DeleteFile(pathFunc, nameFunc func(*gin.Context) string) func(*gin.Context) {
	return func(c *gin.Context) {
		path := RemoveParentPaths(pathFunc(c))
		name := nameFunc(c)
		file := filepath.Join(path, name)
		if !strings.HasPrefix(filepath.Clean(file), path) {
			message.Forbidden(c).Abort(c)
			return
		}
		err := os.Remove(file)
		if err != nil {
			if !os.IsNotExist(err) {
				request.AbortWithError(c, err)
				return
			}
		}
		message.Ok(c).JSON(c)
	}
}

func CheckResourceAvailable(db *gorm.DB, mdl any) bool {
	if condMdl, ok := mdl.(model.ConditionsModel); ok {
		tx := db.Model(mdl)
		table := mdl.(model.TableModel).TableName()
		query, args := condMdl.DefaultConditions(db, table)
		if query != "" {
			tx = tx.Where("("+query+")", args...)
		}
		var count int64
		tx.Count(&count)
		if count == 0 {
			return false
		}
	}

	return true
}

func GetAllFileInFolder(pathFunc func(*gin.Context) string) func(c *gin.Context) {
	return func(c *gin.Context) {

		path := RemoveParentPaths(pathFunc(c))
		dirCon, err := os.ReadDir(path)
		if err != nil {
			message.Forbidden(c).Abort(c)
			return
		}
		arrayNames := []string{}
		for i := range dirCon {

			if !dirCon[i].IsDir() {
				name := dirCon[i].Name()
				arrayNames = append(arrayNames, filepath.Join(path, name))
			}
		}
		c.JSON(http.StatusOK, arrayNames)
	}
}

type FileSystemPermissions struct {
	Get        permissions.HandlerFunc
	Post       permissions.HandlerFunc
	GetFile    permissions.HandlerFunc
	Delete     permissions.HandlerFunc
	Conditions permissions.HandlerFunc
}

type FileSystemOptions struct {
	SubFolder bool
}

func FileSystem(apiPath string, filePath func(*gin.Context) string, fsPermissions FileSystemPermissions, options FileSystemOptions) []interfaces.Route {
	routes := []interfaces.Route{}
	routes = append(routes, interfaces.Route{Method: http.MethodGet, Pattern: apiPath, Handler: Folder(filePath), Permissions: permissions.Merge(fsPermissions.Get, fsPermissions.Conditions)})
	routes = append(routes, interfaces.Route{Method: http.MethodPost, Pattern: apiPath, Handler: PostFile(filePath), Permissions: permissions.Merge(fsPermissions.Post, fsPermissions.Conditions)})
	if options.SubFolder {
		routes = append(routes, interfaces.Route{Method: http.MethodGet, Pattern: apiPath + "/{name}", Handler: GetFileOrFolder(filePath, func(c *gin.Context) string { return c.Param("name") }), Permissions: permissions.Merge(fsPermissions.GetFile, fsPermissions.Conditions)})
	} else {
		routes = append(routes, interfaces.Route{Method: http.MethodGet, Pattern: apiPath + "/{name}", Handler: GetFile(filePath, func(c *gin.Context) string { return c.Param("name") }), Permissions: permissions.Merge(fsPermissions.GetFile, fsPermissions.Conditions)})
	}
	routes = append(routes, interfaces.Route{Method: http.MethodDelete, Pattern: apiPath + "/{name}", Handler: DeleteFile(filePath, func(c *gin.Context) string { return c.Param("name") }), Permissions: permissions.Merge(fsPermissions.Delete, fsPermissions.Conditions)})

	if options.SubFolder {
		filePathFolder := func(c *gin.Context) string {
			return path.Join(filePath(c), c.Param("name"))
		}
		routes = append(routes, interfaces.Route{Method: http.MethodPost, Pattern: apiPath + "/{name}", Handler: PostFile(filePathFolder), Permissions: permissions.Merge(fsPermissions.Post, fsPermissions.Conditions)})
		routes = append(routes, interfaces.Route{Method: http.MethodGet, Pattern: apiPath + "/{name}/{fileName}", Handler: GetFile(filePathFolder, func(c *gin.Context) string { return c.Param("fileName") }), Permissions: permissions.Merge(fsPermissions.GetFile, fsPermissions.Conditions)})
		routes = append(routes, interfaces.Route{Method: http.MethodDelete, Pattern: apiPath + "/{name}/{fileName}", Handler: DeleteFile(filePathFolder, func(c *gin.Context) string { return c.Param("fileName") }), Permissions: permissions.Merge(fsPermissions.Delete, fsPermissions.Conditions)})
	}
	return routes
}

func DefaultFileSystemPermissions(ctrl interfaces.Modeler) FileSystemPermissions {
	mdl := ctrl.Model()
	fsPermissions := FileSystemPermissions{
		Get:     permissions.Get(mdl),
		Post:    permissions.Post(mdl),
		GetFile: permissions.Get(mdl),
		Delete:  permissions.Delete(mdl),
	}
	if condMdl, ok := mdl.(model.ConditionsModel); ok {
		fsPermissions.Conditions = func(c *gin.Context) error {
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
	return fsPermissions
}
