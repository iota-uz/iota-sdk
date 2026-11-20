package expr

import (
	"reflect"

	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/common/types/traits"
)

// dynMap is a dyn object or a map of the context as CEL reads it. Reading a key it lacks is an unknown
// whose trail is the key's path, so an expression that needs the key reports missing context and one that
// does not (`has(event.a) ? event.a : ""`, `"a" in m`, `missing || true`) is unaffected: presence tests
// and `in` see the key as absent.
//
// It is deliberately not a traits.Mapper: CEL resolves a select on a Mapper with Find and turns an absent
// key into an error; on an Indexer it tests presence with IsSet and reads with Get.
type dynMap struct {
	m    map[string]any
	path []string
	deep bool // nested objects are dyn too (a dyn value), not declared ones (a map of objects)
}

var (
	_ traits.Indexer     = (*dynMap)(nil)
	_ traits.FieldTester = (*dynMap)(nil)
	_ traits.Container   = (*dynMap)(nil)
	_ traits.Sizer       = (*dynMap)(nil)
	_ traits.Iterable    = (*dynMap)(nil)
)

func newDynMap(m map[string]any, path []segment, deep bool) *dynMap {
	p := make([]string, 0, len(path))
	for _, s := range path {
		if str, ok := s.(string); ok {
			p = append(p, str)
		}
	}
	return &dynMap{m: m, path: p, deep: deep}
}

func (d *dynMap) base() traits.Mapper {
	return types.DefaultTypeAdapter.NativeToValue(d.m).(traits.Mapper)
}

func (d *dynMap) ConvertToNative(t reflect.Type) (any, error) { return d.base().ConvertToNative(t) }

func (d *dynMap) ConvertToType(t ref.Type) ref.Val {
	if t == types.TypeType {
		return types.MapType
	}
	return d.base().ConvertToType(t)
}

func (d *dynMap) Equal(other ref.Val) ref.Val {
	if o, ok := other.(*dynMap); ok {
		other = o.base()
	}
	return d.base().Equal(other)
}

func (d *dynMap) Type() ref.Type { return types.MapType }

func (d *dynMap) Value() any { return d.m }

// Get reads a key; an absent one is an unknown at its path.
func (d *dynMap) Get(key ref.Val) ref.Val {
	k, ok := key.(types.String)
	if !ok {
		return d.base().Get(key)
	}
	v, has := d.m[string(k)]
	if !has {
		trail := types.NewAttributeTrail(d.path[0])
		for _, s := range d.path[1:] {
			types.QualifyAttribute[string](trail, s)
		}
		types.QualifyAttribute[string](trail, string(k))
		return types.NewUnknown(0, trail)
	}
	if nested, ok := v.(map[string]any); ok && d.deep {
		return &dynMap{m: nested, path: append(append([]string{}, d.path...), string(k)), deep: true}
	}
	return types.DefaultTypeAdapter.NativeToValue(v)
}

// IsSet is the presence test of `has()`.
func (d *dynMap) IsSet(key ref.Val) ref.Val {
	k, ok := key.(types.String)
	if !ok {
		return types.False
	}
	_, has := d.m[string(k)]
	return types.Bool(has)
}

func (d *dynMap) Contains(key ref.Val) ref.Val {
	return d.base().Contains(key)
}

func (d *dynMap) Size() ref.Val { return types.Int(len(d.m)) }

func (d *dynMap) Iterator() traits.Iterator { return d.base().Iterator() }
