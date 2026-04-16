package config

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// Redact walks v via reflection and returns a JSON-ish representation
// suitable for logging and /debug/config. Fields tagged secret:"true" are
// replaced with "***" unless they are zero-valued, in which case "" is used.
// Nested structs, pointers, and slices of structs are walked recursively.
// Safe on nil inputs.
func Redact(v any) string {
	if v == nil {
		return "null"
	}
	node := redactValue(reflect.ValueOf(v))
	buf, err := json.MarshalIndent(node, "", "  ")
	if err != nil {
		return fmt.Sprintf("<redact error: %v>", err)
	}
	return string(buf)
}

// redactValue converts a reflect.Value to a redacted representation.
// Returns a type suitable for json.Marshal.
func redactValue(rv reflect.Value) any {
	// Dereference pointers.
	for rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return nil
		}
		rv = rv.Elem()
	}

	switch rv.Kind() { //nolint:exhaustive // scalars and unsupported kinds fall through to the default branch
	case reflect.Struct:
		return redactStruct(rv)
	case reflect.Slice:
		return redactSlice(rv)
	default:
		return rv.Interface()
	}
}

func redactStruct(rv reflect.Value) map[string]any {
	rt := rv.Type()
	out := make(map[string]any, rt.NumField())
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		if !field.IsExported() {
			continue
		}
		fv := rv.Field(i)
		key := fieldKey(field)
		if key == "-" {
			continue
		}

		if field.Tag.Get("secret") == "true" {
			if fv.IsZero() {
				out[key] = ""
			} else {
				out[key] = "***"
			}
			continue
		}

		out[key] = redactValue(fv)
	}
	return out
}

func redactSlice(rv reflect.Value) any {
	if rv.IsNil() {
		return nil
	}
	out := make([]any, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		out[i] = redactValue(rv.Index(i))
	}
	return out
}

// fieldKey returns the JSON key for a struct field, respecting json:"name" tags.
func fieldKey(field reflect.StructField) string {
	tag := field.Tag.Get("json")
	if tag == "" {
		return field.Name
	}
	name, _, _ := strings.Cut(tag, ",")
	if name == "" {
		return field.Name
	}
	return name
}
