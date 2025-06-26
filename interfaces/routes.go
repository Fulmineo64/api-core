package interfaces

import (
	"github.com/gin-gonic/gin"
)

type GetHandler interface {
	Get(c *gin.Context)
}

type GetOneHandler interface {
	GetOne(c *gin.Context)
}

type GetStructureHandler interface {
	GetStructure(c *gin.Context)
}

type GetRelStructureHandler interface {
	GetRelStructure(c *gin.Context)
}

type PostHandler interface {
	Post(c *gin.Context)
}

type PatchHandler interface {
	Patch(c *gin.Context)
}

type PatchOneHandler interface {
	PatchOne(c *gin.Context)
}

type DeleteHandler interface {
	Delete(c *gin.Context)
}
