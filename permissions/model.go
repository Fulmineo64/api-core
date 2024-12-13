package permissions

import (
	"context"
	"net/http"
	"reflect"

	"api_core/message"

	"gorm.io/gorm/schema"
)

type HandlerFunc func(r *http.Request) message.Message

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

func CheckModel(r *http.Request, modelVal reflect.Value, modelSchema *schema.Schema, cache map[string]struct{}, checkDelete bool) error {
	for _, rel := range modelSchema.Relationships.HasMany {
		relField := rel.Field.ReflectValueOf(context.Background(), modelVal)
		if !relField.IsNil() {
			len := relField.Len()
			for i := 0; i < len; i++ {
				item := relField.Index(i)
				if _, ok := cache[item.Type().String()+"_get"]; !ok {
					if msg := Get(item.Interface())(r); msg != nil {
						return msg
					}
					cache[item.Type().String()+"_get"] = struct{}{}
				}
				if checkDelete {
					if _, ok := cache[item.Type().String()+"_del"]; !ok {
						deleteField := item.FieldByName("Delete")
						if deleteField.IsValid() && deleteField.Bool() {
							if msg := Delete(item.Interface())(r); msg != nil {
								return msg
							}
							cache[item.Type().String()+"_del"] = struct{}{}
						}
					}
				}
				msg := CheckModel(r, item, rel.FieldSchema, cache, checkDelete)
				if msg != nil {
					return msg
				}
			}
		}
	}

	return nil
}
