package query

import (
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"sync"

	"api_core/app/dialectors"
	"api_core/message"
	"api_core/model"
	"api_core/params"
	"api_core/permissions"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

var fkAlias = "___FK___"

type QueryArgs struct {
	// Args
	Sel       string
	Rel       string
	Params    string
	P         string
	PagStart  string
	PagEnd    string
	Ord       string
	Primaries map[string]interface{}
	// Model
	Model any
	// Output
	Info   ModelInfo
	Result []map[string]any
	Count  int64
}

type QueryConfig struct {
	// Options
	SkipValidation bool
	SkipDefaults   bool
	P              map[string]struct{}
	Ord            map[string]struct{}
	Dialector      dialectors.Dialector
}

func Query(r *http.Request, db *gorm.DB, args *QueryArgs, config QueryConfig) error {
	if args.Sel != "" {
		args.Sel = parseSel(args.Sel)
	}

	if args.Rel != "" {
		args.Rel = parseSel(args.Rel)
	}

	modelSchema, err := schema.Parse(args.Model, &sync.Map{}, db.NamingStrategy)
	if err != nil {
		return err
	}

	// Obtain all the relations from the arguments
	// TODO: Extract and validate relations here

	args.Info = ModelInfo{Select: []string{}, SelectArgs: []any{}, Relations: map[string]*params.Conditions{}, Nested: map[string]NestedModel{}, Schema: modelSchema}
	msg := GetModelInfo(r, modelSchema, getSelect(args.Sel, args.Rel), &args.Info, args, config)
	if msg != nil {
		return msg
	}

	if len(args.Primaries) > 0 {
		newPrimaries := map[string]interface{}{}
		for key, val := range args.Primaries {
			newPrimaries[args.Info.Table+"."+key] = val
		}
		args.Primaries = newPrimaries
	}

	conds := params.Conditions{Nested: map[string]*params.Conditions{}}
	err = params.ToStmt(r, args.Params, args.P, modelSchema, args.Info.Table, &conds, config.P)
	if err != nil {
		return err
	}
	if !config.SkipValidation {
		if err := validateRelations(r, modelSchema, args.Info.Nested); err != nil {
			return err
		}
		if err := validateRelations(r, modelSchema, conds.Nested); err != nil {
			return err
		}
	}

	if args.Ord != "" && config.Ord != nil {
		orders := strings.Split(args.Ord, ",")
		order := []string{}
		for _, b := range orders {
			bSplit := strings.Split(b, " ")
			_, ok := config.Ord[bSplit[0]]
			if ok {
				order = append(order, b)
			}
		}
		args.Ord = strings.Join(order, ",")
	}

	args.Result = []map[string]any{}
	err = QueryRecursive(r, db, args, config, &args.Info, &conds, &args.Result)
	if err != nil {
		return err
	}

	if !ShouldPaginate(args.PagStart, args.PagEnd) {
		args.Count = int64(len(args.Result))
	}

	if len(args.Primaries) != 0 && args.Count == 0 {
		return message.ItemNotFound(r)
	}

	return nil
}

func QueryRecursive(r *http.Request, db *gorm.DB, args *QueryArgs, config QueryConfig, info *ModelInfo, conds *params.Conditions, result *[]map[string]any) error {
	tx := db.Select(strings.Join(info.Select, ","), info.SelectArgs...)
	tx.Statement.Distinct = info.Distinct
	if info.Aggregate {
		for _, field := range info.Select {
			if !strings.HasPrefix(field, "SUM(") && !strings.HasPrefix(field, "COUNT(") {
				tx = tx.Group(field)
			}
		}
	}

	if tx.Statement.Table == "" {
		table := info.Schema.Table
		if table != info.Table && !strings.Contains(table, ") AS ") {
			table += " AS " + info.Table
		}
		tx.Table(table)
	}

	if !config.SkipDefaults {
		mdl := reflect.New(info.Schema.ModelType).Interface()
		// Handles default conditions
		if model, ok := mdl.(model.ConditionsModel); ok {
			query, args := model.DefaultConditions(tx, info.Table)
			tx.Where(query, args...)
		}

		if model, ok := mdl.(model.JoinsModel); ok {
			tx.Joins(model.DefaultJoins(tx, info.Table))
		}
	}

	if len(conds.Query) > 0 {
		tx.Where(conds.Query, conds.Args...)
	}

	JoinRelations(r, tx, config, info, RelationsFromModelInfo(info, conds.Nested))

	var pagination bool
	if args == nil {
		tx = tx.Order(info.Order)
	} else {
		pagination = ShouldPaginate(args.PagStart, args.PagEnd)

		order := info.Order
		if len(args.Primaries) == 0 {
			// order := Order(args.Ord, db, args.Model, info)
			// if len(args.Ord) > 0 {
			// 	var msg message.Message
			// 	order, msg = ParseOrder(c, order, info)
			// 	if msg != nil {
			// 		return msg
			// 	}
			// }

			if info.Aggregate && pagination {
				return message.ConflictingPaginationAndAggregation(r)
			}
			if info.Distinct && args.Ord == "" {
				order = ""
			}
			if len(order) > 0 {
				if !info.Aggregate && pagination {
					tx = tx.Scopes(Count(&args.Count)).Scopes(Paginate(args.PagStart, args.PagEnd))
				}
				tx = tx.Order(order)
			} else if pagination {
				return message.ManualPagination(r)
			}
		} else {
			tx = tx.Where(args.Primaries)
		}
	}
	if r.URL.Query().Get("SUM") == "1" {
		n := clause.OrderBy{}.Name()
		// ord := tx.Statement.Clauses[n]
		delete(tx.Statement.Clauses, n)
		tx = tx.Session(&gorm.Session{NewDB: true}).Table(`(?) AS T`, tx)
		fields := make([]string, len(info.Fields))
		for i, field := range info.Fields {
			t := field.Type.String()
			if t == "int" || t == "*int" || t == "float64" || t == "*float64" {
				fields[i] = "SUM(T." + field.Name + ")"
			}
		}
		tx.Select(fields)
		//	tx.Statement.Clauses[n] = ord
	}

	rows, err := tx.Rows()
	if err != nil {
		return err
	}
	defer rows.Close()

	// Add the foreign key to the fields for now
	if len(info.Select) != len(info.Fields) {
		info.Fields = append([]reflect.StructField{{
			Name: fkAlias,
			Type: reflect.TypeOf(""),
		}}, info.Fields...)
	}

	for rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		rowFields := make([]any, len(info.Fields))
		for i := 0; i < len(rowFields); i++ {
			// t := info.Fields[i].Type
			// if t.Kind() == reflect.Pointer {
			// 	t = t.Elem()
			// }
			// switch t.Kind() {
			// case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
			// 	rowFields[i] = new(int)
			// case reflect.Float32, reflect.Float64:
			// 	rowFields[i] = new(float64)
			// case reflect.String:
			// 	rowFields[i] = new(string)
			// default:
			// 	rowFields[i] = reflect.New(t).Interface()
			// }

			t := info.Fields[i].Type
			if t.Kind() != reflect.Pointer {
				t = reflect.PointerTo(t)
			}
			rowFields[i] = reflect.New(t).Interface()
		}

		if err := rows.Scan(rowFields...); err != nil {
			return err
		}

		rowMap := make(map[string]any, len(info.Fields))
		for i := 0; i < len(rowFields); i++ {
			rowMap[info.Fields[i].Name] = reflect.ValueOf(rowFields[i]).Elem().Interface()
		}

		*result = append(*result, rowMap)
	}

	// Computed fields

	for _, computedField := range info.ComputedFields {
		for i := range *result {
			item := (*result)[i]
			msg := computedField.Fn(r, db, info, &item)
			if msg != nil {
				return msg
			}
		}
	}

	//

	type SetContainer struct {
		keyMap map[string][]int
		keySet any
	}

	containers := map[string]SetContainer{}

	for relName, rel := range info.Nested {
		nestedConds := &params.Conditions{}
		if conds.Nested != nil {
			if c, ok := conds.Nested[relName]; ok && (c.Type == "N" || c.Type == "M") {
				nestedConds = c
			}
		}

		keys := make([]string, len(rel.References))
		for i, ref := range rel.References {
			keys[i] = ref.PrimaryKey.Name
		}

		fieldKey := strings.Join(keys, "___")

		var container SetContainer
		if cont, ok := containers[fieldKey]; ok {
			container = cont
		} else {
			container = SetContainer{
				keyMap: map[string][]int{},
			}
			if len(rel.References) == 1 {
				container.keySet = map[string]struct{}{}
			} else {
				container.keySet = map[string]any{}
			}
			for i, res := range *result {
				keys := make([]string, len(rel.References))
				var keysValid bool
				for i, ref := range rel.References {
					var key string
					// //possibile problema con gli int32
					// intVal, ok := r[ref.PrimaryKey.Name].(*int)
					// if ok && intVal != nil {
					// 	val = strconv.Itoa(*intVal)
					// }
					val := reflect.ValueOf(res[ref.PrimaryKey.Name])
					if val.IsValid() {
						if !val.IsZero() {
							key = fmt.Sprint(val.Elem().Interface())
						}
					} else {
						// The field isn't defined in the map; return an error
						return message.MissingForeignKey(r, ref.PrimaryKey.Name, relName)
					}
					keys[i] = key
					if key != "" {
						keysValid = true
					}
				}
				if keysValid {
					// Generate the keySet to use it in the next query as a filter
					var ks any = container.keySet
					for i, key := range keys {
						if i == len(keys)-1 {
							ks.(map[string]struct{})[key] = struct{}{}
						} else if i == len(keys)-2 {
							current := ks.(map[string]any)
							if _, ok := current[key]; !ok {
								m := map[string]struct{}{}
								ks.(map[string]any)[key] = m
							}
							ks = current[key]
						} else {
							current := ks.(map[string]any)
							if _, ok := current[key]; !ok {
								m := map[string]any{}
								current[key] = m
							}
							ks = current[key]
						}
					}
					// Generate the keyMap containing row references divided by key
					valueKey := strings.Join(keys, "___")
					if _, ok := container.keyMap[valueKey]; !ok {
						container.keyMap[valueKey] = []int{}
					}
					container.keyMap[valueKey] = append(container.keyMap[valueKey], i)
				}
			}
			containers[fieldKey] = container
		}

		index := strings.LastIndex(relName, ".")
		if index != -1 {
			relName = relName[index+1:]
		}

		for i := range *result {
			if rel.Slice {
				(*result)[i][relName] = []map[string]any{}
			} else {
				(*result)[i][relName] = nil
			}
		}

		// Proceed only if there is at least one valid keyMap/condition
		if len(container.keyMap) != 0 {
			if nestedConds.Query != "" {
				nestedConds.Query += " AND "
			}
			nestedConds.Query += keySetToStr(rel.ModelInfo.Table, rel.References, container.keySet)
			rows := []map[string]any{}
			err := QueryRecursive(r, db, nil, config, rel.ModelInfo, nestedConds, &rows)
			if err != nil {
				return err
			}
			for i, row := range rows {
				for _, index := range container.keyMap[*row[fkAlias].(*string)] {
					delete(row, fkAlias)
					if rel.Slice {
						(*result)[index][relName] = append((*result)[index][relName].([]map[string]any), rows[i])
					} else {
						(*result)[index][relName] = rows[i]
					}
				}
			}
		}
	}

	return nil
}

func getSelect(sel, rel string) string {
	if sel == "" {
		sel = "*"
	}
	if rel != "" {
		keys := map[string]struct{}{}
		rels := strings.Split(rel, ",")
		for _, r := range rels {
			pieces := strings.Split(r, ".")
			var key string
			for i, piece := range pieces {
				prev := key
				if i > 0 {
					key += "."
				}
				key += ">" + piece
				if _, ok := keys[key]; !ok {
					keys[key] = struct{}{}
					sel += ","
					if i > 0 {
						sel += prev + "."
					}
					sel += ">" + piece + ".*"
				}
			}
		}
	}
	return sel
}

func validateRelations[T params.NextNesterer[T]](r *http.Request, modelSchema *schema.Schema, relations map[string]T) message.Message {
	for key, nested := range relations {
		pieces := strings.Split(key, ".")
		relSchema := modelSchema
		for _, piece := range pieces {
			rel, ok := relSchema.Relationships.Relations[piece]
			if ok {
				relSchema = rel.FieldSchema
				if msg := permissions.Get(reflect.New(rel.FieldSchema.ModelType).Interface())(r); msg != nil {
					return message.UnauthorizedRelations(r, piece).Add(msg)
				}
			} else {
				return message.InvalidRelation(r, piece)
			}
		}
		if nestedRels := nested.NextNested(); len(nestedRels) > 0 {
			if _, ok := nestedRels[key]; ok && len(nestedRels) == 1 {
				nestedRels = nestedRels[key].NextNested()
			}
			if msg := validateRelations(r, relSchema, nestedRels); msg != nil {
				return msg
			}
		}
	}
	return nil
}

func keySetToStr(table string, refs []*schema.Reference, vals any) string {
	result := ""
	if len(refs) == 1 {
		keySet := vals.(map[string]struct{})
		keys := make([]string, 0, len(keySet))
		var hasNull bool
		for key := range keySet {
			if key == "" {
				hasNull = true
			} else {
				keys = append(keys, strings.ReplaceAll(key, "'", "''"))
			}
		}

		field := table + "." + refs[0].ForeignKey.DBName
		if len(keys) != 0 {
			if hasNull {
				result += "("
			}
			result += field + " IN ('" + strings.Join(keys, "','") + "')"
		}
		if hasNull {
			if len(keys) != 0 {
				result += " OR "
			}
			result += field + " IS NULL"
			if len(keys) != 0 {
				result += ")"
			}
		}
	} else {
		m := vals.(map[string]any)
		if len(m) > 1 {
			result += "("
		}
		first := true
		for k, v := range m {
			if first {
				first = false
			} else {
				result += ") OR ("
			}
			result += table + "." + refs[0].ForeignKey.DBName + " = " + k + " AND " + keySetToStr(table, refs[1:], v)
		}
		if len(m) > 1 {
			result += ")"
		}
	}
	return result
}

func parseSel(sel string) string {
	regex := regexp.MustCompile(`[\n\r\t\s]+`)
	var distinct bool
	if strings.HasPrefix(sel, "DISTINCT ") {
		distinct = true
		sel = strings.TrimPrefix(sel, "DISTINCT ")
	}
	sel = strings.ReplaceAll(sel, " AS ", "#AS#")
	sel = regex.ReplaceAllString(sel, "")
	sel = strings.ReplaceAll(sel, "#AS#", " AS ")
	if distinct {
		sel = "DISTINCT " + sel
	}
	return sel
}
