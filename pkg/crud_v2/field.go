package crud_v2

import (
	"fmt"
	"reflect"
	"time"
)

type FieldOption func(field *field)
type FieldType string
type FieldRule func(fieldValue FieldValue) error

const (
	StringFieldType    FieldType = "string"
	IntFieldType       FieldType = "int"
	BoolFieldType      FieldType = "bool"
	FloatFieldType     FieldType = "float"
	DateFieldType      FieldType = "date"
	TimeFieldType      FieldType = "time"
	DateTimeFieldType  FieldType = "datetime"
	TimestampFieldType FieldType = "timestamp"
)

func RequiredRule() FieldRule {
	return func(fv FieldValue) error {
		val := fv.Value()
		if val == nil || val == "" {
			return fmt.Errorf("field %q is required", fv.Field().Name())
		}
		return nil
	}
}

type Field interface {
	Key() bool
	Name() string
	Type() FieldType

	Readonly() bool
	Searchable() bool
	Hidden() bool

	Rules() []FieldRule

	InitialValue() any
	Value(value any) FieldValue
}

type FieldValue interface {
	Field() Field
	Value() any
	IsZero() bool
}

type fieldValue struct {
	field Field
	value any
}

func (fv *fieldValue) Field() Field {
	return fv.field
}

func (fv *fieldValue) Value() any {
	return fv.value
}

func (fv *fieldValue) IsZero() bool {
	return reflect.ValueOf(fv.value).IsZero()
}

func WithKey(key bool) FieldOption {
	return func(field *field) {
		field.key = key
	}
}

func WithReadonly(readonly bool) FieldOption {
	return func(field *field) {
		field.readonly = readonly
	}
}

func WithHidden(hidden bool) FieldOption {
	return func(field *field) {
		field.hidden = hidden
	}
}

func WithSearchable(searchable bool) FieldOption {
	return func(field *field) {
		field.searchable = searchable
	}
}

func WithInitialValue(initialValue any) FieldOption {
	return func(field *field) {
		field.initialValue = initialValue
	}
}

func WithRules(rules []FieldRule) FieldOption {
	return func(field *field) {
		field.rules = append(field.rules, rules...)
	}
}

func WithRule(rule FieldRule) FieldOption {
	return func(field *field) {
		field.rules = append(field.rules, rule)
	}
}

func NewField(
	name string,
	type_ FieldType,
	opts ...FieldOption,
) Field {
	f := &field{
		key:          false,
		name:         name,
		type_:        type_,
		searchable:   false,
		readonly:     false,
		hidden:       false,
		initialValue: nil,
		rules:        make([]FieldRule, 0),
	}

	for _, opt := range opts {
		opt(f)
	}

	if f.searchable && f.type_ != StringFieldType {
		panic(fmt.Sprintf("field %q: searchable allowed only for type %q, got %q", name, StringFieldType, f.type_))
	}

	return f
}

type field struct {
	key          bool
	name         string
	type_        FieldType
	readonly     bool
	hidden       bool
	searchable   bool
	initialValue any
	rules        []FieldRule
}

func (f *field) Key() bool {
	return f.key
}

func (f *field) Name() string {
	return f.name
}

func (f *field) Type() FieldType {
	return f.type_
}

func (f *field) Readonly() bool {
	return f.readonly
}

func (f *field) Searchable() bool {
	return f.searchable
}

func (f *field) Hidden() bool {
	return f.hidden
}

func (f *field) InitialValue() any {
	return f.initialValue
}

func (f *field) Rules() []FieldRule {
	return f.rules
}

func (f *field) Value(value any) FieldValue {
	if !isValidType(f.Type(), value) {
		panic(fmt.Sprintf(
			"invalid type for field %q: expected %s, got %T",
			f.name, f.Type(), value,
		))
	}
	return &fieldValue{
		field: f,
		value: value,
	}
}

func isValidType(fieldType FieldType, value any) bool {
	switch fieldType {
	case StringFieldType:
		_, ok := value.(string)
		return ok

	case IntFieldType:
		switch value.(type) {
		case int, int32, int64:
			return true
		default:
			return false
		}

	case BoolFieldType:
		_, ok := value.(bool)
		return ok

	case FloatFieldType:
		switch value.(type) {
		case float32, float64:
			return true
		default:
			return false
		}

	case DateFieldType, TimeFieldType, DateTimeFieldType, TimestampFieldType:
		_, ok := value.(time.Time)
		return ok

	default:
		return false
	}
}
