package controller

import "api_core/app"

func init() {
	RegisterModels(&app.SessionModel{})
}

type Controller struct{}
