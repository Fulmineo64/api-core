package controller

import (
	"api_core/app"
	"api_core/ctx"
	"api_core/message"
	"api_core/permissions"
	"api_core/query"
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

type GenericController[T any] struct {
}

func (c GenericController[T]) Model() *T {
	return new(T)
}

func (c GenericController[T]) ModelType() reflect.Type {
	var zero [0]T
	return reflect.TypeOf(zero).Elem()
}

func (c GenericController[T]) ModelSlice() []T {
	return make([]T, 0)
}

func (c GenericController[T]) Get(w http.ResponseWriter, r *http.Request) {
	HandleGet(w, r, ctx.DB(r), map[string]interface{}{}, c.Model())
}

func (c GenericController[T]) GetOne(w http.ResponseWriter, r *http.Request) {
	primaries := map[string]interface{}{}
	err := GetPathParams(r, c.Model(), utils.GetPrimaryFields(c.ModelType()), &primaries)
	if AbortIfError(w, r, err) {
		return
	}
	err = HandleGet(w, r, ctx.DB(r), primaries, c.Model())
	if AbortIfError(w, r, err) {
		return
	}
}

func (c GenericController[T]) GetStructure(w http.ResponseWriter, r *http.Request) {
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

func (c GenericController[T]) GetRelStructure(w http.ResponseWriter, r *http.Request) {
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

func (c GenericController[T]) Post(w http.ResponseWriter, r *http.Request) {
	jsonData, err := io.ReadAll(r.Body)

	if err != nil || len(jsonData) == 0 {
		message.InvalidJSON(r).Write(w, r)
		return
	}

	if jsonData[0] == '[' {
		model := c.ModelSlice()
		msg := LoadModel(r, jsonData, model)
		if AbortIfError(w, r, msg) {
			return
		}
		msg = ValidateModels(r, model)
		if AbortIfError(w, r, msg) {
			return
		}
		msg = CreateToDb(w, r, ctx.DB(r), model)
		if AbortIfError(w, r, msg) {
			return
		}
	} else {
		model := c.Model()
		msg := LoadModel(r, jsonData, model)
		if AbortIfError(w, r, msg) {
			return
		}
		msg = ValidateModel(r, model)
		if AbortIfError(w, r, msg) {
			return
		}
		msg = CreateToDb(w, r, ctx.DB(r), model)
		if AbortIfError(w, r, msg) {
			return
		}
	}
}

func (c GenericController[T]) PatchOne(w http.ResponseWriter, r *http.Request) {
	model := c.Model()
	jsonMap := make(map[string]interface{})
	jsonData, _ := io.ReadAll(r.Body)
	modelType := c.ModelType()
	primaryFields := utils.GetPrimaryFields(modelType)

	err := LoadModel(r, jsonData, model)
	if AbortIfError(w, r, err) {
		return
	}
	err = GetPathParams(r, model, primaryFields, model)
	if AbortIfError(w, r, err) {
		return
	}
	err = LoadAndValidateMap(r, jsonData, jsonMap, modelType)
	if AbortIfError(w, r, err) {
		return
	}
	err = GetPathParams(r, model, primaryFields, &jsonMap)
	if AbortIfError(w, r, err) {
		return
	}
	err = UpdateToDb(w, r, model, jsonMap)
	if AbortIfError(w, r, err) {
		return
	}
}

func (c GenericController[T]) Patch(w http.ResponseWriter, r *http.Request) {
	modelSlice := c.ModelSlice()
	jsonMaps := []map[string]interface{}{}
	jsonData, _ := io.ReadAll(r.Body)
	modelType := c.ModelType()

	msg := LoadModel(r, jsonData, modelSlice)
	if AbortIfError(w, r, msg) {
		return
	}
	msg = LoadAndValidateMaps(r, jsonData, &jsonMaps, modelType)
	if AbortIfError(w, r, msg) {
		return
	}
	msg = ValidateMapsPrimaries(r, jsonMaps, utils.GetPrimaryFields(modelType))
	if AbortIfError(w, r, msg) {
		return
	}
	if len(jsonMaps) > 0 {
		db := ctx.DB(r).Session(&gorm.Session{CreateBatchSize: 50})

		modelSliceVal := reflect.ValueOf(modelSlice).Elem()

		modelSchema, err := schema.Parse(modelSliceVal.Index(0), &sync.Map{}, db.NamingStrategy)
		if err != nil {
			message.InternalServerError(r).Write(w, r)
			return
		}

		checked := map[string]struct{}{}
		for i := range jsonMaps {
			err := permissions.CheckModel(r, modelSliceVal.Index(i), modelSchema, checked, true)
			if AbortIfError(w, r, err) {
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

func (c GenericController[T]) Delete(w http.ResponseWriter, r *http.Request) {
	primaryFields := utils.GetPrimaryFields(c.ModelType())
	models := []interface{}{}
	err := PathParamsToModels(r, c.ModelType(), primaryFields, &models)
	if AbortIfError(w, r, err) {
		return
	}
	err = DeleteFromDb(r, models)
	if AbortIfError(w, r, err) {
		return
	}
}

func (c GenericController[T]) Routes() []Route {
	routes := []Route{}
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
			routes = append(routes, Route{
				Method:          http.MethodGet,
				Name:            "/",
				PermissionsFunc: m.PermissionsGet,
				HandlerFunc:     c.Get,
			})
			if len(primaryFields) > 0 {
				routes = append(routes, Route{
					Method:          http.MethodGet,
					Name:            "/" + params,
					PermissionsFunc: m.PermissionsGet,
					HandlerFunc:     c.GetOne,
				})
			}
			routes = append(routes, Route{
				Method:          http.MethodGet,
				Name:            "/structure",
				PermissionsFunc: m.PermissionsGet,
				HandlerFunc:     c.GetStructure,
			}, Route{
				Method:          http.MethodGet,
				Name:            "/structure/{rel}",
				PermissionsFunc: m.PermissionsGet,
				HandlerFunc:     c.GetRelStructure,
			})
		}
		if m, ok := mdl.(permissions.ModelWithPermissionsPost); ok {
			routes = append(routes, Route{
				Method:          http.MethodPost,
				Name:            "/",
				PermissionsFunc: m.PermissionsPost,
				HandlerFunc:     c.Post,
			})
		}
		if m, ok := mdl.(permissions.ModelWithPermissionsPatch); ok {
			routes = append(routes, Route{
				Method:          http.MethodPost,
				Name:            "/",
				PermissionsFunc: m.PermissionsPatch,
				HandlerFunc:     c.Patch,
			})
			routes = append(routes, Route{
				Method:          http.MethodPost,
				Name:            "/" + params,
				PermissionsFunc: m.PermissionsPatch,
				HandlerFunc:     c.PatchOne,
			})
		}
		if m, ok := mdl.(permissions.ModelWithPermissionsDelete); ok {
			routes = append(routes, Route{
				Method:          http.MethodPost,
				Name:            "/" + params,
				PermissionsFunc: m.PermissionsDelete,
				HandlerFunc:     c.Delete,
			})
		}
	}
	return routes
}

func (c GenericController[T]) Endpoint(controller any) string {
	t := reflect.TypeOf(controller)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.Name()
}

func PathParamsToModels(r *http.Request, modelType reflect.Type, fields []string, destination *[]interface{}) error {
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
