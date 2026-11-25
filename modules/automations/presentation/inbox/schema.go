package inbox

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Field is one decision field of a human node's form (a property of its JSON Schema), as a control.
type Field struct {
	Name, Title, Description string
	// Kind is the control: text, textarea, number, integer, boolean, enum, json (a schema without a
	// type the form can render) or document (the whole fields object as JSON: a schema whose fields
	// are not its own top-level properties, see documentField). A boolean that is not required is a
	// choice of unset, yes and no, so it can be left out.
	Kind string
	// Enum are the choices of an enum: each value as JSON text (a string, a number, a boolean), so a
	// choice goes back with its type.
	Enum     []EnumOption
	Required bool
	Value    string // what the user entered
}

// EnumOption is one choice of an enum field: Value is the choice as JSON text, Label how it reads.
type EnumOption struct{ Value, Label string }

// KindDocument is the one control of a form schema whose fields the form cannot list: the fields
// object typed as JSON, submitted as "fields" and checked by the same schema as any decision.
const KindDocument = "document"

// documentFormName is the form name of the document control.
const documentFormName = "fields"

// fieldsOf reads the top-level properties of a form schema in their written order. A schema that
// constrains its fields some other way ($ref, allOf, a required name without a property...) and
// lists no property of its own gets one document control instead, so its task stays decidable.
func fieldsOf(schema json.RawMessage) []Field {
	if len(schema) == 0 {
		return nil
	}
	var doc struct {
		Properties json.RawMessage `json:"properties"`
		Required   []string        `json:"required"`
	}
	if err := json.Unmarshal(schema, &doc); err != nil {
		return nil
	}
	var names []string
	var props map[string]json.RawMessage
	if len(doc.Properties) > 0 {
		names, props = orderedObject(doc.Properties)
	}
	if len(names) == 0 {
		return documentField(schema, len(doc.Required) > 0)
	}
	if !listable(schema, doc.Required, props) {
		// Properties of its own and more the controls cannot show (allOf, $ref, a required name without
		// a property): the whole fields object as JSON, so every constraint can be met.
		return []Field{{Kind: KindDocument, Required: len(doc.Required) > 0}}
	}
	out := make([]Field, 0, len(names))
	for _, name := range names {
		var p struct {
			Type        any               `json:"type"`
			Title       string            `json:"title"`
			Description string            `json:"description"`
			Enum        []json.RawMessage `json:"enum"`
			Format      string            `json:"format"`
			MaxLength   int               `json:"maxLength"`
		}
		if err := json.Unmarshal(props[name], &p); err != nil {
			continue
		}
		f := Field{Name: name, Title: strings.TrimSpace(p.Title), Description: strings.TrimSpace(p.Description), Required: slices.Contains(doc.Required, name)}
		if f.Title == "" {
			f.Title = humanize(name)
		}
		typ, _ := p.Type.(string)
		switch {
		case len(p.Enum) > 0:
			f.Kind = "enum"
			// The values as the schema wrote them: a declared null is a choice of its own, apart from
			// leaving the field out.
			for _, raw := range p.Enum {
				var compact bytes.Buffer
				if err := json.Compact(&compact, raw); err != nil {
					continue
				}
				value := compact.String()
				label := value
				var s string
				if err := json.Unmarshal(raw, &s); err == nil {
					label = s
				}
				if value == "null" {
					label = ""
				}
				f.Enum = append(f.Enum, EnumOption{Value: value, Label: label})
			}
		case typ == "string" && (p.Format == "textarea" || p.MaxLength == 0 || p.MaxLength > 200):
			f.Kind = "textarea"
		case typ == "string":
			f.Kind = "text"
		case typ == "integer":
			f.Kind = "integer"
		case typ == "number":
			f.Kind = "number"
		case typ == "boolean":
			f.Kind = "boolean"
		default:
			f.Kind = "json"
		}
		out = append(out, f)
	}
	return out
}

// plainKeys are the keywords of an object schema that ask for no field: a schema of only these needs
// no control.
var plainKeys = map[string]bool{
	"type": true, "title": true, "description": true, "$schema": true, "$id": true, "$comment": true,
	"properties": true, "required": true, "additionalProperties": true, "examples": true, "default": true,
	"readOnly": true, "writeOnly": true, "deprecated": true,
}

// composedKeys are the keywords of an object schema that can constrain fields beside its own properties
// (dependentRequired only names properties: the controls can meet it).
var composedKeys = []string{"$ref", "$dynamicRef", "allOf", "anyOf", "oneOf", "not", "if", "then", "else",
	"dependentSchemas", "dependencies"}

// listable reports a schema whose fields are exactly its own properties: no composition keyword and no
// required name without a property.
func listable(schema json.RawMessage, required []string, props map[string]json.RawMessage) bool {
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(schema, &keys); err != nil {
		return true
	}
	for _, k := range composedKeys {
		if _, ok := keys[k]; ok {
			return false
		}
	}
	for _, name := range required {
		if _, ok := props[name]; !ok {
			return false
		}
	}
	return true
}

// documentField is the control of a schema without top-level properties: none when the schema asks
// for nothing (an empty object passes it), else the fields object as JSON.
func documentField(schema json.RawMessage, required bool) []Field {
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(schema, &keys); err != nil {
		return nil
	}
	plain := !required
	for k := range keys {
		if !plainKeys[k] {
			plain = false
			break
		}
	}
	if plain {
		return nil
	}
	return []Field{{Kind: KindDocument, Required: required}}
}

// orderedObject reads the keys of a JSON object in their order.
func orderedObject(raw json.RawMessage) ([]string, map[string]json.RawMessage) {
	props := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &props); err != nil {
		return nil, nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	var names []string
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return nil, nil
	}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		key, _ := tok.(string)
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			break
		}
		names = append(names, key)
	}
	return names, props
}

func humanize(name string) string {
	s := strings.ReplaceAll(strings.ReplaceAll(name, "_", " "), "-", " ")
	if s == "" {
		return name
	}
	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return s
	}
	return string(unicode.ToUpper(r)) + s[size:]
}

var (
	integerToken = regexp.MustCompile(`^[+-]?[0-9]+$`)
	numberToken  = regexp.MustCompile(`^[+-]?([0-9]+\.?[0-9]*|\.[0-9]+)([eE][+-]?[0-9]+)?$`)
)

// numberOf is the JSON number a person typed, exactly as typed (no float rounding, an integer of any
// size): a decimal comma reads as a point, a leading + and leading zeros go. ok is false for text
// that is no number; err is set for a number JSON cannot carry (NaN, Infinity).
func numberOf(v string, integer bool) (json.Number, bool, error) {
	if !integer {
		v = strings.ReplaceAll(v, ",", ".")
	}
	if integer && !integerToken.MatchString(v) || !integer && !numberToken.MatchString(v) {
		if f, err := strconv.ParseFloat(v, 64); err == nil && (math.IsNaN(f) || math.IsInf(f, 0)) {
			return "", false, fmt.Errorf("%q is not a finite number", v)
		}
		return "", false, nil
	}
	neg := strings.HasPrefix(v, "-")
	v = strings.TrimLeft(v, "+-")
	mantissa, exp := v, ""
	if i := strings.IndexAny(v, "eE"); i >= 0 {
		mantissa, exp = v[:i], v[i:]
	}
	whole, frac, _ := strings.Cut(mantissa, ".")
	whole = strings.TrimLeft(whole, "0")
	if whole == "" {
		whole = "0"
	}
	out := whole
	if frac != "" {
		out += "." + frac
	}
	out += exp
	if neg {
		out = "-" + out
	}
	if !json.Valid([]byte(out)) {
		return "", false, nil
	}
	return json.Number(out), true, nil
}

// fieldsFromForm reads the decision fields of a submitted form (field.<name>) by their controls; the
// values the user entered are kept for a form shown again. A value that is not of its kind stays a
// string and the schema check refuses it. An error is a submission no decision can carry (a number
// JSON cannot write, a document that is not JSON): the caller refuses it as an invalid decision.
func fieldsFromForm(fields []Field, form url.Values) (json.RawMessage, []Field, error) {
	if len(fields) == 0 {
		return nil, nil, nil
	}
	if len(fields) == 1 && fields[0].Kind == KindDocument {
		f := fields[0]
		f.Value = form.Get(documentFormName)
		kept := []Field{f}
		v := strings.TrimSpace(f.Value)
		if v == "" {
			return nil, kept, nil
		}
		if !json.Valid([]byte(v)) {
			return nil, kept, errors.New("the fields are not JSON")
		}
		return json.RawMessage(v), kept, nil
	}
	out := map[string]any{}
	kept := make([]Field, len(fields))
	var bad error
	for i, f := range fields {
		raw := form.Get("field." + f.Name)
		f.Value = raw
		kept[i] = f
		v := strings.TrimSpace(raw)
		switch f.Kind {
		case "boolean":
			if f.Required {
				// A checkbox: unchecked is an explicit false.
				out[f.Name] = v != ""
				continue
			}
			// Unset, yes or no: unset leaves the property out.
			switch v {
			case "":
				continue
			case "true":
				out[f.Name] = true
				continue
			case "false":
				out[f.Name] = false
				continue
			}
		case "integer", "number":
			if v == "" {
				continue
			}
			n, ok, err := numberOf(v, f.Kind == "integer")
			if err != nil {
				bad = errors.Join(bad, fmt.Errorf("%s: %w", f.Name, err))
				continue
			}
			if ok {
				out[f.Name] = n
				continue
			}
		case "enum":
			if v == "" {
				continue
			}
			// A choice goes back as the schema wrote it (no float rounding of a large integer, any
			// nested value as is); anything else is sent as typed and the schema refuses it.
			if i := slices.IndexFunc(f.Enum, func(o EnumOption) bool { return o.Value == v }); i >= 0 {
				out[f.Name] = json.RawMessage(f.Enum[i].Value)
				continue
			}
		case "json":
			if v == "" {
				continue
			}
			if json.Valid([]byte(v)) {
				out[f.Name] = json.RawMessage(v)
				continue
			}
		default:
			if v == "" {
				continue
			}
		}
		out[f.Name] = v
	}
	if bad != nil {
		return nil, kept, bad
	}
	b, err := json.Marshal(out)
	if err != nil {
		return nil, kept, err
	}
	return b, kept, nil
}
