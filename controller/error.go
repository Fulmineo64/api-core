package controller

import (
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
)

var EnableRecover bool

func AbortIfError(w http.ResponseWriter, r *http.Request, err error) bool {
	if err != nil {
		AbortWithError(w, r, err)
	}
	return err != nil
}

func AbortWithError(w http.ResponseWriter, r *http.Request, err error) {
	Hooks.AbortWithError.Run(w, r, err)
}

func RecoverIfEnabled(w http.ResponseWriter, r *http.Request) {
	if EnableRecover {
		Recover(w, r)
	}
}

func Recover(w http.ResponseWriter, r *http.Request) {
	if err := recover(); err != nil {
		stck := strings.Split(string(debug.Stack()), "\n")
		errStr := fmt.Sprint(err) + "\n" + strings.Join(stck[3:], "\n")
		Hooks.OnRecover.Run(w, r, errStr)
	}
}
