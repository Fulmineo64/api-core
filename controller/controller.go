package controller

import (
	"api_core/app"

	"github.com/gin-gonic/gin"
)

func init() {
	RegisterModels(&app.SessionModel{})
}

// Controller è il cuore della logica di business e del routing, può implementare le seguenti interfaces per estendere ed aggiungere funzionalità
type Controller struct{}

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
	Path() string
}

type Renderer interface {
	Render(c *gin.Context)
}

type Getter interface {
	Get(c *gin.Context)
}

type GetOner interface {
	GetOne(c *gin.Context)
}

type GetStructurer interface {
	GetStructure(c *gin.Context)
}

type GetRelStructurer interface {
	GetRelStructure(c *gin.Context)
}

type Poster interface {
	Post(c *gin.Context)
}

type PatchHandlerer interface {
	Patch(c *gin.Context)
}

type PatchOner interface {
	PatchOne(c *gin.Context)
}

type DeleteHandlerer interface {
	Delete(c *gin.Context)
}
