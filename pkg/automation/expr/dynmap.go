package expr

import (
	"reflect"

	"cel.dev/cel-go/common/operators"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/common/types/traits"
	"cel.dev/cel-go/interpreter"
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
	path []segment // keys (string) and list indices (int64)
	deep bool      // nested objects are dyn too (a dyn value), not declared ones (a map of objects)
}

var (
	_ traits.Indexer     = (*dynMap)(nil)
	_ traits.FieldTester = (*dynMap)(nil)
	_ traits.Container   = (*dynMap)(nil)
	_ traits.Sizer       = (*dynMap)(nil)
	_ traits.Iterable    = (*dynMap)(nil)
)

func newDynMap(m map[string]any, path []segment, deep bool) *dynMap {
	return &dynMap{m: m, path: append([]segment(nil), path...), deep: deep}
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
		root, _ := d.path[0].(string)
		trail := types.NewAttributeTrail(root)
		for _, s := range d.path[1:] {
			switch q := s.(type) {
			case string:
				types.QualifyAttribute[string](trail, q)
			case int64:
				types.QualifyAttribute[int64](trail, q)
			}
		}
		types.QualifyAttribute[string](trail, string(k))
		return types.NewUnknown(0, trail)
	}
	return d.wrap(v, appendPath(d.path, string(k)))
}

// wrap reads a nested dyn value: objects stay dynMaps with their path, lists keep their indices in it.
func (d *dynMap) wrap(v any, path []segment) ref.Val {
	if !d.deep {
		return types.DefaultTypeAdapter.NativeToValue(v)
	}
	switch x := v.(type) {
	case map[string]any:
		return &dynMap{m: x, path: path, deep: true}
	case []any:
		items := make([]ref.Val, len(x))
		for i, e := range x {
			items[i] = d.wrap(e, appendPath(path, int64(i)))
		}
		return types.NewRefValList(types.DefaultTypeAdapter, items)
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

// Iterator yields the keys in sorted order: a comprehension over a map is deterministic.
func (d *dynMap) Iterator() traits.Iterator {
	keys := sortedKeys(d.m)
	return types.DefaultTypeAdapter.NativeToValue(keys).(traits.Lister).Iterator()
}

// plainEquality makes == and != independent of operand order: CEL asks the left operand, and a native map
// does not recognize a dynMap, so both operands are compared as plain values.
func plainEquality(i interpreter.InterpretableV2) (interpreter.InterpretableV2, error) {
	call, ok := i.(interpreter.InterpretableCall)
	if !ok || len(call.Args()) != 2 {
		return i, nil
	}
	switch call.Function() {
	case operators.Equals:
		return &plainEq{InterpretableCall: call}, nil
	case operators.NotEquals:
		return &plainEq{InterpretableCall: call, negate: true}, nil
	}
	return i, nil
}

type plainEq struct {
	interpreter.InterpretableCall
	negate bool
}

func (e *plainEq) Exec(frame *interpreter.ExecutionFrame) ref.Val {
	args := e.Args()
	return e.compare(args[0].Exec(frame), args[1].Exec(frame))
}

func (e *plainEq) Eval(act interpreter.Activation) ref.Val {
	args := e.Args()
	return e.compare(args[0].Eval(act), args[1].Eval(act))
}

func (e *plainEq) compare(l, r ref.Val) ref.Val {
	if types.IsUnknownOrError(l) {
		return l
	}
	if types.IsUnknownOrError(r) {
		return r
	}
	out := types.Equal(plainVal(l), plainVal(r))
	if b, ok := out.(types.Bool); ok && e.negate {
		return !b
	}
	return out
}

// plainVal turns a value holding dynMaps into native CEL values.
func plainVal(v ref.Val) ref.Val {
	switch v.(type) {
	case *dynMap, traits.Lister, traits.Mapper:
		native, err := ToNative(v)
		if err != nil {
			return v
		}
		return types.DefaultTypeAdapter.NativeToValue(native)
	}
	return v
}
