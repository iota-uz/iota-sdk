package crud

import "math"

type StringField interface {
	Field

	MinLen() int
	MaxLen() int
	Multiline() bool
	Pattern() string
	Trim() bool
	Uppercase() bool
	Lowercase() bool
}

func NewStringField(
	name string,
	opts ...FieldOption,
) StringField {
	f := newField(
		name,
		StringFieldType,
		opts...,
	).(*field)

	return &stringField{field: f}
}

type stringField struct {
	*field
}

func (s *stringField) MinLen() int {
	if val, ok := s.attrs[MinLen].(int); ok {
		return val
	}
	return 0
}

func (s *stringField) MaxLen() int {
	if val, ok := s.attrs[MaxLen].(int); ok {
		return val
	}
	return math.MaxInt32
}

func (s *stringField) Multiline() bool {
	if val, ok := s.attrs[Multiline].(bool); ok {
		return val
	}
	return false
}

func (s *stringField) Pattern() string {
	if val, ok := s.attrs[Pattern].(string); ok {
		return val
	}
	return ""
}

func (s *stringField) Trim() bool {
	if val, ok := s.attrs[Trim].(bool); ok {
		return val
	}
	return false
}

func (s *stringField) Uppercase() bool {
	if val, ok := s.attrs[Uppercase].(bool); ok {
		return val
	}
	return false
}

func (s *stringField) Lowercase() bool {
	if val, ok := s.attrs[Lowercase].(bool); ok {
		return val
	}
	return false
}

func (s *stringField) AsStringField() (StringField, error) {
	return s, nil
}
