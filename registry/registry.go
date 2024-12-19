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

func RegisterBasicController(name string, c BasicController) {
	if ControllerByName[name] != nil {
		endpoint := c.Endpoint(c)
		// TODO: Decide how to implement override levels
		panic("Controller " + endpoint + " is already registered. To override it please specify an override level.")
	}

	ControllerByName[name] = c
}

func RegisterTypedController[T any](name string, c TypedController[T]) {
	RegisterBasicController(name, c)
	modelType := c.ModelType()
	if modelType != nil {
		ControllerByModel[modelType.String()] = c
	}
}

func RegisterModel(name string, m any) {
	ModelByName[name] = m
}
