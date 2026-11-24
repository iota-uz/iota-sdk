package mcp

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// ToolPin is the schema of a tool as a graph version pinned it when it was saved.
type ToolPin struct {
	InputSchema  json.RawMessage `json:"input_schema"`
	OutputSchema json.RawMessage `json:"output_schema,omitempty"`
	// Hash is the SHA-256 (hex) of the canonical JSON of {input_schema, output_schema}: keys sorted, no
	// insignificant whitespace, an absent output schema left out.
	Hash string `json:"hash"`
	// ReadOnly is the tool's readOnlyHint when it was pinned: a dry run executes the call.
	ReadOnly bool `json:"read_only,omitempty"`
}

// PinOf is the pin of a tool as the server describes it now.
func PinOf(t Tool) (ToolPin, error) {
	h, err := SchemaHash(t.InputSchema, t.OutputSchema)
	if err != nil {
		return ToolPin{}, err
	}
	in, err := canonicalJSON(t.InputSchema)
	if err != nil {
		return ToolPin{}, err
	}
	p := ToolPin{InputSchema: in, Hash: h, ReadOnly: t.ReadOnly}
	if len(t.OutputSchema) > 0 && string(t.OutputSchema) != "null" {
		if p.OutputSchema, err = canonicalJSON(t.OutputSchema); err != nil {
			return ToolPin{}, err
		}
	}
	return p, nil
}

// SchemaHash is the hash of a tool's schemas (ToolPin.Hash).
func SchemaHash(input, output json.RawMessage) (string, error) {
	doc := map[string]any{}
	in, err := decodeJSON(input)
	if err != nil {
		return "", fmt.Errorf("input_schema: %w", err)
	}
	doc["input_schema"] = in
	if len(output) > 0 && string(output) != "null" {
		out, err := decodeJSON(output)
		if err != nil {
			return "", fmt.Errorf("output_schema: %w", err)
		}
		doc["output_schema"] = out
	}
	b, err := marshalCanonical(doc)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// check reports whether the pin's hash is its own schemas'.
func (p ToolPin) check() error {
	h, err := SchemaHash(p.InputSchema, p.OutputSchema)
	if err != nil {
		return err
	}
	if h != p.Hash {
		return fmt.Errorf("the hash is not the hash of the pinned schemas")
	}
	return nil
}

func decodeJSON(raw json.RawMessage) (any, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, fmt.Errorf("empty")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}

func canonicalJSON(raw json.RawMessage) (json.RawMessage, error) {
	v, err := decodeJSON(raw)
	if err != nil {
		return nil, err
	}
	return marshalCanonical(v)
}

// marshalCanonical writes keys sorted (encoding/json sorts map keys), without HTML escaping or a newline.
func marshalCanonical(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// Argument is one argument of a tool call: exactly one of a template, a CEL expression or a value.
type Argument struct {
	Template *string         `json:"template,omitempty"`
	CEL      *string         `json:"cel,omitempty"`
	Value    json.RawMessage `json:"value,omitempty"`
}

// argIssue is one refusal of the arguments by the pinned input schema.
type argIssue struct {
	path, code, message string
}

// Codes of argument issues.
const (
	CodeArgumentUnknown  = "argument_unknown"
	CodeArgumentMissing  = "argument_missing"
	CodeArgumentType     = "argument_type"
	CodeToolNotFound     = "tool_not_found"
	CodeToolsUnavailable = "tools_unavailable"
	CodeSchemaChanged    = "tool_schema_changed"
	CodeSchemaInvalid    = "tool_schema_invalid"
)

// checkArguments checks the arguments of a node against the pinned input schema, as the object a run
// assembles from them, with the compiled schema ($ref, allOf, additionalProperties schemas and every
// other keyword): literal values are validated where they land, every required property must be given
// at every level, and a template or a CEL expression (known by its type only) must give a type the
// property takes. typeOf is the CEL type of an expression (nil: CEL is not checked).
func checkArguments(input json.RawMessage, args map[string]Argument, typeOf func(string) (string, error)) []argIssue {
	root, out := argTree(args)
	schema := compileInput(input)
	if schema == nil {
		return out // a schema that does not compile: the server checks the call itself
	}
	var holders []holder
	base := assemble(root, nil, &holders, typeOf)
	for _, leaf := range leaves(schema, base) {
		if underHolder(leaf.InstanceLocation, holders) {
			continue
		}
		// A branch (if/then/else, oneOf, anyOf, not, dependentSchemas) may be chosen by a value known
		// only at run time: its failures are not reported while any argument is dynamic.
		if len(holders) > 0 && inBranch(leaf.SchemaURL) {
			continue
		}
		out = append(out, issuesOf(root, leaf)...)
	}
	// Each template or expression is tried with a value of every type it may give: it fits when one
	// passes the type checks where it lands.
	for _, h := range holders {
		if len(h.probes) == 0 {
			continue
		}
		fits := false
		for _, probe := range h.probes {
			setAtLoc(base, h.loc, probe)
			bad := false
			for _, leaf := range leaves(schema, base) {
				if isTypeError(leaf) && within(leaf.InstanceLocation, h.loc) {
					bad = true
					break
				}
			}
			if !bad {
				fits = true
				break
			}
		}
		setAtLoc(base, h.loc, nil)
		if !fits {
			out = append(out, argIssue{path: "arguments." + h.node.key + "." + h.field, code: CodeArgumentType,
				message: fmt.Sprintf("%s gives %s; %s takes another type", h.what, h.gives, orArguments(strings.Join(h.loc, ".")))})
		}
	}
	seen := map[string]bool{}
	uniq := out[:0]
	for _, i := range out {
		if k := i.path + "\x00" + i.message; !seen[k] {
			seen[k] = true
			uniq = append(uniq, i)
		}
	}
	sort.SliceStable(uniq, func(i, j int) bool { return uniq[i].path < uniq[j].path })
	return uniq
}

// holder is a template or an expression inside the assembled arguments.
type holder struct {
	loc    []string
	node   *argNode
	field  string // template | cel
	what   string
	gives  string
	probes []any
}

// assemble is the arguments object with literal values in place and null where a template or an
// expression goes (each recorded with the values of the types it may give).
func assemble(n *argNode, loc []string, holders *[]holder, typeOf func(string) (string, error)) any {
	switch {
	case n.isLit:
		return n.literal
	case n.tmpl != nil:
		// A template's text is read as the number or boolean the schema wants (buildArguments).
		*holders = append(*holders, holder{loc: append([]string(nil), loc...), node: n, field: "template", what: "a template", gives: "text",
			probes: []any{"", json.Number("0"), true}})
		return nil
	case n.cel != nil:
		h := holder{loc: append([]string(nil), loc...), node: n, field: "cel", what: "the expression"}
		if typeOf != nil {
			if t, err := typeOf(*n.cel); err == nil {
				h.gives, h.probes = t, probesOf(jsonTypeOfCEL(t))
			}
		}
		*holders = append(*holders, h)
		return nil
	}
	obj := make(map[string]any, len(n.children))
	for k, c := range n.children {
		obj[k] = assemble(c, append(loc, k), holders, typeOf)
	}
	return obj
}

// probesOf are values of a JSON type (none: dyn, any value may come).
func probesOf(t string) []any {
	switch t {
	case "string":
		return []any{""}
	case "integer":
		return []any{json.Number("0")}
	case "number":
		return []any{json.Number("0.5")}
	case "boolean":
		return []any{false}
	case "array":
		return []any{[]any{}}
	case "object":
		return []any{map[string]any{}}
	case "null":
		return []any{nil}
	}
	return nil
}

func setAtLoc(root any, loc []string, v any) {
	cur, _ := root.(map[string]any)
	for i, k := range loc {
		if cur == nil {
			return
		}
		if i == len(loc)-1 {
			cur[k] = v
			return
		}
		cur, _ = cur[k].(map[string]any)
	}
}

func compileInput(input json.RawMessage) *jsonschema.Schema {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(input))
	if err != nil {
		return nil
	}
	c := jsonschema.NewCompiler()
	c.UseLoader(noLoader{})
	if err := c.AddResource("mcp-input.json", doc); err != nil {
		return nil
	}
	s, err := c.Compile("mcp-input.json")
	if err != nil {
		return nil
	}
	return s
}

// leaves are the innermost failures of validating v.
func leaves(s *jsonschema.Schema, v any) []*jsonschema.ValidationError {
	err := s.Validate(v)
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		return nil
	}
	var out []*jsonschema.ValidationError
	var walk func(e *jsonschema.ValidationError)
	walk = func(e *jsonschema.ValidationError) {
		if len(e.Causes) == 0 {
			out = append(out, e)
			return
		}
		for _, c := range e.Causes {
			walk(c)
		}
	}
	walk(ve)
	return out
}

// inBranch reports whether a schema location lies inside a conditional or alternative keyword.
func inBranch(schemaURL string) bool {
	_, frag, _ := strings.Cut(schemaURL, "#")
	for _, seg := range strings.Split(frag, "/") {
		switch seg {
		case "if", "then", "else", "oneOf", "anyOf", "not", "dependentSchemas", "dependentRequired":
			return true
		}
	}
	return false
}

func isTypeError(e *jsonschema.ValidationError) bool {
	switch e.ErrorKind.(type) {
	case *kind.Type, *kind.FalseSchema:
		return true
	}
	return false
}

// within reports whether loc is at or under at.
func within(loc, at []string) bool {
	if len(loc) < len(at) {
		return false
	}
	for i := range at {
		if loc[i] != at[i] {
			return false
		}
	}
	return true
}

func underHolder(loc []string, holders []holder) bool {
	for _, h := range holders {
		if within(loc, h.loc) {
			return true
		}
	}
	return false
}

var english = message.NewPrinter(language.English)

// issuesOf is a failure as issues on the arguments that built the value it is about.
func issuesOf(root *argNode, e *jsonschema.ValidationError) []argIssue {
	loc := e.InstanceLocation
	switch k := e.ErrorKind.(type) {
	case *kind.Required:
		// A literal object that lacks a property is that value's issue; an object built of several
		// arguments lacks an argument.
		path := "arguments"
		if n := nodeAt(root, loc); n.isLit && len(loc) > 0 {
			path = "arguments." + n.key + ".value"
		}
		out := make([]argIssue, 0, len(k.Missing))
		for _, m := range k.Missing {
			out = append(out, argIssue{path: path, code: CodeArgumentMissing, message: fmt.Sprintf("the tool needs argument %s", join(strings.Join(loc, "."), m))})
		}
		return out
	case *kind.AdditionalProperties:
		out := make([]argIssue, 0, len(k.Properties))
		for _, p := range k.Properties {
			n := nodeAt(root, append(append([]string(nil), loc...), p))
			out = append(out, argIssue{path: "arguments." + n.key, code: CodeArgumentUnknown, message: fmt.Sprintf("the tool has no argument %s", join(strings.Join(loc, "."), p))})
		}
		return out
	}
	n := nodeAt(root, loc)
	path := "arguments." + n.key
	if n.isLit {
		path += ".value"
	}
	return []argIssue{{path: path, code: CodeArgumentType,
		message: fmt.Sprintf("%s does not fit the tool's schema: %s", orArguments(strings.Join(loc, ".")), clip(e.ErrorKind.LocalizedString(english), 300))}}
}

// nodeAt is the deepest node of the assembled arguments on the way to loc.
func nodeAt(root *argNode, loc []string) *argNode {
	cur := root
	for _, k := range loc {
		next, ok := cur.children[k]
		if !ok {
			break
		}
		cur = next
	}
	if cur.key == "" {
		cur = &argNode{key: firstKey(root)}
	}
	return cur
}

// argNode is the assembled arguments: an object of children, a literal (from a value) or a placeholder
// (a template or an expression, known by its type only).
type argNode struct {
	children map[string]*argNode
	literal  any
	isLit    bool
	tmpl     *string
	cel      *string
	key      string // the argument key that set it
}

// argTree assembles the arguments by their dotted keys, shorter keys first.
func argTree(args map[string]Argument) (*argNode, []argIssue) {
	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		di, dj := strings.Count(keys[i], "."), strings.Count(keys[j], ".")
		if di != dj {
			return di < dj
		}
		return keys[i] < keys[j]
	})
	root := &argNode{children: map[string]*argNode{}}
	var issues []argIssue
	for _, k := range keys {
		a := args[k]
		cur := root
		parts := strings.Split(k, ".")
		ok := true
		for _, part := range parts[:len(parts)-1] {
			if !explode(cur) {
				issues = append(issues, argIssue{path: "arguments." + k, code: CodeArgumentType, message: fmt.Sprintf("%s goes into %s, which is not an object", k, cur.key)})
				ok = false
				break
			}
			next := cur.children[part]
			if next == nil {
				next = &argNode{children: map[string]*argNode{}, key: k}
				cur.children[part] = next
			}
			cur = next
		}
		if !ok {
			continue
		}
		if !explode(cur) {
			issues = append(issues, argIssue{path: "arguments." + k, code: CodeArgumentType, message: fmt.Sprintf("%s goes into %s, which is not an object", k, cur.key)})
			continue
		}
		leaf := &argNode{key: k, tmpl: a.Template, cel: a.CEL}
		if a.Value != nil {
			v, err := decodeJSON(a.Value)
			if err != nil {
				issues = append(issues, argIssue{path: "arguments." + k + ".value", code: CodeArgumentType, message: "the value is not JSON"})
				continue
			}
			leaf.literal, leaf.isLit = v, true
		}
		last := parts[len(parts)-1]
		if prev := cur.children[last]; prev != nil && len(prev.children) > 0 && leaf.isLit {
			// A whole object under a key a deeper one already set: the deeper one wins, as a run merges.
			if m, isMap := leaf.literal.(map[string]any); isMap {
				for ck, cv := range m {
					if _, set := prev.children[ck]; !set {
						prev.children[ck] = &argNode{literal: cv, isLit: true, key: k}
					}
				}
				continue
			}
		}
		cur.children[last] = leaf
	}
	return root, issues
}

// explode makes n an object node: a literal object becomes children; false when n is not an object.
func explode(n *argNode) bool {
	if n.children != nil {
		return true
	}
	if n.tmpl != nil || n.cel != nil {
		return false
	}
	m, ok := n.literal.(map[string]any)
	if !n.isLit || !ok {
		return false
	}
	n.children = map[string]*argNode{}
	for k, v := range m {
		n.children[k] = &argNode{literal: v, isLit: true, key: n.key}
	}
	n.isLit, n.literal = false, nil
	return true
}

func join(prefix, k string) string {
	if prefix == "" {
		return k
	}
	return prefix + "." + k
}

func orArguments(dotted string) string {
	if dotted == "" {
		return "the arguments"
	}
	return dotted
}

// firstKey is an argument key that built an object node, for the path of its issue.
func firstKey(n *argNode) string {
	keys := make([]string, 0, len(n.children))
	for _, c := range n.children {
		keys = append(keys, c.key)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return n.key
	}
	return keys[0]
}

// jsonTypeOfCEL is the JSON type a CEL type renders to ("" when any may come: dyn, a type parameter).
func jsonTypeOfCEL(t string) string {
	switch {
	case t == "string" || t == "google.protobuf.Timestamp" || t == "timestamp":
		return "string"
	case t == "int" || t == "uint":
		return "integer"
	case t == "double" || t == "google.protobuf.Duration" || t == "duration":
		return "number"
	case t == "bool":
		return "boolean"
	case t == "null_type":
		return "null"
	case strings.HasPrefix(t, "list("):
		return "array"
	case strings.HasPrefix(t, "map("):
		return "object"
	case strings.Contains(t, "."):
		return "object" // a declared object of the expression language
	}
	return ""
}

// buildArguments assembles the arguments object of a tools/call from the rendered params: a template is
// its text (read as the number or boolean the pinned schema wants), a CEL expression its value, a value
// itself; a dotted key goes into the object argument it names.
func buildArguments(input json.RawMessage, rendered map[string]any) (map[string]any, error) {
	keys := make([]string, 0, len(rendered))
	for k := range rendered {
		keys = append(keys, k)
	}
	// Shorter keys first: "a" is set before "a.b" goes into it.
	sort.Slice(keys, func(i, j int) bool {
		di, dj := strings.Count(keys[i], "."), strings.Count(keys[j], ".")
		if di != dj {
			return di < dj
		}
		return keys[i] < keys[j]
	})
	out := map[string]any{}
	var templates [][]string
	for _, k := range keys {
		entry, _ := rendered[k].(map[string]any)
		var v any
		switch {
		case entry == nil:
			return nil, fmt.Errorf("argument %s is not an object", k)
		case hasKey(entry, "template"):
			v, _ = entry["template"].(string)
			templates = append(templates, strings.Split(k, "."))
		case hasKey(entry, "cel"):
			v = entry["cel"]
		case hasKey(entry, "value"):
			v = entry["value"]
		default:
			return nil, fmt.Errorf("argument %s has no template, cel or value", k)
		}
		if err := setDotted(out, strings.Split(k, "."), v); err != nil {
			return nil, fmt.Errorf("argument %s: %w", k, err)
		}
	}
	coerceTemplates(input, out, templates)
	return out, nil
}

// coerceTemplates reads each template's text as the scalar the pinned schema takes where it lands, by
// the compiled schema ($ref included), as the save checked it: the text stays when it fits, else the
// first of its integer, number or boolean reading that does. Text that fits nothing stays text: the
// server refuses it with isError and the run takes rejected.
func coerceTemplates(input json.RawMessage, args map[string]any, locs [][]string) {
	if len(locs) == 0 {
		return
	}
	schema := compileInput(input)
	if schema == nil {
		return
	}
	fits := func(loc []string) bool {
		for _, leaf := range leaves(schema, args) {
			if isTypeError(leaf) && within(leaf.InstanceLocation, loc) {
				return false
			}
		}
		return true
	}
	for _, loc := range locs {
		text, ok := valueAtLoc(args, loc).(string)
		if !ok || fits(loc) {
			continue
		}
		for _, cand := range readings(text) {
			setAtLoc(args, loc, cand)
			if fits(loc) {
				break
			}
			setAtLoc(args, loc, text)
		}
	}
}

var jsonNumber = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)

// readings are the scalar readings of a template's text.
func readings(text string) []any {
	s := strings.TrimSpace(text)
	var out []any
	if jsonNumber.MatchString(s) {
		if _, err := strconv.ParseFloat(s, 64); err == nil {
			out = append(out, json.Number(s)) // a JSON number: an integer reading fits integer and number
		}
	}
	if s == "true" || s == "false" {
		out = append(out, s == "true")
	}
	return out
}

func valueAtLoc(root any, loc []string) any {
	cur := root
	for _, k := range loc {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = m[k]
	}
	return cur
}

func hasKey(m map[string]any, k string) bool {
	_, ok := m[k]
	return ok
}

func setDotted(m map[string]any, parts []string, v any) error {
	if len(parts) == 1 {
		if existing, ok := m[parts[0]].(map[string]any); ok {
			if add, ok := v.(map[string]any); ok {
				for k, x := range add {
					if _, set := existing[k]; !set {
						existing[k] = x
					}
				}
				return nil
			}
		}
		m[parts[0]] = v
		return nil
	}
	child, ok := m[parts[0]]
	switch c := child.(type) {
	case nil:
		if ok {
			return fmt.Errorf("%s is null", parts[0])
		}
		next := map[string]any{}
		m[parts[0]] = next
		return setDotted(next, parts[1:], v)
	case map[string]any:
		cp := make(map[string]any, len(c)+1)
		for k, x := range c {
			cp[k] = x
		}
		m[parts[0]] = cp
		return setDotted(cp, parts[1:], v)
	}
	return fmt.Errorf("%s is not an object", parts[0])
}

// outputSchemas compiles the pinned output schemas once per hash.
var outputSchemas sync.Map // hash → *jsonschema.Schema or error

func compiledOutput(p ToolPin) (*jsonschema.Schema, error) {
	if len(p.OutputSchema) == 0 {
		return nil, nil //nolint:nilnil // no output schema pinned: nothing to check
	}
	if v, ok := outputSchemas.Load(p.Hash); ok {
		if s, ok := v.(*jsonschema.Schema); ok {
			return s, nil
		}
		return nil, v.(error)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(p.OutputSchema))
	if err != nil {
		return nil, err
	}
	c := jsonschema.NewCompiler()
	c.UseLoader(noLoader{})
	if err := c.AddResource("mcp-output.json", doc); err != nil {
		return nil, err
	}
	s, err := c.Compile("mcp-output.json")
	if err != nil {
		outputSchemas.Store(p.Hash, err)
		return nil, err
	}
	outputSchemas.Store(p.Hash, s)
	return s, nil
}

// noLoader refuses every remote $ref: a server's schema never makes the process fetch anything.
type noLoader struct{}

func (noLoader) Load(url string) (any, error) {
	return nil, fmt.Errorf("mcp: a schema may not load %s", url)
}
