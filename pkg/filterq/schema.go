package filterq

// Field is the validation surface for one filterable field: its type and the
// operators it accepts. UI metadata (labels, options) lives in the consumer's
// field registry, not here.
type Field struct {
	Key       string
	Type      FieldType
	Operators []Operator // nil → DefaultOperators(Type)
}

// AllowsOp reports whether the operator is permitted on this field.
func (f Field) AllowsOp(op Operator) bool {
	ops := f.Operators
	if ops == nil {
		ops = DefaultOperators(f.Type)
	}
	for _, o := range ops {
		if o == op {
			return true
		}
	}
	return false
}

// Schema is the set of fields Decode validates against.
type Schema struct {
	Fields []Field
}

// Field looks up a field by key.
func (s Schema) Field(key string) (Field, bool) {
	for _, f := range s.Fields {
		if f.Key == key {
			return f, true
		}
	}
	return Field{}, false
}
