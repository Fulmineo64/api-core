package interfaces

import (
	"api_core/route"
)

type Namer interface {
	Name() string
}

type Endpointer interface {
	Endpoint() string
}

type Router interface {
	Routes(controller any) []route.Route
}

type Modeler interface {
	Model() any
}

type RouterModeler interface {
	Router
	Modeler
}
