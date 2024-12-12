package controller

import (
	"api_core/ctx"
	"api_core/message"
	"api_core/permissions"
	"api_core/utils"
	"encoding/base64"
	"fmt"
	"io"
	"io/fs"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"api_core/model"

	"github.com/gin-gonic/gin"
	"github.com/go-chi/render"
	"gorm.io/gorm"
)

func RemoveParentPaths(path string) string {
	return filepath.Clean(strings.ReplaceAll(path, "..", ""))
}

func Folder(pathFunc func(*http.Request) string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		type fileInfo struct {
			NAME          string
			PATH          string
			DATE_MODIFIED time.Time
			SIZE          string
		}

		filesDetailed := []fileInfo{}
		files := []string{}

		detailed := r.URL.Query().Get("detailed")

		basePath := pathFunc(r)
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
			render.JSON(w, r, gin.H{"files": filesDetailed})
		} else {
			render.JSON(w, r, gin.H{"files": files})
		}
	}
}

func GetFile(pathFunc, nameFunc func(*http.Request) string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		path := RemoveParentPaths(pathFunc(r))
		name := nameFunc(r)
		file := filepath.Join(path, name)
		if !strings.HasPrefix(filepath.Clean(file), path) {
			message.Forbidden(r).Write(w, r)
			return
		}

		if r.URL.Query().Get("download") == "" && r.URL.Query().Get("base64") == "" {
			w.Header().Set("Content-Disposition", "inline; filename="+name)
		} else if r.URL.Query().Get("base64") != "" {
			bytebuffer, err := os.ReadFile(file)
			if err != nil {
				message.Forbidden(r).Write(w, r)
				return
			}
			content := base64.StdEncoding.EncodeToString(bytebuffer)
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write([]byte(content))
			w.WriteHeader(http.StatusOK)
			return
		} else {
			w.Header().Set("Content-Description", "File Transfer")
			w.Header().Set("Content-Transfer-Encoding", "binary")
			w.Header().Set("Content-Disposition", "attachment; filename="+name)
			w.Header().Set("Content-Type", "application/octet-stream")
		}
		http.ServeFile(w, r, file)
	}
}

func GetFileOrFolder(pathFunc, nameFunc func(*http.Request) string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		path := RemoveParentPaths(pathFunc(r))
		name := nameFunc(r)
		file := filepath.Join(path, name)
		if !strings.HasPrefix(filepath.Clean(file), path) {
			message.Forbidden(r).Write(w, r)
			return
		}

		fileInfo, err := os.Stat(file)
		if err != nil {
			message.FileNotFound(r).Write(w, r)
			return
		}

		if fileInfo.IsDir() {
			folderFunc := func(r *http.Request) string {
				return file
			}
			Folder(folderFunc)(w, r)
		} else {
			filePathFunc := func(*http.Request) string {
				return path
			}
			fileNameFunc := func(*http.Request) string {
				return name
			}
			GetFile(filePathFunc, fileNameFunc)(w, r)
		}
	}
}

func PostFile(pathFunc func(*http.Request) string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		path := RemoveParentPaths(pathFunc(r))
		numeroFiles := r.URL.Query().Get("numeroFiles")

		if len(numeroFiles) > 0 {
			fileLength, err := strconv.Atoi(numeroFiles)
			if err != nil {
				// ... handle error
				panic(err)
			}
			for i := 0; i < fileLength; i++ {
				file, fileHeader, err := r.FormFile("file" + strconv.Itoa(i))
				if err != nil {
					message.BadRequest(r).Text(err.Error()).Write(w, r)
					log.Println(err)
					return
				}
				defer file.Close()
				newFileName := fileHeader.Filename
				err = SaveUploadedFile(file, filepath.Join(path, newFileName))
				if err != nil {
					message.InternalServerError(r).Text("Unable to save the file").Write(w, r)
					return
				}
			}
		} else {
			file, fileHeader, err := r.FormFile("file")
			if err != nil {
				message.BadRequest(r).Text(err.Error()).Write(w, r)
				log.Println(err)
				return
			}
			defer file.Close()
			newFileName := fileHeader.Filename
			err = SaveUploadedFile(file, filepath.Join(path, newFileName))
			if err != nil {
				message.InternalServerError(r).Text("Unable to save the file").Write(w, r)
				return
			}
		}
		message.Ok(r).Write(w, r)
	}
}

func SaveUploadedFile(file multipart.File, path string) error {
	// Create the directory if it doesn't exist
	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return err
	}

	// Create the file at the specified destination
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()

	// Copy the content of the uploaded file to the destination file
	_, err = io.Copy(out, file)
	if err != nil {
		return err
	}

	return nil
}

func DeleteFile(pathFunc, nameFunc func(*http.Request) string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		path := RemoveParentPaths(pathFunc(r))
		name := nameFunc(r)
		file := filepath.Join(path, name)
		if !strings.HasPrefix(filepath.Clean(file), path) {
			message.Forbidden(r).Write(w, r)
			return
		}
		err := os.Remove(file)
		if err != nil {
			if !os.IsNotExist(err) {
				AbortWithError(w, r, err)
				return
			}
		}
		message.Ok(r).Write(w, r)
	}
}

func CRDController() {
	/*

	 */

	// TODO: Registrare funzioni che gestiscono GET all, GET singolo, POST, DELETE per la cartella specificata, le route disponibili sempre con la stringa CRD
	// TODO: Aggiungere una PermissionFunc specificata dall'utente per ogni metodo (GET, POST e DELETE)
	// TODO: Creare permission func customizzata in base alla risorsa padre (ottenibile da controller.GetModel)
	// TODO: Controllare, nella permission func customizzata, se il modello ha la funzione DefaultConditions, se si eseguire una query COUNT con esse per determinare se l'utente ha accesso alla risorsa base
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

func GetAllFileInFolder(pathFunc func(*http.Request) string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		path := RemoveParentPaths(pathFunc(r))
		dirCon, err := os.ReadDir(path)
		if err != nil {
			message.Forbidden(r).Write(w, r)
			return
		}
		arrayNames := []string{}
		for i := range dirCon {

			if !dirCon[i].IsDir() {
				name := dirCon[i].Name()
				arrayNames = append(arrayNames, filepath.Join(path, name))
			}
		}
		render.JSON(w, r, arrayNames)
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

func FileSystem(ctrl AddRouter, apiPath string, filePath func(*http.Request) string, fsPermissions FileSystemPermissions, options FileSystemOptions) {
	ctrl.AddRoute(http.MethodGet, apiPath, permissions.Merge(fsPermissions.Get, fsPermissions.Conditions), Folder(filePath))
	ctrl.AddRoute(http.MethodPost, apiPath, permissions.Merge(fsPermissions.Post, fsPermissions.Conditions), PostFile(filePath))
	if options.SubFolder {
		ctrl.AddRoute(http.MethodGet, apiPath+"/:name", permissions.Merge(fsPermissions.GetFile, fsPermissions.Conditions), GetFileOrFolder(filePath, func(r *http.Request) string { return r.URL.Query().Get("name") }))
	} else {
		ctrl.AddRoute(http.MethodGet, apiPath+"/:name", permissions.Merge(fsPermissions.GetFile, fsPermissions.Conditions), GetFile(filePath, func(r *http.Request) string { return r.URL.Query().Get("name") }))
	}
	ctrl.AddRoute(http.MethodDelete, apiPath+"/:name", permissions.Merge(fsPermissions.Delete, fsPermissions.Conditions), DeleteFile(filePath, func(r *http.Request) string { return r.URL.Query().Get("name") }))

	if options.SubFolder {
		filePathFolder := func(r *http.Request) string {
			return path.Join(filePath(r), r.URL.Query().Get("name"))
		}
		ctrl.AddRoute(http.MethodPost, apiPath+"/:name", permissions.Merge(fsPermissions.Post, fsPermissions.Conditions), PostFile(filePathFolder))
		ctrl.AddRoute(http.MethodGet, apiPath+"/:name/:fileName", permissions.Merge(fsPermissions.GetFile, fsPermissions.Conditions), GetFile(filePathFolder, func(r *http.Request) string { return r.URL.Query().Get("fileName") }))
		ctrl.AddRoute(http.MethodDelete, apiPath+"/:name/:fileName", permissions.Merge(fsPermissions.Delete, fsPermissions.Conditions), DeleteFile(filePathFolder, func(r *http.Request) string { return r.URL.Query().Get("fileName") }))
	}
}

func DefaultFileSystemPermissions(ctrl GetModeler) FileSystemPermissions {
	mdl := ctrl.GetModel()
	fsPermissions := FileSystemPermissions{
		Get:     permissions.Get(mdl),
		Post:    permissions.Post(mdl),
		GetFile: permissions.Get(mdl),
		Delete:  permissions.Delete(mdl),
	}
	if condMdl, ok := mdl.(model.ConditionsModel); ok {
		fsPermissions.Conditions = func(r *http.Request) message.Message {
			db := ctx.DB(r)
			primaries := map[string]interface{}{}
			msg := GetPathParamsMsg(r, ctrl.NewModel(), utils.GetPrimaryFields(ctrl.GetModelType()), &primaries)
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
				return message.ItemNotFound(r)
			}
			return nil
		}
	}
	return fsPermissions
}
