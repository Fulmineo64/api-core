package app

import (
	"api_core/utils"

	"gorm.io/gorm"
)

var Flags = utils.NewKeySet()
var Properties = map[string]string{}
var DB *gorm.DB
