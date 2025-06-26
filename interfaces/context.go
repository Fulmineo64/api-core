package interfaces

import (
	"golang.org/x/text/message"
	"gorm.io/gorm"
)

type DBContext interface {
	DB() *gorm.DB
}

type I18nContext interface {
	I18n() *message.Printer
}

type WriteContext interface {
	JSON(code int, obj any)
	AbortWithStatusJSON(code int, obj any)
}

type Context interface {
	DBContext
	I18nContext
}
