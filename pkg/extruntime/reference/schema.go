package reference

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
)

// SchemaDialect is the JSON Schema dialect of every generated schema; it is
// also the dialect of OpenAPI 3.1 documents, so fragments embed as they are.
const SchemaDialect = "https://json-schema.org/draft/2020-12/schema"

// Schema is a JSON Schema (2020-12) fragment.
type Schema = map[string]any

// MaxSafeInteger is the largest integer a wire `int` may carry (2^53 - 1): the
// range in which a JSON number survives a double.
const MaxSafeInteger = 1<<53 - 1

// Patterns of the string forms of manifest types. They are the same
// expressions manifest.ValidateValue applies, widened to ECMA-262 syntax.
const (
	patternInt64 = `^-?(0|[1-9][0-9]{0,18})$`
	patternDate  = `^[0-9]{4}-[0-9]{2}-[0-9]{2}$`
	// patternDateTime is what time.RFC3339Nano parsing accepts in shape: an
	// uppercase T and Z, a "." or "," before the fraction (Go takes both), and
	// a numeric offset with a colon. Calendar ranges are left to "format".
	patternDateTime = `^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}([.,][0-9]+)?(Z|[+-][0-9]{2}:[0-9]{2})$`
	patternUUID     = `^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`
)

// TypeSchema renders t as the JSON Schema of its wire form, the exact rules of
// manifest.ValidateValue (docs/extension-runtime.md §2.1):
//
//   - int is a JSON integer in the safe range ±(2^53-1). The server also
//     requires the literal to carry no fraction or exponent (1, not 1.0 or
//     1e2); JSON Schema validates the parsed number and cannot say so, which is
//     stated in the description;
//   - int64 and decimal are strings (a JSON number would lose digits), with
//     the patterns the server applies. The pattern of a decimal enforces its
//     scale; its precision (significant digits, leading zeros not counted) has
//     no regular expression short of a lookahead and is stated in the
//     description;
//   - date, datetime and uuid are strings in their canonical forms;
//   - ref is {entity, key} with the entity pinned when the type names one;
//   - enum is one of its string values;
//   - object lists its properties, requires the non-optional ones and refuses
//     unknown ones;
//   - nullable widens the type with null; optional only lets an object key be
//     absent (null still needs nullable), as the server does.
//
// The result has no "$schema"; use RootSchema for a standalone document.
func TypeSchema(t manifest.Type) Schema {
	s := Schema{}
	switch t.Kind {
	case manifest.KindString:
		s["type"] = "string"
		if t.MaxLength > 0 {
			s["maxLength"] = t.MaxLength
		}
	case manifest.KindInt:
		s["type"] = "integer"
		s["minimum"] = -MaxSafeInteger
		s["maximum"] = MaxSafeInteger
		s["description"] = "Integer literal within ±(2^53-1), written without a fraction or exponent: 1, not 1.0 or 1e2 (the server rejects those spellings; a JSON Schema validator cannot tell them apart)."
	case manifest.KindInt64:
		s["type"] = "string"
		s["pattern"] = patternInt64
		s["description"] = "Decimal string of a 64-bit signed integer."
	case manifest.KindDecimal:
		decimalSchema(s, t)
	case manifest.KindBool:
		s["type"] = "boolean"
	case manifest.KindDate:
		s["type"] = "string"
		s["format"] = "date"
		s["pattern"] = patternDate
	case manifest.KindDateTime:
		s["type"] = "string"
		s["format"] = "date-time"
		s["pattern"] = patternDateTime
		s["description"] = "RFC 3339 timestamp with an offset, uppercase T and Z (a comma is accepted before the fraction, as Go parses it)."
	case manifest.KindUUID:
		s["type"] = "string"
		s["format"] = "uuid"
		s["pattern"] = patternUUID
	case manifest.KindEnum:
		s["type"] = "string"
		s["enum"] = append([]any(nil), stringsToAny(t.Values)...)
	case manifest.KindJSON:
		// Any JSON value; the server treats a null as a missing one.
		if !t.Nullable {
			s["not"] = Schema{"type": "null"}
		}
		return s
	case manifest.KindRef:
		entity := Schema{"type": "string", "minLength": 1}
		if t.Entity != "" {
			entity = Schema{"const": t.Entity}
		}
		s["type"] = "object"
		s["properties"] = Schema{"entity": entity, "key": Schema{"type": "string", "minLength": 1}}
		s["required"] = []any{"entity", "key"}
	case manifest.KindArray:
		if t.Items == nil {
			return itemlessArray(t)
		}
		s["type"] = "array"
		s["items"] = TypeSchema(*t.Items)
	case manifest.KindObject:
		objectSchema(s, t)
	}
	if t.Nullable {
		makeNullable(s)
	}
	return s
}

// RootSchema is TypeSchema as a standalone document ("$schema" set).
func RootSchema(t manifest.Type) Schema {
	s := TypeSchema(t)
	s["$schema"] = SchemaDialect
	return s
}

func stringsToAny(in []string) []any {
	out := make([]any, len(in))
	for i, v := range in {
		out[i] = v
	}
	return out
}

// itemlessArray is the schema of an array type that declares no item type. The
// server rejects every value of it (even []), so the schema accepts none; a
// nullable one still takes null.
func itemlessArray(t manifest.Type) Schema {
	const desc = "An array without a declared item type: the runtime rejects every array here. Declare the item type."
	if t.Nullable {
		return Schema{"type": "null", "description": desc}
	}
	return Schema{"not": Schema{}, "description": desc}
}

func decimalSchema(s Schema, t manifest.Type) {
	s["type"] = "string"
	frac := `(\.[0-9]+)?`
	desc := "Decimal string"
	if t.Scale > 0 {
		frac = `(\.[0-9]{1,` + strconv.Itoa(t.Scale) + `})?`
		desc += fmt.Sprintf(" with at most %d fractional digits", t.Scale)
	}
	s["pattern"] = `^-?(0|[1-9][0-9]*)` + frac + `$`
	if t.Precision > 0 {
		// Not a length bound: leading zeros are not counted ("0.0005" has one
		// digit), so any bound on the length would reject valid values.
		desc += fmt.Sprintf(" and at most %d significant digits (leading zeros are not counted)", t.Precision)
	}
	s["description"] = desc + "."
}

func objectSchema(s Schema, t manifest.Type) {
	s["type"] = "object"
	props := Schema{}
	required := []any{}
	for _, p := range t.Properties {
		props[p.Name] = TypeSchema(p.Type)
		if !p.Type.Optional {
			required = append(required, p.Name)
		}
	}
	s["properties"] = props
	if len(required) > 0 {
		s["required"] = required
	}
	s["additionalProperties"] = false
}

// makeNullable widens the schema with null: the type becomes a union with
// "null" and an enum gains the null member.
func makeNullable(s Schema) {
	switch ty := s["type"].(type) {
	case string:
		s["type"] = []any{ty, "null"}
	case nil:
		// no type keyword (json): any value, null included
	}
	if e, ok := s["enum"].([]any); ok {
		s["enum"] = append(append([]any(nil), e...), nil)
	}
}

// Marshal renders v as indented JSON without HTML escaping.
func Marshal(v any) []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		// Schemas and examples are built from strings, numbers and maps only.
		return []byte("null\n")
	}
	return bytes.TrimRight(buf.Bytes(), "\n")
}

// Example is a valid sample value of t in exact wire form. Refs point at a
// uuid key (the key type of package entities); ExampleWith picks the key by
// entity.
func Example(t manifest.Type) json.RawMessage { return ExampleWith(t, nil) }

// ExampleWith is Example with the key sample of a ref chosen per entity (nil:
// a uuid). The result always satisfies manifest.ValidateValue(t, ...).
func ExampleWith(t manifest.Type, refKey func(entity string) string) json.RawMessage {
	b, err := json.Marshal(sample(t, refKey))
	if err != nil {
		return json.RawMessage("null")
	}
	return b
}

// orderedObject marshals its members in declaration order.
type orderedObject []member

type member struct {
	name  string
	value any
}

func (o orderedObject) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, m := range o {
		if i > 0 {
			buf.WriteByte(',')
		}
		k, err := json.Marshal(m.name)
		if err != nil {
			return nil, err
		}
		v, err := json.Marshal(m.value)
		if err != nil {
			return nil, err
		}
		buf.Write(k)
		buf.WriteByte(':')
		buf.Write(v)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

const (
	sampleUUID     = "123e4567-e89b-12d3-a456-426614174000"
	sampleDate     = "2024-01-15"
	sampleDateTime = "2024-01-15T09:30:00Z"
)

func sample(t manifest.Type, refKey func(string) string) any {
	switch t.Kind {
	case manifest.KindString:
		s := "text"
		if t.MaxLength > 0 && len([]rune(s)) > t.MaxLength {
			s = string([]rune(s)[:t.MaxLength])
		}
		return s
	case manifest.KindInt:
		return 1
	case manifest.KindInt64:
		return "1"
	case manifest.KindDecimal:
		return sampleDecimal(t)
	case manifest.KindBool:
		return true
	case manifest.KindDate:
		return sampleDate
	case manifest.KindDateTime:
		return sampleDateTime
	case manifest.KindUUID:
		return sampleUUID
	case manifest.KindEnum:
		if len(t.Values) > 0 {
			return t.Values[0]
		}
		return ""
	case manifest.KindJSON:
		return map[string]any{}
	case manifest.KindRef:
		entity := t.Entity
		if entity == "" {
			entity = "acme.entity"
		}
		key := sampleUUID
		if refKey != nil {
			if k := refKey(entity); k != "" {
				key = k
			}
		}
		return orderedObject{{"entity", entity}, {"key", key}}
	case manifest.KindArray:
		if t.Items == nil {
			return []any{}
		}
		return []any{sample(*t.Items, refKey)}
	case manifest.KindObject:
		obj := orderedObject{}
		for _, p := range t.Properties {
			obj = append(obj, member{p.Name, sample(p.Type, refKey)})
		}
		return obj
	}
	return nil
}

func sampleDecimal(t manifest.Type) string {
	frac := 0
	if t.Scale > 0 {
		frac = min(t.Scale, 2)
	}
	if t.Precision > 0 && 1+frac > t.Precision {
		frac = max(t.Precision-1, 0)
	}
	if frac == 0 {
		return "1"
	}
	return "1." + strings.Repeat("5", frac)
}

// Signature renders t compactly for tables: `{ id: uuid, note?: string(255) | null }`.
func Signature(t manifest.Type) string {
	var s string
	switch t.Kind {
	case manifest.KindString:
		s = "string"
		if t.MaxLength > 0 {
			s += "(" + strconv.Itoa(t.MaxLength) + ")"
		}
	case manifest.KindDecimal:
		s = "decimal"
		if t.Precision > 0 || t.Scale > 0 {
			s += "(" + strconv.Itoa(t.Precision) + "," + strconv.Itoa(t.Scale) + ")"
		}
	case manifest.KindEnum:
		s = "enum(" + strings.Join(t.Values, "|") + ")"
	case manifest.KindRef:
		s = "ref"
		if t.Entity != "" {
			s += "<" + t.Entity + ">"
		}
	case manifest.KindArray:
		if t.Items != nil {
			s = Signature(*t.Items)
			if strings.ContainsAny(s, " |") {
				s = "(" + s + ")"
			}
			s += "[]"
		} else {
			s = "array"
		}
	case manifest.KindObject:
		parts := make([]string, 0, len(t.Properties))
		for _, p := range t.Properties {
			name := p.Name
			if p.Type.Optional {
				name += "?"
			}
			parts = append(parts, name+": "+Signature(p.Type))
		}
		if len(parts) == 0 {
			s = "{}"
		} else {
			s = "{ " + strings.Join(parts, ", ") + " }"
		}
	case manifest.KindInt, manifest.KindInt64, manifest.KindBool, manifest.KindDate,
		manifest.KindDateTime, manifest.KindUUID, manifest.KindJSON:
		s = string(t.Kind)
	default:
		s = string(t.Kind)
	}
	if t.Nullable {
		s += " | null"
	}
	return s
}
