package permissions

import (
	"context"
	"reflect"

	"api_core/message"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm/schema"
)

type HandlerFunc func(c *gin.Context) error

type ModelWithPermissionsGet interface {
	PermissionsGet(c *gin.Context) error
}

type ModelWithPermissionsPost interface {
	PermissionsPost(c *gin.Context) error
}

type ModelWithPermissionsPatch interface {
	PermissionsPatch(c *gin.Context) error
}

type ModelWithPermissionsDelete interface {
	PermissionsDelete(c *gin.Context) error
}

func Get(model interface{}) HandlerFunc {
	if modelPerm, ok := model.(ModelWithPermissionsGet); ok {
		return modelPerm.PermissionsGet
	} else {
		return func(c *gin.Context) error {
			return message.Forbidden(c)
		}
	}
}

func Post(model interface{}) HandlerFunc {
	if modelPerm, ok := model.(ModelWithPermissionsPost); ok {
		return modelPerm.PermissionsPost
	} else {
		return func(c *gin.Context) error {
			return message.Forbidden(c)
		}
	}
}

func Patch(model interface{}) HandlerFunc {
	if modelPerm, ok := model.(ModelWithPermissionsPatch); ok {
		return modelPerm.PermissionsPatch
	} else {
		return func(c *gin.Context) error {
			return message.Forbidden(c)
		}
	}
}

func Delete(model interface{}) HandlerFunc {
	if modelPerm, ok := model.(ModelWithPermissionsDelete); ok {
		return modelPerm.PermissionsDelete
	} else {
		return func(c *gin.Context) error {
			return message.Forbidden(c)
		}
	}
}

func CheckModel(c *gin.Context, modelVal reflect.Value, modelSchema *schema.Schema, cache map[string]struct{}, checkDelete bool) error {
	for _, rel := range modelSchema.Relationships.HasMany {
		relField := rel.Field.ReflectValueOf(context.Background(), modelVal)
		if !relField.IsNil() {
			len := relField.Len()
			for i := 0; i < len; i++ {
				item := relField.Index(i)
				if _, ok := cache[item.Type().String()+"_get"]; !ok {
					if msg := Get(item.Interface())(c); msg != nil {
						return msg
					}
					cache[item.Type().String()+"_get"] = struct{}{}
				}
				if checkDelete {
					if _, ok := cache[item.Type().String()+"_del"]; !ok {
						deleteField := item.FieldByName("Delete")
						if deleteField.IsValid() && deleteField.Bool() {
							if msg := Delete(item.Interface())(c); msg != nil {
								return msg
							}
							cache[item.Type().String()+"_del"] = struct{}{}
						}
					}
				}
				msg := CheckModel(c, item, rel.FieldSchema, cache, checkDelete)
				if msg != nil {
					return msg
				}
			}
		}
	}

	return nil
}
