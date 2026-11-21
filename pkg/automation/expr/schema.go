package expr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

// OrderedObject is a JSON object that keeps the order of its keys when marshaled.
type OrderedObject []KV

// KV is one key of an OrderedObject.
type KV struct {
	Key   string
	Value any
}

// MarshalJSON writes the keys in order.
func (o OrderedObject) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, kv := range o {
		if i > 0 {
			b.WriteByte(',')
		}
		k, err := json.Marshal(kv.Key)
		if err != nil {
			return nil, err
		}
		b.Write(k)
		b.WriteByte(':')
		v, err := json.Marshal(kv.Value)
		if err != nil {
			return nil, err
		}
		b.Write(v)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// JSONSchema is the field as JSON Schema: a string, integer, number, boolean, array or object; dyn is {}.
// An object lists its properties in declaration order, requires those that are not optional or dyn and
// takes no other keys.
func (f Field) JSONSchema() json.RawMessage {
	return mustJSON(f.schema())
}

// JSONSchema is the fields as the JSON Schema of an object.
func (fs Fields) JSONSchema() json.RawMessage {
	return mustJSON(fs.schema())
}

// mustJSON marshals values built from strings, bools, slices and OrderedObject, which cannot fail.
func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("expr: marshal a schema: %v", err))
	}
	return b
}

func (f Field) schema() any {
	switch f.Kind {
	case String:
		return OrderedObject{{"type", "string"}}
	case Int:
		return OrderedObject{{"type", "integer"}}
	case Double:
		return OrderedObject{{"type", "number"}}
	case Bool:
		return OrderedObject{{"type", "boolean"}}
	case Timestamp:
		return OrderedObject{{"type", "string"}, {"format", "date-time"}}
	case Enum:
		return OrderedObject{{"type", "string"}, {"enum", f.Enum}}
	case List:
		return OrderedObject{{"type", "array"}, {"items", f.Elem.schema()}}
	case Map:
		return OrderedObject{{"type", "object"}, {"additionalProperties", f.Elem.schema()}}
	case Optional:
		return f.Elem.schema()
	case Object:
		return Fields(f.Props).schema()
	case Dyn:
		return OrderedObject{}
	}
	return OrderedObject{}
}

func (fs Fields) schema() any {
	props := OrderedObject{}
	required := []string{}
	for _, p := range fs {
		props = append(props, KV{p.Name, p.Field.schema()})
		if p.Field.Required() {
			required = append(required, p.Name)
		}
	}
	return OrderedObject{{"type", "object"}, {"properties", props}, {"required", required}, {"additionalProperties", false}}
}

// FieldFromJSONSchema reads a JSON Schema of the subset a node's declared output may use (a Code node's
// `output_schema`): `type` object (`properties`, `required`), array (`items`), string (`enum`), integer,
// number, boolean, and `description` anywhere. An object's properties not in `required` are optional.
// Anything outside the subset is an error, not dyn.
func FieldFromJSONSchema(s json.RawMessage) (Field, error) {
	dec := json.NewDecoder(bytes.NewReader(s))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return Field{}, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return Field{}, fmt.Errorf("unexpected data after the schema")
	}
	order, err := keyOrder(s)
	if err != nil {
		return Field{}, err
	}
	return fieldFromSchema(v, order, "")
}

// FieldFromSchemaValue is FieldFromJSONSchema for an already decoded value (property order is then sorted).
func FieldFromSchemaValue(v any) (Field, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return Field{}, err
	}
	return FieldFromJSONSchema(b)
}

func fieldFromSchema(v any, order map[string][]string, at string) (Field, error) {
	o, ok := v.(map[string]any)
	if !ok {
		return Field{}, fmt.Errorf("%sa schema is an object", where(at))
	}
	allowed := map[string]bool{"type": true, "description": true}
	typ, _ := o["type"].(string)
	if d, ok := o["description"]; ok {
		if s, ok := d.(string); !ok || len(s) > 500 {
			return Field{}, fmt.Errorf("%sdescription is a string of at most 500 characters", where(at))
		}
	}
	var f Field
	switch typ {
	case "string":
		allowed["enum"] = true
		f = StringField
		if e, ok := o["enum"]; ok {
			list, ok := e.([]any)
			if !ok || len(list) == 0 || len(list) > 100 {
				return Field{}, fmt.Errorf("%senum is a list of 1 to 100 strings", where(at))
			}
			values := make([]string, 0, len(list))
			for _, x := range list {
				s, ok := x.(string)
				if !ok || len(s) > 200 {
					return Field{}, fmt.Errorf("%senum values are strings of at most 200 characters", where(at))
				}
				values = append(values, s)
			}
			f = EnumOf(values...)
		}
	case "integer":
		f = IntField
	case "number":
		f = DoubleField
	case "boolean":
		f = BoolField
	case "array":
		allowed["items"] = true
		items, ok := o["items"]
		if !ok {
			return Field{}, fmt.Errorf("%san array declares its items", where(at))
		}
		elem, err := fieldFromSchema(items, order, at+"/items")
		if err != nil {
			return Field{}, err
		}
		f = ListOf(elem)
	case "object":
		allowed["properties"] = true
		allowed["required"] = true
		props, ok := o["properties"].(map[string]any)
		if !ok {
			return Field{}, fmt.Errorf("%san object declares its properties", where(at))
		}
		if len(props) > 100 {
			return Field{}, fmt.Errorf("%sat most 100 properties", where(at))
		}
		required := map[string]bool{}
		if r, ok := o["required"]; ok {
			list, ok := r.([]any)
			if !ok {
				return Field{}, fmt.Errorf("%srequired is a list of property names", where(at))
			}
			for _, x := range list {
				name, ok := x.(string)
				if !ok {
					return Field{}, fmt.Errorf("%srequired is a list of property names", where(at))
				}
				if _, ok := props[name]; !ok {
					return Field{}, fmt.Errorf("%s%s is required but not one of the properties", where(at), name)
				}
				required[name] = true
			}
		}
		names := order[at+"/properties"]
		if len(names) != len(props) {
			names = names[:0]
			for k := range props {
				names = append(names, k)
			}
			sort.Strings(names)
		}
		fields := Fields{}
		for _, name := range names {
			pf, err := fieldFromSchema(props[name], order, at+"/properties/"+name)
			if err != nil {
				return Field{}, err
			}
			if !required[name] {
				pf = OptionalOf(pf)
			}
			fields = append(fields, Prop{Name: name, Field: pf})
		}
		if err := fields.Validate(); err != nil {
			return Field{}, fmt.Errorf("%s%w", where(at), err)
		}
		f = ObjectOf(fields)
	default:
		return Field{}, fmt.Errorf("%stype is one of object, array, string, integer, number, boolean", where(at))
	}
	for _, k := range sortedKeys(o) {
		if !allowed[k] {
			return Field{}, fmt.Errorf("%s%s is not part of the schema subset", where(at), k)
		}
	}
	return f, nil
}

func where(at string) string {
	if at == "" {
		return ""
	}
	return at + ": "
}

// keyOrder maps each JSON pointer of an object in the document to its keys in source order.
func keyOrder(raw []byte) (map[string][]string, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	out := map[string][]string{}
	var walk func(ptr string) error
	walk = func(ptr string) error {
		t, err := dec.Token()
		if err != nil {
			return err
		}
		d, ok := t.(json.Delim)
		if !ok {
			return nil
		}
		switch d {
		case '{':
			keys := []string{}
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return err
				}
				k, _ := kt.(string)
				for _, seen := range keys {
					if seen == k {
						return fmt.Errorf("%s: %s is declared twice", ptr, k)
					}
				}
				keys = append(keys, k)
				if err := walk(ptr + "/" + k); err != nil {
					return err
				}
			}
			out[ptr] = keys
			_, err = dec.Token()
			return err
		case '[':
			for i := 0; dec.More(); i++ {
				if err := walk(fmt.Sprintf("%s/%d", ptr, i)); err != nil {
					return err
				}
			}
			_, err = dec.Token()
			return err
		}
		return nil
	}
	return out, walk("")
}
