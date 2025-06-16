package crud

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFieldOptions(t *testing.T) {
	t.Run("WithKey", func(t *testing.T) {
		field := NewStringField("test", WithKey(true))
		assert.True(t, field.Key())
		
		field2 := NewStringField("test2", WithKey(false))
		assert.False(t, field2.Key())
	})

	t.Run("WithReadonly", func(t *testing.T) {
		field := NewStringField("test", WithReadonly(true))
		assert.True(t, field.Readonly())
		
		field2 := NewStringField("test2", WithReadonly(false))
		assert.False(t, field2.Readonly())
	})

	t.Run("WithHidden", func(t *testing.T) {
		field := NewStringField("test", WithHidden(true))
		assert.True(t, field.Hidden())
		
		field2 := NewStringField("test2", WithHidden(false))
		assert.False(t, field2.Hidden())
	})

	t.Run("WithSearchable", func(t *testing.T) {
		field := NewStringField("test", WithSearchable(true))
		assert.True(t, field.Searchable())
		
		field2 := NewStringField("test2", WithSearchable(false))
		assert.False(t, field2.Searchable())
	})

	t.Run("WithSearchable panics for non-string fields", func(t *testing.T) {
		assert.Panics(t, func() {
			NewIntField("test", WithSearchable(true))
		})
	})

	t.Run("WithInitialValue", func(t *testing.T) {
		field := NewStringField("test", WithInitialValue("initial"))
		assert.Equal(t, "initial", field.InitialValue())
		
		field2 := NewIntField("test2", WithInitialValue(42))
		assert.Equal(t, 42, field2.InitialValue())
	})

	t.Run("WithRules", func(t *testing.T) {
		rules := []FieldRule{RequiredRule(), EmailRule()}
		field := NewStringField("test", WithRules(rules))
		assert.Len(t, field.Rules(), 2)
	})

	t.Run("WithRule", func(t *testing.T) {
		field := NewStringField("test", WithRule(RequiredRule()))
		assert.Len(t, field.Rules(), 1)
		
		// Test multiple rules
		field2 := NewStringField("test2", 
			WithRule(RequiredRule()),
			WithRule(EmailRule()),
		)
		assert.Len(t, field2.Rules(), 2)
	})

	t.Run("WithAttrs", func(t *testing.T) {
		attrs := map[string]any{
			"custom1": "value1",
			"custom2": 42,
		}
		field := NewStringField("test", WithAttrs(attrs))
		assert.Equal(t, "value1", field.Attrs()["custom1"])
		assert.Equal(t, 42, field.Attrs()["custom2"])
	})

	t.Run("WithAttr", func(t *testing.T) {
		field := NewStringField("test", 
			WithAttr("custom1", "value1"),
			WithAttr("custom2", 42),
		)
		assert.Equal(t, "value1", field.Attrs()["custom1"])
		assert.Equal(t, 42, field.Attrs()["custom2"])
	})
}

func TestStringFieldOptions(t *testing.T) {
	t.Run("WithMinLen", func(t *testing.T) {
		field := NewStringField("test", WithMinLen(5))
		stringField, err := field.AsStringField()
		require.NoError(t, err)
		assert.Equal(t, 5, stringField.MinLen())
		assert.Len(t, field.Rules(), 1) // Should add validation rule
	})

	t.Run("WithMaxLen", func(t *testing.T) {
		field := NewStringField("test", WithMaxLen(10))
		stringField, err := field.AsStringField()
		require.NoError(t, err)
		assert.Equal(t, 10, stringField.MaxLen())
		assert.Len(t, field.Rules(), 1) // Should add validation rule
	})

	t.Run("WithMultiline", func(t *testing.T) {
		field := NewStringField("test", WithMultiline(true))
		stringField, err := field.AsStringField()
		require.NoError(t, err)
		assert.True(t, stringField.Multiline())
		
		field2 := NewStringField("test2", WithMultiline(false))
		stringField2, err := field2.AsStringField()
		require.NoError(t, err)
		assert.False(t, stringField2.Multiline())
	})

	t.Run("WithPattern", func(t *testing.T) {
		pattern := "^[a-z]+$"
		field := NewStringField("test", WithPattern(pattern))
		stringField, err := field.AsStringField()
		require.NoError(t, err)
		assert.Equal(t, pattern, stringField.Pattern())
		assert.Len(t, field.Rules(), 1) // Should add validation rule
	})

	t.Run("WithTrim", func(t *testing.T) {
		field := NewStringField("test", WithTrim(true))
		stringField, err := field.AsStringField()
		require.NoError(t, err)
		assert.True(t, stringField.Trim())
	})

	t.Run("WithUppercase", func(t *testing.T) {
		field := NewStringField("test", WithUppercase(true))
		stringField, err := field.AsStringField()
		require.NoError(t, err)
		assert.True(t, stringField.Uppercase())
	})

	t.Run("WithLowercase", func(t *testing.T) {
		field := NewStringField("test", WithLowercase(true))
		stringField, err := field.AsStringField()
		require.NoError(t, err)
		assert.True(t, stringField.Lowercase())
	})
}

func TestIntFieldOptions(t *testing.T) {
	t.Run("WithMin", func(t *testing.T) {
		field := NewIntField("test", WithMin(10))
		intField, err := field.AsIntField()
		require.NoError(t, err)
		assert.Equal(t, int64(10), intField.Min())
	})

	t.Run("WithMax", func(t *testing.T) {
		field := NewIntField("test", WithMax(100))
		intField, err := field.AsIntField()
		require.NoError(t, err)
		assert.Equal(t, int64(100), intField.Max())
	})

	t.Run("WithStep", func(t *testing.T) {
		field := NewIntField("test", WithStep(5))
		intField, err := field.AsIntField()
		require.NoError(t, err)
		assert.Equal(t, int64(5), intField.Step())
	})

	t.Run("WithMultipleOf", func(t *testing.T) {
		field := NewIntField("test", WithMultipleOf(3))
		intField, err := field.AsIntField()
		require.NoError(t, err)
		assert.Equal(t, int64(3), intField.MultipleOf())
		assert.Len(t, field.Rules(), 1) // Should add validation rule
	})
}

func TestFloatFieldOptions(t *testing.T) {
	t.Run("WithFloatMin", func(t *testing.T) {
		field := NewFloatField("test", WithFloatMin(1.5))
		floatField, err := field.AsFloatField()
		require.NoError(t, err)
		assert.Equal(t, 1.5, floatField.Min())
	})

	t.Run("WithFloatMax", func(t *testing.T) {
		field := NewFloatField("test", WithFloatMax(10.5))
		floatField, err := field.AsFloatField()
		require.NoError(t, err)
		assert.Equal(t, 10.5, floatField.Max())
	})

	t.Run("WithPrecision", func(t *testing.T) {
		field := NewFloatField("test", WithPrecision(3))
		floatField, err := field.AsFloatField()
		require.NoError(t, err)
		assert.Equal(t, 3, floatField.Precision())
	})

	t.Run("WithFloatStep", func(t *testing.T) {
		field := NewFloatField("test", WithFloatStep(0.5))
		floatField, err := field.AsFloatField()
		require.NoError(t, err)
		assert.Equal(t, 0.5, floatField.Step())
	})
}

func TestDateFieldOptions(t *testing.T) {
	t.Run("WithMinDate", func(t *testing.T) {
		minDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		field := NewDateField("test", WithMinDate(minDate))
		dateField, err := field.AsDateField()
		require.NoError(t, err)
		assert.Equal(t, minDate, dateField.MinDate())
	})

	t.Run("WithMaxDate", func(t *testing.T) {
		maxDate := time.Date(2030, 12, 31, 0, 0, 0, 0, time.UTC)
		field := NewDateField("test", WithMaxDate(maxDate))
		dateField, err := field.AsDateField()
		require.NoError(t, err)
		assert.Equal(t, maxDate, dateField.MaxDate())
	})

	t.Run("WithFormat", func(t *testing.T) {
		format := "02/01/2006"
		field := NewDateField("test", WithFormat(format))
		dateField, err := field.AsDateField()
		require.NoError(t, err)
		assert.Equal(t, format, dateField.Format())
	})

	t.Run("WithWeekdaysOnly", func(t *testing.T) {
		field := NewDateField("test", WithWeekdaysOnly(true))
		dateField, err := field.AsDateField()
		require.NoError(t, err)
		assert.True(t, dateField.WeekdaysOnly())
		assert.Len(t, field.Rules(), 1) // Should add validation rule
	})
}

func TestTimeFieldOptions(t *testing.T) {
	t.Run("WithFormat for TimeField", func(t *testing.T) {
		format := "3:04 PM"
		field := NewTimeField("test", WithFormat(format))
		timeField, err := field.AsTimeField()
		require.NoError(t, err)
		assert.Equal(t, format, timeField.Format())
	})
}

func TestDateTimeFieldOptions(t *testing.T) {
	t.Run("WithMinDate for DateTimeField", func(t *testing.T) {
		minDate := time.Date(2020, 1, 1, 10, 30, 0, 0, time.UTC)
		field := NewDateTimeField("test", WithMinDate(minDate))
		dateTimeField, err := field.AsDateTimeField()
		require.NoError(t, err)
		assert.Equal(t, minDate, dateTimeField.MinDateTime())
	})

	t.Run("WithMaxDate for DateTimeField", func(t *testing.T) {
		maxDate := time.Date(2030, 12, 31, 23, 59, 59, 0, time.UTC)
		field := NewDateTimeField("test", WithMaxDate(maxDate))
		dateTimeField, err := field.AsDateTimeField()
		require.NoError(t, err)
		assert.Equal(t, maxDate, dateTimeField.MaxDateTime())
	})

	t.Run("WithTimezone", func(t *testing.T) {
		timezone := "America/New_York"
		field := NewDateTimeField("test", WithTimezone(timezone))
		dateTimeField, err := field.AsDateTimeField()
		require.NoError(t, err)
		assert.Equal(t, timezone, dateTimeField.Timezone())
	})
}

func TestBoolFieldOptions(t *testing.T) {
	t.Run("WithDefaultValue for BoolField", func(t *testing.T) {
		field := NewBoolField("test", WithDefaultValue(true))
		boolField, err := field.AsBoolField()
		require.NoError(t, err)
		assert.True(t, boolField.DefaultValue())
	})

	t.Run("WithTrueLabel", func(t *testing.T) {
		field := NewBoolField("test", WithTrueLabel("Yes"))
		boolField, err := field.AsBoolField()
		require.NoError(t, err)
		assert.Equal(t, "Yes", boolField.TrueLabel())
	})

	t.Run("WithFalseLabel", func(t *testing.T) {
		field := NewBoolField("test", WithFalseLabel("No"))
		boolField, err := field.AsBoolField()
		require.NoError(t, err)
		assert.Equal(t, "No", boolField.FalseLabel())
	})
}

func TestUUIDFieldOptions(t *testing.T) {
	t.Run("WithUUIDVersion", func(t *testing.T) {
		field := NewUUIDField("test", WithUUIDVersion(1))
		uuidField, err := field.AsUUIDField()
		require.NoError(t, err)
		assert.Equal(t, 1, uuidField.Version())
		assert.Len(t, field.Rules(), 1) // Should add validation rule
	})
}

func TestValidationRuleOptions(t *testing.T) {
	t.Run("WithURL", func(t *testing.T) {
		field := NewStringField("test", WithURL())
		assert.Len(t, field.Rules(), 1)
		
		// Test validation
		validURL := field.Value("https://example.com")
		err := field.Rules()[0](validURL)
		assert.NoError(t, err)
		
		invalidURL := field.Value("not a url")
		err = field.Rules()[0](invalidURL)
		assert.Error(t, err)
	})

	t.Run("WithPhone", func(t *testing.T) {
		field := NewStringField("test", WithPhone())
		assert.Len(t, field.Rules(), 1)
		
		// Test validation
		validPhone := field.Value("+12345678901")
		err := field.Rules()[0](validPhone)
		assert.NoError(t, err)
		
		// Another valid phone without +
		validPhone2 := field.Value("12345678901")
		err = field.Rules()[0](validPhone2)
		assert.NoError(t, err)
		
		// Too short (less than 2 digits)
		invalidPhone := field.Value("1")
		err = field.Rules()[0](invalidPhone)
		assert.Error(t, err)
		
		// Invalid characters
		invalidPhone2 := field.Value("123-456-7890")
		err = field.Rules()[0](invalidPhone2)
		assert.Error(t, err)
	})

	t.Run("WithAlpha", func(t *testing.T) {
		field := NewStringField("test", WithAlpha())
		assert.Len(t, field.Rules(), 1)
		
		// Test validation
		validAlpha := field.Value("abcXYZ")
		err := field.Rules()[0](validAlpha)
		assert.NoError(t, err)
		
		invalidAlpha := field.Value("abc123")
		err = field.Rules()[0](invalidAlpha)
		assert.Error(t, err)
	})

	t.Run("WithAlphanumeric", func(t *testing.T) {
		field := NewStringField("test", WithAlphanumeric())
		assert.Len(t, field.Rules(), 1)
		
		// Test validation
		validAlphanumeric := field.Value("abc123XYZ")
		err := field.Rules()[0](validAlphanumeric)
		assert.NoError(t, err)
		
		invalidAlphanumeric := field.Value("abc-123")
		err = field.Rules()[0](invalidAlphanumeric)
		assert.Error(t, err)
	})

	t.Run("WithEmail", func(t *testing.T) {
		field := NewStringField("test", WithEmail())
		assert.Len(t, field.Rules(), 1)
		
		// Test validation
		validEmail := field.Value("test@example.com")
		err := field.Rules()[0](validEmail)
		assert.NoError(t, err)
		
		invalidEmail := field.Value("not-an-email")
		err = field.Rules()[0](invalidEmail)
		assert.Error(t, err)
	})

	t.Run("WithRequired", func(t *testing.T) {
		field := NewStringField("test", WithRequired())
		assert.Len(t, field.Rules(), 1)
		
		// Test validation
		nonEmpty := field.Value("value")
		err := field.Rules()[0](nonEmpty)
		assert.NoError(t, err)
		
		empty := field.Value("")
		err = field.Rules()[0](empty)
		assert.Error(t, err)
	})

	t.Run("WithPositive", func(t *testing.T) {
		field := NewIntField("test", WithPositive())
		assert.Len(t, field.Rules(), 1)
		
		// Test validation
		positive := field.Value(int64(5))
		err := field.Rules()[0](positive)
		assert.NoError(t, err)
		
		zero := field.Value(int64(0))
		err = field.Rules()[0](zero)
		assert.Error(t, err)
		
		negative := field.Value(int64(-5))
		err = field.Rules()[0](negative)
		assert.Error(t, err)
	})

	t.Run("WithNonNegative", func(t *testing.T) {
		field := NewIntField("test", WithNonNegative())
		assert.Len(t, field.Rules(), 1)
		
		// Test validation
		positive := field.Value(int64(5))
		err := field.Rules()[0](positive)
		assert.NoError(t, err)
		
		zero := field.Value(int64(0))
		err = field.Rules()[0](zero)
		assert.NoError(t, err)
		
		negative := field.Value(int64(-5))
		err = field.Rules()[0](negative)
		assert.Error(t, err)
	})

	t.Run("WithNotEmpty", func(t *testing.T) {
		field := NewStringField("test", WithNotEmpty())
		assert.Len(t, field.Rules(), 1)
		
		// Test validation
		nonEmpty := field.Value("value")
		err := field.Rules()[0](nonEmpty)
		assert.NoError(t, err)
		
		empty := field.Value("")
		err = field.Rules()[0](empty)
		assert.Error(t, err)
		
		whitespace := field.Value("   ")
		err = field.Rules()[0](whitespace)
		assert.Error(t, err)
	})

	t.Run("WithFutureDate", func(t *testing.T) {
		field := NewDateField("test", WithFutureDate())
		assert.Len(t, field.Rules(), 1)
		
		// Test validation
		futureDate := field.Value(time.Now().Add(24 * time.Hour))
		err := field.Rules()[0](futureDate)
		assert.NoError(t, err)
		
		pastDate := field.Value(time.Now().Add(-24 * time.Hour))
		err = field.Rules()[0](pastDate)
		assert.Error(t, err)
	})

	t.Run("WithPastDate", func(t *testing.T) {
		field := NewDateField("test", WithPastDate())
		assert.Len(t, field.Rules(), 1)
		
		// Test validation
		pastDate := field.Value(time.Now().Add(-24 * time.Hour))
		err := field.Rules()[0](pastDate)
		assert.NoError(t, err)
		
		futureDate := field.Value(time.Now().Add(24 * time.Hour))
		err = field.Rules()[0](futureDate)
		assert.Error(t, err)
	})
}

func TestMultipleOptions(t *testing.T) {
	t.Run("Combining multiple options", func(t *testing.T) {
		field := NewStringField("email",
			WithKey(true),
			WithReadonly(true),
			WithSearchable(true),
			WithMinLen(5),
			WithMaxLen(100),
			WithPattern(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`),
			WithTrim(true),
			WithLowercase(true),
			WithEmail(),
			WithRequired(),
		)
		
		assert.True(t, field.Key())
		assert.True(t, field.Readonly())
		assert.True(t, field.Searchable())
		
		stringField, err := field.AsStringField()
		require.NoError(t, err)
		assert.Equal(t, 5, stringField.MinLen())
		assert.Equal(t, 100, stringField.MaxLen())
		assert.NotEmpty(t, stringField.Pattern())
		assert.True(t, stringField.Trim())
		assert.True(t, stringField.Lowercase())
		
		// Should have 5 rules: MinLen, MaxLen, Pattern, Email, Required
		assert.Len(t, field.Rules(), 5)
	})

	t.Run("Int field with multiple options", func(t *testing.T) {
		field := NewIntField("age",
			WithMin(0),
			WithMax(150),
			WithStep(1),
			WithMultipleOf(1),
			WithNonNegative(),
			WithRequired(),
		)
		
		intField, err := field.AsIntField()
		require.NoError(t, err)
		assert.Equal(t, int64(0), intField.Min())
		assert.Equal(t, int64(150), intField.Max())
		assert.Equal(t, int64(1), intField.Step())
		assert.Equal(t, int64(1), intField.MultipleOf())
		
		// Should have 3 rules: MultipleOf, NonNegative, Required
		assert.Len(t, field.Rules(), 3)
	})

	t.Run("Date field with multiple options", func(t *testing.T) {
		minDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		maxDate := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)
		
		field := NewDateField("appointment",
			WithMinDate(minDate),
			WithMaxDate(maxDate),
			WithFormat("02/01/2006"),
			WithWeekdaysOnly(true),
			WithFutureDate(),
			WithRequired(),
		)
		
		dateField, err := field.AsDateField()
		require.NoError(t, err)
		assert.Equal(t, minDate, dateField.MinDate())
		assert.Equal(t, maxDate, dateField.MaxDate())
		assert.Equal(t, "02/01/2006", dateField.Format())
		assert.True(t, dateField.WeekdaysOnly())
		
		// Should have 3 rules: WeekdayRule, FutureDateRule, Required
		assert.Len(t, field.Rules(), 3)
	})
}

func TestFieldDefaultValues(t *testing.T) {
	t.Run("StringField defaults", func(t *testing.T) {
		field := NewStringField("test")
		stringField, err := field.AsStringField()
		require.NoError(t, err)
		
		assert.Equal(t, 0, stringField.MinLen())
		assert.Equal(t, 2147483647, stringField.MaxLen()) // math.MaxInt32
		assert.False(t, stringField.Multiline())
		assert.Empty(t, stringField.Pattern())
		assert.False(t, stringField.Trim())
		assert.False(t, stringField.Uppercase())
		assert.False(t, stringField.Lowercase())
	})

	t.Run("IntField defaults", func(t *testing.T) {
		field := NewIntField("test")
		intField, err := field.AsIntField()
		require.NoError(t, err)
		
		assert.Equal(t, int64(-9223372036854775808), intField.Min()) // math.MinInt64
		assert.Equal(t, int64(9223372036854775807), intField.Max())  // math.MaxInt64
		assert.Equal(t, int64(1), intField.Step())
		assert.Equal(t, int64(1), intField.MultipleOf())
	})

	t.Run("FloatField defaults", func(t *testing.T) {
		field := NewFloatField("test")
		floatField, err := field.AsFloatField()
		require.NoError(t, err)
		
		assert.InDelta(t, -1.7976931348623157e+308, floatField.Min(), 1e300) // -math.MaxFloat64
		assert.InDelta(t, 1.7976931348623157e+308, floatField.Max(), 1e300)  // math.MaxFloat64
		assert.Equal(t, 2, floatField.Precision())
		assert.Equal(t, 0.01, floatField.Step())
	})

	t.Run("BoolField defaults", func(t *testing.T) {
		field := NewBoolField("test")
		boolField, err := field.AsBoolField()
		require.NoError(t, err)
		
		assert.False(t, boolField.DefaultValue())
		assert.Empty(t, boolField.TrueLabel())
		assert.Empty(t, boolField.FalseLabel())
	})

	t.Run("DateField defaults", func(t *testing.T) {
		field := NewDateField("test")
		dateField, err := field.AsDateField()
		require.NoError(t, err)
		
		assert.Equal(t, time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC), dateField.MinDate())
		assert.Equal(t, time.Date(9999, 12, 31, 23, 59, 59, 999999999, time.UTC), dateField.MaxDate())
		assert.Equal(t, "2006-01-02", dateField.Format())
		assert.False(t, dateField.WeekdaysOnly())
	})

	t.Run("TimeField defaults", func(t *testing.T) {
		field := NewTimeField("test")
		timeField, err := field.AsTimeField()
		require.NoError(t, err)
		
		assert.Equal(t, "15:04:05", timeField.Format())
	})

	t.Run("DateTimeField defaults", func(t *testing.T) {
		field := NewDateTimeField("test")
		dateTimeField, err := field.AsDateTimeField()
		require.NoError(t, err)
		
		assert.Equal(t, time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC), dateTimeField.MinDateTime())
		assert.Equal(t, time.Date(9999, 12, 31, 23, 59, 59, 999999999, time.UTC), dateTimeField.MaxDateTime())
		assert.Equal(t, "2006-01-02 15:04:05", dateTimeField.Format())
		assert.Equal(t, "UTC", dateTimeField.Timezone())
		assert.False(t, dateTimeField.WeekdaysOnly())
	})

	t.Run("UUIDField defaults", func(t *testing.T) {
		field := NewUUIDField("test")
		uuidField, err := field.AsUUIDField()
		require.NoError(t, err)
		
		assert.Equal(t, 4, uuidField.Version())
	})
}

func TestOptionValidation(t *testing.T) {
	t.Run("Options create proper field values", func(t *testing.T) {
		// String field with validation
		stringField := NewStringField("username",
			WithMinLen(3),
			WithMaxLen(20),
			WithPattern("^[a-zA-Z0-9_]+$"),
		)
		
		// Valid value
		validValue := stringField.Value("john_doe")
		for _, rule := range stringField.Rules() {
			err := rule(validValue)
			assert.NoError(t, err)
		}
		
		// Too short
		shortValue := stringField.Value("jo")
		hasError := false
		for _, rule := range stringField.Rules() {
			if err := rule(shortValue); err != nil {
				hasError = true
				break
			}
		}
		assert.True(t, hasError)
		
		// Too long
		longValue := stringField.Value("this_is_a_very_long_username")
		hasError = false
		for _, rule := range stringField.Rules() {
			if err := rule(longValue); err != nil {
				hasError = true
				break
			}
		}
		assert.True(t, hasError)
		
		// Invalid pattern
		invalidPattern := stringField.Value("john-doe")
		hasError = false
		for _, rule := range stringField.Rules() {
			if err := rule(invalidPattern); err != nil {
				hasError = true
				break
			}
		}
		assert.True(t, hasError)
	})

	t.Run("UUID field with version validation", func(t *testing.T) {
		uuidField := NewUUIDField("id", WithUUIDVersion(4))
		
		// Valid UUID v4
		uuid4 := uuid.New() // Creates v4 by default
		validValue := uuidField.Value(uuid4)
		for _, rule := range uuidField.Rules() {
			err := rule(validValue)
			assert.NoError(t, err)
		}
	})

	t.Run("Date field with weekday validation", func(t *testing.T) {
		dateField := NewDateField("workday", WithWeekdaysOnly(true))
		
		// Monday (weekday)
		monday := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC) // January 1, 2024 is Monday
		validValue := dateField.Value(monday)
		for _, rule := range dateField.Rules() {
			err := rule(validValue)
			assert.NoError(t, err)
		}
		
		// Sunday (weekend)
		sunday := time.Date(2024, 1, 7, 0, 0, 0, 0, time.UTC) // January 7, 2024 is Sunday
		invalidValue := dateField.Value(sunday)
		hasError := false
		for _, rule := range dateField.Rules() {
			if err := rule(invalidValue); err != nil {
				hasError = true
				break
			}
		}
		assert.True(t, hasError)
	})
}