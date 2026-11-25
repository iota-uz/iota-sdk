package aichat

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
)

// Schema is a node's JSON Schema prepared for a provider's strict structured output and for checking the
// answer here.
type Schema struct {
	// Strict is what the provider gets: every object lists all its properties as required and is closed;
	// a property the author left optional is nullable, since strict mode has no optional properties. A
	// schema that is not an object is wrapped in {"value": ...}: strict mode wants an object at the root.
	Strict json.RawMessage
	// Closed is what the answer is checked against here: the author's schema with every object closed,
	// since the node's output declares exactly the author's properties.
	Closed json.RawMessage
	// Wrapped: the answer is {"value": ...} and Unwrap takes the value out.
	Wrapped bool

	original any
}

// Prepare builds a Schema from the author's JSON Schema (the subset expr.FieldFromJSONSchema reads).
func Prepare(raw json.RawMessage) (*Schema, error) {
	var doc any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("schema: %w", err)
	}
	obj, ok := doc.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("schema is an object")
	}
	// The provider takes a schema of at most MaxSchemaDepth levels of nesting, the wrapper of a schema that is
	// not an object included.
	depth := schemaDepth(obj)
	if t, _ := obj["type"].(string); t != "object" {
		depth++
	}
	if depth > MaxSchemaDepth {
		return nil, fmt.Errorf("the schema is nested %d levels deep; at most %d", depth, MaxSchemaDepth)
	}
	s := &Schema{original: doc}
	closed, err := json.Marshal(closeObjects(obj))
	if err != nil {
		return nil, fmt.Errorf("schema: %w", err)
	}
	s.Closed = closed
	strict := strictify(obj, false)
	if t, _ := obj["type"].(string); t != "object" {
		s.Wrapped = true
		strict = map[string]any{
			"type":                 "object",
			"properties":           map[string]any{"value": strict},
			"required":             []any{"value"},
			"additionalProperties": false,
		}
	}
	// The provider takes at most MaxEnumValues enum values in all: counted in the schema it gets, where each
	// optional enum gained a null.
	if n := countEnumValues(strict); n > MaxEnumValues {
		return nil, fmt.Errorf("the schema has %d enum values once optional properties are made nullable; at most %d", n, MaxEnumValues)
	}
	b, err := json.Marshal(strict)
	if err != nil {
		return nil, fmt.Errorf("schema: %w", err)
	}
	s.Strict = b
	return s, nil
}

// Unwrap checks the provider's envelope and takes the author's value out of it: a schema that is not an
// object was wrapped in {"value": ...}, and the answer must be exactly that object. It then drops the
// null the provider put into each property the author left optional, so the value has the shape of the
// author's schema. v is a decoded JSON value (json.Number for numbers); an error is an answer outside
// the schema the provider was given.
func (s *Schema) Unwrap(v any) (any, error) {
	if s.Wrapped {
		m, ok := v.(map[string]any)
		if !ok {
			return nil, errors.New("the answer is not the {\"value\": ...} object the schema asked for")
		}
		inner, ok := m["value"]
		if !ok || len(m) != 1 {
			return nil, errors.New("the answer is not the {\"value\": ...} object the schema asked for")
		}
		v = inner
	}
	return dropNulls(s.original, v), nil
}

func closeObjects(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x)+1)
		for k, e := range x {
			if k == "properties" {
				props, _ := e.(map[string]any)
				cp := make(map[string]any, len(props))
				for name, p := range props {
					cp[name] = closeObjects(p)
				}
				out[k] = cp
				continue
			}
			out[k] = closeObjects(e)
		}
		if t, _ := x["type"].(string); t == "object" {
			out["additionalProperties"] = false
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = closeObjects(e)
		}
		return out
	}
	return v
}

// strictify is the strict form of a schema; nullable makes the type accept null too.
func strictify(s map[string]any, nullable bool) map[string]any {
	out := make(map[string]any, len(s)+2)
	for k, v := range s {
		out[k] = v
	}
	typ, _ := s["type"].(string)
	switch typ {
	case "object":
		props, _ := s["properties"].(map[string]any)
		required := map[string]bool{}
		if list, ok := s["required"].([]any); ok {
			for _, r := range list {
				if name, ok := r.(string); ok {
					required[name] = true
				}
			}
		}
		names := make([]string, 0, len(props))
		for name := range props {
			names = append(names, name)
		}
		sort.Strings(names)
		newProps := make(map[string]any, len(props))
		all := make([]any, 0, len(names))
		for _, name := range names {
			child, _ := props[name].(map[string]any)
			newProps[name] = strictify(child, !required[name])
			all = append(all, name)
		}
		out["properties"] = newProps
		out["required"] = all
		out["additionalProperties"] = false
	case "array":
		if items, ok := s["items"].(map[string]any); ok {
			out["items"] = strictify(items, false)
		}
	}
	if nullable && typ != "" {
		out["type"] = []any{typ, "null"}
		if enum, ok := s["enum"].([]any); ok {
			out["enum"] = append(append([]any(nil), enum...), nil)
		}
	}
	return out
}

// dropNulls removes, along the author's schema, the properties the author left optional whose value is
// null.
func dropNulls(schema any, v any) any {
	s, _ := schema.(map[string]any)
	switch x := v.(type) {
	case map[string]any:
		props, _ := s["properties"].(map[string]any)
		required := map[string]bool{}
		if list, ok := s["required"].([]any); ok {
			for _, r := range list {
				if name, ok := r.(string); ok {
					required[name] = true
				}
			}
		}
		for name, child := range x {
			// Only a property the author declared and left optional may be a placeholder null; a null of
			// any other field stays, and the closed schema refuses it.
			if _, declared := props[name]; declared && child == nil && !required[name] {
				delete(x, name)
				continue
			}
			x[name] = dropNulls(props[name], child)
		}
		return x
	case []any:
		items := s["items"]
		for i, e := range x {
			x[i] = dropNulls(items, e)
		}
		return x
	}
	return v
}

// DecodeJSON reads one JSON value (numbers as json.Number); text around it is an error.
func DecodeJSON(text string) (any, error) {
	dec := json.NewDecoder(bytes.NewReader([]byte(text)))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("it is not a JSON value")
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("there is text after the JSON value")
	}
	return v, nil
}

// MaxEnumValues is the enum values in all the providers' strict mode takes in one schema.
const MaxEnumValues = 1000

// countEnumValues counts the values of every enum of a schema: the `enum` keyword of the schema, of each
// property's schema and of the items'. The keys of `properties` are names the author chose, not keywords: a
// property called enum is a property.
func countEnumValues(schema map[string]any) int {
	n := 0
	if list, ok := schema["enum"].([]any); ok {
		n += len(list)
	}
	if props, ok := schema["properties"].(map[string]any); ok {
		for _, p := range props {
			if child, ok := p.(map[string]any); ok {
				n += countEnumValues(child)
			}
		}
	}
	if items, ok := schema["items"].(map[string]any); ok {
		n += countEnumValues(items)
	}
	return n
}

// MaxSchemaDepth is the levels of nesting of objects and arrays the providers' strict mode takes.
const MaxSchemaDepth = 10

// schemaDepth is the levels of nesting of objects and arrays in a schema: a scalar has none, an object one
// more than its deepest property, an array one more than its items.
func schemaDepth(schema map[string]any) int {
	switch schema["type"] {
	case "object":
		deepest := 0
		props, _ := schema["properties"].(map[string]any)
		for _, p := range props {
			if child, ok := p.(map[string]any); ok {
				deepest = max(deepest, schemaDepth(child))
			}
		}
		return 1 + deepest
	case "array":
		items, _ := schema["items"].(map[string]any)
		return 1 + schemaDepth(items)
	}
	return 0
}
