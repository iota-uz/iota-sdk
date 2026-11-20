// Package expr is the expression layer of automation graphs: the field language that declares payloads,
// node outputs and context variables, CEL expressions checked against it, and `{{ }}` templates.
//
// The package is pure: no database, no IO, no SDK modules. It does not import pkg/automation/graph.
package expr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// FieldKind is the kind of a Field.
type FieldKind int

const (
	// String is a JSON string (CEL string).
	String FieldKind = iota + 1
	// Int is an integer in the JS-safe range (CEL int).
	Int
	// Double is any JSON number (CEL double).
	Double
	// Bool is a JSON boolean.
	Bool
	// Dyn is any JSON value (CEL dyn).
	Dyn
	// Enum is a string out of Field.Enum (CEL string).
	Enum
	// List is a JSON array of Field.Elem.
	List
	// Map is a JSON object with string keys and values of Field.Elem.
	Map
	// Object is a JSON object with the declared Field.Props.
	Object
	// Optional is Field.Elem that may be left out or null: it then reads as the zero value of its type.
	Optional
	// Timestamp is an RFC 3339 instant (CEL timestamp). An SDK addition to the vocabulary: the step's `now`.
	Timestamp
)

var kindNames = map[FieldKind]string{
	String: "string", Int: "int", Double: "double", Bool: "bool", Dyn: "dyn", Timestamp: "timestamp",
	Enum: "enum", List: "list", Map: "map", Object: "object", Optional: "optional",
}

func (k FieldKind) String() string {
	if n, ok := kindNames[k]; ok {
		return n
	}
	return fmt.Sprintf("FieldKind(%d)", int(k))
}

// Field is one declaration of the field language. Its JSON form is the vocabulary's:
// "string" | "int" | "double" | "bool" | "dyn" | "timestamp" | {"enum": [...]} | {"list": F} | {"map": F} |
// {"object": {...}} | {"optional": F}. The order of an object's properties is kept.
type Field struct {
	Kind  FieldKind
	Enum  []string
	Elem  *Field // List, Map, Optional
	Props []Prop // Object, in declaration order
}

// Prop is one property of an object field.
type Prop struct {
	Name  string
	Field Field
}

// Fields is an ordered set of named fields: an event payload, a node output, a subject's context.
type Fields []Prop

// Constructors for fields declared in Go.
var (
	StringField    = Field{Kind: String}
	IntField       = Field{Kind: Int}
	DoubleField    = Field{Kind: Double}
	BoolField      = Field{Kind: Bool}
	DynField       = Field{Kind: Dyn}
	TimestampField = Field{Kind: Timestamp}
)

// EnumOf is {"enum": values}.
func EnumOf(values ...string) Field { return Field{Kind: Enum, Enum: values} }

// ListOf is {"list": f}.
func ListOf(f Field) Field { return Field{Kind: List, Elem: &f} }

// MapOf is {"map": f}.
func MapOf(f Field) Field { return Field{Kind: Map, Elem: &f} }

// OptionalOf is {"optional": f}.
func OptionalOf(f Field) Field { return Field{Kind: Optional, Elem: &f} }

// ObjectOf is {"object": props}.
func ObjectOf(props Fields) Field { return Field{Kind: Object, Props: props} }

// Get returns the property with this name.
func (fs Fields) Get(name string) (Field, bool) {
	for _, p := range fs {
		if p.Name == name {
			return p.Field, true
		}
	}
	return Field{}, false
}

// Prop returns the property of an object field (an optional object is looked through).
func (f Field) Prop(name string) (Field, bool) {
	f = f.Unwrap()
	if f.Kind != Object {
		return Field{}, false
	}
	return Fields(f.Props).Get(name)
}

// Unwrap returns the field under any Optional.
func (f Field) Unwrap() Field {
	for f.Kind == Optional && f.Elem != nil {
		f = *f.Elem
	}
	return f
}

// Required reports whether a value must be present: not optional and not dyn.
func (f Field) Required() bool { return f.Kind != Optional && f.Kind != Dyn }

var identRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// reserved are CEL words that cannot name a field read with a select.
var reserved = map[string]bool{
	"true": true, "false": true, "null": true, "in": true, "as": true, "break": true, "const": true,
	"continue": true, "else": true, "for": true, "function": true, "if": true, "import": true, "let": true,
	"loop": true, "package": true, "namespace": true, "return": true, "var": true, "void": true, "while": true,
}

// Validate reports a declaration that cannot be used: an empty enum, a property that is not a CEL identifier.
func (f Field) Validate() error {
	switch f.Kind {
	case String, Int, Double, Bool, Dyn, Timestamp:
		return nil
	case Enum:
		if len(f.Enum) == 0 {
			return fmt.Errorf("an enum needs at least one value")
		}
		return nil
	case List, Map, Optional:
		if f.Elem == nil {
			return fmt.Errorf("%s needs an element field", f.Kind)
		}
		if err := f.Elem.Validate(); err != nil {
			return fmt.Errorf("%s: %w", f.Kind, err)
		}
		return nil
	case Object:
		return Fields(f.Props).Validate()
	}
	return fmt.Errorf("unknown field kind %d", int(f.Kind))
}

// Validate reports a property name that is not a CEL identifier, a name used twice or an invalid field.
func (fs Fields) Validate() error {
	seen := map[string]bool{}
	for _, p := range fs {
		if !identRe.MatchString(p.Name) || reserved[p.Name] {
			return fmt.Errorf("%q is not a field name: letters, digits and _, not a CEL keyword", p.Name)
		}
		if seen[p.Name] {
			return fmt.Errorf("%q is declared twice", p.Name)
		}
		seen[p.Name] = true
		if err := p.Field.Validate(); err != nil {
			return fmt.Errorf("%s: %w", p.Name, err)
		}
	}
	return nil
}

// String is the field's JSON form.
func (f Field) String() string {
	b, _ := json.Marshal(f)
	return string(b)
}

// MarshalJSON writes the vocabulary's form.
func (f Field) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	if err := f.writeJSON(&b); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func (f Field) writeJSON(b *bytes.Buffer) error {
	switch f.Kind {
	case String, Int, Double, Bool, Dyn, Timestamp:
		b.WriteString(`"` + f.Kind.String() + `"`)
	case Enum:
		values, err := json.Marshal(f.Enum)
		if err != nil {
			return err
		}
		b.WriteString(`{"enum":`)
		b.Write(values)
		b.WriteString(`}`)
	case List, Map, Optional:
		if f.Elem == nil {
			return fmt.Errorf("%s without an element", f.Kind)
		}
		b.WriteString(`{"` + f.Kind.String() + `":`)
		if err := f.Elem.writeJSON(b); err != nil {
			return err
		}
		b.WriteString(`}`)
	case Object:
		b.WriteString(`{"object":`)
		if err := Fields(f.Props).writeJSON(b); err != nil {
			return err
		}
		b.WriteString(`}`)
	default:
		return fmt.Errorf("unknown field kind %d", int(f.Kind))
	}
	return nil
}

// MarshalJSON writes the fields as an object in declaration order.
func (fs Fields) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	if err := fs.writeJSON(&b); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func (fs Fields) writeJSON(b *bytes.Buffer) error {
	b.WriteString("{")
	for i, p := range fs {
		if i > 0 {
			b.WriteString(",")
		}
		name, _ := json.Marshal(p.Name)
		b.Write(name)
		b.WriteString(":")
		if err := p.Field.writeJSON(b); err != nil {
			return err
		}
	}
	b.WriteString("}")
	return nil
}

// UnmarshalJSON reads the vocabulary's form.
func (f *Field) UnmarshalJSON(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	v, err := readField(dec)
	if err != nil {
		return err
	}
	*f = v
	return nil
}

// UnmarshalJSON reads an object of fields, keeping the order of its keys.
func (fs *Fields) UnmarshalJSON(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	t, err := dec.Token()
	if err != nil {
		return err
	}
	if t == nil {
		*fs = nil
		return nil
	}
	if d, ok := t.(json.Delim); !ok || d != '{' {
		return fmt.Errorf("fields: an object of field declarations, got %v", t)
	}
	v, err := readProps(dec)
	if err != nil {
		return err
	}
	*fs = v
	return nil
}

func readProps(dec *json.Decoder) (Fields, error) {
	var out Fields
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return nil, err
		}
		name, ok := t.(string)
		if !ok {
			return nil, fmt.Errorf("fields: a property name, got %v", t)
		}
		f, err := readField(dec)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		out = append(out, Prop{Name: name, Field: f})
	}
	if _, err := dec.Token(); err != nil { // '}'
		return nil, err
	}
	if err := out.Validate(); err != nil {
		return nil, err
	}
	return out, nil
}

func readField(dec *json.Decoder) (Field, error) {
	t, err := dec.Token()
	if err != nil {
		return Field{}, err
	}
	switch v := t.(type) {
	case string:
		for k, n := range kindNames {
			if n == v && k <= Dyn || n == v && k == Timestamp {
				return Field{Kind: k}, nil
			}
		}
		return Field{}, fmt.Errorf("%q is not a field: string, int, double, bool, dyn, timestamp or an object form", v)
	case json.Delim:
		if v != '{' {
			return Field{}, fmt.Errorf("a field is a name or an object, got %v", v)
		}
	default:
		return Field{}, fmt.Errorf("a field is a name or an object, got %v", t)
	}
	kt, err := dec.Token()
	if err != nil {
		return Field{}, err
	}
	key, ok := kt.(string)
	if !ok {
		return Field{}, fmt.Errorf("a field object has one key, got %v", kt)
	}
	var f Field
	switch key {
	case "enum":
		var values []string
		if err := dec.Decode(&values); err != nil {
			return Field{}, fmt.Errorf("enum: %w", err)
		}
		f = Field{Kind: Enum, Enum: values}
	case "list", "map", "optional":
		elem, err := readField(dec)
		if err != nil {
			return Field{}, fmt.Errorf("%s: %w", key, err)
		}
		kind := map[string]FieldKind{"list": List, "map": Map, "optional": Optional}[key]
		f = Field{Kind: kind, Elem: &elem}
	case "object":
		ot, err := dec.Token()
		if err != nil {
			return Field{}, err
		}
		if d, ok := ot.(json.Delim); !ok || d != '{' {
			return Field{}, fmt.Errorf("object: an object of fields, got %v", ot)
		}
		props, err := readProps(dec)
		if err != nil {
			return Field{}, fmt.Errorf("object: %w", err)
		}
		f = Field{Kind: Object, Props: props}
	default:
		return Field{}, fmt.Errorf("%q is not a field form: enum, list, map, object or optional", key)
	}
	if dec.More() {
		return Field{}, fmt.Errorf("a field object has one key")
	}
	if _, err := dec.Token(); err != nil { // '}'
		return Field{}, err
	}
	return f, f.Validate()
}

// TypeName is the CEL-like name of the field's type, for messages and completions.
func (f Field) TypeName() string {
	switch f.Kind {
	case String, Int, Double, Bool, Dyn, Timestamp:
		return f.Kind.String()
	case Enum:
		return "string"
	case List:
		return "list(" + f.Elem.TypeName() + ")"
	case Map:
		return "map(string, " + f.Elem.TypeName() + ")"
	case Optional:
		return f.Elem.TypeName()
	case Object:
		names := make([]string, 0, len(f.Props))
		for _, p := range f.Props {
			names = append(names, p.Name)
		}
		return "object{" + strings.Join(names, ", ") + "}"
	}
	return "dyn"
}
