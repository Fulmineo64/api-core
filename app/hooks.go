package app

import (
	"log"
	"net/http"

	"api_core/message"
)

type AbortWithErrorHook struct {
	Hook[func(http.ResponseWriter, *http.Request, error)]
}

func (h *AbortWithErrorHook) Run(w http.ResponseWriter, r *http.Request, err error) {
	for _, fn := range h.Funcs {
		fn(w, r, err)
	}
}

type OnRecoverHook struct {
	Hook[func(http.ResponseWriter, *http.Request, string)]
}

func (h *OnRecoverHook) Run(w http.ResponseWriter, r *http.Request, err string) {
	for _, fn := range h.Funcs {
		fn(w, r, err)
	}
}

type ControllerHooks struct {
	AbortWithError AbortWithErrorHook
	OnRecover      OnRecoverHook
}

var Hooks = ControllerHooks{}

func init() {
	Hooks.AbortWithError.Add("default", func(w http.ResponseWriter, r *http.Request, err error) {
		if msg, ok := err.(message.Message); ok {
			msg.Write(w, r)
		} else {
			log.Println(err)
			message.InternalServerError(r).Write(w, r)
		}
	})
	Hooks.OnRecover.Add("default", func(w http.ResponseWriter, r *http.Request, err string) {
		log.Printf("recovered panic: %s\n", err)
	})
}
