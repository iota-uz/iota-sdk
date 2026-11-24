package mcp

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
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

// schemaNode is the part of a JSON Schema the argument checks read.
type schemaNode struct {
	Types      []string
	Properties map[string]json.RawMessage
	Required   []string
	Closed     bool // additionalProperties: false
	Enum       []any
	HasEnum    bool
}

func readSchema(raw json.RawMessage) schemaNode {
	var doc map[string]any
	if v, err := decodeJSON(raw); err == nil {
		doc, _ = v.(map[string]any)
	}
	var n schemaNode
	switch t := doc["type"].(type) {
	case string:
		n.Types = []string{t}
	case []any:
		for _, x := range t {
			if s, ok := x.(string); ok {
				n.Types = append(n.Types, s)
			}
		}
	}
	if props, ok := doc["properties"].(map[string]any); ok {
		n.Properties = map[string]json.RawMessage{}
		for k, v := range props {
			if b, err := json.Marshal(v); err == nil {
				n.Properties[k] = b
			}
		}
		if len(n.Types) == 0 {
			n.Types = []string{"object"}
		}
	}
	if req, ok := doc["required"].([]any); ok {
		for _, r := range req {
			if s, ok := r.(string); ok {
				n.Required = append(n.Required, s)
			}
		}
	}
	if ap, ok := doc["additionalProperties"].(bool); ok && !ap {
		n.Closed = true
	}
	if e, ok := doc["enum"].([]any); ok {
		n.Enum, n.HasEnum = e, true
	}
	return n
}

func (n schemaNode) allows(t string) bool {
	if len(n.Types) == 0 {
		return true
	}
	for _, x := range n.Types {
		if x == t || (t == "integer" && x == "number") {
			return true
		}
	}
	return false
}

// at is the schema of a dotted argument key inside the input schema: ok=false when a closed object has
// no such property; known=false when the schema does not say (an open object, no properties).
func at(input json.RawMessage, key string) (schemaNode, bool, bool) {
	cur := readSchema(input)
	for _, part := range strings.Split(key, ".") {
		raw, has := cur.Properties[part]
		switch {
		case has:
			cur = readSchema(raw)
		case cur.Closed:
			return schemaNode{}, false, true
		default:
			return schemaNode{}, true, false
		}
	}
	return cur, true, true
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

// checkArguments checks the arguments of a node against the pinned input schema: every required
// property is given, no key names a property a closed object lacks, and the type of each value,
// template and CEL expression is compatible. typeOf is the CEL type of an expression (nil: CEL is not
// checked).
func checkArguments(input json.RawMessage, args map[string]Argument, typeOf func(string) (string, error)) []argIssue {
	var out []argIssue
	root := readSchema(input)
	for _, r := range root.Required {
		found := false
		for k := range args {
			if k == r || strings.HasPrefix(k, r+".") {
				found = true
				break
			}
		}
		if !found {
			out = append(out, argIssue{path: "arguments", code: CodeArgumentMissing, message: fmt.Sprintf("the tool needs argument %s", r)})
		}
	}
	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		a := args[k]
		path := "arguments." + k
		node, ok, known := at(input, k)
		if !ok {
			out = append(out, argIssue{path: path, code: CodeArgumentUnknown, message: fmt.Sprintf("the tool has no argument %s", k)})
			continue
		}
		if !known {
			continue
		}
		switch {
		case a.Value != nil:
			if msg := valueFits(node, a.Value); msg != "" {
				out = append(out, argIssue{path: path + ".value", code: CodeArgumentType, message: msg})
			}
		case a.Template != nil:
			if !node.allows("string") && !node.allows("number") && !node.allows("boolean") {
				out = append(out, argIssue{path: path + ".template", code: CodeArgumentType,
					message: fmt.Sprintf("a template gives text; %s is %s", k, strings.Join(node.Types, " or "))})
			}
		case a.CEL != nil && typeOf != nil:
			t, err := typeOf(*a.CEL)
			if err != nil {
				continue // the graph's own check of the expression reports it
			}
			if js := jsonTypeOfCEL(t); js != "" && !node.allows(js) {
				out = append(out, argIssue{path: path + ".cel", code: CodeArgumentType,
					message: fmt.Sprintf("the expression is %s; %s is %s", t, k, strings.Join(node.Types, " or "))})
			}
		}
	}
	return out
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

// valueFits checks a literal value against a property schema's type and enum.
func valueFits(node schemaNode, raw json.RawMessage) string {
	v, err := decodeJSON(raw)
	if err != nil {
		return "the value is not JSON"
	}
	t := jsonTypeOf(v)
	if !node.allows(t) {
		return fmt.Sprintf("the value is %s, the tool takes %s", t, strings.Join(node.Types, " or "))
	}
	if node.HasEnum {
		got, _ := marshalCanonical(v)
		for _, e := range node.Enum {
			if want, err := marshalCanonical(e); err == nil && bytes.Equal(got, want) {
				return ""
			}
		}
		return "the value is not one the tool allows"
	}
	return ""
}

func jsonTypeOf(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case string:
		return "string"
	case json.Number:
		if r, ok := new(big.Rat).SetString(string(x)); ok && r.IsInt() {
			return "integer"
		}
		return "number"
	case float64:
		if x == float64(int64(x)) {
			return "integer"
		}
		return "number"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	}
	return "object"
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
	for _, k := range keys {
		entry, _ := rendered[k].(map[string]any)
		var v any
		switch {
		case entry == nil:
			return nil, fmt.Errorf("argument %s is not an object", k)
		case hasKey(entry, "template"):
			text, _ := entry["template"].(string)
			node, _, _ := at(input, k)
			v = coerce(node, text)
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
	return out, nil
}

func hasKey(m map[string]any, k string) bool {
	_, ok := m[k]
	return ok
}

// coerce reads a template's text as the scalar the schema wants when it does not take text.
func coerce(node schemaNode, text string) any {
	if node.allows("string") || len(node.Types) == 0 {
		return text
	}
	s := strings.TrimSpace(text)
	if node.allows("integer") {
		if _, err := strconv.ParseInt(s, 10, 64); err == nil {
			return json.Number(s)
		}
	}
	if node.allows("number") {
		if f, err := strconv.ParseFloat(s, 64); err == nil && !isNaNOrInf(f) {
			return json.Number(s)
		}
	}
	if node.allows("boolean") && (s == "true" || s == "false") {
		return s == "true"
	}
	return text // the server refuses it with isError: the run takes rejected
}

func isNaNOrInf(f float64) bool {
	return f != f || f > 1.7976931348623157e308 || f < -1.7976931348623157e308
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
