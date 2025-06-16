package crud

import (
	"errors"
	"fmt"
	"math"
	"reflect"
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
	DateFieldType      FieldType = "date"
	TimeFieldType      FieldType = "time"
	DateTimeFieldType  FieldType = "datetime"
	TimestampFieldType FieldType = "timestamp"
	UUIDFieldType      FieldType = "uuid"
)

const (
	MinLen       string = "minLen"
	MaxLen       string = "maxLen"
	Multiline    string = "multiline"
	Min          string = "min"
	Max          string = "max"
	Precision    string = "precision"
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
	AsDateField() (DateField, error)
	AsTimeField() (TimeField, error)
	AsDateTimeField() (DateTimeField, error)
	AsTimestampField() (TimestampField, error)
	AsUUIDField() (UUIDField, error)
}

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

type IntField interface {
	Field

	Min() int64
	Max() int64
	Step() int64
	MultipleOf() int64
}

type BoolField interface {
	Field

	DefaultValue() bool
	TrueLabel() string
	FalseLabel() string
}

type FloatField interface {
	Field

	Min() float64
	Max() float64
	Precision() int
	Step() float64
}

type DateField interface {
	Field

	MinDate() time.Time
	MaxDate() time.Time
	Format() string
	WeekdaysOnly() bool
}

type TimeField interface {
	Field

	Format() string
}

type DateTimeField interface {
	Field

	MinDateTime() time.Time
	MaxDateTime() time.Time
	Format() string
	Timezone() string
	WeekdaysOnly() bool
}

type TimestampField interface {
	Field
}

type UUIDField interface {
	Field

	Version() int
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

func WithAttrs(attrs map[string]any) FieldOption {
	return func(field *field) {
		for k, v := range attrs {
			field.attrs[k] = v
		}
	}
}

func WithAttr(key string, value any) FieldOption {
	return func(field *field) {
		field.attrs[key] = value
	}
}

func WithMinLen(minLen int) FieldOption {
	return func(field *field) {
		field.attrs[MinLen] = minLen
		field.rules = append(field.rules, MinLengthRule(minLen))
	}
}

func WithMaxLen(maxLen int) FieldOption {
	return func(field *field) {
		field.attrs[MaxLen] = maxLen
		field.rules = append(field.rules, MaxLengthRule(maxLen))
	}
}

func WithMultiline(multiline bool) FieldOption {
	return func(field *field) {
		field.attrs[Multiline] = multiline
	}
}

func WithMin(min int64) FieldOption {
	return func(field *field) {
		field.attrs[Min] = min
	}
}

func WithMax(max int64) FieldOption {
	return func(field *field) {
		field.attrs[Max] = max
	}
}

func WithFloatMin(min float64) FieldOption {
	return func(field *field) {
		field.attrs[Min] = min
	}
}

func WithFloatMax(max float64) FieldOption {
	return func(field *field) {
		field.attrs[Max] = max
	}
}

func WithPrecision(precision int) FieldOption {
	return func(field *field) {
		field.attrs[Precision] = precision
	}
}

func WithMinDate(minDate time.Time) FieldOption {
	return func(field *field) {
		field.attrs[MinDate] = minDate
	}
}

func WithMaxDate(maxDate time.Time) FieldOption {
	return func(field *field) {
		field.attrs[MaxDate] = maxDate
	}
}

func WithPattern(pattern string) FieldOption {
	return func(field *field) {
		field.attrs[Pattern] = pattern
		field.rules = append(field.rules, PatternRule(pattern))
	}
}

func WithTrim(trim bool) FieldOption {
	return func(field *field) {
		field.attrs[Trim] = trim
	}
}

func WithUppercase(uppercase bool) FieldOption {
	return func(field *field) {
		field.attrs[Uppercase] = uppercase
	}
}

func WithLowercase(lowercase bool) FieldOption {
	return func(field *field) {
		field.attrs[Lowercase] = lowercase
	}
}

func WithStep(step int64) FieldOption {
	return func(field *field) {
		field.attrs[Step] = step
	}
}

func WithMultipleOf(multiple int64) FieldOption {
	return func(field *field) {
		field.attrs[MultipleOf] = multiple
		field.rules = append(field.rules, MultipleOfRule(multiple))
	}
}

func WithFloatStep(step float64) FieldOption {
	return func(field *field) {
		field.attrs[Step] = step
	}
}

func WithFormat(format string) FieldOption {
	return func(field *field) {
		field.attrs[Format] = format
	}
}

func WithTimezone(timezone string) FieldOption {
	return func(field *field) {
		field.attrs[Timezone] = timezone
	}
}

func WithWeekdaysOnly(weekdaysOnly bool) FieldOption {
	return func(field *field) {
		field.attrs[WeekdaysOnly] = weekdaysOnly
		if weekdaysOnly {
			field.rules = append(field.rules, WeekdayRule())
		}
	}
}

func WithUUIDVersion(version int) FieldOption {
	return func(field *field) {
		field.attrs[UUIDVersion] = version
		field.rules = append(field.rules, UUIDVersionRule(version))
	}
}

func WithDefaultValue(defaultValue any) FieldOption {
	return func(field *field) {
		field.attrs[DefaultValue] = defaultValue
	}
}

func WithTrueLabel(label string) FieldOption {
	return func(field *field) {
		field.attrs[TrueLabel] = label
	}
}

func WithFalseLabel(label string) FieldOption {
	return func(field *field) {
		field.attrs[FalseLabel] = label
	}
}

func WithURL() FieldOption {
	return func(field *field) {
		field.rules = append(field.rules, URLRule())
	}
}

func WithPhone() FieldOption {
	return func(field *field) {
		field.rules = append(field.rules, PhoneRule())
	}
}

func WithAlpha() FieldOption {
	return func(field *field) {
		field.rules = append(field.rules, AlphaRule())
	}
}

func WithAlphanumeric() FieldOption {
	return func(field *field) {
		field.rules = append(field.rules, AlphanumericRule())
	}
}

func WithEmail() FieldOption {
	return func(field *field) {
		field.rules = append(field.rules, EmailRule())
	}
}

func WithRequired() FieldOption {
	return func(field *field) {
		field.rules = append(field.rules, RequiredRule())
	}
}

func WithPositive() FieldOption {
	return func(field *field) {
		field.rules = append(field.rules, PositiveRule())
	}
}

func WithNonNegative() FieldOption {
	return func(field *field) {
		field.rules = append(field.rules, NonNegativeRule())
	}
}

func WithNotEmpty() FieldOption {
	return func(field *field) {
		field.rules = append(field.rules, NotEmptyRule())
	}
}

func WithFutureDate() FieldOption {
	return func(field *field) {
		field.rules = append(field.rules, FutureDateRule())
	}
}

func WithPastDate() FieldOption {
	return func(field *field) {
		field.rules = append(field.rules, PastDateRule())
	}
}

func newField(
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
		attrs:        map[string]any{},
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
	attrs        map[string]any
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

func (f *field) Attrs() map[string]any {
	return f.attrs
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

func NewIntField(
	name string,
	opts ...FieldOption,
) IntField {
	f := newField(
		name,
		IntFieldType,
		opts...,
	).(*field)

	return &intField{field: f}
}

type intField struct {
	*field
}

func (i *intField) Min() int64 {
	if val, ok := i.attrs[Min].(int64); ok {
		return val
	}
	return math.MinInt64
}

func (i *intField) Max() int64 {
	if val, ok := i.attrs[Max].(int64); ok {
		return val
	}
	return math.MaxInt64
}

func (i *intField) Step() int64 {
	if val, ok := i.attrs[Step].(int64); ok {
		return val
	}
	return 1
}

func (i *intField) MultipleOf() int64 {
	if val, ok := i.attrs[MultipleOf].(int64); ok {
		return val
	}
	return 1
}

func (i *intField) AsIntField() (IntField, error) {
	return i, nil
}

func NewBoolField(
	name string,
	opts ...FieldOption,
) BoolField {
	f := newField(
		name,
		BoolFieldType,
		opts...,
	).(*field)

	return &boolField{field: f}
}

type boolField struct {
	*field
}

func (b *boolField) DefaultValue() bool {
	if val, ok := b.attrs[DefaultValue].(bool); ok {
		return val
	}
	return false
}

func (b *boolField) TrueLabel() string {
	if val, ok := b.attrs[TrueLabel].(string); ok {
		return val
	}
	return ""
}

func (b *boolField) FalseLabel() string {
	if val, ok := b.attrs[FalseLabel].(string); ok {
		return val
	}
	return ""
}

func (b *boolField) AsBoolField() (BoolField, error) {
	return b, nil
}

func NewFloatField(
	name string,
	opts ...FieldOption,
) FloatField {
	f := newField(
		name,
		FloatFieldType,
		opts...,
	).(*field)

	return &floatField{field: f}
}

type floatField struct {
	*field
}

func (f *floatField) Min() float64 {
	if val, ok := f.attrs[Min].(float64); ok {
		return val
	}
	return -math.MaxFloat64
}

func (f *floatField) Max() float64 {
	if val, ok := f.attrs[Max].(float64); ok {
		return val
	}
	return math.MaxFloat64
}

func (f *floatField) Precision() int {
	if val, ok := f.attrs[Precision].(int); ok {
		return val
	}
	return 2
}

func (f *floatField) Step() float64 {
	if val, ok := f.attrs[Step].(float64); ok {
		return val
	}
	return 0.01
}

func (f *floatField) AsFloatField() (FloatField, error) {
	return f, nil
}

func NewDateField(
	name string,
	opts ...FieldOption,
) DateField {
	f := newField(
		name,
		DateFieldType,
		opts...,
	).(*field)

	return &dateField{field: f}
}

type dateField struct {
	*field
}

func (d *dateField) MinDate() time.Time {
	if val, ok := d.attrs[MinDate].(time.Time); ok {
		return val
	}
	// Return the minimum possible time value
	return time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)
}

func (d *dateField) MaxDate() time.Time {
	if val, ok := d.attrs[MaxDate].(time.Time); ok {
		return val
	}
	// Return the maximum possible time value
	return time.Date(9999, 12, 31, 23, 59, 59, 999999999, time.UTC)
}

func (d *dateField) Format() string {
	if val, ok := d.attrs[Format].(string); ok {
		return val
	}
	return "2006-01-02"
}

func (d *dateField) WeekdaysOnly() bool {
	if val, ok := d.attrs[WeekdaysOnly].(bool); ok {
		return val
	}
	return false
}

func (d *dateField) AsDateField() (DateField, error) {
	return d, nil
}

func NewTimeField(
	name string,
	opts ...FieldOption,
) TimeField {
	f := newField(
		name,
		TimeFieldType,
		opts...,
	).(*field)

	return &timeField{field: f}
}

type timeField struct {
	*field
}

func (t *timeField) Format() string {
	if val, ok := t.attrs[Format].(string); ok {
		return val
	}
	return "15:04:05"
}

func (t *timeField) AsTimeField() (TimeField, error) {
	return t, nil
}

func NewDateTimeField(
	name string,
	opts ...FieldOption,
) DateTimeField {
	f := newField(
		name,
		DateTimeFieldType,
		opts...,
	).(*field)

	return &dateTimeField{field: f}
}

type dateTimeField struct {
	*field
}

func (dt *dateTimeField) MinDateTime() time.Time {
	if val, ok := dt.attrs[MinDate].(time.Time); ok {
		return val
	}
	// Return the minimum possible time value
	return time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)
}

func (dt *dateTimeField) MaxDateTime() time.Time {
	if val, ok := dt.attrs[MaxDate].(time.Time); ok {
		return val
	}
	// Return the maximum possible time value
	return time.Date(9999, 12, 31, 23, 59, 59, 999999999, time.UTC)
}

func (dt *dateTimeField) Format() string {
	if val, ok := dt.attrs[Format].(string); ok {
		return val
	}
	return "2006-01-02 15:04:05"
}

func (dt *dateTimeField) Timezone() string {
	if val, ok := dt.attrs[Timezone].(string); ok {
		return val
	}
	return "UTC"
}

func (dt *dateTimeField) WeekdaysOnly() bool {
	if val, ok := dt.attrs[WeekdaysOnly].(bool); ok {
		return val
	}
	return false
}

func (dt *dateTimeField) AsDateTimeField() (DateTimeField, error) {
	return dt, nil
}

func NewTimestampField(
	name string,
	opts ...FieldOption,
) TimestampField {
	f := newField(
		name,
		TimestampFieldType,
		opts...,
	).(*field)

	return &timestampField{field: f}
}

type timestampField struct {
	*field
}

func (t *timestampField) AsTimestampField() (TimestampField, error) {
	return t, nil
}

func NewUUIDField(
	name string,
	opts ...FieldOption,
) UUIDField {
	f := newField(
		name,
		UUIDFieldType,
		opts...,
	).(*field)

	return &uuidField{field: f}
}

type uuidField struct {
	*field
}

func (u *uuidField) Version() int {
	if val, ok := u.attrs[UUIDVersion].(int); ok {
		return val
	}
	return 4
}

func (u *uuidField) AsUUIDField() (UUIDField, error) {
	return u, nil
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
