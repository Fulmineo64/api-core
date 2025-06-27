package controller

import (
	"api_core/message"
	"api_core/model"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

func GetPathParams(c *gin.Context, model interface{}, fields []string, destination interface{}) error {
	msg := GetPathParamsMsg(c, model, fields, destination)
	if msg != nil {
		return msg
	}
	return nil
}

func GetPathParamsMsg(c *gin.Context, model interface{}, fields []string, destination interface{}) message.Message {
	dest := reflect.ValueOf(destination).Elem()
	mdl := reflect.ValueOf(model)
	for _, field := range fields {
		_, found := mdl.Type().FieldByName(field)
		val := c.Param(field)
		if len(val) == 0 || !found {
			return message.InvalidUrlParameter(c, field)
		}

		msg := assignValue(c, mdl.FieldByName(field), field, val, dest)
		if msg != nil {
			return msg
		}
	}
	return nil
}

func assignValue(c *gin.Context, mdlField reflect.Value, field, val string, dest reflect.Value) message.Message {
	switch mdlField.Interface().(type) {
	case int:
		{
			intVal, err := strconv.Atoi(val)
			if err != nil {
				return message.InvalidParamType(c, field, "int")
			}
			applyValue(c, dest, field, intVal)
		}
	case int16:
		{
			intVal, err := strconv.Atoi(val)
			if err != nil {
				return message.InvalidParamType(c, field, "int")
			}
			applyValue(c, dest, field, int16(intVal))
		}
	case int32:
		{
			intVal, err := strconv.Atoi(val)
			if err != nil {
				return message.InvalidParamType(c, field, "int")
			}
			applyValue(c, dest, field, int32(intVal))
		}
	case int64:
		{
			intVal, err := strconv.ParseInt(val, 10, 64)
			if err != nil {
				return message.InvalidParamType(c, field, "int")
			}
			applyValue(c, dest, field, intVal)
		}
	case *int:
		{
			intVal, err := strconv.Atoi(val)
			if err != nil {
				return message.InvalidParamType(c, field, "int")
			}
			applyValue(c, dest, field, &intVal)
		}
	case *int16:
		{
			intVal, err := strconv.Atoi(val)
			if err != nil {
				return message.InvalidParamType(c, field, "int")
			}
			i := int16(intVal)
			applyValue(c, dest, field, &i)
		}
	case *int32:
		{
			intVal, err := strconv.Atoi(val)
			if err != nil {
				return message.InvalidParamType(c, field, "int")
			}
			i := int32(intVal)
			applyValue(c, dest, field, &i)
		}
	case *int64:
		{
			intVal, err := strconv.ParseInt(val, 10, 64)
			if err != nil {
				return message.InvalidParamType(c, field, "int")
			}
			applyValue(c, dest, field, &intVal)
		}
	case string:
		{
			applyValue(c, dest, field, val)
		}
	default:
		{
			return message.UnsupportedParamType(c, mdlField.Type().Name())
		}
	}
	return nil
}

func applyValue(c *gin.Context, destination reflect.Value, field string, value interface{}) message.Message {
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
			return message.InternalServerError(c)
		}
	}
	return nil
}

func LoadModel(c *gin.Context, jsonData []byte, model interface{}) error {
	err := json.Unmarshal(jsonData, model)

	if err != nil {
		return message.InvalidJSON(c).Text(err.Error())
	}
	return nil
}

func ValidateStruct(c *gin.Context, mdl interface{}) error {
	if validationModel, ok := mdl.(model.ValidationModel); ok {
		if msg := validationModel.Validate(c); msg != nil {
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

func ValidateModel(c *gin.Context, model interface{}) error {
	err := ValidateStruct(c, model)
	if err != nil {
		return message.Unprocessable(c).Text(err.Error())
	}
	return nil
}

func ValidateModels(c *gin.Context, models interface{}) error {
	modelsSlice := reflect.Indirect(reflect.ValueOf(models))

	typ := modelsSlice.Type()
	if typ.Kind() != reflect.Slice {
		return message.ExpectedSlice(c)
	}
	res := ""
	for i := 0; i < modelsSlice.Len(); i++ {
		err := ValidateStruct(c, modelsSlice.Index(i).Interface())
		if err != nil {
			res += "Riga " + strconv.Itoa(i) + ": " + err.Error() + "\n"
		}
	}

	if len(res) > 0 {
		return message.Unprocessable(c).Text(res)
	}
	return nil
}

func validateVar(c *gin.Context, value interface{}, rules string, field string) message.Message {
	validate := validator.New()
	err := validate.Var(value, rules)
	if err != nil {
		return message.InvalidFieldValue(c, field, rules, value)
	}
	return nil
}

func ValidateMap(c *gin.Context, jsonMap map[string]interface{}, modelType reflect.Type) []error {
	var errors []error
	for key, value := range jsonMap {
		field, found := modelType.FieldByName(key)
		if found {
			rules := field.Tag.Get("validate")
			msg := validateVar(c, value, rules, field.Name)
			if msg != nil {
				errors = append(errors, msg)
			}
		} else {
			delete(jsonMap, key)
		}
	}
	return errors
}

func ValidateMaps(c *gin.Context, jsonMaps []map[string]interface{}, modelType reflect.Type) error {
	var err string
	for i, jsonMap := range jsonMaps {
		var errString string
		errors := ValidateMap(c, jsonMap, modelType)
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
	return errors.New(err)
}

func LoadAndValidateMap(c *gin.Context, jsonData []byte, jsonMap map[string]interface{}, modelType reflect.Type) error {
	msg := LoadModel(c, jsonData, &jsonMap)
	if msg != nil {
		return msg
	}

	errors := ValidateMap(c, jsonMap, modelType)
	var errString string
	for _, err := range errors {
		errString += err.Error()
	}

	if len(jsonMap) == 0 {
		return message.Unprocessable(c)
	}

	if len(errors) > 0 {
		return message.Conflict(c).Text(errString)
	}

	return nil
}

func LoadAndValidateMaps(c *gin.Context, jsonData []byte, jsonMaps *[]map[string]interface{}, modelType reflect.Type) error {
	err := LoadModel(c, jsonData, jsonMaps)
	if err != nil {
		return err
	}
	err = ValidateMaps(c, *jsonMaps, modelType)
	if err != nil {
		return message.Conflict(c).Text(err.Error())
	}
	if len(*jsonMaps) == 0 {
		return message.Unprocessable(c)
	}
	return nil
}

func ValidateMapsPrimaries(c *gin.Context, jsonMaps []map[string]interface{}, primaryKeys []string) error {
	var err string
	for i, jsonMap := range jsonMaps {
		var errString string
		for _, field := range primaryKeys {
			if jsonMap[field] == nil {
				errString += message.InvalidFieldRequired(c, field).Error() + "\n"
			}
		}
		if errString != "" {
			err += message.RowError(c, i, "\n"+errString).Error()
		}
	}

	if err != "" {
		return message.Conflict(c).Text(err)
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
