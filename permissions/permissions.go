package permissions

import (
	"api_core/request"
	"api_core/utils"
	"net/http"
)

var permissions = utils.NewKeySet()

// Has - Checks if all the @{keys} are present
func Has(keys ...string) bool {
	return permissions.Has(keys...)
}

// HasOne - Checks if at least one of the @{keys} is present
func HasOne(keys ...string) bool {
	return permissions.HasOne(keys...)
}

func Add(key string) {
	permissions.Add(key)
}

func Clear() {
	permissions.Clear()
}

func Validate(r *http.Request, permissions ...HandlerFunc) error {
	for _, permission := range permissions {
		err := permission(r)
		if err != nil {
			return err
		}
	}
	return nil
}

func Merge(permissionFunctions ...HandlerFunc) HandlerFunc {
	return func(r *http.Request) error {
		for _, permissionFunc := range permissionFunctions {
			if permissionFunc != nil {
				msg := permissionFunc(r)
				if msg != nil {
					return msg
				}
			}
		}
		return nil
	}
}

func Middleware(permissionFuncs ...HandlerFunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err := Validate(r, permissionFuncs...)
			if request.AbortIfError(w, r, err) {
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
