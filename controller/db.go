package controller

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"api_core/message"
	"api_core/model"
	"api_core/permissions"
	"api_core/request"
	"api_core/response"
	"api_core/utils"

	"github.com/go-chi/render"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type MSSqlError interface {
	Error() string
	SQLErrorClass() uint8
	SQLErrorLineNo() int32
	SQLErrorMessage() string
	SQLErrorNumber() int32
	SQLErrorProcName() string
	SQLErrorServerName() string
	SQLErrorState() uint8
}

func CreateToDb(w http.ResponseWriter, r *http.Request, db *gorm.DB, model interface{}, args ...string) error {
	db = db.Session(&gorm.Session{CreateBatchSize: 25})

	modelSchema, err := schema.Parse(model, &sync.Map{}, db.NamingStrategy)
	if err != nil {
		return message.InternalServerError(r)
	}

	modelsSlice := reflect.Indirect(reflect.ValueOf(model))
	if modelsSlice.Type().Kind() == reflect.Slice {
		checked := map[string]struct{}{}
		l := modelsSlice.Len()
		for i := 0; i < l; i++ {
			msg := permissions.CheckModel(r, modelsSlice.Index(i), modelSchema, checked, false)
			if msg != nil {
				return msg
			}
		}
	} else {
		checked := map[string]struct{}{}
		msg := permissions.CheckModel(r, modelsSlice, modelSchema, checked, false)
		if msg != nil {
			return msg
		}
	}
	tx := db.Session(&gorm.Session{SkipDefaultTransaction: true}).Begin()
	tx = tx.Create(model)
	if tx.Error != nil {
		tx.Rollback()
		return message.Conflict(r).Text(tx.Error.Error())
	} else {
		tx.Commit()
	}

	if len(args) == 0 {
		response.JSON(w, r, model)
	}
	return nil
}

func UpdateToDb(w http.ResponseWriter, r *http.Request, model interface{}, values any) error {
	db := request.DB(r).Session(&gorm.Session{CreateBatchSize: 50})

	modelSchema, err := schema.Parse(model, &sync.Map{}, db.NamingStrategy)
	if err != nil {
		return message.InternalServerError(r)
	}

	modelsSlice := reflect.Indirect(reflect.ValueOf(model))
	if modelsSlice.Type().Kind() == reflect.Slice {
		return errors.New("not supported")
	} else {
		checked := map[string]struct{}{}
		msg := permissions.CheckModel(r, modelsSlice, modelSchema, checked, true)
		if msg != nil {
			return msg
		}
	}

	tx := db.Session(&gorm.Session{FullSaveAssociations: true, SkipDefaultTransaction: true}).Begin()
	v := reflect.ValueOf(model)
	err = DeleteRelations(r, tx, v, modelSchema)
	if err != nil {
		tx.Rollback()
		return err
	}
	if tx.Error != nil {
		tx.Rollback()
		return err
	}
	tx = tx.Model(model).Updates(values)
	if tx.Error != nil {
		tx.Rollback()
		return err
	} else {
		tx.Commit()
	}
	response.JSON(w, r, model)
	return nil
}

func DeleteFromDb(r *http.Request, models []any) error {
	if len(models) == 0 {
		return nil
	}

	db := request.DB(r)
	tx := db.Begin()

	modelSchema, err := schema.Parse(models[0], &sync.Map{}, db.NamingStrategy)
	if err != nil {
		return message.InternalServerError(r)
	}

	for _, mdl := range models {
		tx := tx.Session(&gorm.Session{SkipDefaultTransaction: true})

		if condMdl, ok := mdl.(model.ConditionsModel); ok {
			query, args := condMdl.DefaultConditions(db, modelSchema.Table)
			if query != "" {
				tx = tx.Where("("+query+")", args...)
			}
		}

		LoadForeignKeys(tx, reflect.ValueOf(mdl), modelSchema)
		res := tx.Delete(mdl)
		if res.Error != nil {
			tx.Rollback()
			return res.Error
		}
	}

	tx.Commit()

	render.Status(r, http.StatusOK)
	return nil
}

func DeleteRelations(r *http.Request, db *gorm.DB, modelVal reflect.Value, modelSchema *schema.Schema) error {
	relSchema := modelSchema
	relArr := relSchema.Relationships.HasOne
	relArr = append(relArr, relSchema.Relationships.HasMany...)
	for _, rel := range relArr {
		if !rel.Field.Updatable || !modelVal.Elem().IsValid() {
			continue
		}
		slice := modelVal.Elem().FieldByName(rel.Field.Name)
		if rel.Type == "has_one" {
			if slice.IsZero() {
				continue
			}
			typ := slice.Type()
			if typ.Kind() != reflect.Ptr {
				typ = reflect.PointerTo(typ)
				slice = slice.Addr()
			}
			slice = reflect.Append(reflect.New(reflect.SliceOf(typ)).Elem(), slice)
		}
		if !slice.IsNil() && slice.Type().Kind() == reflect.Slice {
			// Find any necessary foreign keys to delete relationships
			fkMap := map[string]struct{}{}
			relArr := rel.FieldSchema.Relationships.HasOne
			relArr = append(relArr, rel.FieldSchema.Relationships.HasMany...)
			for _, r := range relArr {
				if !r.Field.Updatable {
					continue
				}
				for _, ref := range r.References {
					fkMap[ref.PrimaryKey.DBName] = struct{}{}
				}
			}
			for _, p := range rel.FieldSchema.PrimaryFieldDBNames {
				delete(fkMap, p)
			}
			fkFields := []string{}
			for k := range fkMap {
				fkFields = append(fkFields, k)
			}
			for i := slice.Len() - 1; i >= 0; i-- {
				item := slice.Index(i)
				relVal := item
				if item.Type().Kind() == reflect.Ptr {
					item = item.Elem()
				} else {
					relVal = relVal.Addr()
				}
				err := DeleteRelations(r, db, relVal, rel.FieldSchema)
				if err != nil {
					return err
				}

				deleteField := item.FieldByName("Delete")
				if deleteField.IsValid() && deleteField.Bool() {
					// Loads the missing foreign keys
					if len(fkFields) > 0 {
						res := db.Select(fkFields).Find(relVal.Interface())
						// If the record is not found in the database, it means it has already been deleted, so move on to the next
						if res.RowsAffected == 0 {
							continue
						}
					}
					LoadForeignKeys(db, relVal, rel.FieldSchema)
					result := db.Delete(relVal.Interface())
					if result.Error != nil {
						return result.Error
					}
					if rel.Type != "has_one" {
						slice.Index(i).Set(slice.Index(slice.Len() - 1))
						slice.Set(slice.Slice(0, slice.Len()-1))
					}
				}
			}
		}
	}
	return nil
}

func LoadForeignKeys(db *gorm.DB, modelVal reflect.Value, modelSchema *schema.Schema) {
	keySet := map[string]struct{}{}
	for key, rel := range modelSchema.Relationships.Relations {
		if rel.Field.Updatable && !strings.HasPrefix(key, "_") {
			for _, ref := range rel.References {
				fieldName := ref.ForeignKey.Name
				if ref.OwnPrimaryKey {
					fieldName = ref.PrimaryKey.Name
				}
				fld := modelVal.Elem().FieldByName(fieldName)
				if !fld.IsValid() || fld.IsZero() {
					keySet[fieldName] = struct{}{}
				}
			}
		}
	}
	keyArr := []string{}
	for k := range keySet {
		keyArr = append(keyArr, k)
	}
	if len(keyArr) > 0 {
		db.Session(&gorm.Session{NewDB: true}).Select(keyArr).Find(modelVal.Interface())
	}
}

type Relation struct {
	Label      string
	Model      model.TableModel
	ForeignKey string
}

/*
CheckUnique performs a uniqueness check on a specific resource.
*/
func CheckUnique(r *http.Request, db *gorm.DB, model interface{}, primary string, fields []string) message.Message {
	val := reflect.ValueOf(model)
	var cond string
	args := []interface{}{}
	for _, f := range fields {
		if cond != "" {
			cond += " AND "
		}
		cond += f + " = ?"
		args = append(args, val.Elem().FieldByName(f).Interface())
	}

	tx := db.Model(model).Select(primary).Where(cond, args...)
	if f := val.Elem().FieldByName(primary); f.IsValid() {
		tx.Where(primary+" != ?", f.Interface())
	}
	var count int64
	tx.Count(&count)
	if count > 0 {
		str := ""
		for i := 0; i < len(args); i++ {
			str += fields[i] + " " + fmt.Sprint(reflect.Indirect(reflect.ValueOf(args[i])).Interface())
		}
		return message.DuplicateUnique(r, tx.Statement.Table, str)
	} else {
		return nil
	}
}

/*
CheckRelations checks if the resource can be deleted.
This method verifies whether the resource to be deleted has been used in any other user-provided relationships.
If the resource is in use, it returns the controllers that use the resource; otherwise, it returns nothing in case of success.
*/
func CheckRelations(r *http.Request, db *gorm.DB, id interface{}, relations ...Relation) message.Message {
	errors := []string{}
	for _, rel := range relations {
		errors = append(errors, CheckRelatedModel(db, rel.Model, rel.Label, "", rel.ForeignKey+" = ?", id)...)
	}
	if len(errors) > 0 {
		return message.DeleteFailed(r, errors)
	}
	return nil
}

func CheckRelatedModel(db *gorm.DB, model any, label string, field string, where string, whereArgs ...any) []string {
	db = db.Session(&gorm.Session{NewDB: true})
	errors := []string{}
	if len(label) == 0 {
		label = reflect.TypeOf(model).Elem().Name()
	}
	if len(field) == 0 {
		field, _ = utils.GetPrimaryName(model)
	}
	if len(field) == 0 {
		// No specified field to show or primary key present
		var res int64
		result := db.Model(model).Where(where, whereArgs).Count(&res)
		if result.Error == nil {
			if res > 0 {
				errors = append(errors, label)
			}
		} else {
			errors = append(errors, "Invalid model")
		}
	} else {
		// Has specified field to show or primary key present
		var res []string
		result := db.Model(model).Select(field).Where(where, whereArgs).Scan(&res)
		if result.Error == nil {
			if len(res) > 0 {
				errors = append(errors, label+": "+strings.Join(res, ", "))
			}
		} else {
			errors = append(errors, "Invalid model")
		}
	}
	return errors
}

func CheckQueryRelation(relName string, query *gorm.DB) []string {
	errors := []string{}
	var res []int
	result := query.Scan(&res)
	if result.Error == nil {
		if len(res) > 0 {
			for _, r := range res {
				errors = append(errors, relName+" "+strconv.Itoa(r))
			}
		}
	} else {
		errors = append(errors, "Invalid relation")
	}
	return errors
}

func ErrorsToMsg(r *http.Request, errors []string) message.Message {
	if len(errors) > 0 {
		return message.DeleteFailed(r, errors)
	}
	return nil
}

func DeleteModels(db *gorm.DB, models interface{}) (err error) {
	var wg sync.WaitGroup
	val := reflect.ValueOf(models).Elem()
	l := val.Len()
	for i := 0; i < l; i++ {
		wg.Add(1)
		go func(i int) {
			w := db.Statement.Context.Value("w").(http.ResponseWriter)
			r := db.Statement.Context.Value("r").(*http.Request)
			defer request.RecoverIfEnabled(w, r)
			// TODO: LoadForeignKeys should be placed here
			res := db.Delete(val.Index(i).Addr().Interface())
			if res.Error != nil {
				err = res.Error
			}
			wg.Done()
		}(i)
	}
	wg.Wait()
	return
}
