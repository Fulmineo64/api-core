package controller

import "api_core/utils"

var ControllerByName = map[string]any{}
var ControllerByModel = map[string]any{}
var ModelByName = map[string]any{}

func RegisterModels(models ...any) {
	for _, model := range models {
		ModelByName[utils.Name(model)] = model
	}
}

func RegisterControllers(controllers ...any) {
	for _, controller := range controllers {
		ControllerByName[utils.Name(controller)] = controller
		if m, ok := controller.(Modeler); ok {
			ControllerByModel[utils.Name(m.Model())] = controller
		}
	}
}

func RegisterModelControllers(controllers ...Modeler) {
	for _, controller := range controllers {
		RegisterModels(controller.Model())
		RegisterControllers(controller)
	}
}
