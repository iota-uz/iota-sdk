package js

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// ToJS transforms a Go struct into a JavaScript object representation.
// It supports basic types, nested structs, maps, slices, and can include
// function references.
func ToJS(v interface{}) (string, error) {
	return toJSInternal(reflect.ValueOf(v), 0)
}

// toJSInternal handles the actual transformation
func toJSInternal(v reflect.Value, indent int) (string, error) {

	// Handle nil values
	if !v.IsValid() || (v.Kind() == reflect.Ptr && v.IsNil()) {
		return "null", nil
	}

	// Dereference pointers
	if v.Kind() == reflect.Ptr {
		return toJSInternal(v.Elem(), indent)
	}

	// Check if the value is a templ.JSExpression
	if v.Type().String() == "templ.JSExpression" {
		// Return JSExpression as-is without encoding
		return string(v.String()), nil
	}

	switch v.Kind() {
	case reflect.Bool:
		return fmt.Sprintf("%v", v.Bool()), nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return fmt.Sprintf("%v", v.Int()), nil

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return fmt.Sprintf("%v", v.Uint()), nil

	case reflect.Float32, reflect.Float64:
		return fmt.Sprintf("%v", v.Float()), nil

	case reflect.String:
		// Escape quotes and special characters for JS
		s := strings.ReplaceAll(v.String(), "\\", "\\\\")
		s = strings.ReplaceAll(s, "'", "\\'")
		s = strings.ReplaceAll(s, "\n", "\\n")
		s = strings.ReplaceAll(s, "\r", "\\r")
		s = strings.ReplaceAll(s, "\t", "\\t")
		return fmt.Sprintf("'%s'", s), nil

	case reflect.Func:
		// For function types, we create a placeholder that can be filled in
		// with a real function reference later
		return fmt.Sprintf("/* function reference: %s */", v.Type()), nil

	case reflect.Slice, reflect.Array:
		if v.Len() == 0 {
			return "[]", nil
		}

		parts := make([]string, 0, v.Len())
		for i := 0; i < v.Len(); i++ {
			part, err := toJSInternal(v.Index(i), indent+1)
			if err != nil {
				return "", err
			}
			parts = append(parts, part)
		}

		return fmt.Sprintf("[%s]", strings.Join(parts, ",")), nil

	case reflect.Map:
		if v.Len() == 0 {
			return "{}", nil
		}

		parts := make([]string, 0, v.Len())
		iter := v.MapRange()
		for iter.Next() {
			k := iter.Key()
			val := iter.Value()

			keyStr := k.String()
			if k.Kind() != reflect.String {
				keyBytes, err := json.Marshal(k.Interface())
				if err != nil {
					return "", err
				}
				keyStr = string(keyBytes)
			}

			valueStr, err := toJSInternal(val, indent+1)
			if err != nil {
				return "", err
			}

			// Format the key appropriately for JS
			keyJS := keyStr
			if k.Kind() == reflect.String {
				keyJS = fmt.Sprintf("'%s'", strings.ReplaceAll(keyStr, "'", "\\'"))
			}

			parts = append(parts, fmt.Sprintf("%s: %s", keyJS, valueStr))
		}

		return fmt.Sprintf("{%s}", strings.Join(parts, ",")), nil

	case reflect.Struct:
		t := v.Type()
		if v.NumField() == 0 {
			return "{}", nil
		}

		parts := make([]string, 0, v.NumField())
		for i := 0; i < v.NumField(); i++ {
			field := t.Field(i)
			fieldValue := v.Field(i)

			// Skip unexported fields
			if !fieldValue.CanInterface() {
				continue
			}

			// Get the field name, respecting json tags
			fieldName := field.Name
			jsonTag := field.Tag.Get("json")

			// Process json tag if present
			omitEmpty := false
			if jsonTag != "" {
				tagParts := strings.Split(jsonTag, ",")

				// Check if field should be omitted
				if tagParts[0] == "-" {
					// Skip this field if json tag is "-"
					continue
				}

				// Set field name from tag if not "-"
				if tagParts[0] != "" {
					fieldName = tagParts[0]
				}

				// Check for omitempty tag
				for _, opt := range tagParts[1:] {
					if opt == "omitempty" {
						omitEmpty = true
						break
					}
				}
			}

			// Handle omitempty
			if omitEmpty {
				// Check if the field is empty
				isZero := false

				switch fieldValue.Kind() {
				case reflect.Bool:
					isZero = !fieldValue.Bool()
				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
					isZero = fieldValue.Int() == 0
				case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
					isZero = fieldValue.Uint() == 0
				case reflect.Float32, reflect.Float64:
					isZero = fieldValue.Float() == 0
				case reflect.String:
					isZero = fieldValue.String() == ""
				case reflect.Map, reflect.Slice, reflect.Array:
					isZero = fieldValue.Len() == 0
				case reflect.Ptr, reflect.Interface:
					isZero = fieldValue.IsNil()
				}

				if isZero {
					continue
				}
			}

			valueStr, err := toJSInternal(fieldValue, indent+1)
			if err != nil {
				return "", err
			}

			parts = append(parts, fmt.Sprintf("'%s': %s", fieldName, valueStr))
		}

		return fmt.Sprintf("{%s}", strings.Join(parts, ",")), nil

	default:
		// For other types, try to marshal to JSON and use that
		js, err := json.Marshal(v.Interface())
		if err != nil {
			return "", fmt.Errorf("unsupported type %s: %w", v.Type(), err)
		}
		return string(js), nil
	}
}
