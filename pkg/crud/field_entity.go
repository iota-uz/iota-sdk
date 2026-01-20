package crud

import (
	"context"
	"reflect"
	"time"

	"github.com/google/uuid"
)

// entityFieldBase is a non-generic base that implements the Field interface.
// It provides the metadata for entity fields without the generic type parameter.
type entityFieldBase struct {
	name string
}

func (f *entityFieldBase) Key() bool                          { return false }
func (f *entityFieldBase) Name() string                       { return f.name }
func (f *entityFieldBase) Type() FieldType                    { return EntityFieldType }
func (f *entityFieldBase) Readonly() bool                     { return true }
func (f *entityFieldBase) Searchable() bool                   { return false }
func (f *entityFieldBase) Sortable() bool                     { return false }
func (f *entityFieldBase) Hidden() bool                       { return true }
func (f *entityFieldBase) Virtual() bool                      { return true }
func (f *entityFieldBase) Rules() []FieldRule                 { return nil }
func (f *entityFieldBase) Attrs() map[string]any              { return nil }
func (f *entityFieldBase) InitialValue(_ context.Context) any { return nil }
func (f *entityFieldBase) RendererType() string               { return "" }
func (f *entityFieldBase) LocalizationKey() string            { return "" }

// Value creates a FieldValue from an any value.
// For EntityField, this wraps the value in an entityFieldValueAny.
func (f *entityFieldBase) Value(value any) FieldValue {
	return &entityFieldValueAny{
		field: f,
		value: value,
	}
}

// Type assertion methods - EntityField doesn't support casting to other field types
func (f *entityFieldBase) AsStringField() (StringField, error)     { return nil, ErrFieldTypeMismatch }
func (f *entityFieldBase) AsIntField() (IntField, error)           { return nil, ErrFieldTypeMismatch }
func (f *entityFieldBase) AsBoolField() (BoolField, error)         { return nil, ErrFieldTypeMismatch }
func (f *entityFieldBase) AsFloatField() (FloatField, error)       { return nil, ErrFieldTypeMismatch }
func (f *entityFieldBase) AsDecimalField() (DecimalField, error)   { return nil, ErrFieldTypeMismatch }
func (f *entityFieldBase) AsDateField() (DateField, error)         { return nil, ErrFieldTypeMismatch }
func (f *entityFieldBase) AsTimeField() (TimeField, error)         { return nil, ErrFieldTypeMismatch }
func (f *entityFieldBase) AsDateTimeField() (DateTimeField, error) { return nil, ErrFieldTypeMismatch }
func (f *entityFieldBase) AsTimestampField() (TimestampField, error) {
	return nil, ErrFieldTypeMismatch
}
func (f *entityFieldBase) AsUUIDField() (UUIDField, error) { return nil, ErrFieldTypeMismatch }

// entityFieldValueAny is a non-generic FieldValue for entity fields.
type entityFieldValueAny struct {
	field *entityFieldBase
	value any
}

func (fv *entityFieldValueAny) Field() Field { return fv.field }
func (fv *entityFieldValueAny) Value() any   { return fv.value }

func (fv *entityFieldValueAny) IsZero() bool {
	if fv.value == nil {
		return true
	}
	v := reflect.ValueOf(fv.value)
	if !v.IsValid() {
		return true
	}
	return v.IsZero()
}

func (fv *entityFieldValueAny) AsString() (string, error)   { return "", ErrFieldTypeMismatch }
func (fv *entityFieldValueAny) AsInt() (int, error)         { return 0, ErrFieldTypeMismatch }
func (fv *entityFieldValueAny) AsInt32() (int32, error)     { return 0, ErrFieldTypeMismatch }
func (fv *entityFieldValueAny) AsInt64() (int64, error)     { return 0, ErrFieldTypeMismatch }
func (fv *entityFieldValueAny) AsBool() (bool, error)       { return false, ErrFieldTypeMismatch }
func (fv *entityFieldValueAny) AsFloat32() (float32, error) { return 0, ErrFieldTypeMismatch }
func (fv *entityFieldValueAny) AsFloat64() (float64, error) { return 0, ErrFieldTypeMismatch }
func (fv *entityFieldValueAny) AsDecimal() (string, error)  { return "", ErrFieldTypeMismatch }
func (fv *entityFieldValueAny) AsTime() (time.Time, error)  { return time.Time{}, ErrFieldTypeMismatch }
func (fv *entityFieldValueAny) AsUUID() (uuid.UUID, error)  { return uuid.UUID{}, ErrFieldTypeMismatch }
func (fv *entityFieldValueAny) AsJSON() (string, error)     { return "", ErrFieldTypeMismatch }

// EntityField represents a field that holds a pre-mapped related entity from a JOIN.
// It is generic over the entity type T, providing type-safe access to related entities.
type EntityField[T any] struct {
	base *entityFieldBase
}

// NewEntityField creates a new EntityField with the given name.
// EntityField is used to hold entities loaded via JOINs, providing type-safe
// access to related data without additional queries.
func NewEntityField[T any](name string) *EntityField[T] {
	return &EntityField[T]{
		base: &entityFieldBase{name: name},
	}
}

// Field interface implementation - delegates to base

func (f *EntityField[T]) Key() bool                            { return f.base.Key() }
func (f *EntityField[T]) Name() string                         { return f.base.Name() }
func (f *EntityField[T]) Type() FieldType                      { return f.base.Type() }
func (f *EntityField[T]) Readonly() bool                       { return f.base.Readonly() }
func (f *EntityField[T]) Searchable() bool                     { return f.base.Searchable() }
func (f *EntityField[T]) Sortable() bool                       { return f.base.Sortable() }
func (f *EntityField[T]) Hidden() bool                         { return f.base.Hidden() }
func (f *EntityField[T]) Virtual() bool                        { return f.base.Virtual() }
func (f *EntityField[T]) Rules() []FieldRule                   { return f.base.Rules() }
func (f *EntityField[T]) Attrs() map[string]any                { return f.base.Attrs() }
func (f *EntityField[T]) InitialValue(ctx context.Context) any { return f.base.InitialValue(ctx) }
func (f *EntityField[T]) RendererType() string                 { return f.base.RendererType() }
func (f *EntityField[T]) LocalizationKey() string              { return f.base.LocalizationKey() }

// Type assertion methods - EntityField doesn't support casting to other field types
func (f *EntityField[T]) AsStringField() (StringField, error)       { return nil, ErrFieldTypeMismatch }
func (f *EntityField[T]) AsIntField() (IntField, error)             { return nil, ErrFieldTypeMismatch }
func (f *EntityField[T]) AsBoolField() (BoolField, error)           { return nil, ErrFieldTypeMismatch }
func (f *EntityField[T]) AsFloatField() (FloatField, error)         { return nil, ErrFieldTypeMismatch }
func (f *EntityField[T]) AsDecimalField() (DecimalField, error)     { return nil, ErrFieldTypeMismatch }
func (f *EntityField[T]) AsDateField() (DateField, error)           { return nil, ErrFieldTypeMismatch }
func (f *EntityField[T]) AsTimeField() (TimeField, error)           { return nil, ErrFieldTypeMismatch }
func (f *EntityField[T]) AsDateTimeField() (DateTimeField, error)   { return nil, ErrFieldTypeMismatch }
func (f *EntityField[T]) AsTimestampField() (TimestampField, error) { return nil, ErrFieldTypeMismatch }
func (f *EntityField[T]) AsUUIDField() (UUIDField, error)           { return nil, ErrFieldTypeMismatch }

// Value implements the Field interface and creates a FieldValue from any value.
// For type-safe entity wrapping, use TypedValue instead.
func (f *EntityField[T]) Value(value any) FieldValue {
	return f.base.Value(value)
}

// TypedValue creates an EntityFieldValue holding the given entity with type safety.
func (f *EntityField[T]) TypedValue(entity T) FieldValue {
	return &entityFieldValue[T]{
		base:   f.base,
		entity: entity,
	}
}

// entityFieldValue holds a pre-mapped entity of type T.
type entityFieldValue[T any] struct {
	base   *entityFieldBase
	entity T
}

func (fv *entityFieldValue[T]) Field() Field { return fv.base }
func (fv *entityFieldValue[T]) Value() any   { return fv.entity }

func (fv *entityFieldValue[T]) IsZero() bool {
	v := reflect.ValueOf(fv.entity)
	if !v.IsValid() {
		return true
	}
	return v.IsZero()
}

// FieldValue interface methods - return errors since entity isn't a primitive type
func (fv *entityFieldValue[T]) AsString() (string, error)   { return "", ErrFieldTypeMismatch }
func (fv *entityFieldValue[T]) AsInt() (int, error)         { return 0, ErrFieldTypeMismatch }
func (fv *entityFieldValue[T]) AsInt32() (int32, error)     { return 0, ErrFieldTypeMismatch }
func (fv *entityFieldValue[T]) AsInt64() (int64, error)     { return 0, ErrFieldTypeMismatch }
func (fv *entityFieldValue[T]) AsBool() (bool, error)       { return false, ErrFieldTypeMismatch }
func (fv *entityFieldValue[T]) AsFloat32() (float32, error) { return 0, ErrFieldTypeMismatch }
func (fv *entityFieldValue[T]) AsFloat64() (float64, error) { return 0, ErrFieldTypeMismatch }
func (fv *entityFieldValue[T]) AsDecimal() (string, error)  { return "", ErrFieldTypeMismatch }
func (fv *entityFieldValue[T]) AsTime() (time.Time, error)  { return time.Time{}, ErrFieldTypeMismatch }
func (fv *entityFieldValue[T]) AsUUID() (uuid.UUID, error)  { return uuid.UUID{}, ErrFieldTypeMismatch }
func (fv *entityFieldValue[T]) AsJSON() (string, error)     { return "", ErrFieldTypeMismatch }

// Entity returns the stored entity with its concrete type.
func (fv *entityFieldValue[T]) Entity() T {
	return fv.entity
}

// AsEntity extracts an entity from a FieldValue if it's an EntityFieldValue of the correct type.
// Returns the entity and true if successful, or zero value and false if the FieldValue
// is not an EntityFieldValue or is of a different entity type.
func AsEntity[T any](fv FieldValue) (T, bool) {
	var zero T
	if efv, ok := fv.(*entityFieldValue[T]); ok {
		return efv.entity, true
	}
	return zero, false
}
