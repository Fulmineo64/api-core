package registry

import (
	"reflect"
)

var ControllerByName = map[string]BasicController{}
var ControllerByModel = map[string]BasicController{}
var ModelByName = map[string]any{}

func FindControllerByModel(modelType reflect.Type) BasicController {
	return ControllerByModel[modelType.String()]
}

func RegisterBasicController(name string, c BasicController, override ...bool) {
	if ControllerByName[name] != nil && (len(override) != 1 || !override[0]) {
		endpoint := c.Endpoint(c)
		// TODO: Decide how to implement override levels
		panic("Controller " + endpoint + " is already registered. To override it please specify an override level.")
	}
	ControllerByName[name] = c
}

func RegisterTypedController[T any](name string, c TypedController[T], override ...bool) {
	RegisterBasicController(name, c, override...)
	modelType := c.ModelType()
	if modelType != nil {
		ControllerByModel[modelType.String()] = c
	}
}

func RegisterModel(name string, m any, override ...bool) {
	if ModelByName[name] != nil && (len(override) != 1 || !override[0]) {
		panic("Model " + name + " is already registered. To override it please specify an override level.")
	}
	ModelByName[name] = m
}
