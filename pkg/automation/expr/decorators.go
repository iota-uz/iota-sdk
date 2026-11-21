package expr

import (
	"fmt"
	"sort"

	"cel.dev/cel-go/common/operators"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/common/types/traits"
	"cel.dev/cel-go/interpreter"
)

// Program decorators that keep evaluation deterministic and dyn maps transparent.

// plainEquality makes == and != independent of operand order: CEL asks the left operand, and a native map
// does not recognize a dynMap, so dynMaps on either side are compared as plain CEL values.
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

// plainVal replaces dynMaps inside a value with native CEL maps. Keys keep their CEL types: `{1: "x"}` and
// `{"1": "x"}` stay different.
func plainVal(v ref.Val) ref.Val {
	switch x := v.(type) {
	case *dynMap:
		out := make(map[ref.Val]ref.Val, len(x.m))
		for k, e := range x.m {
			out[types.String(k)] = plainVal(types.DefaultTypeAdapter.NativeToValue(e))
		}
		return types.NewRefValMap(types.DefaultTypeAdapter, out)
	case traits.Mapper:
		out := map[ref.Val]ref.Val{}
		it := x.Iterator()
		for it.HasNext() == types.True {
			k := it.Next()
			out[k] = plainVal(x.Get(k))
		}
		return types.NewRefValMap(types.DefaultTypeAdapter, out)
	case traits.Lister:
		n, ok := x.Size().(types.Int)
		if !ok {
			return v
		}
		out := make([]ref.Val, int(n))
		for i := range out {
			out[i] = plainVal(x.Get(types.Int(i)))
		}
		return types.NewRefValList(types.DefaultTypeAdapter, out)
	}
	return v
}

// sortedMapLiterals makes a map built in the expression iterate its keys in order, so a comprehension
// over a map literal (`{"b": 2, "a": 1}.map(k, k)`) gives the same list every time.
func sortedMapLiterals(i interpreter.InterpretableV2) (interpreter.InterpretableV2, error) {
	c, ok := i.(interpreter.InterpretableConstructor)
	if !ok || c.Type() != types.MapType {
		return i, nil
	}
	return &sortedMapLiteral{InterpretableConstructor: c}, nil
}

type sortedMapLiteral struct {
	interpreter.InterpretableConstructor
}

func (s *sortedMapLiteral) Exec(frame *interpreter.ExecutionFrame) ref.Val {
	return sortMap(s.InterpretableConstructor.Exec(frame))
}

func (s *sortedMapLiteral) Eval(act interpreter.Activation) ref.Val {
	return sortMap(s.InterpretableConstructor.Eval(act))
}

func sortMap(v ref.Val) ref.Val {
	if m, ok := v.(traits.Mapper); ok {
		return &sortedMap{Mapper: m}
	}
	return v
}

// sortedMap is a CEL map whose iteration is ordered by key: by type, then value.
type sortedMap struct {
	traits.Mapper
}

func (m *sortedMap) Iterator() traits.Iterator {
	var keys []ref.Val
	it := m.Mapper.Iterator()
	for it.HasNext() == types.True {
		keys = append(keys, it.Next())
	}
	sort.Slice(keys, func(i, j int) bool { return keyLess(keys[i], keys[j]) })
	return types.NewRefValList(types.DefaultTypeAdapter, keys).Iterator()
}

func (m *sortedMap) Equal(other ref.Val) ref.Val {
	if o, ok := other.(*sortedMap); ok {
		other = o.Mapper
	}
	return m.Mapper.Equal(plainVal(other))
}

func keyLess(a, b ref.Val) bool {
	ta, tb := a.Type().TypeName(), b.Type().TypeName()
	if ta != tb {
		return ta < tb
	}
	switch x := a.(type) {
	case types.String:
		return x < b.(types.String)
	case types.Int:
		return x < b.(types.Int)
	case types.Uint:
		return x < b.(types.Uint)
	case types.Bool:
		return !bool(x) && bool(b.(types.Bool))
	}
	return fmt.Sprint(a.Value()) < fmt.Sprint(b.Value())
}

// unknownIDs gives an unknown read from a dyn map the id of the attribute that read it: the map does not
// know the expression, the attribute does, and missingPath orders unknowns by where they were read.
func unknownIDs(i interpreter.InterpretableV2) (interpreter.InterpretableV2, error) {
	if a, ok := i.(interpreter.InterpretableAttribute); ok {
		return &idAttribute{InterpretableAttribute: a}, nil
	}
	return i, nil
}

type idAttribute struct {
	interpreter.InterpretableAttribute
}

func (a *idAttribute) Exec(frame *interpreter.ExecutionFrame) ref.Val {
	return withID(a.ID(), a.InterpretableAttribute.Exec(frame))
}

func (a *idAttribute) Eval(act interpreter.Activation) ref.Val {
	return withID(a.ID(), a.InterpretableAttribute.Eval(act))
}

func withID(id int64, v ref.Val) ref.Val {
	unk, ok := v.(*types.Unknown)
	if !ok {
		return v
	}
	trails, ok := unk.GetAttributeTrails(0)
	if !ok {
		return v
	}
	var out *types.Unknown
	for _, other := range unk.IDs() {
		if other == 0 {
			continue
		}
		ts, _ := unk.GetAttributeTrails(other)
		for _, t := range ts {
			out = types.MergeUnknowns(out, types.NewUnknown(other, t))
		}
	}
	for _, t := range trails {
		out = types.MergeUnknowns(out, types.NewUnknown(id, t))
	}
	return out
}
