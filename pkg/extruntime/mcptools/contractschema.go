package mcptools

import (
	"sort"
	"strconv"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
)

// jsonSchema is a JSON Schema (draft 2020-12) document as plain data.
type jsonSchema = map[string]any

// SchemaOfType renders a manifest wire type as the JSON Schema of its wire
// form: what a caller of the contract sends and receives. The runtime
// validates against the manifest type itself; this is its documentation.
func SchemaOfType(t manifest.Type) jsonSchema {
	s := baseSchema(t)
	if t.Nullable {
		return jsonSchema{"anyOf": []any{s, jsonSchema{"type": "null"}}}
	}
	return s
}

func baseSchema(t manifest.Type) jsonSchema {
	switch t.Kind {
	case manifest.KindString:
		s := jsonSchema{"type": "string"}
		if t.MaxLength > 0 {
			s["maxLength"] = t.MaxLength
		}
		return s
	case manifest.KindInt:
		return jsonSchema{"type": "integer", "description": "safe integer (|n| < 2^53)"}
	case manifest.KindInt64:
		return jsonSchema{"type": "string", "pattern": "^-?[0-9]+$", "description": "int64 as a decimal string"}
	case manifest.KindDecimal:
		s := jsonSchema{"type": "string", "pattern": "^-?[0-9]+(\\.[0-9]+)?$", "description": "decimal as a string"}
		if t.Precision > 0 {
			s["description"] = "decimal as a string, up to " + strconv.Itoa(t.Precision) + " digits, " + strconv.Itoa(t.Scale) + " after the point"
		}
		return s
	case manifest.KindBool:
		return jsonSchema{"type": "boolean"}
	case manifest.KindDate:
		return jsonSchema{"type": "string", "format": "date"}
	case manifest.KindDateTime:
		return jsonSchema{"type": "string", "format": "date-time", "description": "RFC 3339 with offset"}
	case manifest.KindUUID:
		return jsonSchema{"type": "string", "format": "uuid"}
	case manifest.KindEnum:
		values := make([]any, len(t.Values))
		for i, v := range t.Values {
			values[i] = v
		}
		return jsonSchema{"type": "string", "enum": values}
	case manifest.KindJSON:
		return jsonSchema{"description": "any JSON value"}
	case manifest.KindRef:
		key := jsonSchema{"type": "string"}
		entity := jsonSchema{"type": "string"}
		if t.Entity != "" {
			entity["const"] = t.Entity
		}
		return jsonSchema{
			"type": "object", "additionalProperties": false, "required": []any{"entity", "key"},
			"properties":  jsonSchema{"entity": entity, "key": key},
			"description": "reference to a row of an entity",
		}
	case manifest.KindArray:
		s := jsonSchema{"type": "array"}
		if t.Items != nil {
			s["items"] = SchemaOfType(*t.Items)
		}
		return s
	case manifest.KindObject:
		props := jsonSchema{}
		required := []any{}
		for _, p := range t.Properties {
			props[p.Name] = SchemaOfType(p.Type)
			if !p.Type.Optional {
				required = append(required, p.Name)
			}
		}
		s := jsonSchema{"type": "object", "properties": props, "additionalProperties": false}
		if len(required) > 0 {
			s["required"] = required
		}
		return s
	}
	return jsonSchema{}
}

// rowSchema is the wire form of a row of an entity or overlay: "key" and the
// declared fields.
func rowSchema(r manifest.Resource) jsonSchema {
	props := jsonSchema{"key": jsonSchema{"type": "string"}}
	required := []any{"key"}
	for _, f := range r.Fields {
		props[f.ID] = SchemaOfType(f.Type)
		if !f.Type.Optional && !f.Type.Nullable && f.Default == nil {
			required = append(required, f.ID)
		}
	}
	return jsonSchema{"type": "object", "properties": props, "required": required}
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
