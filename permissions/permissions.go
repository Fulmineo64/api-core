package permissions

import "api_core/utils"

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
