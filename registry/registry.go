package registry

import (
	"api_core/interfaces"
)

var ControllerByName = map[string]interfaces.BasicController{}
var ControllerByModel = map[string]interfaces.BasicController{}
var ModelByName = map[string]any{}

func RegisterBasicController(name string, c interfaces.BasicController) {
	if ControllerByName[name] != nil {
		endpoint := c.Endpoint(c)
		// TODO: Decide how to implement override levels
		panic("Controller " + endpoint + " is already registered. To override it please specify an override level.")
	}
	ControllerByName[name] = c
}

func OverrideBasicController(name string, c interfaces.BasicController) {
	ControllerByName[name] = c
}

func RegisterTypedController[T any](name string, c interfaces.TypedController[T]) {
	RegisterBasicController(name, c)
	modelType := c.ModelType()
	if modelType != nil {
		ControllerByModel[modelType.String()] = c
	}
}

func OverrideTypedController[T any](name string, c interfaces.TypedController[T]) {
	OverrideBasicController(name, c)
	modelType := c.ModelType()
	if modelType != nil {
		ControllerByModel[modelType.String()] = c
	}
}

func RegisterModel(name string, m any) {
	if ModelByName[name] != nil {
		panic("Model " + name + " is already registered. To override it please specify an override level.")
	}
	ModelByName[name] = m
}

func OverrideModel(name string, m any) {
	ModelByName[name] = m
}
