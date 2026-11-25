package graph

import (
	"encoding/json"
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// ParamsSpec checks the params of a node or a trigger.
type ParamsSpec interface {
	JSONSchema() json.RawMessage
	// Parse returns the params with defaults applied, or issues with their paths. Values are not trimmed:
	// spaces at the edges of a value its schema forbids are an issue.
	Parse(raw map[string]any) (map[string]any, []Issue)
}

// Issue is one reason params are refused; Path is keys and indices from the params' root.
type Issue struct {
	Path    []any
	Message string
	// Code is the graph error code of the issue; empty: CodeParams. A custom ParamsSpec sets it for a
	// refusal the editor tells apart (an unknown model, a model without vision).
	Code string
}

type schemaParams struct {
	raw    json.RawMessage
	schema *jsonschema.Schema
	doc    map[string]any // the schema, for defaults
}

// SchemaParams is a ParamsSpec by JSON Schema (draft 2020-12) with the keywords x-unique, x-unique-ci and
// x-max-ms. `default` of an object's properties is applied to absent keys.
func SchemaParams(schema json.RawMessage) (ParamsSpec, error) {
	s, err := compileSchema(schema)
	if err != nil {
		return nil, fmt.Errorf("params schema: %w", err)
	}
	var doc map[string]any
	if err := expr.UnmarshalJSON(schema, &doc); err != nil {
		return nil, fmt.Errorf("params schema: %w", err)
	}
	return &schemaParams{raw: append(json.RawMessage(nil), schema...), schema: s, doc: doc}, nil
}

// MustSchemaParams is SchemaParams for schemas written in Go: it panics on an invalid schema.
func MustSchemaParams(schema string) ParamsSpec {
	p, err := SchemaParams(json.RawMessage(schema))
	if err != nil {
		panic(err)
	}
	return p
}

func (p *schemaParams) JSONSchema() json.RawMessage { return append(json.RawMessage(nil), p.raw...) }

func (p *schemaParams) Parse(raw map[string]any) (map[string]any, []Issue) {
	if raw == nil {
		raw = map[string]any{}
	}
	v, err := jsonValue(raw)
	if err != nil {
		return nil, []Issue{{Message: "params are not JSON: " + err.Error()}}
	}
	obj, ok := v.(map[string]any)
	if !ok {
		return nil, []Issue{{Message: "params are an object"}}
	}
	dropNullOptionals(p.doc, obj)
	applyDefaults(p.doc, obj)
	found := validateInstance(p.schema, obj)
	issues := make([]Issue, 0, len(found))
	for _, i := range found {
		issues = append(issues, Issue{Path: i.Path, Message: i.Message})
	}
	if len(issues) > 0 {
		return nil, issues
	}
	return obj, nil
}

// dropNullOptionals removes null values of properties an object schema does not require: left out and
// null mean the same for an optional param.
func dropNullOptionals(schema map[string]any, obj map[string]any) {
	required := map[string]bool{}
	if r, ok := schema["required"].([]any); ok {
		for _, x := range r {
			if s, ok := x.(string); ok {
				required[s] = true
			}
		}
	}
	props, _ := schema["properties"].(map[string]any)
	for k, v := range obj {
		_, declared := props[k]
		if v == nil && declared && !required[k] {
			delete(obj, k) // an undeclared key stays for additionalProperties to refuse
			continue
		}
		if sub, ok := props[k].(map[string]any); ok {
			if child, ok := v.(map[string]any); ok {
				dropNullOptionals(sub, child)
			}
		}
	}
}

// applyDefaults sets `default` of absent properties, in nested objects too.
func applyDefaults(schema map[string]any, obj map[string]any) {
	props, _ := schema["properties"].(map[string]any)
	for k, s := range props {
		sub, ok := s.(map[string]any)
		if !ok {
			continue
		}
		if _, present := obj[k]; !present {
			if d, ok := sub["default"]; ok {
				if copied, err := jsonValue(d); err == nil {
					obj[k] = copied
				}
			}
		}
		applyDefaultsIn(sub, obj[k])
	}
}

// applyDefaultsIn applies defaults inside a value: an object by its properties, each item of an array by
// the array's items schema.
func applyDefaultsIn(schema map[string]any, v any) {
	switch x := v.(type) {
	case map[string]any:
		applyDefaults(schema, x)
	case []any:
		if items, ok := schema["items"].(map[string]any); ok {
			for _, item := range x {
				applyDefaultsIn(items, item)
			}
		}
	}
}

// FieldsParams is the ParamsSpec of params declared in the field language: an object of exactly these
// fields; optional and dyn ones may be left out.
func FieldsParams(fields expr.Fields) (ParamsSpec, error) {
	if err := fields.Validate(); err != nil {
		return nil, err
	}
	return SchemaParams(paramsSchemaOf(fields))
}

// paramsSchemaOf is the fields' JSON Schema with the format's bounds: strings ≤ 10000, lists ≤ 1000.
func paramsSchemaOf(fields expr.Fields) json.RawMessage {
	var s map[string]any
	_ = expr.UnmarshalJSON(fields.JSONSchema(), &s)
	bound(s)
	b, err := json.Marshal(s)
	if err != nil {
		panic(fmt.Sprintf("graph: marshal a params schema: %v", err))
	}
	return b
}

func bound(s map[string]any) {
	switch s["type"] {
	case "string":
		if _, ok := s["maxLength"]; !ok {
			s["maxLength"] = 10000
		}
	case "array":
		if _, ok := s["maxItems"]; !ok {
			s["maxItems"] = 1000
		}
		if items, ok := s["items"].(map[string]any); ok {
			bound(items)
		}
	case "object":
		if props, ok := s["properties"].(map[string]any); ok {
			for _, p := range props {
				if pm, ok := p.(map[string]any); ok {
					bound(pm)
				}
			}
		}
		if ap, ok := s["additionalProperties"].(map[string]any); ok {
			bound(ap)
		}
	}
}

// trimmedPattern is a string without whitespace at its edges: the Unicode White_Space characters, as
// strings.TrimSpace trims them, written literally so the pattern reads the same in Go and JavaScript.
const trimmedPattern = "^[^" + whiteSpaceClass + "]([\\s\\S]*[^" + whiteSpaceClass + "])?$"

const whiteSpaceClass = "\t\n\v\f\r \u0085\u00a0\u1680\u2000-\u200a\u2028\u2029\u202f\u205f\u3000"

// MatchParams is the ParamsSpec of a trigger on an event that declares match fields and no params of its
// own: per field, a list of values (strings, each once, trimmed, ≤ 50) or a flag (bool); all optional.
func MatchParams(match map[string]MatchField) (ParamsSpec, error) {
	props := map[string]any{}
	for name, f := range match {
		switch f.Type {
		case MatchBool:
			props[name] = map[string]any{"type": "boolean"}
		case MatchStrings:
			unique := "x-unique"
			if f.Fold == FoldLower {
				unique = "x-unique-ci"
			}
			props[name] = map[string]any{
				"type":     "array",
				"maxItems": 50,
				"items":    map[string]any{"type": "string", "minLength": 1, "maxLength": 200, "pattern": trimmedPattern},
				unique:     true,
			}
		default:
			return nil, fmt.Errorf("match field %s: type is strings or bool", name)
		}
	}
	b, err := json.Marshal(map[string]any{"type": "object", "properties": props, "additionalProperties": false})
	if err != nil {
		return nil, err
	}
	return SchemaParams(b)
}
