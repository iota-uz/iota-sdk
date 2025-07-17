package crud

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var (
	ErrFieldTypeMismatch = errors.New("field type mismatch")
)

type FieldOption func(field *field)
type FieldType string
type FieldRule func(fieldValue FieldValue) error

const (
	StringFieldType    FieldType = "string"
	IntFieldType       FieldType = "int"
	BoolFieldType      FieldType = "bool"
	FloatFieldType     FieldType = "float"
	DecimalFieldType   FieldType = "decimal"
	DateFieldType      FieldType = "date"
	TimeFieldType      FieldType = "time"
	DateTimeFieldType  FieldType = "datetime"
	TimestampFieldType FieldType = "timestamp"
	UUIDFieldType      FieldType = "uuid"
	JSONFieldType      FieldType = "json"
)

const (
	MinLen       string = "minLen"
	MaxLen       string = "maxLen"
	Multiline    string = "multiline"
	Min          string = "min"
	Max          string = "max"
	Precision    string = "precision"
	Scale        string = "scale"
	MinDate      string = "minDate"
	MaxDate      string = "maxDate"
	Pattern      string = "pattern"
	Trim         string = "trim"
	Uppercase    string = "uppercase"
	Lowercase    string = "lowercase"
	Step         string = "step"
	MultipleOf   string = "multipleOf"
	Format       string = "format"
	Timezone     string = "timezone"
	WeekdaysOnly string = "weekdaysOnly"
	UUIDVersion  string = "uuidVersion"
	DefaultValue string = "defaultValue"
	TrueLabel    string = "trueLabel"
	FalseLabel   string = "falseLabel"
)

type Field interface {
	Key() bool
	Name() string
	Type() FieldType

	Readonly() bool
	Searchable() bool
	Hidden() bool

	Rules() []FieldRule

	Attrs() map[string]any

	InitialValue() any
	Value(value any) FieldValue

	AsStringField() (StringField, error)
	AsIntField() (IntField, error)
	AsBoolField() (BoolField, error)
	AsFloatField() (FloatField, error)
	AsDecimalField() (DecimalField, error)
	AsDateField() (DateField, error)
	AsTimeField() (TimeField, error)
	AsDateTimeField() (DateTimeField, error)
	AsTimestampField() (TimestampField, error)
	AsUUIDField() (UUIDField, error)
}

// Base field implementation
type field struct {
	key            bool
	name           string
	type_          FieldType
	readonly       bool
	hidden         bool
	searchable     bool
	attrs          map[string]any
	initialValueFn func() any
	rules          []FieldRule
}

func newField(
	name string,
	type_ FieldType,
	opts ...FieldOption,
) Field {
	f := &field{
		key:        false,
		name:       name,
		type_:      type_,
		searchable: false,
		readonly:   false,
		hidden:     false,
		attrs:      map[string]any{},
		initialValueFn: func() any {
			return nil
		},
		rules: make([]FieldRule, 0),
	}

	for _, opt := range opts {
		opt(f)
	}

	if f.searchable && f.type_ != StringFieldType {
		panic(fmt.Sprintf("field %q: searchable allowed only for type %q, got %q", name, StringFieldType, f.type_))
	}

	return f
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

func (f *field) Attrs() map[string]any {
	return f.attrs
}

func (f *field) InitialValue() any {
	return f.initialValueFn()
}

func (f *field) Rules() []FieldRule {
	return f.rules
}

func (f *field) Value(value any) FieldValue {
	if !isValidType(f.Type(), value) {
		return &fieldValue{
			field: f,
			value: nil,
			err: fmt.Errorf(
				"invalid type for field %q: expected %s, got %T",
				f.name, f.Type(), value,
			),
		}
	}
	return &fieldValue{
		field: f,
		value: value,
		err:   nil,
	}
}

func (f *field) AsStringField() (StringField, error) {
	return nil, fmt.Errorf("%w: field %q is %s, not %s", ErrFieldTypeMismatch, f.name, f.type_, StringFieldType)
}

func (f *field) AsIntField() (IntField, error) {
	return nil, fmt.Errorf("%w: field %q is %s, not %s", ErrFieldTypeMismatch, f.name, f.type_, IntFieldType)
}

func (f *field) AsBoolField() (BoolField, error) {
	return nil, fmt.Errorf("%w: field %q is %s, not %s", ErrFieldTypeMismatch, f.name, f.type_, BoolFieldType)
}

func (f *field) AsFloatField() (FloatField, error) {
	return nil, fmt.Errorf("%w: field %q is %s, not %s", ErrFieldTypeMismatch, f.name, f.type_, FloatFieldType)
}

func (f *field) AsDecimalField() (DecimalField, error) {
	return nil, fmt.Errorf("%w: field %q is %s, not %s", ErrFieldTypeMismatch, f.name, f.type_, DecimalFieldType)
}

func (f *field) AsDateField() (DateField, error) {
	return nil, fmt.Errorf("%w: field %q is %s, not %s", ErrFieldTypeMismatch, f.name, f.type_, DateFieldType)
}

func (f *field) AsTimeField() (TimeField, error) {
	return nil, fmt.Errorf("%w: field %q is %s, not %s", ErrFieldTypeMismatch, f.name, f.type_, TimeFieldType)
}

func (f *field) AsDateTimeField() (DateTimeField, error) {
	return nil, fmt.Errorf("%w: field %q is %s, not %s", ErrFieldTypeMismatch, f.name, f.type_, DateTimeFieldType)
}

func (f *field) AsTimestampField() (TimestampField, error) {
	return nil, fmt.Errorf("%w: field %q is %s, not %s", ErrFieldTypeMismatch, f.name, f.type_, TimestampFieldType)
}

func (f *field) AsUUIDField() (UUIDField, error) {
	return nil, fmt.Errorf("%w: field %q is %s, not %s", ErrFieldTypeMismatch, f.name, f.type_, UUIDFieldType)
}

func isValidType(fieldType FieldType, value any) bool {
	if value == nil {
		return true
	}

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

	case DecimalFieldType:
		return true

	case DateFieldType, TimeFieldType, DateTimeFieldType, TimestampFieldType:
		_, ok := value.(time.Time)
		return ok

	case UUIDFieldType:
		_, ok := value.(uuid.UUID)
		return ok

	case JSONFieldType:
		return true

	default:
		return false
	}
}
