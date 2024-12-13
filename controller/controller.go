package controller

import (
	"net/http"
	"reflect"
	_ "time/tzdata"

	"api_core/permissions"
)

type AddRouter interface {
	AddRoute(method string, name string, permissionsFunc permissions.HandlerFunc, handlersFunc ...http.HandlerFunc)
}

type GetModeler interface {
	GetModel() any
	GetModelType() reflect.Type

	NewModel() any
	NewSliceOfModel() any
}

type Route struct {
	Method          string
	Name            string
	PermissionsFunc permissions.HandlerFunc
	HandlerFunc     http.HandlerFunc
}

type CRUDSController interface {
	AddRouter
	GetModeler

	SetBasePath(basePath string)
	SetEndpointIfAbsent(name string)
	GetEndpoint() string
	GetEndpointPath() string

	Get(w http.ResponseWriter, r *http.Request)
	GetOne(w http.ResponseWriter, r *http.Request)
	GetStructure(w http.ResponseWriter, r *http.Request)
	GetRelStructure(w http.ResponseWriter, r *http.Request)
	Post(w http.ResponseWriter, r *http.Request)
	Patch(w http.ResponseWriter, r *http.Request)
	PatchMany(w http.ResponseWriter, r *http.Request)
	Delete(w http.ResponseWriter, r *http.Request)

	CanImport() bool

	AddCustomRoutes()
	AdditionalModels() []reflect.Type
	GetRoutes() []Route
}

/*type Controller struct {
	Model    interface{}
	BasePath string
	Endpoint string
	Routes   []types.Route
}

func (c Controller) NewModel() interface{} {
	return reflect.New(c.GetModelType()).Interface()
}

func (c Controller) NewSliceOfModel() interface{} {
	return reflect.New(reflect.SliceOf(c.GetModelType())).Interface()
}

func (c Controller) GetModel() interface{} {
	return c.Model
}

func (c Controller) GetModelType() reflect.Type {
	if c.Model == nil {
		return nil
	}
	return reflect.Indirect(reflect.ValueOf(c.Model)).Type()
}

func (c *Controller) SetBasePath(basePath string) {
	c.BasePath = basePath
}

func (c *Controller) SetEndpointIfAbsent(name string) {
	if len(c.Endpoint) > 0 {
		return
	}
	c.Endpoint = name
}

func (c Controller) GetEndpoint() string {
	return c.Endpoint
}

func (c Controller) GetEndpointPath() string {
	return c.BasePath + "/" + c.Endpoint
}

func (c Controller) Get(w http.ResponseWriter, r *http.Request) {
	HandleGet(w, r, ctx.DB(r), map[string]interface{}{}, c.NewModel())
}

func (c Controller) GetOne(w http.ResponseWriter, r *http.Request) {
	primaries := map[string]interface{}{}
	err := GetPathParams(r, c.NewModel(), utils.GetPrimaryFields(c.GetModelType()), &primaries)
	if AbortIfError(w, r, err) {
		return
	}
	err = HandleGet(w, r, ctx.DB(r), primaries, c.NewModel())
	if AbortIfError(w, r, err) {
		return
	}
}

func (c Controller) GetStructure(w http.ResponseWriter, r *http.Request) {
	relations := GetRelations(r)
	splittedRelations := [][]string{}
	for _, rel := range relations {
		splittedRelations = append(splittedRelations, strings.Split(rel, "."))
	}

	modelSchema, err := schema.Parse(c.GetModel(), &sync.Map{}, app.DB.NamingStrategy)
	if err != nil {
		message.InternalServerError(r).Write(w, r)
		return
	}

	render.JSON(w, r, structure.GetStructInfo(r, modelSchema, splittedRelations))
}

func (c Controller) GetRelStructure(w http.ResponseWriter, r *http.Request) {
	modelSchema, err := schema.Parse(c.GetModel(), &sync.Map{}, app.DB.NamingStrategy)
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

	render.JSON(w, r, structure.GetStructInfo(r, relSchema, [][]string{}))
}

func (c Controller) Post(w http.ResponseWriter, r *http.Request) {
	jsonData, err := io.ReadAll(r.Body)

	if err != nil || len(jsonData) == 0 {
		message.InvalidJSON(r).Write(w, r)
		return
	}

	if jsonData[0] == '[' {
		model := c.NewSliceOfModel()
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
		model := c.NewModel()
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

func (c Controller) Patch(w http.ResponseWriter, r *http.Request) {
	model := c.NewModel()
	jsonMap := make(map[string]interface{})
	jsonData, _ := io.ReadAll(r.Body)
	modelType := c.GetModelType()
	primaryFields := utils.GetPrimaryFields(modelType)

	msg := LoadModel(r, jsonData, model)
	if AbortIfError(w, r, msg) {
		return
	}
	msg = GetPathParams(r, model, primaryFields, model)
	if AbortIfError(w, r, msg) {
		return
	}
	msg = LoadAndValidateMap(r, jsonData, jsonMap, modelType)
	if AbortIfError(w, r, msg) {
		return
	}
	msg = GetPathParams(r, model, primaryFields, &jsonMap)
	if AbortIfError(w, r, msg) {
		return
	}
	msg = UpdateToDb(w, r, model, jsonMap)
	if AbortIfError(w, r, msg) {
		return
	}
}

func (c Controller) PatchMany(w http.ResponseWriter, r *http.Request) {
	modelSlice := c.NewSliceOfModel()
	jsonMaps := []map[string]interface{}{}
	jsonData, _ := io.ReadAll(r.Body)
	modelType := c.GetModelType()

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
			err := CheckModelPermissions(r, modelSliceVal.Index(i), modelSchema, checked, true)
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

func (c Controller) Delete(r *http.Request) {
	primaryFields := utils.GetPrimaryFields(c.GetModelType())
	models := []interface{}{}
	PathParamsToModels(r, c.GetModelType(), primaryFields, &models)
	DeleteFromDb(r, models)
}

func (c *Controller) CanImport() bool {
	return false
}

func (c *Controller) AddRoute(method string, name string, permissionsFunc permissions.HandlerFunc, handlerFunc http.HandlerFunc) {
	c.Routes = append(c.Routes, types.Route{Method: method, Name: name, PermissionsFunc: permissionsFunc, HandlerFunc: handlerFunc})
}

func (c Controller) AddCustomRoutes() {}

func (c Controller) AdditionalModels() []reflect.Type {
	return []reflect.Type{}
}

func (c Controller) GetRoutes() []types.Route {
	return c.Routes
}*/
