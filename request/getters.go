package request

import (
	"api_core/app"

	"github.com/gin-gonic/gin"
	"golang.org/x/text/message"
	"gorm.io/gorm"
)

var (
	GinGetter     func(*gorm.DB) *gin.Context
	DBGetter      func(*gin.Context) *gorm.DB
	I18nGetter    func(*gin.Context) *message.Printer
	SessionGetter func(*gin.Context) *app.Session
)

func Gin(db *gorm.DB) *gin.Context {
	return GinGetter(db)
}

func DB(c *gin.Context) *gorm.DB {
	return DBGetter(c)
}

func I18n(c *gin.Context) *message.Printer {
	return I18nGetter(c)
}

func Session(c *gin.Context) *app.Session {
	return SessionGetter(c)
}

type DbContextKey string

var (
	GinKey     DbContextKey = "gin"
	DBKey      string       = "db"
	I18nKey    string       = "i18n"
	SessionKey string       = "s"
)
