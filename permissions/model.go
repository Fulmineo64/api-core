package permissions

import (
	"net/http"

	"api_core/message"
)

type HandlerFunc func(r *http.Request) message.Message

type ModelWithPermissionsPrefix interface {
	PermissionsPrefix() string
}

type ModelWithPermissionsGet interface {
	PermissionsGet(r *http.Request) message.Message
}

type ModelWithPermissionsPost interface {
	PermissionsPost(r *http.Request) message.Message
}

type ModelWithPermissionsPatch interface {
	PermissionsPatch(r *http.Request) message.Message
}

type ModelWithPermissionsDelete interface {
	PermissionsDelete(r *http.Request) message.Message
}

func Get(model interface{}) HandlerFunc {
	if modelPerm, ok := model.(ModelWithPermissionsGet); ok {
		return modelPerm.PermissionsGet
	} else {
		return func(r *http.Request) message.Message {
			return message.Forbidden(r)
		}
	}
}

func Post(model interface{}) HandlerFunc {
	if modelPerm, ok := model.(ModelWithPermissionsPost); ok {
		return modelPerm.PermissionsPost
	} else {
		return func(r *http.Request) message.Message {
			return message.Forbidden(r)
		}
	}
}

func Patch(model interface{}) HandlerFunc {
	if modelPerm, ok := model.(ModelWithPermissionsPatch); ok {
		return modelPerm.PermissionsPatch
	} else {
		return func(r *http.Request) message.Message {
			return message.Forbidden(r)
		}
	}
}

func Delete(model interface{}) HandlerFunc {
	if modelPerm, ok := model.(ModelWithPermissionsDelete); ok {
		return modelPerm.PermissionsDelete
	} else {
		return func(r *http.Request) message.Message {
			return message.Forbidden(r)
		}
	}
}

func Merge(permissionFunctions ...HandlerFunc) HandlerFunc {
	return func(r *http.Request) message.Message {
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
