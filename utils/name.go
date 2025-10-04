package utils

import (
	"reflect"
	"strings"
)

type Namer interface {
	Name() string
}

func Name(c any) string {
	if n, ok := c.(Namer); ok {
		return n.Name()
	}
	name := reflect.TypeOf(c).String()
	pieces := strings.Split(name, ".")
	return pieces[len(pieces)-1]
}
