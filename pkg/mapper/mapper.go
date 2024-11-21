// Package mapper is deprecated
package mapper

import (
	"errors"
	"reflect"
)

type Behaviour int

const (
	Strict Behaviour = iota
	Lenient
)

// Pointer is a utility function that returns a pointer to the given value.
func Pointer[T any](v T) *T {
	return &v
}

// StrictMapping is a wrapper around DefaultMapping with Strict behaviour.
func StrictMapping(source interface{}, target interface{}) error {
	return DefaultMapping(source, target, Strict)
}

// LenientMapping is a wrapper around DefaultMapping with Lenient behaviour.
func LenientMapping(source interface{}, target interface{}) error {
	return DefaultMapping(source, target, Lenient)
}

// DefaultMapping is a utility function that maps fields from source struct to target struct
// source and target must be pointers to structs.
func DefaultMapping(source interface{}, target interface{}, behaviour Behaviour) error {
	sourceValue := reflect.ValueOf(source)
	targetValue := reflect.ValueOf(target)
	if sourceValue.Kind() != reflect.Ptr || targetValue.Kind() != reflect.Ptr {
		return errors.New("both arguments must be pointers")
	}
	sourceValue = sourceValue.Elem()
	targetValue = targetValue.Elem()

	if sourceValue.Kind() != reflect.Struct || targetValue.Kind() != reflect.Struct {
		return errors.New("both arguments must be pointers to structs")
	}

	for i := range sourceValue.NumField() {
		sourceField := sourceValue.Field(i)
		targetField := targetValue.FieldByName(sourceValue.Type().Field(i).Name)
		if !targetField.IsValid() {
			if behaviour == Strict {
				return errors.New("field not found in target")
			}
			continue
		}
		if sourceField.Kind() == reflect.Ptr && sourceField.IsNil() {
			continue
		}
		if targetField.Kind() == reflect.Ptr {
			targetField.Set(reflect.New(targetField.Type().Elem()))
			targetField = targetField.Elem()
		}

		if sourceField.Kind() == reflect.Ptr {
			sourceField = sourceField.Elem()
		}

		if sourceField.Type() != targetField.Type() {
			return errors.New("field types do not match")
		}
		targetField.Set(sourceField)
	}
	return nil
}
