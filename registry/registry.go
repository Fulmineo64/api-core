package registry

import (
	"api_core/interfaces"
	"reflect"
	"strings"
)

var ControllerByName = map[string]interfaces.Pather{}
var ControllerByModel = map[string]interfaces.Pather{}
var ModelByName = map[string]any{}

func Name(c any) string {
	if n, ok := c.(interfaces.Namer); ok {
		return n.Name()
	}
	name := reflect.TypeOf(c).String()
	pieces := strings.Split(name, ".")
	return pieces[len(pieces)-1]
}

func Models(models ...any) {
	for _, model := range models {
		ModelByName[Name(model)] = model
	}
}

func Controllers(controllers ...interfaces.Pather) {
	for _, controller := range controllers {
		ControllerByName[Name(controller)] = controller
		if m, ok := controller.(interfaces.Modeler); ok {
			ControllerByModel[Name(m.Model())] = controller
		}
	}
}

func ModelControllers(controllers ...interfaces.PatherModeler) {
	for _, controller := range controllers {
		Models(controller.Model())
		Controllers(controller)
	}
}
