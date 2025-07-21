package crud

import (
	"encoding/json"
	"fmt"
)

// JSONField interface for type-safe JSON field handling
type JSONField[T any] interface {
	Field
	Validator() func(T) error
}

// JSONFieldConfig holds configuration for JSON field
type JSONFieldConfig[T any] struct {
	Validator func(T) error
}

// NewJSONField creates a new type-safe JSON field
func NewJSONField[T any](
	name string,
	config JSONFieldConfig[T],
	opts ...FieldOption,
) JSONField[T] {
	f := newField(
		name,
		JSONFieldType,
		opts...,
	).(*field)

	return &jsonField[T]{
		field:     f,
		validator: config.Validator,
	}
}

// jsonField implements JSONField interface
type jsonField[T any] struct {
	*field
	validator func(T) error
}

func (j *jsonField[T]) Validator() func(T) error {
	return j.validator
}

func (j *jsonField[T]) Value(value any) FieldValue {
	if value == nil {
		return &fieldValue{
			field: j.field,
			value: nil,
		}
	}

	// If value is already a JSON string (from database), use it directly
	if jsonStr, ok := value.(string); ok {
		return &fieldValue{
			field: j.field,
			value: jsonStr,
		}
	}

	// If value is a map[string]interface{} (from database JSON parsing), convert to JSON string
	if mapVal, ok := value.(map[string]interface{}); ok {
		jsonBytes, err := json.Marshal(mapVal)
		if err != nil {
			panic(fmt.Errorf("failed to marshal JSON for field %q: %v", j.name, err))
		}
		return &fieldValue{
			field: j.field,
			value: string(jsonBytes),
		}
	}

	// Type-safe validation for generic field
	if j.validator != nil {
		if typedValue, ok := value.(T); ok {
			if err := j.validator(typedValue); err != nil {
				panic(fmt.Errorf("validation failed for field %q: %v", j.name, err))
			}
		}
		// If we can't cast to T, we still continue but skip validation
		// This can happen during database reads when value is map[string]interface{}
	}

	// If value is an object, marshal it
	jsonBytes, err := json.Marshal(value)
	if err != nil {
		panic(fmt.Errorf("failed to marshal JSON for field %q: %v", j.name, err))
	}

	return &fieldValue{
		field: j.field,
		value: string(jsonBytes),
	}
}

func (j *jsonField[T]) InitialValue() any {
	initialVal := j.initialValueFn()
	if initialVal == nil {
		return nil
	}

	jsonStr, ok := initialVal.(string)
	if !ok {
		// Return a special error value that will be handled by the caller
		return fmt.Errorf("initial value for JSON field %q must be a string, got %T", j.name, initialVal)
	}

	if jsonStr == "" {
		return nil
	}

	// Create zero value of type T
	var result T
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		// Return a special error value that will be handled by the caller
		return fmt.Errorf("failed to unmarshal JSON for field %q: %v", j.name, err)
	}

	return result
}
