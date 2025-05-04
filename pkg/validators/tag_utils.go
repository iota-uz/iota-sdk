package validators

import (
	"reflect"

	"github.com/go-playground/validator/v10"
)

func FieldLabel[T any](dto T, err validator.FieldError) string {
	fieldName := err.Field()
	t := reflect.TypeOf(dto)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return fieldName
	}
	if field, ok := t.FieldByName(fieldName); ok {
		if label := field.Tag.Get("label"); label != "" {
			return label
		}
	}
	return fieldName
}
