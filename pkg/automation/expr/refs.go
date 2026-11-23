package expr

import (
	"cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/operators"
	"cel.dev/cel-go/common/types"
)

// IndexStep stands in a chain of References for an index that is not a string literal: `items[0].url` is
// [items, IndexStep, url].
const IndexStep = "[]"

// References lists the select chains of src that start at a name not bound inside the expression:
// `record.title`, `event.mail.subject`; `has(event.mail.x)` gives the chain of its field too. An index by a
// string literal continues a chain (`m["a"]` is `m.a`); any other index continues it with IndexStep (its own
// expression is read for chains too). The chains come in source
// order, each once. A source that does not parse has none.
func (e *Env) References(src string) [][]string {
	parsed, iss := e.cel.Parse(src)
	if iss != nil && iss.Err() != nil {
		return nil
	}
	var out [][]string
	seen := map[string]bool{}
	add := func(chain []string) {
		key := joinPath(strings2any(chain))
		if !seen[key] {
			seen[key] = true
			out = append(out, chain)
		}
	}
	var walk func(x ast.Expr, bound map[string]bool)
	walk = func(x ast.Expr, bound map[string]bool) {
		if base, fields, indexes := peelChain(x); base != nil {
			for _, ix := range indexes {
				walk(ix, bound)
			}
			if base.Kind() != ast.IdentKind {
				walk(base, bound) // the chain starts at a call or a literal, not at a name
				return
			}
			if name := base.AsIdent(); !bound[name] {
				add(append([]string{name}, fields...))
			}
			return
		}
		switch x.Kind() { //nolint:exhaustive // a literal reads no chain
		case ast.IdentKind:
			if name := x.AsIdent(); !bound[name] {
				add([]string{name})
			}
		case ast.CallKind:
			c := x.AsCall()
			if c.IsMemberFunction() {
				walk(c.Target(), bound)
			}
			for _, arg := range c.Args() {
				walk(arg, bound)
			}
		case ast.ListKind:
			for _, el := range x.AsList().Elements() {
				walk(el, bound)
			}
		case ast.MapKind:
			for _, en := range x.AsMap().Entries() {
				me := en.AsMapEntry()
				walk(me.Key(), bound)
				walk(me.Value(), bound)
			}
		case ast.StructKind:
			for _, f := range x.AsStruct().Fields() {
				walk(f.AsStructField().Value(), bound)
			}
		case ast.ComprehensionKind:
			c := x.AsComprehension()
			walk(c.IterRange(), bound)
			walk(c.AccuInit(), bound)
			inner := map[string]bool{c.IterVar(): true, c.AccuVar(): true}
			if c.HasIterVar2() {
				inner[c.IterVar2()] = true
			}
			for name := range bound {
				inner[name] = true
			}
			for _, sub := range []ast.Expr{c.LoopCondition(), c.LoopStep(), c.Result()} {
				walk(sub, inner)
			}
		}
	}
	walk(parsed.NativeRep().Expr(), map[string]bool{})
	return out
}

// peelChain strips the selects and the indexes off x: its base, the field names (IndexStep for an index that
// is not a string literal) and the index expressions. base is nil when x is neither.
func peelChain(x ast.Expr) (ast.Expr, []string, []ast.Expr) {
	var fields []string
	var indexes []ast.Expr
	base := x
	for {
		switch base.Kind() { //nolint:exhaustive // only selects and indexes continue a chain
		case ast.SelectKind:
			s := base.AsSelect()
			fields = append(fields, s.FieldName())
			base = s.Operand()
			continue
		case ast.CallKind:
			c := base.AsCall()
			if c.FunctionName() == operators.Index && len(c.Args()) == 2 {
				var key types.String
				isKey := false
				if arg := c.Args()[1]; arg.Kind() == ast.LiteralKind {
					key, isKey = arg.AsLiteral().(types.String)
				}
				if isKey {
					fields = append(fields, string(key))
				} else {
					fields = append(fields, IndexStep)
					indexes = append(indexes, c.Args()[1])
				}
				base = c.Args()[0]
				continue
			}
		}
		break
	}
	if len(fields) == 0 {
		return nil, nil, nil
	}
	for i, j := 0, len(fields)-1; i < j; i, j = i+1, j-1 {
		fields[i], fields[j] = fields[j], fields[i]
	}
	return base, fields, indexes
}
