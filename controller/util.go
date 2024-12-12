package controller

import (
	"api_core/message"
	"api_core/model"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strconv"

	"github.com/go-chi/chi"
	"github.com/go-playground/validator/v10"
)

func GetPathParams(r *http.Request, model interface{}, fields []string, destination interface{}) error {
	msg := GetPathParamsMsg(r, model, fields, destination)
	if msg != nil {
		return msg
	}
	return nil
}

func GetPathParamsMsg(r *http.Request, model interface{}, fields []string, destination interface{}) message.Message {
	dest := reflect.ValueOf(destination).Elem()
	mdl := reflect.ValueOf(model).Elem()
	for _, field := range fields {
		_, found := mdl.Type().FieldByName(field)
		val := chi.URLParam(r, field)
		if len(val) == 0 || !found {
			return message.InvalidUrlParameter(r, field)
		}

		msg := assignValue(r, mdl.FieldByName(field), field, val, dest)
		if msg != nil {
			return msg
		}
	}
	return nil
}

func assignValue(r *http.Request, mdlField reflect.Value, field, val string, dest reflect.Value) message.Message {
	switch mdlField.Interface().(type) {
	case int:
		{
			intVal, err := strconv.Atoi(val)
			if err != nil {
				return message.InvalidParamType(r, field, "int")
			}
			applyValue(r, dest, field, intVal)
		}
	case int16:
		{
			intVal, err := strconv.Atoi(val)
			if err != nil {
				return message.InvalidParamType(r, field, "int")
			}
			applyValue(r, dest, field, int16(intVal))
		}
	case int32:
		{
			intVal, err := strconv.Atoi(val)
			if err != nil {
				return message.InvalidParamType(r, field, "int")
			}
			applyValue(r, dest, field, int32(intVal))
		}
	case int64:
		{
			intVal, err := strconv.ParseInt(val, 10, 64)
			if err != nil {
				return message.InvalidParamType(r, field, "int")
			}
			applyValue(r, dest, field, intVal)
		}
	case *int:
		{
			intVal, err := strconv.Atoi(val)
			if err != nil {
				return message.InvalidParamType(r, field, "int")
			}
			applyValue(r, dest, field, &intVal)
		}
	case *int16:
		{
			intVal, err := strconv.Atoi(val)
			if err != nil {
				return message.InvalidParamType(r, field, "int")
			}
			i := int16(intVal)
			applyValue(r, dest, field, &i)
		}
	case *int32:
		{
			intVal, err := strconv.Atoi(val)
			if err != nil {
				return message.InvalidParamType(r, field, "int")
			}
			i := int32(intVal)
			applyValue(r, dest, field, &i)
		}
	case *int64:
		{
			intVal, err := strconv.ParseInt(val, 10, 64)
			if err != nil {
				return message.InvalidParamType(r, field, "int")
			}
			applyValue(r, dest, field, &intVal)
		}
	case string:
		{
			applyValue(r, dest, field, val)
		}
	default:
		{
			return message.UnsupportedParamType(r, mdlField.Type().Name())
		}
	}
	return nil
}

func applyValue(r *http.Request, destination reflect.Value, field string, value interface{}) message.Message {
	switch destination.Type().Kind() {
	case reflect.Map:
		{
			destination.SetMapIndex(reflect.ValueOf(field), reflect.ValueOf(value))
		}
	case reflect.Struct:
		{
			destination.FieldByName(field).Set(reflect.ValueOf(value))
		}
	default:
		{
			return message.InternalServerError(r)
		}
	}
	return nil
}

func LoadModel(r *http.Request, jsonData []byte, model interface{}) error {
	err := json.Unmarshal(jsonData, model)

	if err != nil {
		return message.InvalidJSON(r).Text(err.Error())
	}
	return nil
}

func ValidateStruct(r *http.Request, mdl interface{}) error {
	if validationModel, ok := mdl.(model.ValidationModel); ok {
		if msg := validationModel.Validate(r); msg != nil {
			return msg
		}
	}
	validate := validator.New()
	err := validate.Struct(mdl)
	if err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			return nil
		}

		campiObbligatori := ""
		for _, err := range err.(validator.ValidationErrors) {
			campiObbligatori += err.Field() + " " + err.ActualTag() + " " + err.Param()
		}

		return errors.New(campiObbligatori)
	}
	return nil
}

func ValidateModel(r *http.Request, model interface{}) error {
	err := ValidateStruct(r, model)
	if err != nil {
		return message.Unprocessable(r).Text(err.Error())
	}
	return nil
}

func ValidateModels(r *http.Request, models interface{}) error {
	modelsSlice := reflect.Indirect(reflect.ValueOf(models))

	typ := modelsSlice.Type()
	if typ.Kind() != reflect.Slice {
		return message.ExpectedSlice(r)
	}
	res := ""
	for i := 0; i < modelsSlice.Len(); i++ {
		err := ValidateStruct(r, modelsSlice.Index(i).Interface())
		if err != nil {
			res += "Riga " + strconv.Itoa(i) + ": " + err.Error() + "\n"
		}
	}

	if len(res) > 0 {
		return message.Unprocessable(r).Text(res)
	}
	return nil
}

func validateVar(r *http.Request, value interface{}, rules string, field string) message.Message {
	validate := validator.New()
	err := validate.Var(value, rules)
	if err != nil {
		return message.InvalidFieldValue(r, field, rules, value)
	}
	return nil
}

func ValidateMap(r *http.Request, jsonMap map[string]interface{}, modelType reflect.Type) []error {
	var errors []error
	for key, value := range jsonMap {
		field, found := modelType.FieldByName(key)
		if found {
			rules := field.Tag.Get("validate")
			msg := validateVar(r, value, rules, field.Name)
			if msg != nil {
				errors = append(errors, msg)
			}
		} else {
			delete(jsonMap, key)
		}
	}
	return errors
}

func ValidateMaps(r *http.Request, jsonMaps []map[string]interface{}, modelType reflect.Type) error {
	var err string
	for i, jsonMap := range jsonMaps {
		var errString string
		errors := ValidateMap(r, jsonMap, modelType)
		for _, err := range errors {
			errString += err.Error() + "\n"
		}

		if errString != "" {
			err += "Riga " + strconv.Itoa(i) + ":\n" + errString
		}
	}
	if err == "" {
		return nil
	}
	return fmt.Errorf(err)
}

func LoadAndValidateMap(r *http.Request, jsonData []byte, jsonMap map[string]interface{}, modelType reflect.Type) error {
	msg := LoadModel(r, jsonData, &jsonMap)
	if msg != nil {
		return msg
	}

	errors := ValidateMap(r, jsonMap, modelType)
	var errString string
	for _, err := range errors {
		errString += err.Error()
	}

	if len(jsonMap) == 0 {
		return message.Unprocessable(r)
	}

	if len(errors) > 0 {
		return message.Conflict(r).Text(errString)
	}

	return nil
}

func LoadAndValidateMaps(r *http.Request, jsonData []byte, jsonMaps *[]map[string]interface{}, modelType reflect.Type) error {
	err := LoadModel(r, jsonData, jsonMaps)
	if err != nil {
		return err
	}
	err = ValidateMaps(r, *jsonMaps, modelType)
	if err != nil {
		return message.Conflict(r).Text(err.Error())
	}
	if len(*jsonMaps) == 0 {
		return message.Unprocessable(r)
	}
	return nil
}

func ValidateMapsPrimaries(r *http.Request, jsonMaps []map[string]interface{}, primaryKeys []string) error {
	var err string
	for i, jsonMap := range jsonMaps {
		var errString string
		for _, field := range primaryKeys {
			if jsonMap[field] == nil {
				errString += message.InvalidFieldRequired(r, field).Error() + "\n"
			}
		}
		if errString != "" {
			err += message.RowError(r, i, "\n"+errString).Error()
		}
	}

	if err != "" {
		return message.Conflict(r).Text(err)
	}
	return nil
}

func GetMapKeys(mapToFlatten map[string]interface{}) []string {
	var keys []string
	for k := range mapToFlatten {
		keys = append(keys, k)
	}
	return keys
}
