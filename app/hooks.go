package app

import (
	"log"

	"api_core/message"

	"github.com/gin-gonic/gin"
)

type AbortWithErrorHook struct {
	Hook[func(*gin.Context, error)]
}

func (h *AbortWithErrorHook) Run(c *gin.Context, err error) {
	for _, fn := range h.Funcs {
		fn(c, err)
	}
}

type OnRecoverHook struct {
	Hook[func(*gin.Context, string)]
}

func (h *OnRecoverHook) Run(c *gin.Context, err string) {
	for _, fn := range h.Funcs {
		fn(c, err)
	}
}

type ControllerHooks struct {
	AbortWithError AbortWithErrorHook
	OnRecover      OnRecoverHook
}

var Hooks = ControllerHooks{}

func init() {
	Hooks.AbortWithError.Add("default", func(c *gin.Context, err error) {
		if msg, ok := err.(message.Message); ok {
			msg.Write(c)
		} else {
			log.Println(err)
			message.InternalServerError(c).Write(c)
		}
	})
	Hooks.OnRecover.Add("default", func(c *gin.Context, err string) {
		log.Printf("recovered panic: %s\n", err)
	})
}
