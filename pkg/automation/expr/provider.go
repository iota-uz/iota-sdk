package expr

import (
	"fmt"
	"strings"

	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
)

// objectTypePrefix names the CEL object types of declared objects; the rest of the name is the field's path.
const objectTypePrefix = "automation."

// provider declares the objects of the field language as CEL object types whose values are plain
// map[string]any: a field the declaration lacks is a check error, not an empty value.
type provider struct {
	*types.Registry
	objects map[string]Field
}

func newProvider() *provider {
	return &provider{Registry: types.NewEmptyRegistry(), objects: map[string]Field{}}
}

// celType is the CEL type of a field; objects are registered under their path.
func (p *provider) celType(f Field, path string) *types.Type {
	switch f.Kind {
	case String, Enum:
		return types.StringType
	case Int:
		return types.IntType
	case Double:
		return types.DoubleType
	case Bool:
		return types.BoolType
	case Timestamp:
		return types.TimestampType
	case List:
		return types.NewListType(p.celType(*f.Elem, path+"[]"))
	case Map:
		return types.NewMapType(types.StringType, p.celType(*f.Elem, path+"{}"))
	case Optional:
		return p.celType(*f.Elem, path)
	case Dyn:
		return types.DynType
	case Object:
		// Paths are unique: variables and properties are identifiers, elements end in [] or {}.
		name := objectTypePrefix + path
		p.objects[name] = f
		for _, prop := range f.Props {
			p.celType(prop.Field, path+"."+prop.Name)
		}
		return types.NewObjectType(name)
	}
	return types.DynType
}

// fieldOf is the declaration behind a CEL type, for rendering objects in declaration order.
func (p *provider) fieldOf(t *types.Type) Field {
	if t == nil {
		return DynField
	}
	switch t.Kind() {
	case types.StructKind:
		if f, ok := p.objects[t.TypeName()]; ok {
			return f
		}
	case types.ListKind:
		return ListOf(p.fieldOf(t.Parameters()[0]))
	case types.MapKind:
		return MapOf(p.fieldOf(t.Parameters()[1]))
	case types.StringKind:
		return StringField
	case types.IntKind:
		return IntField
	case types.DoubleKind:
		return DoubleField
	case types.BoolKind:
		return BoolField
	case types.TimestampKind:
		return TimestampField
	case types.UnspecifiedKind, types.DynKind, types.AnyKind, types.BytesKind, types.DurationKind, types.ErrorKind,
		types.NullTypeKind, types.OpaqueKind, types.TypeKind, types.TypeParamKind, types.UintKind, types.UnknownKind:
		return DynField
	}
	return DynField
}

func (p *provider) FindStructType(name string) (*types.Type, bool) {
	if _, ok := p.objects[name]; ok {
		return types.NewTypeTypeWithParam(types.NewObjectType(name)), true
	}
	return p.Registry.FindStructType(name)
}

func (p *provider) FindStructFieldNames(name string) ([]string, bool) {
	f, ok := p.objects[name]
	if !ok {
		return p.Registry.FindStructFieldNames(name)
	}
	names := make([]string, 0, len(f.Props))
	for _, prop := range f.Props {
		names = append(names, prop.Name)
	}
	return names, true
}

func (p *provider) FindStructFieldType(name, field string) (*types.FieldType, bool) {
	f, ok := p.objects[name]
	if !ok {
		return p.Registry.FindStructFieldType(name, field)
	}
	prop, ok := Fields(f.Props).Get(field)
	if !ok {
		return nil, false
	}
	path := strings.TrimPrefix(name, objectTypePrefix) + "." + field
	return &types.FieldType{
		Type: p.typeOfProp(prop, path),
		IsSet: func(obj any) bool {
			m, ok := obj.(map[string]any)
			if !ok {
				return false
			}
			_, has := m[field]
			return has
		},
		GetFrom: func(obj any) (any, error) {
			m, ok := obj.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("no such key: %s", field)
			}
			v, has := m[field]
			if !has {
				return nil, fmt.Errorf("no such key: %s", field)
			}
			return v, nil
		},
	}, true
}

// typeOfProp is the CEL type of a property; its objects are already registered by celType.
func (p *provider) typeOfProp(f Field, path string) *types.Type {
	switch f.Kind {
	case Optional:
		return p.typeOfProp(*f.Elem, path)
	case List:
		return types.NewListType(p.typeOfProp(*f.Elem, path+"[]"))
	case Map:
		return types.NewMapType(types.StringType, p.typeOfProp(*f.Elem, path+"{}"))
	case Object:
		return types.NewObjectType(objectTypePrefix + path)
	case String, Int, Double, Bool, Dyn, Enum, Timestamp:
		return p.celType(f, path)
	}
	return p.celType(f, path)
}

// NewValue refuses object literals of declared types: values come from the context.
func (p *provider) NewValue(name string, fields map[string]ref.Val) ref.Val {
	if _, ok := p.objects[name]; ok {
		return types.NewErr("%s cannot be built in an expression", name)
	}
	return p.Registry.NewValue(name, fields)
}
