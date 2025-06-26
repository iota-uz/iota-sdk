package crud

import (
	"database/sql/driver"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
)

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
	AsDecimal() (string, error)
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

	if fv.value == nil {
		return "", nil
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

	if fv.value == nil {
		return 0, nil
	}

	switch v := fv.value.(type) {
	case int:
		return v, nil
	case int32:
		return int(v), nil
	case int64:
		return int(v), nil
	default:
		return 0, fv.valueCastError("int")
	}
}

func (fv *fieldValue) AsInt32() (int32, error) {
	if fv.Field().Type() != IntFieldType {
		return 0, fv.typeMismatch("int32")
	}

	if fv.value == nil {
		return 0, nil
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

	if fv.value == nil {
		return 0, nil
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

	if fv.value == nil {
		return false, nil
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

	if fv.value == nil {
		return 0, nil
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

	if fv.value == nil {
		return 0, nil
	}

	f, ok := fv.value.(float64)
	if !ok {
		return 0, fv.valueCastError("float64")
	}
	return f, nil
}

func (fv *fieldValue) AsDecimal() (string, error) {
	if fv.Field().Type() != DecimalFieldType {
		return "", fv.typeMismatch("decimal")
	}

	if fv.value == nil {
		return "", nil
	}

	switch v := fv.value.(type) {
	case string:
		return v, nil
	case int:
		return fmt.Sprintf("%d", v), nil
	case int32:
		return fmt.Sprintf("%d", v), nil
	case int64:
		return fmt.Sprintf("%d", v), nil
	case float32:
		return fmt.Sprintf("%g", v), nil
	case float64:
		return fmt.Sprintf("%g", v), nil
	default:
		// Try to use driver.Valuer interface first
		if valuer, ok := v.(driver.Valuer); ok {
			val, err := valuer.Value()
			if err != nil {
				return "", fmt.Errorf("failed to get decimal value: %w", err)
			}
			if val == nil {
				return "", nil
			}
			// The value might be string, float64, or other numeric type
			switch v := val.(type) {
			case string:
				return v, nil
			case float64:
				return fmt.Sprintf("%g", v), nil
			case int64:
				return fmt.Sprintf("%d", v), nil
			default:
				return fmt.Sprintf("%v", v), nil
			}
		}

		// Try fmt.Stringer
		if stringer, ok := v.(fmt.Stringer); ok {
			str := stringer.String()
			// Avoid returning internal representation
			if !strings.HasPrefix(str, "{") {
				return str, nil
			}
		}

		return "", fmt.Errorf("cannot convert %T to decimal string", v)
	}
}

func (fv *fieldValue) AsTime() (time.Time, error) {
	switch fv.Field().Type() {
	case DateFieldType, TimeFieldType, DateTimeFieldType, TimestampFieldType:
		if fv.value == nil {
			return time.Time{}, nil
		}

		t, ok := fv.value.(time.Time)
		if !ok {
			return time.Time{}, fv.valueCastError("time.Time")
		}
		return t, nil
	case StringFieldType, IntFieldType, BoolFieldType, FloatFieldType, DecimalFieldType, UUIDFieldType:
		return time.Time{}, fv.typeMismatch("time.Time")
	}
	return time.Time{}, fv.typeMismatch("time.Time")
}

func (fv *fieldValue) AsUUID() (uuid.UUID, error) {
	if fv.Field().Type() != UUIDFieldType {
		return uuid.UUID{}, fv.typeMismatch("uuid.UUID")
	}

	if fv.value == nil {
		return uuid.Nil, nil
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
