package crud

import "fmt"

type FieldValues interface {
	Values() []FieldValue
	KeyValue() FieldValue
	Value(fieldName string) (FieldValue, error)
}

func NewFieldValues(value []FieldValue) FieldValues {
	dict := make(map[string]FieldValue, len(value))
	keyIndex := -1

	for i, fv := range value {
		name := fv.Field().Name()
		if _, exists := dict[name]; exists {
			panic(fmt.Sprintf("duplicate field name: %q", name))
		}
		if fv.Field().Key() {
			if keyIndex == -1 {
				keyIndex = i
			} else {
				panic("expected exactly one key field")
			}
		}
		dict[name] = fv
	}

	if keyIndex == -1 {
		panic("should have at least one key field")
	}

	return &fieldValues{
		dict:     dict,
		values:   value,
		keyValue: value[keyIndex],
	}
}

type fieldValues struct {
	dict     map[string]FieldValue
	values   []FieldValue
	keyValue FieldValue
}

func (f fieldValues) KeyValue() FieldValue {
	return f.keyValue
}

func (f fieldValues) Values() []FieldValue {
	return f.values
}

func (f fieldValues) Value(fieldName string) (FieldValue, error) {
	if field, ok := f.dict[fieldName]; ok {
		return field, nil
	}
	return nil, fmt.Errorf("field %q not found", fieldName)
}
