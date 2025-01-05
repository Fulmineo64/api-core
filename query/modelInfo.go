package query

import (
	"api_core/message"
	"api_core/model"
	"api_core/params"
	"api_core/request"
	"net/http"
	"reflect"
	"regexp"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type ModelInfo struct {
	Select         []string
	SelectArgs     []any
	Fields         []reflect.StructField
	ComputedFields []ComputedField
	Schema         *schema.Schema
	Table          string
	Order          string
	Relations      map[string]*params.Conditions
	Nested         map[string]NestedModel
	Aggregate      bool
	Distinct       bool
}

func GetModelInfo(r *http.Request, modelSchema *schema.Schema, selects string, modelInfo *ModelInfo, args *QueryArgs, config QueryConfig) message.Message {
	modelInfo.Table = strings.TrimSpace(modelInfo.Schema.Table)
	if strings.HasSuffix(modelInfo.Table, ")") {
		modelInfo.Table = queryTableName
	} else if index := strings.LastIndex(modelInfo.Table, ") AS "); index != -1 {
		modelInfo.Table = modelInfo.Table[index+5:]
	}

	if len(selects) > 0 {
		if strings.HasPrefix(selects, "DISTINCT ") {
			selects = selects[9:]
			modelInfo.Distinct = true
		}

		fields := strings.Split(selects, ",")
		for j, field := range fields {
			field = strings.TrimSpace(field)
			if len(field) == 0 {
				continue
			}

			fieldAlias := ""
			index := strings.Index(field, " AS ")
			if index > -1 {
				fieldAlias = field[index+4:]
				field = field[:index]
			}

			sum := false
			count := false
			if strings.HasPrefix(field, "SUM(") && strings.HasSuffix(field, ")") {
				field = field[4 : len(field)-1]
				sum = true
			}
			if strings.HasPrefix(field, "COUNT(") && strings.HasSuffix(field, ")") {
				field = field[6 : len(field)-1]
				count = true
			}

			if pos := strings.Index(field, "<."); pos != -1 {
				if j > 0 {
					if index := strings.LastIndex(fields[j-1], "."); index != -1 {
						field = field[:pos] + fields[j-1][:index] + field[pos+1:]
					}
				}
				fields[j] = field
			}

			relSchema := modelSchema
			info := modelInfo
			pieces := strings.Split(field, ".")
			startIndex := 0
			key := ""
			for i := 0; i < len(pieces)-1; i++ {
				var nested bool
				if strings.HasPrefix(pieces[i], ">") {
					pieces[i] = pieces[i][1:]
					nested = true
				}
				if key != "" {
					key += "."
				}
				key += pieces[i]
				if rel, ok := relSchema.Relationships.Relations[pieces[i]]; ok {
					relSchema = rel.FieldSchema
					if nested {
						if _, ok := info.Nested[key]; !ok {
							n := NestedModel{
								References: rel.References,
								ModelInfo:  &ModelInfo{Fields: []reflect.StructField{}, Relations: map[string]*params.Conditions{}, Nested: map[string]NestedModel{}, Schema: relSchema},
							}
							n.ModelInfo.Table = n.ModelInfo.Schema.Table
							if strings.HasSuffix(strings.TrimSpace(n.ModelInfo.Schema.Table), ")") {
								n.ModelInfo.Table = queryTableName
							}
							mdl := reflect.New(relSchema.ModelType).Interface()
							if ordMdl, ok := mdl.(model.OrderedModel); ok {
								n.ModelInfo.Order = ordMdl.DefaultOrder(request.DB(r), n.ModelInfo.Table)
							}
							l := len(rel.References)
							var fk string
							for j, ref := range rel.References {
								if j > 0 {
									fk += ",'___',"
								}
								if l > 1 {
									fk += "CAST(" + n.ModelInfo.Table + "." + ref.ForeignKey.DBName + " AS NVARCHAR(MAX))"
								} else {
									fk += n.ModelInfo.Table + "." + ref.ForeignKey.DBName
								}
							}
							if len(rel.References) > 1 {
								fk = "CONCAT(" + fk + ")"
							}
							n.ModelInfo.Select = []string{fk + " AS " + fkAlias}
							if rel.Field.FieldType.Kind() == reflect.Slice {
								n.Slice = true
							}
							info.Nested[key] = n
						}
						startIndex = i + 1
						info = info.Nested[key].ModelInfo
						key = ""
					}
				} else {
					return message.InvalidRelation(r, strings.Join(pieces[:i+1], "."))
				}
			}
			fieldName := pieces[len(pieces)-1]
			if fieldAlias != "" {
				fieldAlias = strings.ReplaceAll(fieldAlias, "*", fieldName)
			}

			var structFields []*schema.Field

			if strings.HasSuffix(fieldName, "*") {
				structFields = []*schema.Field{}
				for _, fld := range relSchema.Fields {
					_, okQ := fld.Tag.Lookup("query")
					_, okC := fld.Tag.Lookup("compute")
					if !okQ && !okC {
						structFields = append(structFields, fld)
					}
				}
			} else {
				fld := relSchema.LookUpField(fieldName)
				if fld == nil {
					return message.InvalidField(r, field)
				}
				if (sum || count) && fieldAlias == "" {
					fieldAlias = fieldName
				}
				structFields = []*schema.Field{fld}
			}

			if len(pieces)-startIndex > 1 {
				info.Relations[strings.Join(pieces[startIndex:len(pieces)-1], ".")] = &params.Conditions{}
			}

			for _, field := range structFields {
				var sel string
				table := info.Table
				if len(pieces)-startIndex > 1 {
					table = strings.Join(pieces[startIndex:len(pieces)-1], "__")
				}
				if funcName, ok := field.StructField.Tag.Lookup("compute"); ok {
					if len(funcName) == 0 {
						funcName = "Compute" + field.Name
					}
					mdl := reflect.New(relSchema.ModelType)
					fn := mdl.MethodByName(funcName)
					if fn.IsValid() {
						info.ComputedFields = append(info.ComputedFields, ComputedField{
							Fn: fn.Interface().(func(*http.Request, *gorm.DB, *ModelInfo, *map[string]any) error),
						})
					} else {
						return message.InvalidField(r, field.Name)
					}
					// Here, instead of funcName, I would include the entire function
				} else if funcName, ok := field.StructField.Tag.Lookup("query"); ok {
					field.StructField.Tag = reflect.StructTag(strings.TrimPrefix(string(field.StructField.Tag), `gorm:"-"`))
					if len(funcName) == 0 {
						funcName = "Query" + field.Name
					}
					mdl := reflect.New(relSchema.ModelType)
					fn := mdl.MethodByName(funcName)
					if fn.IsValid() {
						rels := map[string]*params.Conditions{}
						msg := fn.Interface().(func(*http.Request, any, *schema.Schema, string, bool, *string, *[]any, map[string]*params.Conditions) message.Message)(r, mdl.Interface(), relSchema, table, len(pieces)-startIndex > 1, &sel, &info.SelectArgs, rels)
						if msg != nil {
							return msg
						}
						for rel := range rels {
							info.Relations[rel] = &params.Conditions{}
						}
						if len(fieldAlias) == 0 {
							fieldAlias = field.Name
						}
					} else {
						return message.InvalidField(r, field.Name)
					}
				} else if len(field.DBName) != 0 {
					sel = table + "." + config.Dialector.EscapeField(field.DBName)
				}
				if len(sel) > 0 {
					if sum {
						sel = "SUM(" + sel + ")"
						info.Aggregate = true
					}
					if count {
						sel = "COUNT(" + sel + ")"
						info.Aggregate = true
					}
					structField := field.StructField
					if len(fieldAlias) > 0 {
						regex := regexp.MustCompile(`^[\w*]+$`)
						if !regex.MatchString(fieldAlias) {
							return message.InvalidFieldAlias(r, fieldAlias, fieldName)
						}
						structField.Name = strings.ReplaceAll(fieldAlias, "*", field.Name)
						sel += " AS " + config.Dialector.EscapeField(structField.Name)
					}
					info.Fields = append(info.Fields, structField)
					info.Select = append(info.Select, sel)
				}
			}
		}
		// Throw an error if base model sel is empty
		if len(modelInfo.Select) == 0 {
			return message.MissingBaseResourceSelect(r, modelInfo.Table)
		}
	} else {
		for _, field := range modelSchema.Fields {
			if field.Readable && len(field.DBName) != 0 {
				modelInfo.Fields = append(modelInfo.Fields, field.StructField)
				modelInfo.Select = append(modelInfo.Select, modelInfo.Table+"."+field.DBName)
			}
		}
	}
	if len(args.Ord) > 0 {
		msg := ParseOrder(r, args.Ord, modelInfo, config)
		if msg != nil {
			return msg
		}
	} else if ordModel, ok := reflect.New(modelSchema.ModelType).Interface().(model.OrderedModel); ok {
		modelInfo.Order = ordModel.DefaultOrder(request.DB(r), modelInfo.Table)
	} else if modelSchema.PrioritizedPrimaryField != nil {
		modelInfo.Order = modelInfo.Table + "." + modelSchema.PrioritizedPrimaryField.DBName
	}
	return nil
}
