package interfaces

import (
	"api_core/route"
	"net/http"
	"reflect"
)

type Endpointer interface {
	Endpoint(controller any) string
	BasePath() string
	FullPath(controller any) string
}

type Router interface {
	Routes() []route.Route
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

type RestControllerGet interface {
	Get(w http.ResponseWriter, r *http.Request)
	GetOne(w http.ResponseWriter, r *http.Request)
	GetStructure(w http.ResponseWriter, r *http.Request)
	GetRelStructure(w http.ResponseWriter, r *http.Request)
}

type RestControllerPost interface {
	Post(w http.ResponseWriter, r *http.Request)
}

type RestControllerPatch interface {
	Patch(w http.ResponseWriter, r *http.Request)
	PatchOne(w http.ResponseWriter, r *http.Request)
}

type RestControllerDelete interface {
	Delete(w http.ResponseWriter, r *http.Request)
}

type RestController interface {
	RestControllerGet
	RestControllerPost
	RestControllerPatch
	RestControllerDelete
}

type TypedController[T any] interface {
	BasicController
	Modeler[T]
}
