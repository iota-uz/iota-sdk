package crud_v2

import "fmt"

type Fields interface {
	Names() []string
	Fields() []Field
	Searchable() []Field
	GetKeyField() Field
	GetField(name string) Field
}

func NewFields(value []Field) Fields {
	dict := make(map[string]Field, len(value))
	keyIndex := -1

	for i, f := range value {
		name := f.Name()
		if _, exists := dict[name]; exists {
			panic(fmt.Sprintf("duplicate field name: %q", name))
		}
		if f.Key() {
			if keyIndex == -1 {
				keyIndex = i
			} else {
				panic("expected exactly one key field")
			}
		}
		dict[name] = f
	}

	if keyIndex == -1 {
		panic("should have at least one key field")
	}

	return &fields{
		dict:     dict,
		fields:   value,
		keyField: value[keyIndex],
	}
}

type fields struct {
	dict     map[string]Field
	keyField Field
	fields   []Field
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

func (f *fields) GetKeyField() Field {
	return f.keyField
}

func (f *fields) GetField(name string) Field {
	if field, ok := f.dict[name]; ok {
		return field
	}
	panic(fmt.Errorf("field %q not found", name))
}
