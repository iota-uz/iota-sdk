package expr

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/operators"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/common/types/traits"
)

// chain is a select chain of the checked AST from a scope variable: `nodes.hold.output.event.number`.
type chain struct {
	ids     map[int64]bool // the ident and every select of the chain
	path    []string       // variable, then field names
	declLen int            // leading segments that are declared fields (the variable counts)
	offset  int32          // where the chain starts in the source
}

// chains lists the maximal select chains rooted at scope variables. Presence tests (`has`) end a chain
// before their field; index and call operands are not chains.
func (e *Env) chains(a *ast.AST) []chain {
	var out []chain
	refs := a.ReferenceMap()
	var visit func(x ast.NavigableExpr)
	visit = func(x ast.NavigableExpr) {
		if _, _, isStep := chainStep(x); isStep {
			parent, ok := x.Parent()
			top := true
			if ok {
				if operand, _, parentStep := chainStep(parent); parentStep && operand.ID() == x.ID() {
					top = false
				}
			}
			if top {
				if c, ok := e.chainOf(a, refs, x); ok {
					out = append(out, c)
				}
			}
		}
		for _, child := range x.Children() {
			visit(child)
		}
	}
	visit(ast.NavigateAST(a))
	sort.SliceStable(out, func(i, j int) bool { return out[i].offset < out[j].offset })
	return out
}

// chainStep is one step of a chain: a select (not a presence test) or an index by a constant string,
// `m.a` and `m["a"]` alike.
func chainStep(x ast.Expr) (ast.Expr, string, bool) {
	switch x.Kind() { //nolint:exhaustive // only selects and indexes are steps of a chain
	case ast.SelectKind:
		s := x.AsSelect()
		if s.IsTestOnly() {
			return nil, "", false
		}
		return s.Operand(), s.FieldName(), true
	case ast.CallKind:
		c := x.AsCall()
		if c.FunctionName() != operators.Index || len(c.Args()) != 2 || c.Args()[1].Kind() != ast.LiteralKind {
			return nil, "", false
		}
		key, ok := c.Args()[1].AsLiteral().(types.String)
		if !ok {
			return nil, "", false
		}
		return c.Args()[0], string(key), true
	}
	return nil, "", false
}

func (e *Env) chainOf(a *ast.AST, refs map[int64]*ast.ReferenceInfo, top ast.Expr) (chain, bool) {
	var fields []string
	ids := map[int64]bool{}
	x := top
	for {
		operand, field, ok := chainStep(x)
		if !ok {
			break
		}
		ids[x.ID()] = true
		fields = append(fields, field)
		x = operand
	}
	if x.Kind() != ast.IdentKind {
		return chain{}, false
	}
	name := x.AsIdent()
	if r, ok := refs[x.ID()]; !ok || r.Name != name || r.Value != nil {
		return chain{}, false // a comprehension variable or a type name
	}
	f, ok := e.scope.Vars[name]
	if !ok {
		return chain{}, false
	}
	ids[x.ID()] = true
	path := []string{name}
	for i := len(fields) - 1; i >= 0; i-- {
		path = append(path, fields[i])
	}
	declLen := 1
	for _, seg := range path[1:] {
		next, ok := f.Prop(seg)
		if !ok {
			break
		}
		f = next
		declLen++
	}
	offset := int32(math.MaxInt32)
	if r, ok := a.SourceInfo().GetOffsetRange(x.ID()); ok {
		offset = r.Start
	}
	return chain{ids: ids, path: path, declLen: declLen, offset: offset}, true
}

// missingPath is the path of the first unknown in the source: the chain of the field the expression reads
// through declared fields, cut at the missing key of a dyn object.
func (p *Program) missingPath(unk *types.Unknown) string {
	best, bestOffset := "", int32(math.MaxInt32)
	for _, id := range unk.IDs() {
		trails, _ := unk.GetAttributeTrails(id)
		for _, trail := range trails {
			pat := trailSegments(trail)
			path, offset := p.pathFor(id, pat)
			if offset < bestOffset || best == "" {
				best, bestOffset = path, offset
			}
		}
	}
	if best == "" {
		return "context"
	}
	return best
}

func (p *Program) pathFor(id int64, pat []string) (string, int32) {
	for _, c := range p.chains {
		if !c.ids[id] || len(c.path) < len(pat) || !hasPrefix(c.path, pat) {
			continue
		}
		n := c.declLen
		if len(pat) > n {
			n = len(pat)
		}
		return joinPath(c.path[:n]), c.offset
	}
	// An unknown read from a dyn object carries no expression id: find the chain that reads its path.
	for _, c := range p.chains {
		if len(c.path) >= len(pat) && hasPrefix(c.path, pat) {
			return joinPath(pat), c.offset
		}
	}
	offset := int32(math.MaxInt32)
	if r, ok := p.ast.SourceInfo().GetOffsetRange(id); ok {
		offset = r.Start
	}
	return joinPath(pat), offset
}

func hasPrefix(path, prefix []string) bool {
	for i, s := range prefix {
		if path[i] != s {
			return false
		}
	}
	return true
}

func trailSegments(t *types.AttributeTrail) []string {
	out := []string{t.Variable()}
	for _, q := range t.QualifierPath() {
		switch v := q.(type) {
		case string:
			out = append(out, v)
		default:
			out = append(out, fmt.Sprintf("[%v]", v))
		}
	}
	return out
}

// ToNative converts a CEL value to Go: int64, float64, string, bool, nil, time.Time, time.Duration,
// []any and map[string]any.
func ToNative(v ref.Val) (any, error) {
	switch x := v.(type) {
	case types.Bool:
		return bool(x), nil
	case types.Int:
		return int64(x), nil
	case types.Uint:
		return uint64(x), nil
	case types.Double:
		return float64(x), nil
	case types.String:
		return string(x), nil
	case types.Null:
		return nil, nil //nolint:nilnil // CEL null is a Go nil value, not a missing one
	case types.Timestamp:
		return x.UTC(), nil
	case types.Duration:
		return x.Duration, nil
	case *types.Err:
		return nil, x
	case *dynMap:
		return dynNative(x.m)
	}
	if l, ok := v.(traits.Lister); ok {
		n, ok := l.Size().(types.Int)
		if !ok {
			return nil, fmt.Errorf("a list without a size")
		}
		out := make([]any, 0, int(n))
		for i := types.Int(0); i < n; i++ {
			e, err := ToNative(l.Get(i))
			if err != nil {
				return nil, err
			}
			out = append(out, e)
		}
		return out, nil
	}
	if m, ok := v.(traits.Mapper); ok {
		out := map[string]any{}
		it := m.Iterator()
		for it.HasNext() == types.True {
			k := it.Next()
			val, err := ToNative(m.Get(k))
			if err != nil {
				return nil, err
			}
			key := fmt.Sprint(k.Value())
			if _, dup := out[key]; dup {
				return nil, fmt.Errorf("map keys %s collide as JSON keys", key)
			}
			out[key] = val
		}
		return out, nil
	}
	switch raw := v.Value().(type) {
	case map[string]any:
		return dynNative(raw)
	}
	return v.Value(), nil
}

// dynNative unwraps nested dyn maps, inside maps and lists.
func dynNative(m map[string]any) (map[string]any, error) {
	out := make(map[string]any, len(m))
	for k, v := range m {
		n, err := nativeValue(v)
		if err != nil {
			return nil, err
		}
		out[k] = n
	}
	return out, nil
}

func nativeValue(v any) (any, error) {
	switch x := v.(type) {
	case *dynMap:
		return dynNative(x.m)
	case map[string]any:
		return dynNative(x)
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			n, err := nativeValue(e)
			if err != nil {
				return nil, err
			}
			out[i] = n
		}
		return out, nil
	}
	return v, nil
}

// joinPath writes a path: keys joined with dots, list indices as [i].
func joinPath(segments []string) string {
	var b strings.Builder
	for i, s := range segments {
		if i > 0 && !strings.HasPrefix(s, "[") {
			b.WriteByte('.')
		}
		b.WriteString(s)
	}
	return b.String()
}
