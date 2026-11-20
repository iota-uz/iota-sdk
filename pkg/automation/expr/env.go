package expr

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/ext"
)

// Type is the type an expression must have.
type Type string

const (
	// TypeBool is a CEL bool (or dyn).
	TypeBool Type = "bool"
	// TypeString is a CEL string (or dyn).
	TypeString Type = "string"
	// TypeMap is a map or a declared object (or dyn), e.g. `{"title": issue.title}`.
	TypeMap Type = "map"
	// TypeAny is any type.
	TypeAny Type = "any"
)

// Scope is what an expression may read: event, nodes, the subject's variables, tenant, now.
type Scope struct {
	Vars map[string]Field
}

// Limits bound an expression at check and evaluation time.
type Limits struct {
	MaxNodes   int    // nodes of the source AST, a macro being one call
	MaxDepth   int    // depth of the AST, chains of one operator collapsed
	MaxLiteral int    // elements of a list or map literal
	MaxArgs    int    // arguments of a call
	CostLimit  uint64 // runtime cost
}

// DefaultLimits are the limits of the format.
func DefaultLimits() Limits {
	return Limits{MaxNodes: 2000, MaxDepth: 64, MaxLiteral: 200, MaxArgs: 16, CostLimit: 1_000_000}
}

func (l Limits) withDefaults() Limits {
	d := DefaultLimits()
	if l.MaxNodes <= 0 {
		l.MaxNodes = d.MaxNodes
	}
	if l.MaxDepth <= 0 {
		l.MaxDepth = d.MaxDepth
	}
	if l.MaxLiteral <= 0 {
		l.MaxLiteral = d.MaxLiteral
	}
	if l.MaxArgs <= 0 {
		l.MaxArgs = d.MaxArgs
	}
	if l.CostLimit == 0 {
		l.CostLimit = d.CostLimit
	}
	return l
}

// Issue codes: what kind of check refused an expression.
const (
	IssueSyntax = "syntax"
	IssueType   = "type"
	IssueLimit  = "limit"
)

// Issue is one reason an expression or template is refused. Line and Column are 1-based positions in
// the source (0 when unknown).
type Issue struct {
	Code    string
	Message string
	Line    int
	Column  int
}

func (i Issue) Error() string {
	if i.Line > 0 {
		return fmt.Sprintf("%s (at %d:%d)", i.Message, i.Line, i.Column)
	}
	return i.Message
}

// Completion is one suggestion for an editor.
type Completion struct {
	Label string `json:"label"`
	Kind  string `json:"kind"` // variable | field | function
	Type  string `json:"type,omitempty"`
}

// Evaluation error codes.
const (
	CodeExpressionFailed = "EXPRESSION_FAILED"
	CodeInvalidInput     = "INVALID_INPUT"
)

// MissingContextError is a required field the expression needs and the context lacks. Path is the
// select chain of the field (`card.tags`, not `card.tags.size`), cut at a missing key of a dyn object.
type MissingContextError struct{ Path string }

func (e *MissingContextError) Error() string { return "missing context: " + e.Path }

// EvalError is an expression that failed: an index out of bounds, a division by zero, an overflow, a value
// that does not fit its field (INVALID_INPUT).
type EvalError struct{ Code, Message string }

func (e *EvalError) Error() string { return e.Code + ": " + e.Message }

// forbiddenOverloads are standard overloads outside the subset: they depend on the host's formatting or
// leave the JS-safe integer range.
var forbiddenOverloads = map[string]bool{
	"timestamp_to_string": true,
	"duration_to_string":  true,
	"uint64_to_int64":     true,
	"timestamp_to_int64":  true,
	"duration_to_int64":   true,
}

// Env is a CEL environment over a scope: standard functions, ext.Strings and ext.Bindings.
type Env struct {
	scope    Scope
	limits   Limits
	cel      *cel.Env
	provider *provider
	names    []string // scope variables, sorted
}

// NewEnv builds the environment of a scope. Build it once per scope and reuse it.
func NewEnv(s Scope, l Limits) (*Env, error) {
	p := newProvider()
	names := make([]string, 0, len(s.Vars))
	for name := range s.Vars {
		names = append(names, name)
	}
	sort.Strings(names)
	opts := []cel.EnvOption{
		cel.CustomTypeProvider(p),
		cel.CrossTypeNumericComparisons(true),
		cel.DefaultUTCTimeZone(true),
		cel.EnableMacroCallTracking(),
		ext.Strings(),
		ext.Bindings(),
	}
	for _, name := range names {
		f := s.Vars[name]
		if err := f.Validate(); err != nil {
			return nil, fmt.Errorf("variable %s: %w", name, err)
		}
		opts = append(opts, cel.Variable(name, p.celType(f, name)))
	}
	env, err := cel.NewEnv(opts...)
	if err != nil {
		return nil, err
	}
	return &Env{scope: s, limits: l.withDefaults(), cel: env, provider: p, names: names}, nil
}

// Scope is the scope the environment was built for.
func (e *Env) Scope() Scope { return e.scope }

// Program is a checked expression, ready to evaluate.
type Program struct {
	env    *Env
	src    string
	ast    *ast.AST
	prg    cel.Program
	out    *types.Type
	chains []chain
}

// Source is the expression's text.
func (p *Program) Source() string { return p.src }

// OutputType is the CEL type the check gave the expression.
func (p *Program) OutputType() string { return p.out.String() }

// Check parses and type-checks src against the scope; want is the type it must have.
func (e *Env) Check(src string, want Type) (*Program, []Issue) {
	parsed, iss := e.cel.Parse(src)
	if iss != nil && iss.Err() != nil {
		return nil, issuesOf(iss, IssueSyntax)
	}
	if issues := e.checkLimits(parsed.NativeRep()); len(issues) > 0 {
		return nil, issues
	}
	checked, iss := e.cel.Check(parsed)
	if iss != nil && iss.Err() != nil {
		return nil, issuesOf(iss, IssueType)
	}
	native := checked.NativeRep()
	if issues := checkSubset(native); len(issues) > 0 {
		return nil, issues
	}
	out := checked.OutputType()
	if !typeMatches(out, want) {
		return nil, []Issue{{Code: IssueType, Message: fmt.Sprintf("must be of type %s, is %s", want, out), Line: 1, Column: 1}}
	}
	prg, err := e.cel.Program(checked,
		cel.EvalOptions(cel.OptPartialEval|cel.OptOptimize),
		cel.CostLimit(e.limits.CostLimit),
	)
	if err != nil {
		// Constant regular expressions are compiled here: RE2 refuses lookaround and backreferences.
		return nil, []Issue{{Code: IssueSyntax, Message: err.Error(), Line: 1, Column: 1}}
	}
	return &Program{env: e, src: src, ast: native, prg: prg, out: out, chains: e.chains(native)}, nil
}

func typeMatches(t *types.Type, want Type) bool {
	if t.Kind() == types.DynKind {
		return true
	}
	switch want {
	case TypeBool:
		return t.Kind() == types.BoolKind
	case TypeString:
		return t.Kind() == types.StringKind
	case TypeMap:
		return t.Kind() == types.MapKind || t.Kind() == types.StructKind
	}
	return true
}

func issuesOf(iss *cel.Issues, code string) []Issue {
	out := []Issue{}
	for _, e := range iss.Errors() {
		i := Issue{Code: code, Message: e.Message}
		if e.Location != nil && e.Location.Line() > 0 {
			i.Line = e.Location.Line()
			i.Column = e.Location.Column() + 1
		}
		out = append(out, i)
	}
	return out
}

// checkSubset refuses what the format leaves out: uint and bytes values and the forbidden overloads.
func checkSubset(a *ast.AST) []Issue {
	var out []Issue
	at := func(id int64, msg string) {
		loc := a.SourceInfo().GetStartLocation(id)
		i := Issue{Code: IssueType, Message: msg}
		if loc.Line() > 0 {
			i.Line, i.Column = loc.Line(), loc.Column()+1
		}
		out = append(out, i)
	}
	ids := make([]int64, 0, len(a.TypeMap()))
	for id := range a.TypeMap() {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		t := a.TypeMap()[id]
		if usesKind(t, types.UintKind) {
			at(id, "uint is not supported: use int")
			return out
		}
		if usesKind(t, types.BytesKind) {
			at(id, "bytes are not supported: use string")
			return out
		}
	}
	for _, id := range ids {
		for _, o := range a.GetOverloadIDs(id) {
			if forbiddenOverloads[o] {
				at(id, fmt.Sprintf("%s is not supported", o))
				return out
			}
		}
	}
	return out
}

func usesKind(t *types.Type, k types.Kind) bool {
	if t == nil {
		return false
	}
	if t.Kind() == k {
		return true
	}
	if t.Kind() == types.TypeKind {
		return false
	}
	for _, p := range t.Parameters() {
		if usesKind(p, k) {
			return true
		}
	}
	return false
}

// checkLimits counts the source AST: a macro is one call over its source arguments.
func (e *Env) checkLimits(a *ast.AST) []Issue {
	l := e.limits
	info := a.SourceInfo()
	macros := info.MacroCalls()
	var issues []Issue
	report := func(id int64, msg string) {
		if len(issues) > 0 {
			return
		}
		loc := info.GetStartLocation(id)
		i := Issue{Code: IssueLimit, Message: msg}
		if loc.Line() > 0 {
			i.Line, i.Column = loc.Line(), loc.Column()+1
		}
		issues = append(issues, i)
	}
	nodes := 0
	maxDepth := 0
	// source resolves an expression to its source form: an expanded macro to its call.
	source := func(x ast.Expr) ast.Expr {
		if m, ok := macros[x.ID()]; ok {
			return m
		}
		return x
	}
	var walk func(x ast.Expr, depth int, parentFn string)
	walk = func(x ast.Expr, depth int, parentFn string) {
		x = source(x)
		nodes++
		fn := ""
		if x.Kind() == ast.CallKind {
			fn = x.AsCall().FunctionName()
			if fn == parentFn && isOperator(fn) {
				depth-- // a chain of one operator is one level
			}
		}
		if depth > maxDepth {
			maxDepth = depth
		}
		switch x.Kind() {
		case ast.CallKind:
			c := x.AsCall()
			if len(c.Args()) > l.MaxArgs {
				report(x.ID(), fmt.Sprintf("a call takes at most %d arguments", l.MaxArgs))
			}
			if c.IsMemberFunction() {
				walk(c.Target(), depth+1, fn)
			}
			for _, arg := range c.Args() {
				walk(arg, depth+1, fn)
			}
		case ast.ListKind:
			elems := x.AsList().Elements()
			if len(elems) > l.MaxLiteral {
				report(x.ID(), fmt.Sprintf("a list literal has at most %d elements", l.MaxLiteral))
			}
			for _, el := range elems {
				walk(el, depth+1, "")
			}
		case ast.MapKind:
			entries := x.AsMap().Entries()
			if len(entries) > l.MaxLiteral {
				report(x.ID(), fmt.Sprintf("a map literal has at most %d entries", l.MaxLiteral))
			}
			for _, en := range entries {
				me := en.AsMapEntry()
				walk(me.Key(), depth+1, "")
				walk(me.Value(), depth+1, "")
			}
		case ast.StructKind:
			for _, f := range x.AsStruct().Fields() {
				walk(f.AsStructField().Value(), depth+1, "")
			}
		case ast.SelectKind:
			walk(x.AsSelect().Operand(), depth+1, "")
		case ast.ComprehensionKind:
			c := x.AsComprehension()
			for _, sub := range []ast.Expr{c.IterRange(), c.AccuInit(), c.LoopCondition(), c.LoopStep(), c.Result()} {
				walk(sub, depth+1, "")
			}
		}
	}
	walk(a.Expr(), 1, "")
	if nodes > l.MaxNodes {
		report(a.Expr().ID(), fmt.Sprintf("the expression has %d nodes, at most %d", nodes, l.MaxNodes))
	}
	if maxDepth > l.MaxDepth {
		report(a.Expr().ID(), fmt.Sprintf("the expression nests %d levels, at most %d", maxDepth, l.MaxDepth))
	}
	return issues
}

func isOperator(fn string) bool {
	return strings.HasPrefix(fn, "_") && strings.HasSuffix(fn, "_") || fn == "@in"
}

// Complete lists what may follow prefix: the fields of the value it selects from, or the variables and
// the functions of the allow-list.
func (e *Env) Complete(prefix string) []Completion {
	head, partial := "", prefix
	if i := strings.LastIndex(prefix, "."); i >= 0 {
		head, partial = prefix[:i], prefix[i+1:]
	}
	var out []Completion
	if head != "" {
		parts := strings.Split(head, ".")
		f, ok := e.scope.Vars[parts[0]]
		if !ok {
			return nil
		}
		for _, p := range parts[1:] {
			if f, ok = f.Prop(p); !ok {
				return nil
			}
		}
		f = f.Unwrap()
		if f.Kind == Object {
			for _, p := range f.Props {
				if strings.HasPrefix(p.Name, partial) {
					out = append(out, Completion{Label: p.Name, Kind: "field", Type: p.Field.TypeName()})
				}
			}
		}
		for _, fn := range e.functions(true) {
			if strings.HasPrefix(fn, partial) {
				out = append(out, Completion{Label: fn, Kind: "function"})
			}
		}
		return out
	}
	for _, name := range e.names {
		if strings.HasPrefix(name, partial) {
			out = append(out, Completion{Label: name, Kind: "variable", Type: e.scope.Vars[name].TypeName()})
		}
	}
	for _, fn := range e.functions(false) {
		if strings.HasPrefix(fn, partial) {
			out = append(out, Completion{Label: fn, Kind: "function"})
		}
	}
	return out
}

// functions are the callable names of the allow-list (member functions or global ones), without
// operators and without functions whose every overload is forbidden.
func (e *Env) functions(member bool) []string {
	var out []string
	for name, fn := range e.cel.Functions() {
		if isOperator(name) || strings.HasPrefix(name, "@") {
			continue
		}
		ok := false
		for _, o := range fn.OverloadDecls() {
			if forbiddenOverloads[o.ID()] || o.IsMemberFunction() != member {
				continue
			}
			ok = true
		}
		if ok {
			out = append(out, name)
		}
	}
	for _, m := range e.cel.Macros() {
		if m.IsReceiverStyle() == member {
			out = append(out, m.Function())
		}
	}
	sort.Strings(out)
	return dedupe(out)
}

func dedupe(s []string) []string {
	out := s[:0]
	for i, v := range s {
		if i == 0 || v != s[i-1] {
			out = append(out, v)
		}
	}
	return out
}

// Eval evaluates the program over the variables. A required field that is absent gives
// *MissingContextError, a failure *EvalError.
func (p *Program) Eval(vars map[string]any) (any, error) {
	v, err := p.evalVal(vars)
	if err != nil {
		return nil, err
	}
	native, err := ToNative(v)
	if err != nil {
		return nil, &EvalError{Code: CodeExpressionFailed, Message: err.Error()}
	}
	return native, nil
}

func (p *Program) evalVal(vars map[string]any) (ref.Val, error) {
	act, patterns, err := p.activation(vars)
	if err != nil {
		return nil, err
	}
	partial, err := cel.PartialVars(act, patterns...)
	if err != nil {
		return nil, &EvalError{Code: CodeExpressionFailed, Message: err.Error()}
	}
	out, _, err := p.prg.Eval(partial)
	if out != nil {
		if unk, ok := out.(*types.Unknown); ok {
			return nil, &MissingContextError{Path: p.missingPath(unk)}
		}
		if e, ok := out.(*types.Err); ok {
			return nil, &EvalError{Code: CodeExpressionFailed, Message: e.String()}
		}
	}
	if err != nil {
		return nil, &EvalError{Code: CodeExpressionFailed, Message: err.Error()}
	}
	return out, nil
}

// activation normalizes the variables by their fields and returns the unknown patterns: required fields
// that are absent and keys of dyn objects the expression reads and the value lacks.
func (p *Program) activation(vars map[string]any) (map[string]any, []*cel.AttributePatternType, error) {
	n := &normalizer{}
	act := make(map[string]any, len(p.env.names))
	for _, name := range p.env.names {
		f := p.env.scope.Vars[name]
		v, present, err := n.value(f, vars[name], []segment{name})
		if err != nil {
			var in *InputError
			if errors.As(err, &in) {
				return nil, nil, &EvalError{Code: CodeInvalidInput, Message: in.Error()}
			}
			return nil, nil, &EvalError{Code: CodeInvalidInput, Message: err.Error()}
		}
		if present {
			act[name] = v
		}
	}
	missing := n.missing
	missing = append(missing, p.dynMissing(act)...)
	patterns := make([]*cel.AttributePatternType, 0, len(missing))
	for _, m := range missing {
		pat := cel.AttributePattern(m.path[0].(string))
		for _, s := range m.path[1:] {
			switch q := s.(type) {
			case string:
				pat = pat.QualString(q)
			case int64:
				pat = pat.QualInt(q)
			}
		}
		patterns = append(patterns, pat)
	}
	return act, patterns, nil
}
