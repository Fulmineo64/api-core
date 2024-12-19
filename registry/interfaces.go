package registry

import (
	"api_core/permissions"
	"net/http"
	"reflect"
)

type Endpointer interface {
	Endpoint(controller any) string
	BasePath() string
	FullPath(controller any) string
}

type Route struct {
	Method      string
	Pattern     string
	Handler     http.HandlerFunc
	Permissions []permissions.HandlerFunc
}

type Router interface {
	Routes() []Route
}

type BasicController interface {
	Endpointer
	Router
}

type Modeler[T any] interface {
	Model() *T
	ModelSlice() []T
	ModelType() reflect.Type
}

/*type RestController interface {
	Get(w http.ResponseWriter, r *http.Request)
	GetOne(w http.ResponseWriter, r *http.Request)
	GetStructure(w http.ResponseWriter, r *http.Request)
	GetRelStructure(w http.ResponseWriter, r *http.Request)
	Post(w http.ResponseWriter, r *http.Request)
	Patch(w http.ResponseWriter, r *http.Request)
	PatchOne(w http.ResponseWriter, r *http.Request)
	Delete(w http.ResponseWriter, r *http.Request)
}*/

type TypedController[T any] interface {
	BasicController
	Modeler[T]
}
