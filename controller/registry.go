package controller

import (
	"api_core/interfaces"
	"reflect"
	"strings"
)

var ControllerByName = map[string]any{}
var ControllerByModel = map[string]any{}
var ModelByName = map[string]any{}

func Name(c any) string {
	if n, ok := c.(interfaces.Namer); ok {
		return n.Name()
	}
	name := reflect.TypeOf(c).String()
	pieces := strings.Split(name, ".")
	return pieces[len(pieces)-1]
}

func RegisterModels(models ...any) {
	for _, model := range models {
		ModelByName[Name(model)] = model
	}
}

func RegisterControllers(controllers ...any) {
	for _, controller := range controllers {
		ControllerByName[Name(controller)] = controller
		if m, ok := controller.(interfaces.Modeler); ok {
			ControllerByModel[Name(m.Model())] = controller
		}
	}
}

func RegisterModelControllers(controllers ...interfaces.Modeler) {
	for _, controller := range controllers {
		RegisterModels(controller.Model())
		RegisterControllers(controller)
	}
}
