package expr

import (
	"fmt"
	"reflect"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/common/types/traits"
)

// Lookup is a map-typed context value that is read one key at a time: a variable whose value is a Lookup
// is a map<string, dyn> to the expression, and the host is asked only for the keys the expression reads
// (the automation's memory, state, whose keys may be many and large). A key it does not have is ok=false;
// an error fails the expression.
type Lookup interface {
	Lookup(key string) (value any, ok bool, err error)
}

// lazyMap is a Lookup as CEL reads it: `m.get(k)`, `m.has(k)`, `m[k]` and `k in m`. It cannot be counted or
// iterated, since the host does not list its keys.
type lazyMap struct {
	lk   Lookup
	path []segment
}

var (
	_ ref.Val          = (*lazyMap)(nil)
	_ traits.Indexer   = (*lazyMap)(nil)
	_ traits.Container = (*lazyMap)(nil)
)

func (*lazyMap) ConvertToNative(reflect.Type) (any, error) {
	return nil, fmt.Errorf("a lookup-only map cannot be converted: read a key with get(key)")
}

func (*lazyMap) ConvertToType(t ref.Type) ref.Val {
	if t == types.TypeType {
		return types.MapType
	}
	return types.NewErr("a lookup-only map cannot be converted: read a key with get(key)")
}

func (l *lazyMap) Equal(other ref.Val) ref.Val { return types.Bool(l == other) }

func (*lazyMap) Type() ref.Type { return types.MapType }

func (l *lazyMap) Value() any { return l.lk }

// Get is m[k]: the value, or an error for a key it lacks.
func (l *lazyMap) Get(key ref.Val) ref.Val {
	v, ok, err := l.find(key)
	switch {
	case err != nil:
		return err
	case !ok:
		return types.NewErr("no such key: %v", key.Value())
	}
	return v
}

// Contains is `k in m`.
func (l *lazyMap) Contains(key ref.Val) ref.Val {
	_, ok, err := l.find(key)
	if err != nil {
		return err
	}
	return types.Bool(ok)
}

func (l *lazyMap) find(key ref.Val) (ref.Val, bool, ref.Val) {
	k, ok := key.(types.String)
	if !ok {
		return nil, false, types.MaybeNoSuchOverloadErr(key)
	}
	v, found, err := l.lk.Lookup(string(k))
	if err != nil {
		return nil, false, types.NewErr("%s", err.Error())
	}
	if !found {
		return nil, false, nil
	}
	path := appendPath(l.path, string(k))
	d, derr := dynValue(v, path)
	if derr != nil {
		return nil, false, types.NewErr("%s", derr.Error())
	}
	return types.DefaultTypeAdapter.NativeToValue(wrapDyn(d, path)), true, nil
}

// mapLookupFunctions declare the member functions of a map<string, dyn>: `m.get(key)` is the value or null,
// `m.has(key)` tells whether the key is there. They work on every such map and are what the automation's
// memory (state) is read with, since a key may be any text.
func mapLookupFunctions() []cel.EnvOption {
	mapType := cel.DynType // a map<string, dyn>, or a dyn value that is one: the receiver is checked when it is read
	return []cel.EnvOption{
		cel.Function("get",
			cel.MemberOverload("map_get_string", []*cel.Type{mapType, cel.StringType}, cel.DynType,
				cel.BinaryBinding(func(m, key ref.Val) ref.Val {
					v, ok, err := mapFind(m, key)
					switch {
					case err != nil:
						return err
					case !ok:
						return types.NullValue
					}
					return v
				}))),
		cel.Function("has",
			cel.MemberOverload("map_has_string", []*cel.Type{mapType, cel.StringType}, cel.BoolType,
				cel.BinaryBinding(func(m, key ref.Val) ref.Val {
					_, ok, err := mapFind(m, key)
					if err != nil {
						return err
					}
					return types.Bool(ok)
				}))),
	}
}

// mapFind reads a key of any map value the context holds: a lookup-only map, a dyn map of the context or a
// CEL map.
func mapFind(m, key ref.Val) (ref.Val, bool, ref.Val) {
	k, ok := key.(types.String)
	if !ok {
		return nil, false, types.MaybeNoSuchOverloadErr(key)
	}
	switch x := m.(type) {
	case *lazyMap:
		return x.find(key)
	case *dynMap:
		v, has := x.m[string(k)]
		if !has {
			return nil, false, nil
		}
		return x.wrap(v, appendPath(x.path, string(k))), true, nil
	case traits.Mapper:
		v, has := x.Find(key)
		return v, has, nil
	}
	return nil, false, types.MaybeNoSuchOverloadErr(m)
}
