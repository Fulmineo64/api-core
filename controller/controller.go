package controller

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"

	"api_core/message"
	"api_core/permissions"
	"api_core/types/data"

	"github.com/go-chi/chi"
	"github.com/go-chi/render"
	"gorm.io/gorm"
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
}*/

func HandleGet(w http.ResponseWriter, r *http.Request, db *gorm.DB, primaries map[string]interface{}, model any) error {
	args := QueryMapArgs{
		Sel:       chi.URLParam(r, "sel"),
		Rel:       chi.URLParam(r, "rel"),
		Params:    chi.URLParam(r, "params"),
		P:         chi.URLParam(r, "p"),
		PagStart:  chi.URLParam(r, "pagStart"),
		PagEnd:    chi.URLParam(r, "pagEnd"),
		Ord:       chi.URLParam(r, "ord"),
		Primaries: primaries,
		Model:     model,
	}
	err := QueryMap(r, db, &args, QueryMapConfig{})
	if err != nil {
		return err
	}
	err = WriteQueryMapResult(w, r, &args)
	if err != nil {
		return err
	}
	return nil
}

func WriteQueryMapResult(w http.ResponseWriter, r *http.Request, args *QueryMapArgs) error {
	w.Header().Set("X-Total-Count", strconv.Itoa(int(args.Count)))
	var link string
	if ShouldPaginate(args.PagStart, args.PagEnd) {
		limit := GetLimit(args.PagStart, args.PagEnd)
		start := GetOffset(args.PagStart) + limit
		end := start + limit
		if int64(end) < args.Count {
			var params []string
			for key, values := range r.URL.Query() {
				if key == "pagStart" {
					params = append(params, key+"="+strconv.Itoa(start))
				} else if key == "pagEnd" {
					params = append(params, key+"="+strconv.Itoa(end))
				} else {
					params = append(params, key+"="+strings.Join(values, "&"+key+"="))
				}
			}
			link = r.URL.RequestURI() + "?" + strings.Join(params, "&")
			w.Header().Set("Link", link)
		}
	}
	switch r.Header.Get("Accept") {
	case "application/csv", "text/csv":
		w.Header().Set("Content-Type", r.Header.Get("Accept")+"; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=data.csv")
		render.Status(r, http.StatusOK)

		// TODO: Done but needs better checking; sorting based on the input select
		// TODO: Manage the CSV in the correct order

		var csvData [][]string
		tmz := r.Header.Get("Timezone")
		var heading []string
		for _, f := range args.Info.Fields {
			heading = append(heading, f.Name)
		}
		for key := range args.Info.Nested {
			heading = append(heading, key)
		}
		// for i := range heading {
		// 	if strings.Contains(heading[i], "AS") {
		// 		s := strings.Split(heading[i], "AS")
		// 		heading[i] = strings.TrimSpace(s[len(s)-1])
		// 	}
		// }
		l := len(args.Result)
		if l > 0 {

			csvData = append(csvData, heading)

			for i := 0; i < l; i++ {
				item := args.Result[i]
				var row []string
				for _, f := range args.Info.Fields {
					rv := reflect.ValueOf(item[f.Name])
					if rv.IsValid() && !rv.IsZero() && !rv.IsNil() {
						t := reflect.Indirect(rv)
						if t.Type().Kind() == reflect.Ptr {
							t = t.Elem()
						}
						f := t.Interface()
						if f != nil && f != "" {
							if date, ok := f.(data.Date); ok {
								row = append(row, time.Time(date).Format("02/01/2006"))
							} else if datetime, ok := f.(data.Datetime); ok {
								if r.Header.Get("Only-Date") == "" {
									loc, _ := time.LoadLocation(tmz)
									row = append(row, time.Time(datetime).In(loc).Format("02/01/2006 15:04"))
								} else {
									row = append(row, time.Time(datetime).Format("02/01/2006"))
								}
							} else if _, ok := f.(data.RoundedFloat); ok {
								row = append(row, strings.ReplaceAll(fmt.Sprint(f), ".", ","))
							} else if _, ok := f.(float32); ok {
								row = append(row, strings.ReplaceAll(fmt.Sprint(f), ".", ","))
							} else if _, ok := f.(float64); ok {
								row = append(row, strings.ReplaceAll(fmt.Sprint(f), ".", ","))
							} else {
								if _, ok := f.(string); ok {
									row = append(row, fmt.Sprintf("%s", f))
								} else {
									row = append(row, fmt.Sprint(f))
								}
							}
						} else {
							row = append(row, "")
						}
					} else {
						row = append(row, "")
					}
				}
				for key := range args.Info.Nested {
					data, err := json.Marshal(item[key])
					if err != nil {
						return err
					}
					row = append(row, string(data))
				}
				csvData = append(csvData, row)
			}
		}
		if err := csv.NewWriter(w).WriteAll(csvData); err != nil {
			return err
		}
	case "application/xml", "text/xml":
		if reflect.TypeOf(args.Result).Name() == "" || len(chi.URLParam(r, "wrap")) > 0 {
			render.XML(w, r, Response{Data: args.Result, Next: link, Count: args.Count})
		} else {
			render.XML(w, r, args.Result)
		}
	default:
		var result any = args.Result
		if len(args.Primaries) != 0 {
			result = args.Result[0]
			// TODO: It might be advisable to set Count to 1 in this situation
		}
		if len(chi.URLParam(r, "wrap")) > 0 {
			render.JSON(w, r, Response{Data: result, Next: link, Count: args.Count})
		} else {
			render.JSON(w, r, result)
		}
	}
	return nil
}

func WriteDataWithCount(w http.ResponseWriter, r *http.Request, pagStart, pagEnd string, data any, count int64) error {
	w.Header().Set("X-Total-Count", strconv.Itoa(int(count)))
	var link string
	if ShouldPaginate(pagStart, pagEnd) {
		limit := GetLimit(pagStart, pagEnd)
		start := GetOffset(pagStart) + limit
		end := start + limit
		if int64(end) < count {
			var params []string
			for key, values := range r.URL.Query() {
				if key == "pagStart" {
					params = append(params, key+"="+strconv.Itoa(start))
				} else if key == "pagEnd" {
					params = append(params, key+"="+strconv.Itoa(end))
				} else {
					params = append(params, key+"="+strings.Join(values, "&"+key+"="))
				}
			}
			link = r.URL.RequestURI() + "?" + strings.Join(params, "&")
			w.Header().Set("Link", link)
		}
	}
	switch r.Header.Get("Accept") {
	case "application/csv", "text/csv":
		w.Header().Set("Content-Type", r.Header.Get("Accept")+"; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=data.csv")
		render.Status(r, http.StatusOK)

		var csvData [][]string
		v := reflect.ValueOf(data).Elem()
		t := v.Type()
		if t.Kind() != reflect.Slice {
			t = reflect.SliceOf(t)
			v = reflect.Append(reflect.New(t).Elem(), v)
		}

		l := v.Len()
		if l > 0 {

			t = t.Elem()
			if t.Kind() == reflect.Ptr {
				t = t.Elem()
			}

			var row []string
			for j := 0; j < t.NumField(); j++ {
				row = append(row, t.Field(j).Name)
			}
			csvData = append(csvData, row)

			for i := 0; i < l; i++ {
				item := v.Index(i).Elem()
				ti := item.Type()
				var row []string
				for j := 0; j < ti.NumField(); j++ {
					f := item.Field(j)
					if f.IsValid() && !f.IsZero() {
						if f.Type().Kind() == reflect.Ptr {
							f = f.Elem()
						}
						if f.Type().Kind() == reflect.Slice || f.Type().Kind() == reflect.Array {
							v, _ := json.Marshal(f.Interface())
							row = append(row, string(v))
						} else if marshaler, ok := f.Interface().(json.Marshaler); ok {
							v, _ := marshaler.MarshalJSON()
							row = append(row, strings.TrimSuffix(strings.TrimPrefix(string(v), "\""), "\""))
						} else if stringer, ok := f.Interface().(fmt.Stringer); ok {
							row = append(row, stringer.String())
						} else {
							row = append(row, fmt.Sprintf("%v", f.Interface()))
						}
					} else {
						row = append(row, "")
					}
				}
				csvData = append(csvData, row)
			}
		}
		if err := csv.NewWriter(w).WriteAll(csvData); err != nil {
			return err
		}
	case "application/xml", "text/xml":
		if reflect.TypeOf(data).Name() == "" || len(chi.URLParam(r, "wrap")) > 0 {
			render.XML(w, r, Response{Data: data, Next: link, Count: count})
		} else {
			render.XML(w, r, data)
		}
	default:
		if len(chi.URLParam(r, "wrap")) > 0 {
			render.JSON(w, r, Response{Data: data, Next: link, Count: count})
		} else {
			render.JSON(w, r, data)
		}
	}
	return nil
}

func PathParamsToModels(r *http.Request, modelType reflect.Type, fields []string, destination *[]interface{}) error {
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

/*func (c Controller) GetOne(w http.ResponseWriter, r *http.Request) {
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

	pieces := strings.Split(chi.URLParam(r, "rel"), ".")
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
