package controller

import (
	"api_core/app"
	"api_core/message"
	"api_core/permissions"
	"api_core/query"
	"api_core/request"
	"api_core/utils"
	"maps"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func ModelGetHandler(mdl any) gin.HandlerFunc {
	return func(c *gin.Context) {
		err := HandleGet(c, request.DB(c), map[string]interface{}{}, mdl)
		if AbortIfError(c, err) {
			return
		}
	}
}

func ModelGetOneHandler(mdl any) gin.HandlerFunc {
	return func(c *gin.Context) {
		primaries := map[string]interface{}{}
		err := GetPathParams(c, mdl, utils.GetPrimaryFields(reflect.TypeOf(mdl).Elem()), &primaries)
		if AbortIfError(c, err) {
			return
		}
		err = HandleGet(c, request.DB(c), primaries, mdl)
		if AbortIfError(c, err) {
			return
		}
	}
}

func ModelGetStructureHandler(mdl any) gin.HandlerFunc {
	return func(c *gin.Context) {
		relations := query.GetRelations(c)
		splittedRelations := [][]string{}
		for _, rel := range relations {
			splittedRelations = append(splittedRelations, strings.Split(rel, "."))
		}

		modelSchema, err := schema.Parse(mdl, &sync.Map{}, app.DB.NamingStrategy)
		if err != nil {
			message.InternalServerError(c).Write(c)
			return
		}

		c.JSON(http.StatusOK, GetStructInfo(c, modelSchema, splittedRelations))
	}
}

func ModelGetRelStructureHandler(mdl any) gin.HandlerFunc {
	return func(c *gin.Context) {
		modelSchema, err := schema.Parse(mdl, &sync.Map{}, app.DB.NamingStrategy)
		if err != nil {
			message.InternalServerError(c).Write(c)
			return
		}

		pieces := strings.Split(c.Param("rel"), ".")
		relSchema := modelSchema
		for i, piece := range pieces {
			if rel, ok := relSchema.Relationships.Relations[piece]; ok {
				relSchema = rel.FieldSchema
				if msg := permissions.Get(reflect.New(relSchema.ModelType).Interface())(c); msg != nil {
					message.UnauthorizedRelations(c, strings.Join(pieces[:i+1], ".")).Add(msg).Write(c)
					return
				}
			} else {
				message.InvalidRelations(c, strings.Join(pieces[:i+1], ".")).Write(c)
				return
			}
		}

		c.JSON(http.StatusOK, GetStructInfo(c, relSchema, [][]string{}))
	}
}

func ModelPostHandler(mdl any) gin.HandlerFunc {
	return func(c *gin.Context) {
		jsonData, err := c.GetRawData()

		if err != nil || len(jsonData) == 0 {
			message.InvalidJSON(c).Write(c)
			return
		}

		if jsonData[0] == '[' {
			mdlSlice := reflect.New(reflect.SliceOf(reflect.TypeOf(mdl))).Interface()
			msg := LoadModel(c, jsonData, mdlSlice)
			if AbortIfError(c, msg) {
				return
			}
			msg = ValidateModels(c, mdlSlice)
			if AbortIfError(c, msg) {
				return
			}
			msg = CreateToDb(c, request.DB(c), mdlSlice)
			if AbortIfError(c, msg) {
				return
			}
		} else {
			msg := LoadModel(c, jsonData, mdl)
			if AbortIfError(c, msg) {
				return
			}
			msg = ValidateModel(c, mdl)
			if AbortIfError(c, msg) {
				return
			}
			msg = CreateToDb(c, request.DB(c), mdl)
			if AbortIfError(c, msg) {
				return
			}
		}
	}
}

func ModelPatchOneHandler(mdl any) gin.HandlerFunc {
	return func(c *gin.Context) {
		jsonMap := make(map[string]interface{})
		jsonData, _ := c.GetRawData()
		modelType := reflect.TypeOf(mdl).Elem()
		primaryFields := utils.GetPrimaryFields(modelType)

		err := LoadModel(c, jsonData, mdl)
		if AbortIfError(c, err) {
			return
		}
		err = GetPathParams(c, mdl, primaryFields, mdl)
		if AbortIfError(c, err) {
			return
		}
		err = LoadAndValidateMap(c, jsonData, jsonMap, modelType)
		if AbortIfError(c, err) {
			return
		}
		err = GetPathParams(c, mdl, primaryFields, &jsonMap)
		if AbortIfError(c, err) {
			return
		}
		err = UpdateToDb(c, mdl, jsonMap)
		if AbortIfError(c, err) {
			return
		}
	}
}

func ModelPatchHandler(mdl any) gin.HandlerFunc {
	return func(c *gin.Context) {
		mdlSlice := reflect.New(reflect.SliceOf(reflect.TypeOf(mdl)))
		jsonMaps := []map[string]interface{}{}
		jsonData, _ := c.GetRawData()
		modelType := reflect.TypeOf(mdl).Elem()

		msg := LoadModel(c, jsonData, mdlSlice)
		if AbortIfError(c, msg) {
			return
		}
		msg = LoadAndValidateMaps(c, jsonData, &jsonMaps, modelType)
		if AbortIfError(c, msg) {
			return
		}
		msg = ValidateMapsPrimaries(c, jsonMaps, utils.GetPrimaryFields(modelType))
		if AbortIfError(c, msg) {
			return
		}
		if len(jsonMaps) > 0 {
			db := request.DB(c).Session(&gorm.Session{CreateBatchSize: 50})

			modelSliceVal := reflect.ValueOf(mdlSlice).Elem()

			modelSchema, err := schema.Parse(modelSliceVal.Index(0), &sync.Map{}, db.NamingStrategy)
			if err != nil {
				message.InternalServerError(c).Write(c)
				return
			}

			checked := map[string]struct{}{}
			for i := range jsonMaps {
				err := permissions.CheckModel(c, modelSliceVal.Index(i), modelSchema, checked, true)
				if AbortIfError(c, err) {
					return
				}
			}

			db.Session(&gorm.Session{FullSaveAssociations: true}).Transaction(func(tx *gorm.DB) error {
				for i, values := range jsonMaps {
					modelVal := modelSliceVal.Index(i).Addr()
					e := DeleteRelations(c, tx, modelVal, modelSchema)
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

		c.JSON(http.StatusOK, mdlSlice)
	}
}

func pathParamsToModels(c *gin.Context, modelType reflect.Type, fields []string, destination *[]interface{}) error {
	var values = make([][]string, len(fields))
	for i, field := range fields {
		_, found := modelType.FieldByName(field)
		val := c.Param(field)
		values[i] = strings.Split(val, ",")
		if len(val) == 0 || !found || (i > 0 && len(values[i]) != len(values[i-1])) {
			return message.InvalidUrlParameter(c, field)
		}
	}

	for i := 0; i < len(values[0]); i++ {
		item := reflect.New(modelType).Elem()
		for j, field := range fields {
			msg := assignValue(c, item.FieldByName(field), field, values[j][i], item)
			if msg != nil {
				return msg
			}
		}
		*destination = append(*destination, item.Addr().Interface())
	}
	return nil
}

func ModelDeleteHandler(mdl any) gin.HandlerFunc {
	return func(c *gin.Context) {
		typ := reflect.TypeOf(mdl).Elem()
		primaryFields := utils.GetPrimaryFields(typ)
		models := []interface{}{}
		err := pathParamsToModels(c, typ, primaryFields, &models)
		if AbortIfError(c, err) {
			return
		}
		err = DeleteFromDb(c, models)
		if AbortIfError(c, err) {
			return
		}
	}
}

func GetHandler(controller any, model any) gin.HandlerFunc {
	if h, ok := controller.(Getter); ok {
		return h.Get
	}
	return ModelGetHandler(model)
}

func GetOneHandler(controller any, model any) gin.HandlerFunc {
	if h, ok := controller.(GetOner); ok {
		return h.GetOne
	}
	return ModelGetOneHandler(model)
}

func GetStructureHandler(controller any, model any) gin.HandlerFunc {
	if h, ok := controller.(GetStructurer); ok {
		return h.GetStructure
	}
	return ModelGetStructureHandler(model)
}

func GetRelStructureHandler(controller any, model any) gin.HandlerFunc {
	if h, ok := controller.(GetRelStructurer); ok {
		return h.GetRelStructure
	}
	return ModelGetRelStructureHandler(model)
}

func PostHandler(controller any, model any) gin.HandlerFunc {
	if h, ok := controller.(Poster); ok {
		return h.Post
	}
	return ModelPostHandler(model)
}

func PatchHandler(controller any, model any) gin.HandlerFunc {
	if h, ok := controller.(PatchHandlerer); ok {
		return h.Patch
	}
	return ModelPatchHandler(model)
}

func PatchOneHandler(controller any, model any) gin.HandlerFunc {
	if h, ok := controller.(PatchOner); ok {
		return h.PatchOne
	}
	return ModelPatchOneHandler(model)
}

func DeleteHandler(controller any, model any) gin.HandlerFunc {
	if h, ok := controller.(DeleteHandlerer); ok {
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
		params += ":" + field
	}
	return params
}

func FullPath(controller any) string {
	if pather, ok := controller.(Pather); ok {
		return pather.Path() + Endpoint(controller)
	}
	return Endpoint(controller)
}

func Endpoint(controller any) string {
	if e, ok := controller.(Endpointer); ok {
		return "/" + e.Endpoint()
	}
	return "/" + utils.FirstLower(Name(controller))
}

func Routes(controller any) []Route {
	routeMap := map[string]Route{}

	addToMap := func(routes ...Route) {
		for _, r := range routes {
			key := r.Method + " " + r.Pattern
			routeMap[key] = r
		}
	}

	var model any
	if modeler, ok := controller.(Modeler); ok {
		model = modeler.Model()
	}

	if renderer, ok := controller.(Renderer); ok {
		var handlerGet gin.HandlerFunc
		if h, ok := controller.(Getter); ok {
			handlerGet = h.Get
		} else {
			handlerGet = renderer.Render
		}
		var permissionsGet permissions.HandlerFunc
		if model != nil {
			if m, ok := model.(permissions.ModelWithPermissionsGet); ok {
				permissionsGet = m.PermissionsGet
			}
		}
		addToMap(
			Route{
				Method:      http.MethodGet,
				Pattern:     "",
				Permissions: permissionsGet,
				Handler:     handlerGet,
			},
		)
		var handlerPost gin.HandlerFunc
		if h, ok := controller.(Poster); ok {
			handlerPost = h.Post
		} else {
			handlerPost = renderer.Render
		}
		var permissionsPost permissions.HandlerFunc
		if model != nil {
			if m, ok := model.(permissions.ModelWithPermissionsPost); ok {
				permissionsPost = m.PermissionsPost
			}
		}
		addToMap(
			Route{
				Method:      http.MethodPost,
				Pattern:     "",
				Permissions: permissionsPost,
				Handler:     handlerPost,
			},
		)
	} else if model != nil {
		urlPrimaryFields := PrimaryFieldsToURL(utils.GetPrimaryFields(reflect.TypeOf(model).Elem()))

		if m, ok := model.(permissions.ModelWithPermissionsGet); ok {
			addToMap(
				Route{
					Method:      http.MethodGet,
					Pattern:     "",
					Permissions: m.PermissionsGet,
					Handler:     GetHandler(controller, m),
				},
			)
			if len(urlPrimaryFields) > 0 {
				addToMap(
					Route{
						Method:      http.MethodGet,
						Pattern:     urlPrimaryFields,
						Permissions: m.PermissionsGet,
						Handler:     GetOneHandler(controller, m),
					},
				)
			}
			addToMap(
				Route{
					Method:      http.MethodGet,
					Pattern:     "structure",
					Permissions: m.PermissionsGet,
					Handler:     GetStructureHandler(controller, m),
				},
				Route{
					Method:      http.MethodGet,
					Pattern:     "structure/:rel",
					Permissions: m.PermissionsGet,
					Handler:     GetRelStructureHandler(controller, m),
				},
			)
		}

		if m, ok := model.(permissions.ModelWithPermissionsPost); ok {
			addToMap(
				Route{
					Method:      http.MethodPost,
					Pattern:     "",
					Permissions: m.PermissionsPost,
					Handler:     PostHandler(controller, m),
				},
			)
		}

		if m, ok := model.(permissions.ModelWithPermissionsPatch); ok {
			addToMap(
				Route{
					Method:      http.MethodPatch,
					Pattern:     "",
					Permissions: m.PermissionsPatch,
					Handler:     PatchHandler(controller, m),
				},
				Route{
					Method:      http.MethodPatch,
					Pattern:     urlPrimaryFields,
					Permissions: m.PermissionsPatch,
					Handler:     PatchOneHandler(controller, m),
				},
			)
		}

		if m, ok := model.(permissions.ModelWithPermissionsDelete); ok {
			addToMap(
				Route{
					Method:      http.MethodDelete,
					Pattern:     urlPrimaryFields,
					Permissions: m.PermissionsDelete,
					Handler:     DeleteHandler(controller, m),
				},
			)
		}
	}

	if router, ok := controller.(Router); ok {
		addToMap(router.Routes()...)
	}

	return slices.Collect(maps.Values(routeMap))
}

func Group(route Route, controller any) string {
	if grouper, ok := controller.(Grouper); ok {
		return grouper.Group()
	}
	return ""
}

func PermissionsMiddleware(permissionFuncs ...permissions.HandlerFunc) func(*gin.Context) {
	return func(c *gin.Context) {
		err := permissions.Validate(c, permissionFuncs...)
		if AbortIfError(c, err) {
			return
		}
	}
}
