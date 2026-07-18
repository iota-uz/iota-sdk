package frame

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// FromRecords converts a typed slice into a clone-safe detail dataset for
// declarative Lens evidence exports. Exported fields become columns; json tags
// provide stable names. Nested values are encoded as JSON strings.
func FromRecords[T any](name string, records []T) (*FrameSet, error) {
	typeOf := reflect.TypeOf((*T)(nil)).Elem()
	for typeOf.Kind() == reflect.Pointer {
		typeOf = typeOf.Elem()
	}
	if typeOf.Kind() != reflect.Struct {
		return nil, fmt.Errorf("records must contain structs, got %s", typeOf)
	}
	fields := make([]Field, 0, typeOf.NumField())
	indexes := make([]int, 0, typeOf.NumField())
	for i := 0; i < typeOf.NumField(); i++ {
		item := typeOf.Field(i)
		if item.PkgPath != "" {
			continue
		}
		column := item.Tag.Get("json")
		if comma := strings.IndexByte(column, ','); comma >= 0 {
			column = column[:comma]
		}
		if column == "-" {
			continue
		}
		if column == "" {
			column = item.Name
		}
		fields = append(fields, Field{Name: column, Type: fieldTypeFor(typeOf.Field(i).Type), Values: make([]any, 0, len(records))})
		indexes = append(indexes, i)
	}
	for _, record := range records {
		value := reflect.ValueOf(record)
		for value.Kind() == reflect.Pointer {
			if value.IsNil() {
				break
			}
			value = value.Elem()
		}
		for col, index := range indexes {
			var cell any
			if value.IsValid() && value.Kind() == reflect.Struct {
				cell = recordCell(value.Field(index))
			}
			fields[col].Values = append(fields[col].Values, cell)
		}
	}
	fr, err := New(name, fields...)
	if err != nil {
		return nil, err
	}
	return NewFrameSet(fr)
}
func fieldTypeFor(t reflect.Type) FieldType {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.String:
		return FieldTypeString
	case reflect.Bool:
		return FieldTypeBoolean
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
		return FieldTypeNumber
	case reflect.Invalid, reflect.Uintptr, reflect.Complex64, reflect.Complex128, reflect.Array, reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice, reflect.Struct, reflect.UnsafePointer:
		return FieldTypeUnknown
	}
	return FieldTypeUnknown
}
func recordCell(value reflect.Value) any {
	if !value.IsValid() {
		return nil
	}
	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}
	if value.CanInterface() {
		raw := value.Interface()
		switch raw.(type) {
		case Formula, *Formula, Hyperlink, *Hyperlink:
			return raw
		}
		switch value.Kind() {
		case reflect.String, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
			return raw
		case reflect.Invalid, reflect.Uintptr, reflect.Complex64, reflect.Complex128, reflect.Array, reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice, reflect.Struct, reflect.UnsafePointer:
		}
		data, err := json.Marshal(raw)
		if err == nil {
			return string(data)
		}
		return fmt.Sprint(raw)
	}
	return nil
}
