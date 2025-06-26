package app

import (
	"api_core/utils"
	"embed"

	"gorm.io/gorm"
)

var Flags = utils.NewKeySet()
var Properties = map[string]string{}
var DB *gorm.DB
var FS *embed.FS
