package controller

import (
	"api_core/app"
	"api_core/interfaces"
	"api_core/message"
	"api_core/permissions"
	"api_core/query"
	"api_core/registry"
	"api_core/request"
	"api_core/response"
	"api_core/route"
	"api_core/utils"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync"

	"github.com/go-chi/chi"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func ModelGetHandler(mdl any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := HandleGet(w, r, request.DB(r), map[string]interface{}{}, mdl)
		if request.AbortIfError(w, r, err) {
			return
		}
	}
}

func ModelGetOneHandler(mdl any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		primaries := map[string]interface{}{}
		err := GetPathParams(r, mdl, utils.GetPrimaryFields(reflect.TypeOf(mdl)), &primaries)
		if request.AbortIfError(w, r, err) {
			return
		}
		err = HandleGet(w, r, request.DB(r), primaries, mdl)
		if request.AbortIfError(w, r, err) {
			return
		}
	}
}

func ModelGetStructureHandler(mdl any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		relations := query.GetRelations(r)
		splittedRelations := [][]string{}
		for _, rel := range relations {
			splittedRelations = append(splittedRelations, strings.Split(rel, "."))
		}

		modelSchema, err := schema.Parse(mdl, &sync.Map{}, app.DB.NamingStrategy)
		if err != nil {
			message.InternalServerError(r).Write(w, r)
			return
		}

		response.JSON(w, r, GetStructInfo(r, modelSchema, splittedRelations))
	}
}

func ModelGetRelStructureHandler(mdl any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		modelSchema, err := schema.Parse(mdl, &sync.Map{}, app.DB.NamingStrategy)
		if err != nil {
			message.InternalServerError(r).Write(w, r)
			return
		}

		pieces := strings.Split(r.URL.Query().Get("rel"), ".")
		relSchema := modelSchema
		for i, piece := range pieces {
			if rel, ok := relSchema.Relationships.Relations[piece]; ok {
				relSchema = rel.FieldSchema
				if msg := permissions.Get(reflect.New(relSchema.ModelType).Interface())(r); msg != nil {
					message.UnauthorizedRelations(r, strings.Join(pieces[:i+1], ".")).Add(msg).Write(w, r)
					return
				}
			} else {
				message.InvalidRelations(r, strings.Join(pieces[:i+1], ".")).Write(w, r)
				return
			}
		}

		response.JSON(w, r, GetStructInfo(r, relSchema, [][]string{}))
	}
}

func ModelPostHandler(mdl any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jsonData, err := io.ReadAll(r.Body)

		if err != nil || len(jsonData) == 0 {
			message.InvalidJSON(r).Write(w, r)
			return
		}

		if jsonData[0] == '[' {
			mdlSlice := reflect.New(reflect.SliceOf(reflect.TypeOf(mdl)))
			msg := LoadModel(r, jsonData, mdlSlice)
			if request.AbortIfError(w, r, msg) {
				return
			}
			msg = ValidateModels(r, mdlSlice)
			if request.AbortIfError(w, r, msg) {
				return
			}
			msg = CreateToDb(w, r, request.DB(r), mdlSlice)
			if request.AbortIfError(w, r, msg) {
				return
			}
		} else {
			msg := LoadModel(r, jsonData, mdl)
			if request.AbortIfError(w, r, msg) {
				return
			}
			msg = ValidateModel(r, mdl)
			if request.AbortIfError(w, r, msg) {
				return
			}
			msg = CreateToDb(w, r, request.DB(r), mdl)
			if request.AbortIfError(w, r, msg) {
				return
			}
		}
	}
}

func ModelPatchOneHandler(mdl any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jsonMap := make(map[string]interface{})
		jsonData, _ := io.ReadAll(r.Body)
		modelType := reflect.TypeOf(mdl)
		primaryFields := utils.GetPrimaryFields(modelType)

		err := LoadModel(r, jsonData, mdl)
		if request.AbortIfError(w, r, err) {
			return
		}
		err = GetPathParams(r, mdl, primaryFields, mdl)
		if request.AbortIfError(w, r, err) {
			return
		}
		err = LoadAndValidateMap(r, jsonData, jsonMap, modelType)
		if request.AbortIfError(w, r, err) {
			return
		}
		err = GetPathParams(r, mdl, primaryFields, &jsonMap)
		if request.AbortIfError(w, r, err) {
			return
		}
		err = UpdateToDb(w, r, mdl, jsonMap)
		if request.AbortIfError(w, r, err) {
			return
		}
	}
}

func ModelPatchHandler(mdl any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mdlSlice := reflect.New(reflect.SliceOf(reflect.TypeOf(mdl)))
		jsonMaps := []map[string]interface{}{}
		jsonData, _ := io.ReadAll(r.Body)
		modelType := reflect.TypeOf(mdl)

		msg := LoadModel(r, jsonData, mdlSlice)
		if request.AbortIfError(w, r, msg) {
			return
		}
		msg = LoadAndValidateMaps(r, jsonData, &jsonMaps, modelType)
		if request.AbortIfError(w, r, msg) {
			return
		}
		msg = ValidateMapsPrimaries(r, jsonMaps, utils.GetPrimaryFields(modelType))
		if request.AbortIfError(w, r, msg) {
			return
		}
		if len(jsonMaps) > 0 {
			db := request.DB(r).Session(&gorm.Session{CreateBatchSize: 50})

			modelSliceVal := reflect.ValueOf(mdlSlice).Elem()

			modelSchema, err := schema.Parse(modelSliceVal.Index(0), &sync.Map{}, db.NamingStrategy)
			if err != nil {
				message.InternalServerError(r).Write(w, r)
				return
			}

			checked := map[string]struct{}{}
			for i := range jsonMaps {
				err := permissions.CheckModel(r, modelSliceVal.Index(i), modelSchema, checked, true)
				if request.AbortIfError(w, r, err) {
					return
				}
			}

			db.Session(&gorm.Session{FullSaveAssociations: true}).Transaction(func(tx *gorm.DB) error {
				for i, values := range jsonMaps {
					modelVal := modelSliceVal.Index(i).Addr()
					e := DeleteRelations(r, tx, modelVal, modelSchema)
					if e != nil {
						return e
					}
					if tx.Error != nil {
						return tx.Error
					}
					res := tx.Model(modelVal.Interface()).Updates(values)
					if res.Error != nil {
						return res.Error
					}
				}

				return nil
			})
		}

		response.JSON(w, r, mdlSlice)
	}
}

func pathParamsToModels(r *http.Request, modelType reflect.Type, fields []string, destination *[]interface{}) error {
	var values = make([][]string, len(fields))
	for i, field := range fields {
		_, found := modelType.FieldByName(field)
		val := chi.URLParam(r, field)
		values[i] = strings.Split(val, ",")
		if len(val) == 0 || !found || (i > 0 && len(values[i]) != len(values[i-1])) {
			return message.InvalidUrlParameter(r, field)
		}
	}

	for i := 0; i < len(values[0]); i++ {
		item := reflect.New(modelType).Elem()
		for j, field := range fields {
			msg := assignValue(r, item.FieldByName(field), field, values[j][i], item)
			if msg != nil {
				return msg
			}
		}
		*destination = append(*destination, item.Addr().Interface())
	}
	return nil
}

func ModelDeleteHandler(mdl any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		primaryFields := utils.GetPrimaryFields(reflect.TypeOf(mdl))
		models := []interface{}{}
		err := pathParamsToModels(r, reflect.TypeOf(mdl), primaryFields, &models)
		if request.AbortIfError(w, r, err) {
			return
		}
		err = DeleteFromDb(r, models)
		if request.AbortIfError(w, r, err) {
			return
		}
	}
}

func GetHandler(controller any, model any) http.HandlerFunc {
	if h, ok := controller.(interfaces.GetHandler); ok {
		return h.Get
	}
	return ModelGetHandler(model)
}

func GetOneHandler(controller any, model any) http.HandlerFunc {
	if h, ok := controller.(interfaces.GetOneHandler); ok {
		return h.GetOne
	}
	return ModelGetOneHandler(model)
}

func GetStructureHandler(controller any, model any) http.HandlerFunc {
	if h, ok := controller.(interfaces.GetStructureHandler); ok {
		return h.GetStructure
	}
	return ModelGetStructureHandler(model)
}

func GetRelStructureHandler(controller any, model any) http.HandlerFunc {
	if h, ok := controller.(interfaces.GetRelStructureHandler); ok {
		return h.GetRelStructure
	}
	return ModelGetRelStructureHandler(model)
}

func PostHandler(controller any, model any) http.HandlerFunc {
	if h, ok := controller.(interfaces.PostHandler); ok {
		return h.Post
	}
	return ModelPostHandler(model)
}

func PatchHandler(controller any, model any) http.HandlerFunc {
	if h, ok := controller.(interfaces.PatchHandler); ok {
		return h.Patch
	}
	return ModelPatchHandler(model)
}

func PatchOneHandler(controller any, model any) http.HandlerFunc {
	if h, ok := controller.(interfaces.PatchOneHandler); ok {
		return h.PatchOne
	}
	return ModelPatchOneHandler(model)
}

func DeleteHandler(controller any, model any) http.HandlerFunc {
	if h, ok := controller.(interfaces.DeleteHandler); ok {
		return h.Delete
	}
	return ModelDeleteHandler(model)
}

func PrimaryFieldsToURL(primaryFields []string) string {
	params := ""
	for i, field := range primaryFields {
		if i > 0 {
			params += "/"
		}
		params += "{" + field + "}"
	}
	return params
}

func Endpoint(controller any) string {
	if e, ok := controller.(interfaces.Endpointer); ok {
		return "/" + e.Endpoint()
	}
	return "/" + utils.FirstLower(registry.Name(controller))
}

func Routes(controller any) []route.Route {
	if router, ok := controller.(interfaces.Router); ok {
		return router.Routes(controller)
	} else if modeler, ok := controller.(interfaces.Modeler); ok {
		routes := []route.Route{}
		model := modeler.Model()
		urlPrimaryFields := PrimaryFieldsToURL(utils.GetPrimaryFields(reflect.TypeOf(model)))

		if m, ok := model.(permissions.ModelWithPermissionsGet); ok {
			permissions := m.PermissionsGet
			routes = append(routes, route.Route{
				Method:      http.MethodGet,
				Pattern:     "/",
				Permissions: permissions,
				Handler:     GetHandler(controller, m),
			})
			if len(urlPrimaryFields) > 0 {
				routes = append(routes, route.Route{
					Method:      http.MethodGet,
					Pattern:     "/" + urlPrimaryFields,
					Permissions: permissions,
					Handler:     GetOneHandler(controller, m),
				})
			}
			routes = append(routes, route.Route{
				Method:      http.MethodGet,
				Pattern:     "/structure",
				Permissions: permissions,
				Handler:     GetStructureHandler(controller, m),
			}, route.Route{
				Method:      http.MethodGet,
				Pattern:     "/structure/{rel}",
				Permissions: permissions,
				Handler:     GetRelStructureHandler(controller, m),
			})
		}

		if m, ok := model.(permissions.ModelWithPermissionsPost); ok {
			permissions := m.PermissionsPost
			routes = append(routes, route.Route{
				Method:      http.MethodPost,
				Pattern:     "/",
				Permissions: permissions,
				Handler:     PostHandler(controller, m),
			})
		}

		if m, ok := model.(permissions.ModelWithPermissionsPatch); ok {
			permissions := m.PermissionsPatch
			routes = append(routes, route.Route{
				Method:      http.MethodPatch,
				Pattern:     "/",
				Permissions: permissions,
				Handler:     PatchHandler(controller, m),
			})
			routes = append(routes, route.Route{
				Method:      http.MethodPatch,
				Pattern:     "/" + urlPrimaryFields,
				Permissions: permissions,
				Handler:     PatchOneHandler(controller, m),
			})
		}

		if m, ok := model.(permissions.ModelWithPermissionsDelete); ok {
			permissions := m.PermissionsDelete
			routes = append(routes, route.Route{
				Method:      http.MethodDelete,
				Pattern:     "/" + urlPrimaryFields,
				Permissions: permissions,
				Handler:     DeleteHandler(controller, m),
			})
		}
		return routes
	}
	return []route.Route{}
}
