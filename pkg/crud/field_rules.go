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
	return func(fv FieldValue) error {
		if fv.Field().Type() != StringFieldType {
			return fmt.Errorf("email rule only applies to string fields")
		}
		val, ok := fv.Value().(string)
		if !ok || val == "" {
			return nil
		}
		emailPattern := `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
		matched, err := regexp.MatchString(emailPattern, val)
		if err != nil {
			return err
		}
		if !matched {
			return fmt.Errorf("field %q must be a valid email address", fv.Field().Name())
		}
		return nil
	}
}

func PatternRule(pattern string) FieldRule {
	return func(fv FieldValue) error {
		if fv.Field().Type() != StringFieldType {
			return fmt.Errorf("pattern rule only applies to string fields")
		}
		val, ok := fv.Value().(string)
		if !ok || val == "" {
			return nil
		}
		matched, err := regexp.MatchString(pattern, val)
		if err != nil {
			return err
		}
		if !matched {
			return fmt.Errorf("field %q must match pattern", fv.Field().Name())
		}
		return nil
	}
}

func MinValueRule(minValue float64) FieldRule {
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
			if float64(intVal) < minValue {
				return fmt.Errorf("field %q must be at least %g", fv.Field().Name(), minValue)
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
			if floatVal < minValue {
				return fmt.Errorf("field %q must be at least %g", fv.Field().Name(), minValue)
			}
		default:
			return fmt.Errorf("min value rule only applies to int and float fields")
		}
		return nil
	}
}

func MaxValueRule(maxValue float64) FieldRule {
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
			if float64(intVal) > maxValue {
				return fmt.Errorf("field %q must be at most %g", fv.Field().Name(), maxValue)
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
			if floatVal > maxValue {
				return fmt.Errorf("field %q must be at most %g", fv.Field().Name(), maxValue)
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
		return fmt.Errorf("field %q must be one of the allowed values", fv.Field().Name())
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

func MultipleOfRule(multiple int64) FieldRule {
	return func(fv FieldValue) error {
		if fv.Field().Type() != IntFieldType {
			return fmt.Errorf("multiple of rule only applies to int fields")
		}
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
		if intVal%multiple != 0 {
			return fmt.Errorf("field %q must be a multiple of %d", fv.Field().Name(), multiple)
		}
		return nil
	}
}

func WeekdayRule() FieldRule {
	return func(fv FieldValue) error {
		switch fv.Field().Type() {
		case DateFieldType, DateTimeFieldType, TimestampFieldType:
			t, err := fv.AsTime()
			if err != nil {
				return err
			}
			weekday := t.Weekday()
			if weekday == time.Saturday || weekday == time.Sunday {
				return fmt.Errorf("field %q must be a weekday", fv.Field().Name())
			}
			return nil
		default:
			return fmt.Errorf("weekday rule only applies to date/time fields")
		}
	}
}

func UUIDVersionRule(version int) FieldRule {
	return func(fv FieldValue) error {
		if fv.Field().Type() != UUIDFieldType {
			return fmt.Errorf("UUID version rule only applies to UUID fields")
		}
		u, err := fv.AsUUID()
		if err != nil {
			return err
		}
		actualVersion := int(u.Version())
		if actualVersion != version {
			return fmt.Errorf("field %q must be UUID version %d, got version %d", fv.Field().Name(), version, actualVersion)
		}
		return nil
	}
}

func URLRule() FieldRule {
	return func(fv FieldValue) error {
		if fv.Field().Type() != StringFieldType {
			return fmt.Errorf("URL rule only applies to string fields")
		}
		val, ok := fv.Value().(string)
		if !ok || val == "" {
			return nil
		}
		urlPattern := `^(https?://)?([\da-z\.-]+)\.([a-z\.]{2,6})([/\w \.-]*)*/?$`
		matched, err := regexp.MatchString(urlPattern, val)
		if err != nil {
			return err
		}
		if !matched {
			return fmt.Errorf("field %q must be a valid URL", fv.Field().Name())
		}
		return nil
	}
}

func PhoneRule() FieldRule {
	return func(fv FieldValue) error {
		if fv.Field().Type() != StringFieldType {
			return fmt.Errorf("phone rule only applies to string fields")
		}
		val, ok := fv.Value().(string)
		if !ok || val == "" {
			return nil
		}
		// Simple international phone pattern
		phonePattern := `^\+?[1-9]\d{1,14}$`
		matched, err := regexp.MatchString(phonePattern, val)
		if err != nil {
			return err
		}
		if !matched {
			return fmt.Errorf("field %q must be a valid phone number", fv.Field().Name())
		}
		return nil
	}
}

func AlphaRule() FieldRule {
	return func(fv FieldValue) error {
		if fv.Field().Type() != StringFieldType {
			return fmt.Errorf("alpha rule only applies to string fields")
		}
		val, ok := fv.Value().(string)
		if !ok || val == "" {
			return nil
		}
		for _, r := range val {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
				return fmt.Errorf("field %q must contain only alphabetic characters", fv.Field().Name())
			}
		}
		return nil
	}
}

func AlphanumericRule() FieldRule {
	return func(fv FieldValue) error {
		if fv.Field().Type() != StringFieldType {
			return fmt.Errorf("alphanumeric rule only applies to string fields")
		}
		val, ok := fv.Value().(string)
		if !ok || val == "" {
			return nil
		}
		for _, r := range val {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
				return fmt.Errorf("field %q must contain only alphanumeric characters", fv.Field().Name())
			}
		}
		return nil
	}
}
