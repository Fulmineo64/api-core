package ctx

import (
	"api_core/app"
	"net/http"

	"golang.org/x/text/message"
	"gorm.io/gorm"
)

var (
	DBGetter      func(*http.Request) *gorm.DB
	I18nGetter    func(*http.Request) *message.Printer
	SessionGetter func(r *http.Request) *app.Session
)

func DB(r *http.Request) *gorm.DB {
	return DBGetter(r)
}

func I18n(r *http.Request) *message.Printer {
	return I18nGetter(r)
}

func Session(r *http.Request) *app.Session {
	return SessionGetter(r)
}

type ContextType string

const (
	RequestKey ContextType = "r"
	DBKey      ContextType = "db"
	I18nKey    ContextType = "i18n"
	SessionKey ContextType = "s"
)
