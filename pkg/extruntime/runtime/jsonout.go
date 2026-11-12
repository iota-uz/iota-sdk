package runtime

import (
	"encoding"
	"encoding/json"
	"reflect"
)

var (
	jsonMarshaler = reflect.TypeFor[json.Marshaler]()
	textMarshaler = reflect.TypeFor[encoding.TextMarshaler]()
)

// withEmptySlices returns v with every nil slice and map replaced by an empty
// one, so a list that is empty is encoded as [] (and a map as {}) rather than
// null: the OpenAPI document describes them as an array and an object. Byte
// slices (json.RawMessage among them), pointers and values that marshal
// themselves are left as they are. v is not modified.
func withEmptySlices(v any) any {
	if v == nil {
		return nil
	}
	return emptied(reflect.ValueOf(v)).Interface()
}

func marshalsItself(t reflect.Type) bool {
	return t.Implements(jsonMarshaler) || t.Implements(textMarshaler) ||
		reflect.PointerTo(t).Implements(jsonMarshaler) || reflect.PointerTo(t).Implements(textMarshaler)
}

func emptied(v reflect.Value) reflect.Value {
	t := v.Type()
	if marshalsItself(t) {
		return v
	}
	switch v.Kind() {
	case reflect.Interface:
		if v.IsNil() {
			return v
		}
		out := reflect.New(t).Elem()
		out.Set(emptied(v.Elem()))
		return out
	case reflect.Pointer:
		if v.IsNil() {
			return v
		}
		out := reflect.New(t.Elem())
		out.Elem().Set(emptied(v.Elem()))
		return out
	case reflect.Struct:
		out := reflect.New(t).Elem()
		out.Set(v)
		for i := range t.NumField() {
			if t.Field(i).IsExported() {
				out.Field(i).Set(emptied(v.Field(i)))
			}
		}
		return out
	case reflect.Slice:
		if t.Elem().Kind() == reflect.Uint8 {
			return v
		}
		out := reflect.MakeSlice(t, v.Len(), v.Len())
		for i := range v.Len() {
			out.Index(i).Set(emptied(v.Index(i)))
		}
		return out
	case reflect.Array:
		out := reflect.New(t).Elem()
		for i := range v.Len() {
			out.Index(i).Set(emptied(v.Index(i)))
		}
		return out
	case reflect.Map:
		out := reflect.MakeMapWithSize(t, v.Len())
		for _, k := range v.MapKeys() {
			out.SetMapIndex(k, emptied(v.MapIndex(k)))
		}
		return out
	case reflect.Invalid, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128, reflect.Chan, reflect.Func,
		reflect.String, reflect.UnsafePointer:
		return v
	}
	return v
}
