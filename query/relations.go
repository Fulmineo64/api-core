package query

import (
	"net/http"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"api_core/message"
	"api_core/model"
	"api_core/params"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

const queryTableName = "ORIGIN"

type ComputedField struct {
	Fn func(*http.Request, *gorm.DB, *ModelInfo, *map[string]any) message.Message
}

type NestedModel struct {
	References []*schema.Reference
	Slice      bool
	ModelInfo  *ModelInfo
}

func (n NestedModel) NextNested() map[string]NestedModel {
	return n.ModelInfo.Nested
}

func JoinRelations(r *http.Request, d *gorm.DB, config QueryConfig, modelInfo *ModelInfo, relations map[string]*params.Conditions) {
	joins := ""
	joinsArgs := []interface{}{}
	joinedTables := map[string]*schema.Schema{}

	// Check the relations

	relationsKeys := make([]string, 0, len(relations))
	for k := range relations {
		relationsKeys = append(relationsKeys, k)
	}
	sort.Strings(relationsKeys)

	for _, relation := range relationsKeys {
		conditions := relations[relation]
		relSchema := modelInfo.Schema
		pieces := strings.Split(relation, ".")
		for i, piece := range pieces {
			alias := strings.Join(pieces[:i+1], "__")
			if rel, ok := relSchema.Relationships.Relations[piece]; ok {
				relSchema = rel.FieldSchema
				if joinedTables[alias] == nil {
					joinedTables[alias] = rel.FieldSchema
					if conditions.Type != "I" {
						// Outer join
						joins += ` LEFT`
					}
					joins += ` JOIN ` + rel.FieldSchema.Table + ` AS ` + alias + ` ON`
					for j, ref := range rel.References {
						if j > 0 {
							joins += " AND "
						}
						if i > 0 {
							joins += ` ` + strings.Join(pieces[:i], "__") + `.` + ref.PrimaryKey.DBName + ` = ` + alias + `.` + ref.ForeignKey.DBName
						} else {
							joins += ` ` + modelInfo.Table + `.` + ref.PrimaryKey.DBName + ` = ` + alias + `.` + ref.ForeignKey.DBName
						}
					}
					if !config.SkipDefaults {
						mdl := reflect.New(joinedTables[alias].ModelType).Interface()
						if model, ok := mdl.(model.ConditionsModel); ok {
							query, args := model.DefaultConditions(d, alias)
							if query != "" {
								joins += " AND " + query
								joinsArgs = append(joinsArgs, args...)
							}
						}
						if model, ok := mdl.(model.JoinsModel); ok {
							joins += model.DefaultJoins(d, alias)
						}
					}
				}
			} else {
				d.AddError(message.InvalidRelations(r, strings.Join(pieces[:i+1], ".")))
				return
			}
		}
		if conditions.Query != "" && (conditions.Type == "I" || conditions.Type == "O") {
			joins += " AND" + conditions.Query
			joinsArgs = append(joinsArgs, conditions.Args...)
		}
	}

	if d.Error != nil {
		return
	}

	if len(joins) > 0 {
		d.Joins(joins, joinsArgs...)
	}
}

func ModelToTableNames(stmt *string, modelSchema *schema.Schema) []string {
	relations := []string{}
	r := regexp.MustCompile(`([\w\.]*)\.(\w*)`)
	r.ReplaceAllStringFunc(*stmt, func(s string) string {
		slice := strings.Split(s, ".")
		if slice[0] == modelSchema.Table {
			return s
		}
		relations = append(relations, strings.Join(slice[:len(slice)-1], "."))
		return strings.Join(slice[:len(slice)-1], "__") + "." + slice[len(slice)-1]
	})
	return relations
}

func ParseOrder(r *http.Request, order string, info *ModelInfo, config QueryConfig) message.Message {
	if len(order) > 0 {
		local := []string{}
		nested := map[string][]string{}
		fields := strings.Split(order, ",")
		for i, field := range fields {
			field = strings.TrimSpace(field)
			if strings.HasPrefix(field, ">") {
				field = field[1:]
				pieces := strings.Split(field, ".")
				if _, ok := nested[pieces[0]]; !ok {
					nested[pieces[0]] = []string{}
				}
				nested[pieces[0]] = append(nested[pieces[0]], strings.Join(pieces[1:], "."))
			} else {
				var toBoolean bool
				for strings.HasPrefix(field, "!") {
					toBoolean = true
					field = field[1:]
				}

				if pos := strings.Index(field, "<."); pos != -1 {
					if i > 0 {
						if index := strings.LastIndex(fields[i-1], "."); index != -1 {
							field = field[:pos] + fields[i-1][:index] + field[pos+1:]
						}
					}
				}

				relSchema := info.Schema
				pieces := strings.Split(field, ".")
				for i := 0; i < len(pieces)-1; i++ {
					if rel, ok := relSchema.Relationships.Relations[strings.TrimPrefix(pieces[i], ">")]; ok {
						relSchema = rel.FieldSchema
					} else {
						return message.InvalidRelation(r, strings.Join(pieces[:i+1], "."))
					}
				}

				fldName := strings.TrimSuffix(strings.TrimSuffix(field, " DESC"), " ASC")
				piece := strings.TrimSuffix(strings.TrimSuffix(pieces[len(pieces)-1], " DESC"), " ASC")
				fld := relSchema.LookUpField(piece)
				if fld == nil {
					search := config.Dialector.EscapeField(piece)
					found := false
					for _, f := range info.Select {
						if asIndex := strings.LastIndex(f, " AS "); asIndex != -1 {
							if f[asIndex+4:] == search || strings.HasSuffix(f[:asIndex], search) {
								found = true
								break
							}
						} else if strings.HasSuffix(f, search) {
							found = true
							break
						}
					}
					if found {
						if strings.HasSuffix(pieces[len(pieces)-1], " DESC") {
							search += " DESC"
						}
						local = append(local, search)
					} else {
						return message.InvalidField(r, field)
					}
				}

				if fld != nil {
					if info.Distinct {
						found := false
						selField := config.Dialector.EscapeField(fld.Name)
						selRel := selField
						if len(pieces) > 1 {
							selRel = strings.Join(pieces[:len(pieces)-1], "__") + "." + selField
						}
						for _, f := range info.Select {
							if asIndex := strings.LastIndex(f, " AS "); asIndex != -1 {
								if f[asIndex+4:] == selField || strings.HasSuffix(f[:asIndex], selRel) {
									found = true
									break
								}
							} else if strings.HasSuffix(f, selRel) {
								found = true
								break
							}
						}
						if !found {
							return message.ConflictingOrderByAndDistinct(r)
						}
					}

					if _, ok := fld.StructField.Tag.Lookup("query"); ok {
						field = pieces[len(pieces)-1]
					} else {
						alias := info.Table
						l := len(pieces)
						if l > 1 {
							alias = strings.Join(pieces[:l-1], "__")
						}
						field = alias + "." + pieces[l-1]
					}

					if toBoolean {
						field = "CASE WHEN " + fldName + " IS NULL THEN 0 ELSE 1 END" + field[len(fldName):]
					}
					local = append(local, field)
				}
			}
		}

		for key, nestedArray := range nested {
			if n, ok := info.Nested[key]; ok {
				msg := ParseOrder(r, strings.Join(nestedArray, ","), n.ModelInfo, config)
				if msg != nil {
					return msg
				}
			} else {
				return message.InvalidRelation(r, key)
			}
		}

		info.Order = strings.Join(local, ",")
	}
	return nil
}

func RelationsFromModelInfo(mdl *ModelInfo, otherRelations map[string]*params.Conditions) map[string]*params.Conditions {
	set := map[string]*params.Conditions{}
	for rel := range mdl.Relations {
		set[rel] = &params.Conditions{}
	}
	for rel := range otherRelations {
		if otherRelations[rel].Type != "N" {
			set[rel] = otherRelations[rel]
		}
	}
	return set
}

func GetRelations(r *http.Request) []string {
	var relations []string
	if len(r.URL.Query().Get("rel")) > 0 {
		relations = strings.Split(strings.ReplaceAll(r.URL.Query().Get("rel"), " ", ""), ",")
	}
	return relations
}
