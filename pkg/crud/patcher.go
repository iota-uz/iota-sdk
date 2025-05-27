package crud

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	formui "github.com/iota-uz/iota-sdk/components/scaffold/form"
)

// DefaultEntityPatcher is the default implementation of EntityPatcher
type DefaultEntityPatcher[T any] struct{}

// Patch applies form values to the entity
func (p DefaultEntityPatcher[T]) Patch(entity T, formData map[string]string, fields []formui.Field) (T, ValidationError) {
	var validationErrors ValidationError

	// Populate fields from form data
	rVal := reflect.ValueOf(entity)
	if rVal.Kind() == reflect.Ptr {
		rVal = rVal.Elem()
	}

	// Process each field
	for _, field := range fields {
		fieldName := field.Key()
		formValue, exists := formData[fieldName]
		if !exists {
			continue
		}

		// Get the field by name (case-insensitive)
		fv := rVal.FieldByNameFunc(func(name string) bool {
			return strings.EqualFold(name, fieldName)
		})

		if !fv.IsValid() || !fv.CanSet() {
			continue
		}

		// Set field value based on type
		if err := setFieldValue(fv, formValue); err != nil {
			validationErrors.Errors = append(validationErrors.Errors, FieldError{
				Field:   fieldName,
				Message: err.Error(),
			})
		}
	}

	return entity, validationErrors
}

// Helper function to set field value based on type
func setFieldValue(field reflect.Value, value string) error {
	if value == "" {
		return nil // Skip empty values
	}

	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if v, err := strconv.ParseInt(value, 10, 64); err != nil {
			return fmt.Errorf("invalid integer: %w", err)
		} else {
			field.SetInt(v)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if v, err := strconv.ParseUint(value, 10, 64); err != nil {
			return fmt.Errorf("invalid unsigned integer: %w", err)
		} else {
			field.SetUint(v)
		}
	case reflect.Float32, reflect.Float64:
		if v, err := strconv.ParseFloat(value, 64); err != nil {
			return fmt.Errorf("invalid float: %w", err)
		} else {
			field.SetFloat(v)
		}
	case reflect.Bool:
		if v, err := strconv.ParseBool(value); err != nil {
			return fmt.Errorf("invalid boolean: %w", err)
		} else {
			field.SetBool(v)
		}
	case reflect.Struct:
		// Handle common struct types
		if field.Type() == reflect.TypeOf(time.Time{}) {
			if t, err := time.Parse(time.RFC3339, value); err != nil {
				return fmt.Errorf("invalid time format: %w", err)
			} else {
				field.Set(reflect.ValueOf(t))
			}
		} else {
			return fmt.Errorf("unsupported struct type: %v", field.Type())
		}
	case reflect.Invalid, reflect.Uintptr, reflect.Complex64, reflect.Complex128, reflect.Array, reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice, reflect.UnsafePointer:
		return fmt.Errorf("unsupported field type: %v", field.Kind())
	default:
		return fmt.Errorf("unsupported field type: %v", field.Kind())
	}
	return nil
}
