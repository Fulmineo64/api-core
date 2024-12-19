package dialectors

import (
	"errors"

	"gorm.io/gorm"
)

var registeredDialectors = map[string]Dialector{}

func Register(name string, dialector Dialector) {
	registeredDialectors[name] = dialector
}

func ByName(name string) (Dialector, error) {
	if dialector, ok := registeredDialectors[name]; ok {
		return dialector, nil
	}
	return nil, errors.New("please register a valid dilector with dialectors.Register(name, dialector) for the " + name + " dialect")
}

func ByDB(db *gorm.DB) (Dialector, error) {
	return ByName(db.Dialector.Name())
}

func init() {
	Register("postgres", PostgresDialector{})
	Register("sqlserver", SqlserverDialector{})
}
