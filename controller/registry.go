package controller

import (
	"api_core/permissions"
	"api_core/utils"
	"net/http"
	"reflect"
	"strings"

	"github.com/go-chi/chi"
)

var ByName = map[string]CRUDSController{}
var ByModel = map[string]CRUDSController{}

func FindControllerByModel(modelType reflect.Type) CRUDSController {
	return ByModel[modelType.String()]
}

func Register(basePath string, container chi.Router, toRegister string, c CRUDSController) {
	c.SetBasePath(basePath)
	controllerName := reflect.Indirect(reflect.ValueOf(c)).Type().Name()
	c.SetEndpointIfAbsent(utils.FirstLower(controllerName))
	name := c.GetEndpoint()

	if ByName[controllerName] != nil {
		panic("Controller " + name + " already registered!")
	}

	ByName[controllerName] = c

	modelType := c.GetModelType()
	if modelType != nil {
		primaryFields := utils.GetPrimaryFields(c.GetModelType())
		params := ""
		for i, field := range primaryFields {
			if i > 0 {
				params += "/"
			}
			params += ":" + field
		}

		if strings.Contains(toRegister, "C") {
			c.AddRoute(http.MethodPost, "", permissions.Post(c.GetModel()), c.Post)
		}
		if strings.Contains(toRegister, "R") {
			c.AddRoute(http.MethodGet, "", permissions.Get(c.GetModel()), c.Get)
			if len(primaryFields) > 0 {
				c.AddRoute(http.MethodGet, params, permissions.Get(c.GetModel()), c.GetOne)
			}
		}
		if strings.Contains(toRegister, "U") && len(primaryFields) > 0 {
			c.AddRoute(http.MethodPatch, params, permissions.Patch(c.GetModel()), c.Patch)
			c.AddRoute(http.MethodPatch, "", permissions.Patch(c.GetModel()), c.PatchMany)
		}
		if strings.Contains(toRegister, "D") && len(primaryFields) > 0 {
			c.AddRoute(http.MethodDelete, params, permissions.Delete(c.GetModel()), c.Delete)
		}
		if strings.Contains(toRegister, "S") {
			c.AddRoute(http.MethodGet, "structure", permissions.Get(c.GetModel()), c.GetStructure)
			c.AddRoute(http.MethodGet, "structure/:rel", permissions.Get(c.GetModel()), c.GetRelStructure)
		}

		ByModel[modelType.String()] = c
	}

	for _, typ := range c.AdditionalModels() {
		ByModel[typ.String()] = c
	}

	c.AddCustomRoutes()

	container.Group(func(r chi.Router) {
		for _, route := range c.GetRoutes() {
			if route.PermissionsFunc != nil {
				r = r.With(func(h http.Handler) http.Handler {
					return checkPermissions(route.PermissionsFunc)
				})
			}
			r.MethodFunc(route.Method, route.Name, route.HandlerFunc)
		}
	})
}

func checkPermissions(permissionFunc permissions.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		message := permissionFunc(r)
		if message != nil {
			message.Write(w, r)
		}
	}
}
