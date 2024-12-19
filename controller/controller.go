package controller

import (
	"api_core/app"
	"api_core/message"
	"api_core/permissions"
	"api_core/query"
	"api_core/registry"
	"api_core/request"
	"api_core/utils"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync"

	"github.com/go-chi/render"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// # Controller #

type Controller struct {
	basePath string
}

func (c Controller) Endpoint(controller any) string {
	t := reflect.TypeOf(controller)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return "/" + utils.FirstLower(t.Name())
}

func (c *Controller) SetBasePath(basePath string) *Controller {
	c.basePath = basePath
	return c
}

func (c Controller) BasePath() string {
	return c.basePath
}

func (c Controller) FullPath(controller any) string {
	return c.BasePath() + "/" + c.Endpoint(controller)
}

func (c Controller) Routes() []registry.Route {
	return []registry.Route{}
}

// # Typed controller #

type TypedController[T any] struct {
	Controller
}

func (c TypedController[T]) Endpoint(controller any) string {
	return strings.Split(c.Controller.Endpoint(controller), "[")[0]
}

func (c TypedController[T]) FullPath(controller any) string {
	return c.BasePath() + "/" + c.Endpoint(controller)
}

func (c TypedController[T]) Model() *T {
	return new(T)
}

func (c TypedController[T]) ModelType() reflect.Type {
	var zero [0]T
	return reflect.TypeOf(zero).Elem()
}

func (c TypedController[T]) ModelSlice() []T {
	return make([]T, 0)
}

func (c TypedController[T]) Get(w http.ResponseWriter, r *http.Request) {
	HandleGet(w, r, request.DB(r), map[string]interface{}{}, c.Model())
}

func (c TypedController[T]) GetOne(w http.ResponseWriter, r *http.Request) {
	primaries := map[string]interface{}{}
	err := GetPathParams(r, c.Model(), utils.GetPrimaryFields(c.ModelType()), &primaries)
	if request.AbortIfError(w, r, err) {
		return
	}
	err = HandleGet(w, r, request.DB(r), primaries, c.Model())
	if request.AbortIfError(w, r, err) {
		return
	}
}

func (c TypedController[T]) GetStructure(w http.ResponseWriter, r *http.Request) {
	relations := query.GetRelations(r)
	splittedRelations := [][]string{}
	for _, rel := range relations {
		splittedRelations = append(splittedRelations, strings.Split(rel, "."))
	}

	modelSchema, err := schema.Parse(c.Model(), &sync.Map{}, app.DB.NamingStrategy)
	if err != nil {
		message.InternalServerError(r).Write(w, r)
		return
	}

	render.JSON(w, r, GetStructInfo(r, modelSchema, splittedRelations))
}

func (c TypedController[T]) GetRelStructure(w http.ResponseWriter, r *http.Request) {
	modelSchema, err := schema.Parse(c.Model(), &sync.Map{}, app.DB.NamingStrategy)
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

	render.JSON(w, r, GetStructInfo(r, relSchema, [][]string{}))
}

func (c TypedController[T]) Post(w http.ResponseWriter, r *http.Request) {
	jsonData, err := io.ReadAll(r.Body)

	if err != nil || len(jsonData) == 0 {
		message.InvalidJSON(r).Write(w, r)
		return
	}

	if jsonData[0] == '[' {
		model := c.ModelSlice()
		msg := LoadModel(r, jsonData, model)
		if request.AbortIfError(w, r, msg) {
			return
		}
		msg = ValidateModels(r, model)
		if request.AbortIfError(w, r, msg) {
			return
		}
		msg = CreateToDb(w, r, request.DB(r), model)
		if request.AbortIfError(w, r, msg) {
			return
		}
	} else {
		model := c.Model()
		msg := LoadModel(r, jsonData, model)
		if request.AbortIfError(w, r, msg) {
			return
		}
		msg = ValidateModel(r, model)
		if request.AbortIfError(w, r, msg) {
			return
		}
		msg = CreateToDb(w, r, request.DB(r), model)
		if request.AbortIfError(w, r, msg) {
			return
		}
	}
}

func (c TypedController[T]) PatchOne(w http.ResponseWriter, r *http.Request) {
	model := c.Model()
	jsonMap := make(map[string]interface{})
	jsonData, _ := io.ReadAll(r.Body)
	modelType := c.ModelType()
	primaryFields := utils.GetPrimaryFields(modelType)

	err := LoadModel(r, jsonData, model)
	if request.AbortIfError(w, r, err) {
		return
	}
	err = GetPathParams(r, model, primaryFields, model)
	if request.AbortIfError(w, r, err) {
		return
	}
	err = LoadAndValidateMap(r, jsonData, jsonMap, modelType)
	if request.AbortIfError(w, r, err) {
		return
	}
	err = GetPathParams(r, model, primaryFields, &jsonMap)
	if request.AbortIfError(w, r, err) {
		return
	}
	err = UpdateToDb(w, r, model, jsonMap)
	if request.AbortIfError(w, r, err) {
		return
	}
}

func (c TypedController[T]) Patch(w http.ResponseWriter, r *http.Request) {
	modelSlice := c.ModelSlice()
	jsonMaps := []map[string]interface{}{}
	jsonData, _ := io.ReadAll(r.Body)
	modelType := c.ModelType()

	msg := LoadModel(r, jsonData, modelSlice)
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

		modelSliceVal := reflect.ValueOf(modelSlice).Elem()

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

	render.JSON(w, r, modelSlice)
}

func (c TypedController[T]) Delete(w http.ResponseWriter, r *http.Request) {
	primaryFields := utils.GetPrimaryFields(c.ModelType())
	models := []interface{}{}
	err := pathParamsToModels(r, c.ModelType(), primaryFields, &models)
	if request.AbortIfError(w, r, err) {
		return
	}
	err = DeleteFromDb(r, models)
	if request.AbortIfError(w, r, err) {
		return
	}
}

func pathParamsToModels(r *http.Request, modelType reflect.Type, fields []string, destination *[]interface{}) error {
	var values = make([][]string, len(fields))
	for i, field := range fields {
		_, found := modelType.FieldByName(field)
		val := r.URL.Query().Get(field)
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

func (c TypedController[T]) Routes() []registry.Route {
	routes := []registry.Route{}
	var mdl any = *c.Model()
	modelType := c.ModelType()
	if modelType != nil {
		primaryFields := utils.GetPrimaryFields(modelType)
		params := ""
		for i, field := range primaryFields {
			if i > 0 {
				params += "/"
			}
			params += "{" + field + "}"
		}

		if m, ok := mdl.(permissions.ModelWithPermissionsGet); ok {
			permissions := []permissions.HandlerFunc{m.PermissionsGet}
			routes = append(routes, registry.Route{
				Method:      http.MethodGet,
				Pattern:     "/",
				Permissions: permissions,
				Handler:     c.Get,
			})
			if len(primaryFields) > 0 {
				routes = append(routes, registry.Route{
					Method:      http.MethodGet,
					Pattern:     "/" + params,
					Permissions: permissions,
					Handler:     c.GetOne,
				})
			}
			routes = append(routes, registry.Route{
				Method:      http.MethodGet,
				Pattern:     "/structure",
				Permissions: permissions,
				Handler:     c.GetStructure,
			}, registry.Route{
				Method:      http.MethodGet,
				Pattern:     "/structure/{rel}",
				Permissions: permissions,
				Handler:     c.GetRelStructure,
			})
		}
		if m, ok := mdl.(permissions.ModelWithPermissionsPost); ok {
			permissions := []permissions.HandlerFunc{m.PermissionsPost}
			routes = append(routes, registry.Route{
				Method:      http.MethodPost,
				Pattern:     "/",
				Permissions: permissions,
				Handler:     c.Post,
			})
		}
		if m, ok := mdl.(permissions.ModelWithPermissionsPatch); ok {
			permissions := []permissions.HandlerFunc{m.PermissionsPatch}
			routes = append(routes, registry.Route{
				Method:      http.MethodPost,
				Pattern:     "/",
				Permissions: permissions,
				Handler:     c.Patch,
			})
			routes = append(routes, registry.Route{
				Method:      http.MethodPost,
				Pattern:     "/" + params,
				Permissions: permissions,
				Handler:     c.PatchOne,
			})
		}
		if m, ok := mdl.(permissions.ModelWithPermissionsDelete); ok {
			permissions := []permissions.HandlerFunc{m.PermissionsDelete}
			routes = append(routes, registry.Route{
				Method:      http.MethodPost,
				Pattern:     "/" + params,
				Permissions: permissions,
				Handler:     c.Delete,
			})
		}
	}
	return routes
}
