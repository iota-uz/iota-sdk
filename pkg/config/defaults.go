package config

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// applyTagDefaults reflects over v (must be a non-nil pointer to a struct) and,
// for each exported field that carries a non-empty `default:"X"` tag and is
// currently zero-valued (reflect.Value.IsZero), parses X into the field's kind
// and assigns it.
//
// Supported kinds: string, bool, int/int8/int16/int32/int64, uint/uint8/uint16/
// uint32/uint64, float32, float64, time.Duration, []string (comma-split, trimmed),
// and nested structs (recursed via pointer).
//
// Documented limitation: an explicit empty string set by a source is
// indistinguishable from "absent" because reflect.Value.IsZero returns true for
// an empty string. Tag defaults fire in both cases, matching the previous
// SetDefaults() semantic.
func applyTagDefaults(v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return fmt.Errorf("config: applyTagDefaults requires a non-nil pointer, got %T", v)
	}
	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return fmt.Errorf("config: applyTagDefaults requires a pointer to struct, got pointer to %s", rv.Kind())
	}
	return applyDefaults(rv)
}

// applyDefaults walks the struct fields of rv and applies default tag values.
//
// Recurses into:
//   - nested struct fields (value)
//   - nested *struct fields when non-nil
//   - []struct fields (per element)
//
// Scalar pointer fields (*bool, *int, etc.) are allocated when nil and the
// default tag parses into the element's kind.
//
// Note: *struct fields that are nil are NOT recursed into. Users who want
// defaults on an optional nested config must pre-allocate the pointer.
func applyDefaults(rv reflect.Value) error {
	rt := rv.Type()
	for i := range rt.NumField() {
		field := rt.Field(i)
		fv := rv.Field(i)

		// Skip unexported fields.
		if !field.IsExported() {
			continue
		}

		switch field.Type.Kind() {
		case reflect.Struct:
			// Recurse into nested struct fields unconditionally (they may have default tags).
			if err := applyDefaults(fv); err != nil {
				return err
			}
			continue

		case reflect.Ptr:
			elemKind := field.Type.Elem().Kind()
			if elemKind == reflect.Struct {
				// Pointer-to-struct: recurse only if already non-nil.
				// Nil *struct fields are left untouched to avoid allocating
				// optional nested configs that would otherwise stay nil.
				if !fv.IsNil() {
					if err := applyDefaults(fv.Elem()); err != nil {
						return err
					}
				}
				continue
			}
			// Scalar pointer (*bool, *int, *string, *time.Duration, etc.):
			// allocate and set from default tag when nil.
			tag := field.Tag.Get("default")
			if tag == "" {
				continue
			}
			if !fv.IsNil() {
				// Already set — do not clobber.
				continue
			}
			// Allocate a new element of the pointed-to type.
			newElem := reflect.New(field.Type.Elem()).Elem()
			// Re-use setFieldFromString by constructing a synthetic StructField
			// whose Type is the element type (not the pointer type).
			synthField := reflect.StructField{
				Name: field.Name,
				Type: field.Type.Elem(),
				Tag:  field.Tag,
			}
			if err := setFieldFromString(newElem, synthField, tag, rt.Name()); err != nil {
				return err
			}
			// Store the allocated pointer.
			ptr := reflect.New(field.Type.Elem())
			ptr.Elem().Set(newElem)
			fv.Set(ptr)
			continue

		case reflect.Slice:
			// Slice-of-struct: recurse into each existing element.
			if field.Type.Elem().Kind() == reflect.Struct {
				for j := range fv.Len() {
					if err := applyDefaults(fv.Index(j)); err != nil {
						return err
					}
				}
				continue
			}
			// fall through to default tag handling for []string etc.
		}

		tag := field.Tag.Get("default")
		if tag == "" {
			continue
		}
		if !fv.IsZero() {
			continue
		}

		if err := setFieldFromString(fv, field, tag, rt.Name()); err != nil {
			return err
		}
	}
	return nil
}

func setFieldFromString(fv reflect.Value, field reflect.StructField, raw, typeName string) error {
	kind := field.Type.Kind()
	switch kind {
	case reflect.String:
		fv.SetString(raw)

	case reflect.Bool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return fmt.Errorf("config: default tag on field %s.%s: cannot parse bool %q: %w", typeName, field.Name, raw, err)
		}
		fv.SetBool(b)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return fmt.Errorf("config: default tag on field %s.%s: cannot parse int %q: %w", typeName, field.Name, raw, err)
		}
		fv.SetInt(n)

	case reflect.Int64:
		// Special-case time.Duration.
		if field.Type == reflect.TypeOf(time.Duration(0)) {
			d, err := time.ParseDuration(raw)
			if err != nil {
				return fmt.Errorf("config: default tag on field %s.%s: cannot parse duration %q: %w", typeName, field.Name, raw, err)
			}
			fv.SetInt(int64(d))
			return nil
		}
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return fmt.Errorf("config: default tag on field %s.%s: cannot parse int64 %q: %w", typeName, field.Name, raw, err)
		}
		fv.SetInt(n)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return fmt.Errorf("config: default tag on field %s.%s: cannot parse uint %q: %w", typeName, field.Name, raw, err)
		}
		fv.SetUint(n)

	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return fmt.Errorf("config: default tag on field %s.%s: cannot parse float %q: %w", typeName, field.Name, raw, err)
		}
		fv.SetFloat(f)

	case reflect.Slice:
		if field.Type.Elem().Kind() != reflect.String {
			return fmt.Errorf("config: default tag on field %s.%s: unsupported kind %s", typeName, field.Name, field.Type)
		}
		parts := strings.Split(raw, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		fv.Set(reflect.ValueOf(out))

	default:
		return fmt.Errorf("config: default tag on field %s.%s: unsupported kind %s", typeName, field.Name, kind)
	}
	return nil
}
