package interfaces

import "github.com/gin-gonic/gin"

type Namer interface {
	Name() string
}

type Endpointer interface {
	Endpoint() string
}

type Router interface {
	Routes() []Route
}

type Grouper interface {
	Group() string
}

type Middlewarer interface {
	Middleware() []gin.HandlerFunc
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
