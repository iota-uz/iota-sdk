package crud

import (
	"database/sql/driver"
	"encoding/json"
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
	AsJson() (interface{}, error)

	// MultiLang support
	AsMultiLang() (MultiLang, error)
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
	case StringFieldType, IntFieldType, BoolFieldType, FloatFieldType, DecimalFieldType, UUIDFieldType, JsonFieldType:
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

func (fv *fieldValue) AsJson() (interface{}, error) {
	if fv.Field().Type() != JsonFieldType {
		return nil, fv.typeMismatch("json")
	}

	if fv.value == nil {
		return nil, nil
	}

	// If the value is already a string, try to parse it as JSON
	if str, ok := fv.value.(string); ok {
		var result interface{}
		if err := json.Unmarshal([]byte(str), &result); err != nil {
			return nil, fmt.Errorf("failed to parse JSON string: %w", err)
		}
		return result, nil
	}

	// If the value is already a JSON-compatible type, return it as-is
	switch v := fv.value.(type) {
	case map[string]interface{}, []interface{}:
		return v, nil
	default:
		// Try to marshal and unmarshal to ensure it's valid JSON
		bytes, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal value to JSON: %w", err)
		}

		var result interface{}
		if err := json.Unmarshal(bytes, &result); err != nil {
			return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
		}
		return result, nil
	}
}

func (fv *fieldValue) typeMismatch(expected string) error {
	return fmt.Errorf("field '%s' has type '%s', expected '%s'", fv.Field().Name(), fv.Field().Type(), expected)
}

func (fv *fieldValue) valueCastError(expected string) error {
	return fmt.Errorf("field '%s' value is not castable to %s", fv.Field().Name(), expected)
}

// MultiLang support

func (fv *fieldValue) AsMultiLang() (MultiLang, error) {
	if fv.Field().Type() != JsonFieldType {
		return nil, fv.typeMismatch("MultiLang")
	}

	if fv.value == nil {
		return nil, nil
	}

	// Get the JSON field to check schema type
	jsonField, err := fv.Field().AsJsonField()
	if err != nil {
		return nil, err
	}

	schemaType := jsonField.SchemaType()
	if schemaType != "multilang" {
		return nil, fmt.Errorf("field '%s' is not a MultiLang field (schema type: %s)", fv.Field().Name(), schemaType)
	}

	// Create MultiLang object
	multiLang := NewMultiLang()

	// Convert value to JSON string if needed
	var jsonStr string
	switch v := fv.value.(type) {
	case string:
		jsonStr = v
	case []byte:
		jsonStr = string(v)
	case []LangEntry:
		// Handle direct LangEntry array
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal LangEntry array: %w", err)
		}
		jsonStr = string(jsonBytes)
	default:
		// Try to marshal to JSON
		jsonBytes, err := json.Marshal(fv.value)
		if err != nil {
			return nil, fmt.Errorf("failed to convert value to JSON: %w", err)
		}
		jsonStr = string(jsonBytes)
	}

	// Populate the MultiLang object with data
	if err := multiLang.FromJSON(jsonStr); err != nil {
		return nil, fmt.Errorf("failed to populate MultiLang from JSON: %w", err)
	}

	return multiLang, nil
}
