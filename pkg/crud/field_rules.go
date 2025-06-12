package crud

import (
	"fmt"
	"regexp"
	"strings"
	"time"
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

func MinLengthRule(minLength int) FieldRule {
	return func(fv FieldValue) error {
		if fv.Field().Type() != StringFieldType {
			return fmt.Errorf("min length rule only applies to string fields")
		}
		val, ok := fv.Value().(string)
		if !ok {
			return nil
		}
		if len(val) < minLength {
			return fmt.Errorf("field %q must be at least %d characters long", fv.Field().Name(), minLength)
		}
		return nil
	}
}

func MaxLengthRule(maxLength int) FieldRule {
	return func(fv FieldValue) error {
		if fv.Field().Type() != StringFieldType {
			return fmt.Errorf("max length rule only applies to string fields")
		}
		val, ok := fv.Value().(string)
		if !ok {
			return nil
		}
		if len(val) > maxLength {
			return fmt.Errorf("field %q must be at most %d characters long", fv.Field().Name(), maxLength)
		}
		return nil
	}
}

func EmailRule() FieldRule {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return func(fv FieldValue) error {
		if fv.Field().Type() != StringFieldType {
			return fmt.Errorf("email rule only applies to string fields")
		}
		val, ok := fv.Value().(string)
		if !ok || val == "" {
			return nil
		}
		if !emailRegex.MatchString(val) {
			return fmt.Errorf("field %q must be a valid email address", fv.Field().Name())
		}
		return nil
	}
}

func PatternRule(pattern string) FieldRule {
	regex := regexp.MustCompile(pattern)
	return func(fv FieldValue) error {
		if fv.Field().Type() != StringFieldType {
			return fmt.Errorf("pattern rule only applies to string fields")
		}
		val, ok := fv.Value().(string)
		if !ok || val == "" {
			return nil
		}
		if !regex.MatchString(val) {
			return fmt.Errorf("field %q must match pattern %q", fv.Field().Name(), pattern)
		}
		return nil
	}
}

func MinValueRule(minValue int64) FieldRule {
	return func(fv FieldValue) error {
		fieldType := fv.Field().Type()
		switch fieldType {
		case IntFieldType:
			val := fv.Value()
			var intVal int64
			switch v := val.(type) {
			case int:
				intVal = int64(v)
			case int32:
				intVal = int64(v)
			case int64:
				intVal = v
			default:
				return nil
			}
			if intVal < minValue {
				return fmt.Errorf("field %q must be at least %d", fv.Field().Name(), minValue)
			}
		case FloatFieldType:
			val := fv.Value()
			var floatVal float64
			switch v := val.(type) {
			case float32:
				floatVal = float64(v)
			case float64:
				floatVal = v
			default:
				return nil
			}
			if floatVal < float64(minValue) {
				return fmt.Errorf("field %q must be at least %d", fv.Field().Name(), minValue)
			}
		default:
			return fmt.Errorf("min value rule only applies to int and float fields")
		}
		return nil
	}
}

func MaxValueRule(maxValue int64) FieldRule {
	return func(fv FieldValue) error {
		fieldType := fv.Field().Type()
		switch fieldType {
		case IntFieldType:
			val := fv.Value()
			var intVal int64
			switch v := val.(type) {
			case int:
				intVal = int64(v)
			case int32:
				intVal = int64(v)
			case int64:
				intVal = v
			default:
				return nil
			}
			if intVal > maxValue {
				return fmt.Errorf("field %q must be at most %d", fv.Field().Name(), maxValue)
			}
		case FloatFieldType:
			val := fv.Value()
			var floatVal float64
			switch v := val.(type) {
			case float32:
				floatVal = float64(v)
			case float64:
				floatVal = v
			default:
				return nil
			}
			if floatVal > float64(maxValue) {
				return fmt.Errorf("field %q must be at most %d", fv.Field().Name(), maxValue)
			}
		default:
			return fmt.Errorf("max value rule only applies to int and float fields")
		}
		return nil
	}
}

func PositiveRule() FieldRule {
	return func(fv FieldValue) error {
		fieldType := fv.Field().Type()
		switch fieldType {
		case IntFieldType:
			val := fv.Value()
			var intVal int64
			switch v := val.(type) {
			case int:
				intVal = int64(v)
			case int32:
				intVal = int64(v)
			case int64:
				intVal = v
			default:
				return nil
			}
			if intVal <= 0 {
				return fmt.Errorf("field %q must be positive", fv.Field().Name())
			}
		case FloatFieldType:
			val := fv.Value()
			var floatVal float64
			switch v := val.(type) {
			case float32:
				floatVal = float64(v)
			case float64:
				floatVal = v
			default:
				return nil
			}
			if floatVal <= 0 {
				return fmt.Errorf("field %q must be positive", fv.Field().Name())
			}
		default:
			return fmt.Errorf("positive rule only applies to int and float fields")
		}
		return nil
	}
}

func NonNegativeRule() FieldRule {
	return func(fv FieldValue) error {
		fieldType := fv.Field().Type()
		switch fieldType {
		case IntFieldType:
			val := fv.Value()
			var intVal int64
			switch v := val.(type) {
			case int:
				intVal = int64(v)
			case int32:
				intVal = int64(v)
			case int64:
				intVal = v
			default:
				return nil
			}
			if intVal < 0 {
				return fmt.Errorf("field %q must be non-negative", fv.Field().Name())
			}
		case FloatFieldType:
			val := fv.Value()
			var floatVal float64
			switch v := val.(type) {
			case float32:
				floatVal = float64(v)
			case float64:
				floatVal = v
			default:
				return nil
			}
			if floatVal < 0 {
				return fmt.Errorf("field %q must be non-negative", fv.Field().Name())
			}
		default:
			return fmt.Errorf("non-negative rule only applies to int and float fields")
		}
		return nil
	}
}

func InRule(allowedValues ...any) FieldRule {
	return func(fv FieldValue) error {
		val := fv.Value()
		for _, allowed := range allowedValues {
			if val == allowed {
				return nil
			}
		}
		return fmt.Errorf("field %q must be one of %v", fv.Field().Name(), allowedValues)
	}
}

func NotEmptyRule() FieldRule {
	return func(fv FieldValue) error {
		if fv.Field().Type() != StringFieldType {
			return fmt.Errorf("not empty rule only applies to string fields")
		}
		val, ok := fv.Value().(string)
		if !ok {
			return nil
		}
		if strings.TrimSpace(val) == "" {
			return fmt.Errorf("field %q cannot be empty", fv.Field().Name())
		}
		return nil
	}
}

func FutureDateRule() FieldRule {
	return func(fv FieldValue) error {
		fieldType := fv.Field().Type()
		if fieldType != DateFieldType && fieldType != TimeFieldType && fieldType != DateTimeFieldType && fieldType != TimestampFieldType {
			return fmt.Errorf("future date rule only applies to date/time fields")
		}
		val, ok := fv.Value().(time.Time)
		if !ok {
			return nil
		}
		if !val.After(time.Now()) {
			return fmt.Errorf("field %q must be a future date", fv.Field().Name())
		}
		return nil
	}
}

func PastDateRule() FieldRule {
	return func(fv FieldValue) error {
		fieldType := fv.Field().Type()
		if fieldType != DateFieldType && fieldType != TimeFieldType && fieldType != DateTimeFieldType && fieldType != TimestampFieldType {
			return fmt.Errorf("past date rule only applies to date/time fields")
		}
		val, ok := fv.Value().(time.Time)
		if !ok {
			return nil
		}
		if !val.Before(time.Now()) {
			return fmt.Errorf("field %q must be a past date", fv.Field().Name())
		}
		return nil
	}
}
