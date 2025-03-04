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

type Pather interface {
	SetPath(path string)
	Path() string
}

type PatherModeler interface {
	Pather
	Modeler
}

type RouterModeler interface {
	Router
	Modeler
}
