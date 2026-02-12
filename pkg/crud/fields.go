package crud

import "fmt"

type Fields interface {
	Names() []string
	Fields() []Field
	Searchable() []Field
	KeyField() Field
	KeyFields() []Field
	Field(name string) (Field, error)
	FieldValues(values map[string]any) ([]FieldValue, error)
}

func NewFields(value []Field) Fields {
	dict := make(map[string]Field, len(value))
	keyFields := make([]Field, 0)

	for _, f := range value {
		name := f.Name()
		if _, exists := dict[name]; exists {
			panic(fmt.Sprintf("duplicate field name: %q", name))
		}
		if f.Key() {
			keyFields = append(keyFields, f)
		}
		dict[name] = f
	}

	if len(keyFields) == 0 {
		panic("should have at least one key field")
	}

	return &fields{
		dict:      dict,
		fields:    value,
		keyFields: keyFields,
	}
}

type fields struct {
	dict      map[string]Field
	keyFields []Field
	fields    []Field
}

func (f *fields) Names() []string {
	names := make([]string, len(f.fields))
	for i, f := range f.fields {
		names[i] = f.Name()
	}
	return names
}

func (f *fields) Fields() []Field {
	return f.fields
}

func (f *fields) Searchable() []Field {
	searchableFields := make([]Field, 0)
	for _, field := range f.fields {
		if field.Searchable() {
			searchableFields = append(searchableFields, field)
		}
	}

	return searchableFields
}

func (f *fields) KeyField() Field {
	return f.keyFields[0]
}

func (f *fields) KeyFields() []Field {
	return f.keyFields
}

func (f *fields) Field(name string) (Field, error) {
	if field, ok := f.dict[name]; ok {
		return field, nil
	}
	return nil, fmt.Errorf("field %q not found", name)
}

func (f *fields) FieldValues(values map[string]any) ([]FieldValue, error) {
	fvs := make([]FieldValue, len(f.fields))
	for i, field := range f.fields {
		value, ok := values[field.Name()]
		if !ok {
			return nil, fmt.Errorf("missing value for field: %s", field.Name())
		}

		fvs[i] = field.Value(value)
	}

	return fvs, nil
}
