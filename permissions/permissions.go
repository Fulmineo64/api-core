package permissions

import (
	"api_core/utils"

	"github.com/gin-gonic/gin"
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

func Validate(c *gin.Context, permissions ...HandlerFunc) error {
	for _, permission := range permissions {
		if permission == nil {
			continue
		}
		err := permission(c)
		if err != nil {
			return err
		}
	}
	return nil
}

func Merge(permissionFunctions ...HandlerFunc) HandlerFunc {
	return func(c *gin.Context) error {
		for _, permissionFunc := range permissionFunctions {
			if permissionFunc != nil {
				msg := permissionFunc(c)
				if msg != nil {
					return msg
				}
			}
		}
		return nil
	}
}
