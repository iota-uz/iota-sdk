package agents

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

var toolSchemaCache sync.Map // map[reflect.Type]map[string]any

// ToolSchema generates an OpenAI-compatible JSON Schema map for the input type T.
// The schema is cached per type and returned as a fresh copy to prevent mutation.
func ToolSchema[T any]() map[string]any {
	typ := reflect.TypeOf((*T)(nil)).Elem()

	if v, ok := toolSchemaCache.Load(typ); ok {
		return cloneMap(v.(map[string]any))
	}

	schema := schemaForType(typ, make(map[reflect.Type]bool))
	if schema == nil {
		schema = map[string]any{"type": "object", "properties": map[string]any{}}
	}

	toolSchemaCache.Store(typ, schema)
	return cloneMap(schema)
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	// JSON roundtrip is simple and safe for our schema maps.
	b, _ := json.Marshal(in)
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	return out
}

func schemaForType(t reflect.Type, visiting map[reflect.Type]bool) map[string]any {
	if t == nil {
		return nil
	}

	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	if visiting[t] {
		// Cycle guard. Tool input schemas should be acyclic, but degrade gracefully.
		return map[string]any{"type": "object"}
	}

	switch t.Kind() {
	case reflect.Struct:
		visiting[t] = true
		defer delete(visiting, t)

		props := make(map[string]any)
		required := make([]string, 0, 8)

		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.PkgPath != "" { // unexported
				continue
			}

			jsonName, omitempty, skip := parseJSONName(f)
			if skip {
				continue
			}

			fieldSchema := schemaForType(f.Type, visiting)
			if fieldSchema == nil {
				fieldSchema = map[string]any{}
			}

			applyJSONSchemaTag(fieldSchema, f)
			props[jsonName] = fieldSchema

			// Required inference:
			// - omitempty => optional
			// - pointer => optional
			// - explicit required=true in jsonschema tag => required
			if isExplicitRequired(f) {
				required = append(required, jsonName)
				continue
			}
			if !omitempty && f.Type.Kind() != reflect.Pointer {
				required = append(required, jsonName)
			}
		}

		out := map[string]any{
			"type":       "object",
			"properties": props,
		}
		if len(required) > 0 {
			out["required"] = required
		}
		return out

	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return map[string]any{"type": "integer"}
	case reflect.Float32, reflect.Float64:
		return map[string]any{"type": "number"}

	case reflect.Slice, reflect.Array:
		itemsType := t.Elem()
		itemsSchema := schemaForType(itemsType, visiting)
		if itemsSchema == nil {
			itemsSchema = map[string]any{}
		}
		return map[string]any{
			"type":  "array",
			"items": itemsSchema,
		}

	case reflect.Map:
		// For tool inputs we treat maps as objects with free-form values.
		return map[string]any{"type": "object"}

	default:
		// Fallback: allow any.
		return map[string]any{}
	}
}

func parseJSONName(f reflect.StructField) (name string, omitempty bool, skip bool) {
	tag := f.Tag.Get("json")
	if tag == "-" {
		return "", false, true
	}
	if tag == "" {
		// Best-effort default name. Most tool structs should specify json tags.
		return lowerFirst(f.Name), false, false
	}

	parts := strings.Split(tag, ",")
	if len(parts) == 0 {
		return lowerFirst(f.Name), false, false
	}

	if parts[0] == "" {
		name = lowerFirst(f.Name)
	} else {
		name = parts[0]
	}
	for _, p := range parts[1:] {
		if p == "omitempty" {
			omitempty = true
			break
		}
	}
	return name, omitempty, false
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

// applyJSONSchemaTag reads `jsonschema:"k=v;..."` tags and applies supported constraints.
// Supported keys:
// - description (string)
// - default (string|number|bool)
// - enum (pipe-separated list, e.g. "a|b|c")
// - minimum, maximum (number)
// - minItems, maxItems (int)
// - minLength, maxLength (int)
// - pattern (string)
func applyJSONSchemaTag(dst map[string]any, f reflect.StructField) {
	tag := strings.TrimSpace(f.Tag.Get("jsonschema"))
	if tag == "" {
		return
	}

	parts := strings.Split(tag, ";")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		k, v, ok := strings.Cut(p, "=")
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if !ok {
			continue
		}

		switch k {
		case "description":
			dst["description"] = v
		case "pattern":
			dst["pattern"] = v
		case "default":
			if def, ok := parseTypedScalar(v, f.Type); ok {
				dst["default"] = def
			} else {
				dst["default"] = v
			}
		case "enum":
			// Always model enum as JSON array.
			if v == "" {
				continue
			}
			raw := strings.Split(v, "|")
			enums := make([]any, 0, len(raw))
			for _, item := range raw {
				item = strings.TrimSpace(item)
				if item == "" {
					continue
				}
				if ev, ok := parseTypedScalar(item, f.Type); ok {
					enums = append(enums, ev)
				} else {
					enums = append(enums, item)
				}
			}
			if len(enums) > 0 {
				dst["enum"] = enums
			}
		case "minimum", "maximum":
			if num, err := strconv.ParseFloat(v, 64); err == nil {
				dst[k] = num
			}
		case "minItems", "maxItems", "minLength", "maxLength":
			if n, err := strconv.Atoi(v); err == nil {
				dst[k] = n
			}
		}
	}
}

func isExplicitRequired(f reflect.StructField) bool {
	tag := strings.TrimSpace(f.Tag.Get("jsonschema"))
	if tag == "" {
		return false
	}
	parts := strings.Split(tag, ";")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		k, v, ok := strings.Cut(p, "=")
		if !ok {
			continue
		}
		if strings.TrimSpace(k) == "required" && strings.TrimSpace(v) == "true" {
			return true
		}
	}
	return false
}

func parseTypedScalar(raw string, t reflect.Type) (any, bool) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	switch t.Kind() {
	case reflect.Bool:
		b, err := strconv.ParseBool(raw)
		return b, err == nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return nil, false
		}
		return n, true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		n, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return nil, false
		}
		return n, true
	case reflect.Float32, reflect.Float64:
		n, err := strconv.ParseFloat(raw, 64)
		return n, err == nil
	case reflect.String:
		return raw, true
	default:
		return raw, true
	}
}
