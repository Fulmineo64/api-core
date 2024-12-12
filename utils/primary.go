package utils

import (
	"errors"
	"reflect"
	"strings"

	"gorm.io/gorm/schema"
	"gorm.io/gorm/utils"
)

func GetPrimaryName(model interface{}) (string, error) {
	val := reflect.Indirect(reflect.ValueOf(model))
	typ := val.Type()
	num := typ.NumField()
	for i := 0; i < num; i++ {
		field := typ.Field(i)
		if field.Anonymous && field.Name != "BaseModel" && field.Name != "Timestamps" {
			if key, err := GetPrimaryName(val.Field(i).Interface()); err == nil {
				return key, nil
			}
		}
		tag := field.Tag.Get("gorm")
		if strings.Contains(tag, "primaryKey") {
			return field.Name, nil
		}
	}
	return "", errors.New("primary key not found")
}

func GetPrimary(model interface{}) (int, error) {
	values := reflect.Indirect(reflect.ValueOf(model))
	key, err := GetPrimaryName(model)
	if err != nil {
		return 0, err
	} else {
		return int(values.FieldByName(key).Int()), nil
	}
}

func SetPrimary(model interface{}, value int) error {
	key, err := GetPrimaryName(model)
	if err != nil {
		return err
	}
	val := reflect.Indirect(reflect.ValueOf(model))
	val.FieldByName(key).SetInt(int64(value))
	return nil
}

func GetPrimaryFields(modelType reflect.Type) []string {
	primaryFields := []string{}
	numFields := modelType.NumField()
	for i := 0; i < numFields; i++ {
		fieldStruct := modelType.Field(i)
		if fieldStruct.Anonymous {
			primaryFields = append(primaryFields, GetPrimaryFields(fieldStruct.Type)...)
		} else {
			tagSettings := schema.ParseTagSetting(fieldStruct.Tag.Get("gorm"), ";")
			if val, ok := tagSettings["PRIMARYKEY"]; ok && utils.CheckTruth(val) {
				primaryFields = append(primaryFields, fieldStruct.Name)
			}
		}
	}
	return primaryFields
}
