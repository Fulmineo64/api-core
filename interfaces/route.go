package interfaces

import (
	"api_core/permissions"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Route struct {
	Method      string
	Pattern     string
	Permissions permissions.HandlerFunc
	Handler     gin.HandlerFunc
}

func (rt Route) Authenticate(c *gin.Context) error {
	// TODO: Apply authentication here
	return permissions.Validate(c, rt.Permissions)
}

func New(method string, pattern string, handler gin.HandlerFunc, permissions ...permissions.HandlerFunc) Route {
	route := Route{
		Method:  method,
		Pattern: pattern,
		Handler: handler,
	}
	if len(permissions) == 1 {
		route.Permissions = permissions[0]
	}
	return route
}

func Get(pattern string, handler gin.HandlerFunc, permissions ...permissions.HandlerFunc) Route {
	return New(http.MethodGet, pattern, handler, permissions...)
}

func Post(pattern string, handler gin.HandlerFunc, permissions ...permissions.HandlerFunc) Route {
	return New(http.MethodPost, pattern, handler, permissions...)
}

func Patch(pattern string, handler gin.HandlerFunc, permissions ...permissions.HandlerFunc) Route {
	return New(http.MethodPatch, pattern, handler, permissions...)
}

func Delete(pattern string, handler gin.HandlerFunc, permissions ...permissions.HandlerFunc) Route {
	return New(http.MethodDelete, pattern, handler, permissions...)
}
