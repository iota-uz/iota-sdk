package composition

import (
	"fmt"
	"reflect"
	"strings"
)

type Key struct {
	Type reflect.Type
	Name string
}

func KeyFor[T any]() Key {
	return keyFor(typeOf[T](), "")
}

func keyFor(typ reflect.Type, name string) Key {
	if typ == nil {
		panic("composition: key type is required")
	}
	return Key{
		Type: typ,
		Name: strings.TrimSpace(name),
	}
}

func (k Key) DisplayName() string {
	if k.Name != "" {
		return k.Name
	}
	return shortTypeName(k.Type)
}

func (k Key) String() string {
	if k.Name == "" {
		return typeName(k.Type)
	}
	return fmt.Sprintf("%s[%s]", typeName(k.Type), k.Name)
}

func typeOf[T any]() reflect.Type {
	return reflect.TypeOf((*T)(nil)).Elem()
}

func typeName(typ reflect.Type) string {
	if typ == nil {
		return "<nil>"
	}
	return typ.String()
}

func shortTypeName(typ reflect.Type) string {
	if typ == nil {
		return "<nil>"
	}
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ.Name() != "" {
		return typ.Name()
	}
	return typ.String()
}
