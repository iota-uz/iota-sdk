package inbox

import (
	"bytes"
	"encoding/json"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

// Field is one decision field of a human node's form (a property of its JSON Schema), as a control.
type Field struct {
	Name, Title, Description string
	// Kind is the control: text, textarea, number, integer, boolean, enum or json (a schema without a
	// type the form can render).
	Kind string
	// Enum are the choices of an enum: each value as JSON text (a string, a number, a boolean), so a
	// choice goes back with its type.
	Enum     []EnumOption
	Required bool
	Value    string // what the user entered
}

// EnumOption is one choice of an enum field: Value is the choice as JSON text, Label how it reads.
type EnumOption struct{ Value, Label string }

// fieldsOf reads the top-level properties of a form schema in their written order.
func fieldsOf(schema json.RawMessage) []Field {
	if len(schema) == 0 {
		return nil
	}
	var doc struct {
		Properties json.RawMessage `json:"properties"`
		Required   []string        `json:"required"`
	}
	if err := json.Unmarshal(schema, &doc); err != nil || len(doc.Properties) == 0 {
		return nil
	}
	names, props := orderedObject(doc.Properties)
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
	return strings.ToUpper(s[:1]) + s[1:]
}

// fieldsFromForm reads the decision fields of a submitted form (field.<name>) by their controls; the
// values the user entered are kept for a form shown again. A value that is not of its kind stays a
// string and the schema check refuses it.
func fieldsFromForm(fields []Field, form url.Values) (json.RawMessage, []Field) {
	if len(fields) == 0 {
		return nil, nil
	}
	out := map[string]any{}
	kept := make([]Field, len(fields))
	for i, f := range fields {
		raw := form.Get("field." + f.Name)
		f.Value = raw
		kept[i] = f
		v := strings.TrimSpace(raw)
		switch f.Kind {
		case "boolean":
			out[f.Name] = v != ""
			continue
		case "integer":
			if v == "" {
				continue
			}
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				out[f.Name] = n
				continue
			}
		case "number":
			if v == "" {
				continue
			}
			if n, err := strconv.ParseFloat(strings.ReplaceAll(v, ",", "."), 64); err == nil {
				out[f.Name] = n
				continue
			}
		case "enum":
			if v == "" {
				continue
			}
			var x any
			if err := json.Unmarshal([]byte(v), &x); err == nil {
				out[f.Name] = x
				continue
			}
		case "json":
			if v == "" {
				continue
			}
			var x any
			if err := json.Unmarshal([]byte(v), &x); err == nil {
				out[f.Name] = x
				continue
			}
		default:
			if v == "" {
				continue
			}
		}
		out[f.Name] = v
	}
	b, err := json.Marshal(out)
	if err != nil {
		return nil, kept
	}
	return b, kept
}
