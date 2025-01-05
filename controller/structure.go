package controller

import (
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"

	"api_core/app"
	"api_core/message"
	"api_core/model"
	"api_core/permissions"
	"api_core/query"
	"api_core/registry"
	"api_core/response"
	"api_core/utils"

	"gorm.io/gorm/schema"
)

type FieldInfo struct {
	Field           string `json:"field"`
	Label           string `json:"label"`
	Descriptive     string `json:"descriptive"`
	Type            string `json:"type"`
	Primary         bool   `json:"primary"`
	Required        bool   `json:"required"`
	UpdateKey       bool   `json:"updateKey"`
	Deprecated      bool   `json:"deprecated"`
	MaxLength       int    `json:"maxLength"`
	RequiredWithout string `json:"requiredWithout"`
	RequiredWith    string `json:"requiredWith"`
	Updatable       bool   `json:"updatable"`
	Creatable       bool   `json:"creatable"`
	Query           bool   `json:"query"`
}

type RelationInfo struct {
	Field      string      `json:"field"`
	Label      string      `json:"label"`
	ForeignKey string      `json:"foreignKey"`
	References string      `json:"references"`
	Type       string      `json:"type"`
	Endpoint   string      `json:"endpoint"`
	Struct     *StructInfo `json:"struct"`
	Updatable  bool        `json:"updatable"`
	Creatable  bool        `json:"creatable"`
}

type StructInfo struct {
	Fields           []FieldInfo              `json:"fields"`
	Relations        []RelationInfo           `json:"relations"`
	UpdateConditions []model.UpdateConditions `json:"updateConditions"`
}

func GetStructure(mdl any) http.HandlerFunc {
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

func GetRelStructure(mdl any) http.HandlerFunc {
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

func GetStructInfo(r *http.Request, schem *schema.Schema, relations [][]string) StructInfo {
	var checkFn = func(fns []func(*schema.Field) bool, sc *schema.Field) bool {
		for _, fn := range fns {
			if !fn(sc) {
				return false
			}
		}
		return true
	}
	checkFieldFns := []func(*schema.Field) bool{}
	checkRelFns := []func(*schema.Field) bool{
		func(f *schema.Field) bool {
			typ := f.IndirectFieldType
			if typ.Kind() == reflect.Slice {
				typ = typ.Elem()
			}
			mdl := reflect.New(typ).Interface()
			msg := permissions.Get(mdl)(r)
			return msg == nil
		},
	}

	if r.URL.Query().Get("w") == "1" {
		// Checks for only writable
		fn := func(f *schema.Field) bool {
			return f.Creatable && f.Updatable
		}
		checkFieldFns = append(checkFieldFns, fn)
		checkRelFns = append(checkRelFns, fn)
	}
	if r.URL.Query().Get("r") == "1" {
		// Checks for only readable
		fn := func(f *schema.Field) bool {
			return f.Readable
		}
		checkFieldFns = append(checkFieldFns, fn)
		checkRelFns = append(checkRelFns, fn)
	}

	structInfo := StructInfo{
		Fields:           []FieldInfo{},
		Relations:        []RelationInfo{},
		UpdateConditions: []model.UpdateConditions{},
	}

	for _, field := range schem.Fields {
		if field.DBName != "" && checkFn(checkFieldFns, field) {
			structInfo.Fields = append(structInfo.Fields, GetFieldInfo(r, field))
		}
		if _, ok := field.Tag.Lookup("query"); ok {
			fieldInfo := GetFieldInfo(r, field)
			fieldInfo.Query = true
			structInfo.Fields = append(structInfo.Fields, fieldInfo)
		}
	}

	for key, rel := range schem.Relationships.Relations {
		if !strings.HasPrefix(key, "_") && checkFn(checkRelFns, rel.Field) {
			structInfo.Relations = append(structInfo.Relations, GetRelationInfo(r, rel, relations))
		}
	}

	sort.SliceStable(structInfo.Relations, func(i, j int) bool {
		return structInfo.Relations[i].Label < structInfo.Relations[j].Label
	})

	mdl := reflect.New(schem.ModelType).Interface()
	if updateModel, ok := mdl.(model.UpdateConditionsModel); ok {
		structInfo.UpdateConditions = updateModel.UpdateConditions()
	}
	var hasDisplay bool
	index := 0
	i := 0
	for ; i < len(structInfo.Fields) && !hasDisplay; i++ {
		if structInfo.Fields[i].Field == "DISPLAY_NAME" {
			index = i
		}
		hasDisplay = structInfo.Fields[i].Descriptive != ""
	}
	if !hasDisplay {
		structInfo.Fields = append(structInfo.Fields[:index], structInfo.Fields[index+1:]...)
	}
	return structInfo
}

func GetFieldInfo(r *http.Request, field *schema.Field) FieldInfo {
	requiredWithout := ""
	requiredWith := ""
	v := strings.Split(field.Tag.Get("valid"), ",")
	for _, req := range v {
		if strings.HasPrefix(req, "required_without=") {
			requiredWithout = req[17:]
		}
		if strings.HasPrefix(req, "required_with=") {
			requiredWith = req[14:]
		}
	}

	_, isDeprecated := field.Tag.Lookup("deprecated")

	fieldInfo := FieldInfo{
		Field:           field.Name,
		Label:           FieldToString(field),
		Descriptive:     field.Tag.Get("desc"),
		Type:            strings.ReplaceAll(field.StructField.Type.String(), "*", ""),
		Primary:         strings.Contains(field.Tag.Get("gorm"), "primaryKey"),
		UpdateKey:       strings.Contains(field.Tag.Get("import"), "updateKey"),
		Deprecated:      isDeprecated,
		RequiredWithout: requiredWithout,
		RequiredWith:    requiredWith,
		Updatable:       field.Updatable,
		Creatable:       field.Creatable,
	}

	label := field.Tag.Get("label")
	if label != "" {
		fieldInfo.Label = label
	}

	validationsTag := field.Tag.Get("validate")
	if len(validationsTag) > 0 {
		validations := strings.Split(validationsTag, ",")
		for _, validation := range validations {
			if validation == "required" {
				fieldInfo.Required = true
			} else if strings.Contains(validation, "max=") {
				val, err := strconv.Atoi(strings.ReplaceAll(validation, "max=", ""))
				if err == nil {
					fieldInfo.MaxLength = val
				}
			}
		}
	}
	return fieldInfo
}

func GetRelationInfo(r *http.Request, rel *schema.Relationship, relations [][]string) RelationInfo {
	gormTags := strings.Split(rel.Field.Tag.Get("gorm"), ";")
	relationInfo := RelationInfo{
		Field: rel.Field.Name,
		Label: FieldToString(rel.Field),
	}

	typ := rel.Field.StructField.Type

	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	if typ.Kind() == reflect.Slice {
		typ = typ.Elem()
	}

	ctrl := registry.ControllerByModel[typ.String()]
	if ctrl != nil {
		relationInfo.Endpoint = ctrl.BasePath()
	}

	for _, tag := range gormTags {
		if strings.Contains(tag, "foreignKey:") {
			relationInfo.ForeignKey = strings.ReplaceAll(tag, "foreignKey:", "")
		} else if strings.Contains(tag, "references:") {
			relationInfo.References = strings.ReplaceAll(tag, "references:", "")
		}
		relationInfo.Updatable = rel.Field.Updatable
		relationInfo.Creatable = rel.Field.Creatable
	}

	relationInfo.Type = string(rel.Type)
	if relationInfo.Type == "has_one" {
		fld := rel.Schema.LookUpField(relationInfo.References)
		if fld != nil && !fld.PrimaryKey {
			relationInfo.Type = "belongs_to"
		}
	}
	rels := [][]string{}
	for _, relation := range relations {
		if len(relation) > 0 && relation[0] == relationInfo.Field {
			rels = append(rels, relation[1:])
		}
	}
	if len(rels) > 0 {
		structInfo := GetStructInfo(r, rel.FieldSchema, rels)
		relationInfo.Struct = &structInfo
	}

	return relationInfo
}

func FieldToString(field *schema.Field) string {
	return utils.SentenceCase(strings.ReplaceAll(field.Name, "_", " "))
}
