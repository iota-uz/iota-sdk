package crud

import (
	"fmt"
	"reflect"
	"time"

	"github.com/google/uuid"
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
	UUIDFieldType      FieldType = "uuid"
)

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
	AsString() (string, error)
	AsInt() (int, error)
	AsInt32() (int32, error)
	AsInt64() (int64, error)
	AsBool() (bool, error)
	AsFloat32() (float32, error)
	AsFloat64() (float64, error)
	AsTime() (time.Time, error)
	AsUUID() (uuid.UUID, error)
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

func (fv *fieldValue) AsString() (string, error) {
	if fv.Field().Type() != StringFieldType {
		return "", fv.typeMismatch("string")
	}
	s, ok := fv.value.(string)
	if !ok {
		return "", fv.valueCastError("string")
	}
	return s, nil
}

func (fv *fieldValue) AsInt() (int, error) {
	if fv.Field().Type() != IntFieldType {
		return 0, fv.typeMismatch("int")
	}
	i, ok := fv.value.(int)
	if !ok {
		return 0, fv.valueCastError("int")
	}
	return i, nil
}

func (fv *fieldValue) AsInt32() (int32, error) {
	if fv.Field().Type() != IntFieldType {
		return 0, fv.typeMismatch("int32")
	}
	i, ok := fv.value.(int32)
	if !ok {
		return 0, fv.valueCastError("int32")
	}
	return i, nil
}

func (fv *fieldValue) AsInt64() (int64, error) {
	if fv.Field().Type() != IntFieldType {
		return 0, fv.typeMismatch("int64")
	}
	i, ok := fv.value.(int64)
	if !ok {
		return 0, fv.valueCastError("int64")
	}
	return i, nil
}

func (fv *fieldValue) AsBool() (bool, error) {
	if fv.Field().Type() != BoolFieldType {
		return false, fv.typeMismatch("bool")
	}
	b, ok := fv.value.(bool)
	if !ok {
		return false, fv.valueCastError("bool")
	}
	return b, nil
}

func (fv *fieldValue) AsFloat32() (float32, error) {
	if fv.Field().Type() != FloatFieldType {
		return 0, fv.typeMismatch("float32")
	}
	f, ok := fv.value.(float32)
	if !ok {
		return 0, fv.valueCastError("float32")
	}
	return f, nil
}

func (fv *fieldValue) AsFloat64() (float64, error) {
	if fv.Field().Type() != FloatFieldType {
		return 0, fv.typeMismatch("float64")
	}
	f, ok := fv.value.(float64)
	if !ok {
		return 0, fv.valueCastError("float64")
	}
	return f, nil
}

func (fv *fieldValue) AsTime() (time.Time, error) {
	switch fv.Field().Type() {
	case DateFieldType, TimeFieldType, DateTimeFieldType, TimestampFieldType:
		t, ok := fv.value.(time.Time)
		if !ok {
			return time.Time{}, fv.valueCastError("time.Time")
		}
		return t, nil
	default:
		return time.Time{}, fv.typeMismatch("time.Time")
	}
}

func (fv *fieldValue) AsUUID() (uuid.UUID, error) {
	if fv.Field().Type() != UUIDFieldType {
		return uuid.UUID{}, fv.typeMismatch("uuid.UUID")
	}
	u, ok := fv.value.(uuid.UUID)
	if !ok {
		return uuid.UUID{}, fv.valueCastError("uuid.UUID")
	}
	return u, nil
}

func (fv *fieldValue) typeMismatch(expected string) error {
	return fmt.Errorf("field '%s' has type '%s', expected '%s'", fv.Field().Name(), fv.Field().Type(), expected)
}

func (fv *fieldValue) valueCastError(expected string) error {
	return fmt.Errorf("field '%s' value is not castable to %s", fv.Field().Name(), expected)
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

	case UUIDFieldType:
		_, ok := value.(uuid.UUID)
		return ok

	default:
		return false
	}
}
