package crud_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/crud"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFieldOptions(t *testing.T) {
	t.Run("WithKey", func(t *testing.T) {
		field := crud.NewStringField("test", crud.WithKey())
		assert.True(t, field.Key())

		field2 := crud.NewStringField("test2")
		assert.False(t, field2.Key())
	})

	t.Run("WithReadonly", func(t *testing.T) {
		field := crud.NewStringField("test", crud.WithReadonly())
		assert.True(t, field.Readonly())

		field2 := crud.NewStringField("test2")
		assert.False(t, field2.Readonly())
	})

	t.Run("WithHidden", func(t *testing.T) {
		field := crud.NewStringField("test", crud.WithHidden())
		assert.True(t, field.Hidden())

		field2 := crud.NewStringField("test2")
		assert.False(t, field2.Hidden())
	})

	t.Run("WithSearchable", func(t *testing.T) {
		field := crud.NewStringField("test", crud.WithSearchable())
		assert.True(t, field.Searchable())

		field2 := crud.NewStringField("test2")
		assert.False(t, field2.Searchable())
	})

	t.Run("WithSearchable panics for non-string fields", func(t *testing.T) {
		assert.Panics(t, func() {
			crud.NewIntField("test", crud.WithSearchable())
		})
	})

	t.Run("WithInitialValue", func(t *testing.T) {
		field := crud.NewStringField("test", crud.WithInitialValue(func(ctx context.Context) any {
			return "initial"
		}))
		assert.Equal(t, "initial", field.InitialValue(context.Background()))

		field2 := crud.NewIntField("test2", crud.WithInitialValue(func(ctx context.Context) any {
			return 42
		}))
		assert.Equal(t, 42, field2.InitialValue(context.Background()))
	})

	t.Run("WithRules", func(t *testing.T) {
		rules := []crud.FieldRule{crud.RequiredRule(), crud.EmailRule()}
		field := crud.NewStringField("test", crud.WithRules(rules...))
		assert.Len(t, field.Rules(), 2)
	})

	t.Run("WithRule", func(t *testing.T) {
		field := crud.NewStringField("test", crud.WithRule(crud.RequiredRule()))
		assert.Len(t, field.Rules(), 1)

		// Test multiple rules
		field2 := crud.NewStringField("test2",
			crud.WithRule(crud.RequiredRule()),
			crud.WithRule(crud.EmailRule()),
		)
		assert.Len(t, field2.Rules(), 2)
	})

	t.Run("WithAttrs", func(t *testing.T) {
		attrs := map[string]any{
			"custom1": "value1",
			"custom2": 42,
		}
		field := crud.NewStringField("test", crud.WithAttrs(attrs))
		assert.Equal(t, "value1", field.Attrs()["custom1"])
		assert.Equal(t, 42, field.Attrs()["custom2"])
	})

	t.Run("WithAttr", func(t *testing.T) {
		field := crud.NewStringField("test",
			crud.WithAttr("custom1", "value1"),
			crud.WithAttr("custom2", 42),
		)
		assert.Equal(t, "value1", field.Attrs()["custom1"])
		assert.Equal(t, 42, field.Attrs()["custom2"])
	})
}

func TestStringFieldOptions(t *testing.T) {
	t.Run("WithMinLen", func(t *testing.T) {
		field := crud.NewStringField("test", crud.WithMinLen(5))
		stringField, err := field.AsStringField()
		require.NoError(t, err)
		assert.Equal(t, 5, stringField.MinLen())
		assert.Len(t, field.Rules(), 1) // Should add validation rule
	})

	t.Run("WithMaxLen", func(t *testing.T) {
		field := crud.NewStringField("test", crud.WithMaxLen(10))
		stringField, err := field.AsStringField()
		require.NoError(t, err)
		assert.Equal(t, 10, stringField.MaxLen())
		assert.Len(t, field.Rules(), 1) // Should add validation rule
	})

	t.Run("WithMultiline", func(t *testing.T) {
		field := crud.NewStringField("test", crud.WithMultiline())
		stringField, err := field.AsStringField()
		require.NoError(t, err)
		assert.True(t, stringField.Multiline())

		field2 := crud.NewStringField("test2")
		stringField2, err := field2.AsStringField()
		require.NoError(t, err)
		assert.False(t, stringField2.Multiline())
	})

	t.Run("WithPattern", func(t *testing.T) {
		pattern := "^[a-z]+$"
		field := crud.NewStringField("test", crud.WithPattern(pattern))
		stringField, err := field.AsStringField()
		require.NoError(t, err)
		assert.Equal(t, pattern, stringField.Pattern())
		assert.Len(t, field.Rules(), 1) // Should add validation rule
	})

	t.Run("WithTrim", func(t *testing.T) {
		field := crud.NewStringField("test", crud.WithTrim())
		stringField, err := field.AsStringField()
		require.NoError(t, err)
		assert.True(t, stringField.Trim())
	})

	t.Run("WithUppercase", func(t *testing.T) {
		field := crud.NewStringField("test", crud.WithUppercase())
		stringField, err := field.AsStringField()
		require.NoError(t, err)
		assert.True(t, stringField.Uppercase())
	})

	t.Run("WithLowercase", func(t *testing.T) {
		field := crud.NewStringField("test", crud.WithLowercase())
		stringField, err := field.AsStringField()
		require.NoError(t, err)
		assert.True(t, stringField.Lowercase())
	})
}

func TestIntFieldOptions(t *testing.T) {
	t.Run("WithMin", func(t *testing.T) {
		field := crud.NewIntField("test", crud.WithMin(10))
		intField, err := field.AsIntField()
		require.NoError(t, err)
		assert.Equal(t, int64(10), intField.Min())
	})

	t.Run("WithMax", func(t *testing.T) {
		field := crud.NewIntField("test", crud.WithMax(100))
		intField, err := field.AsIntField()
		require.NoError(t, err)
		assert.Equal(t, int64(100), intField.Max())
	})

	t.Run("WithStep", func(t *testing.T) {
		field := crud.NewIntField("test", crud.WithStep(5))
		intField, err := field.AsIntField()
		require.NoError(t, err)
		assert.Equal(t, int64(5), intField.Step())
	})

	t.Run("WithMultipleOf", func(t *testing.T) {
		field := crud.NewIntField("test", crud.WithMultipleOf(3))
		intField, err := field.AsIntField()
		require.NoError(t, err)
		assert.Equal(t, int64(3), intField.MultipleOf())
		assert.Len(t, field.Rules(), 1) // Should add validation rule
	})
}

func TestFloatFieldOptions(t *testing.T) {
	t.Run("WithFloatMin", func(t *testing.T) {
		field := crud.NewFloatField("test", crud.WithFloatMin(1.5))
		floatField, err := field.AsFloatField()
		require.NoError(t, err)
		assert.InEpsilon(t, 1.5, floatField.Min(), 1e-9)
	})

	t.Run("WithFloatMax", func(t *testing.T) {
		field := crud.NewFloatField("test", crud.WithFloatMax(10.5))
		floatField, err := field.AsFloatField()
		require.NoError(t, err)
		assert.InEpsilon(t, 10.5, floatField.Max(), 1e-9)
	})

	t.Run("WithPrecision", func(t *testing.T) {
		field := crud.NewFloatField("test", crud.WithPrecision(3))
		floatField, err := field.AsFloatField()
		require.NoError(t, err)
		assert.Equal(t, 3, floatField.Precision())
	})

	t.Run("WithFloatStep", func(t *testing.T) {
		field := crud.NewFloatField("test", crud.WithFloatStep(0.5))
		floatField, err := field.AsFloatField()
		require.NoError(t, err)
		assert.InEpsilon(t, 0.5, floatField.Step(), 1e-9)
	})
}

func TestDateFieldOptions(t *testing.T) {
	t.Run("WithMinDate", func(t *testing.T) {
		minDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		field := crud.NewDateField("test", crud.WithMinDate(minDate))
		dateField, err := field.AsDateField()
		require.NoError(t, err)
		assert.Equal(t, minDate, dateField.MinDate())
	})

	t.Run("WithMaxDate", func(t *testing.T) {
		maxDate := time.Date(2030, 12, 31, 0, 0, 0, 0, time.UTC)
		field := crud.NewDateField("test", crud.WithMaxDate(maxDate))
		dateField, err := field.AsDateField()
		require.NoError(t, err)
		assert.Equal(t, maxDate, dateField.MaxDate())
	})

	t.Run("WithFormat", func(t *testing.T) {
		format := "02/01/2006"
		field := crud.NewDateField("test", crud.WithFormat(format))
		dateField, err := field.AsDateField()
		require.NoError(t, err)
		assert.Equal(t, format, dateField.Format())
	})

	t.Run("WithWeekdaysOnly", func(t *testing.T) {
		field := crud.NewDateField("test", crud.WithWeekdaysOnly())
		dateField, err := field.AsDateField()
		require.NoError(t, err)
		assert.True(t, dateField.WeekdaysOnly())
		assert.Len(t, field.Rules(), 1) // Should add validation rule
	})
}

func TestTimeFieldOptions(t *testing.T) {
	t.Run("WithFormat for TimeField", func(t *testing.T) {
		format := "3:04 PM"
		field := crud.NewTimeField("test", crud.WithFormat(format))
		timeField, err := field.AsTimeField()
		require.NoError(t, err)
		assert.Equal(t, format, timeField.Format())
	})
}

func TestDateTimeFieldOptions(t *testing.T) {
	t.Run("WithMinDate for DateTimeField", func(t *testing.T) {
		minDate := time.Date(2020, 1, 1, 10, 30, 0, 0, time.UTC)
		field := crud.NewDateTimeField("test", crud.WithMinDate(minDate))
		dateTimeField, err := field.AsDateTimeField()
		require.NoError(t, err)
		assert.Equal(t, minDate, dateTimeField.MinDateTime())
	})

	t.Run("WithMaxDate for DateTimeField", func(t *testing.T) {
		maxDate := time.Date(2030, 12, 31, 23, 59, 59, 0, time.UTC)
		field := crud.NewDateTimeField("test", crud.WithMaxDate(maxDate))
		dateTimeField, err := field.AsDateTimeField()
		require.NoError(t, err)
		assert.Equal(t, maxDate, dateTimeField.MaxDateTime())
	})

	t.Run("WithTimezone", func(t *testing.T) {
		timezone := "America/New_York"
		field := crud.NewDateTimeField("test", crud.WithTimezone(timezone))
		dateTimeField, err := field.AsDateTimeField()
		require.NoError(t, err)
		assert.Equal(t, timezone, dateTimeField.Timezone())
	})
}

func TestBoolFieldOptions(t *testing.T) {
	t.Run("WithDefaultValue for BoolField", func(t *testing.T) {
		field := crud.NewBoolField("test", crud.WithDefaultValue(true))
		boolField, err := field.AsBoolField()
		require.NoError(t, err)
		assert.True(t, boolField.DefaultValue())
	})

	t.Run("WithTrueLabel", func(t *testing.T) {
		field := crud.NewBoolField("test", crud.WithTrueLabel("Yes"))
		boolField, err := field.AsBoolField()
		require.NoError(t, err)
		assert.Equal(t, "Yes", boolField.TrueLabel())
	})

	t.Run("WithFalseLabel", func(t *testing.T) {
		field := crud.NewBoolField("test", crud.WithFalseLabel("No"))
		boolField, err := field.AsBoolField()
		require.NoError(t, err)
		assert.Equal(t, "No", boolField.FalseLabel())
	})
}

func TestUUIDFieldOptions(t *testing.T) {
	t.Run("WithUUIDVersion", func(t *testing.T) {
		field := crud.NewUUIDField("test", crud.WithUUIDVersion(1))
		uuidField, err := field.AsUUIDField()
		require.NoError(t, err)
		assert.Equal(t, 1, uuidField.Version())
		assert.Len(t, field.Rules(), 1) // Should add validation rule
	})
}

func TestValidationRuleOptions(t *testing.T) {
	t.Run("WithURL", func(t *testing.T) {
		field := crud.NewStringField("test", crud.WithURL())
		assert.Len(t, field.Rules(), 1)

		// Test validation
		validURL := field.Value("https://example.com")
		err := field.Rules()[0](validURL)
		require.NoError(t, err)

		invalidURL := field.Value("not a url")
		err = field.Rules()[0](invalidURL)
		require.Error(t, err)
	})

	t.Run("WithPhone", func(t *testing.T) {
		field := crud.NewStringField("test", crud.WithPhone())
		assert.Len(t, field.Rules(), 1)

		// Test validation
		validPhone := field.Value("+12345678901")
		err := field.Rules()[0](validPhone)
		require.NoError(t, err)

		// Another valid phone without +
		validPhone2 := field.Value("12345678901")
		err = field.Rules()[0](validPhone2)
		require.NoError(t, err)

		// Too short (less than 2 digits)
		invalidPhone := field.Value("1")
		err = field.Rules()[0](invalidPhone)
		require.Error(t, err)

		// Invalid characters
		invalidPhone2 := field.Value("123-456-7890")
		err = field.Rules()[0](invalidPhone2)
		require.Error(t, err)
	})

	t.Run("WithAlpha", func(t *testing.T) {
		field := crud.NewStringField("test", crud.WithAlpha())
		assert.Len(t, field.Rules(), 1)

		// Test validation
		validAlpha := field.Value("abcXYZ")
		err := field.Rules()[0](validAlpha)
		require.NoError(t, err)

		invalidAlpha := field.Value("abc123")
		err = field.Rules()[0](invalidAlpha)
		require.Error(t, err)
	})

	t.Run("WithAlphanumeric", func(t *testing.T) {
		field := crud.NewStringField("test", crud.WithAlphanumeric())
		assert.Len(t, field.Rules(), 1)

		// Test validation
		validAlphanumeric := field.Value("abc123XYZ")
		err := field.Rules()[0](validAlphanumeric)
		require.NoError(t, err)

		invalidAlphanumeric := field.Value("abc-123")
		err = field.Rules()[0](invalidAlphanumeric)
		require.Error(t, err)
	})

	t.Run("WithEmail", func(t *testing.T) {
		field := crud.NewStringField("test", crud.WithEmail())
		assert.Len(t, field.Rules(), 1)

		// Test validation
		validEmail := field.Value("test@example.com")
		err := field.Rules()[0](validEmail)
		require.NoError(t, err)

		invalidEmail := field.Value("not-an-email")
		err = field.Rules()[0](invalidEmail)
		require.Error(t, err)
	})

	t.Run("WithRequired", func(t *testing.T) {
		field := crud.NewStringField("test", crud.WithRequired())
		assert.Len(t, field.Rules(), 1)

		// Test validation
		nonEmpty := field.Value("value")
		err := field.Rules()[0](nonEmpty)
		require.NoError(t, err)

		empty := field.Value("")
		err = field.Rules()[0](empty)
		require.Error(t, err)
	})

	t.Run("WithPositive", func(t *testing.T) {
		field := crud.NewIntField("test", crud.WithPositive())
		assert.Len(t, field.Rules(), 1)

		// Test validation
		positive := field.Value(int64(5))
		err := field.Rules()[0](positive)
		require.NoError(t, err)

		zero := field.Value(int64(0))
		err = field.Rules()[0](zero)
		require.Error(t, err)

		negative := field.Value(int64(-5))
		err = field.Rules()[0](negative)
		require.Error(t, err)
	})

	t.Run("WithNonNegative", func(t *testing.T) {
		field := crud.NewIntField("test", crud.WithNonNegative())
		assert.Len(t, field.Rules(), 1)

		// Test validation
		positive := field.Value(int64(5))
		err := field.Rules()[0](positive)
		require.NoError(t, err)

		zero := field.Value(int64(0))
		err = field.Rules()[0](zero)
		require.NoError(t, err)

		negative := field.Value(int64(-5))
		err = field.Rules()[0](negative)
		require.Error(t, err)
	})

	t.Run("WithNotEmpty", func(t *testing.T) {
		field := crud.NewStringField("test", crud.WithNotEmpty())
		assert.Len(t, field.Rules(), 1)

		// Test validation
		nonEmpty := field.Value("value")
		err := field.Rules()[0](nonEmpty)
		require.NoError(t, err)

		empty := field.Value("")
		err = field.Rules()[0](empty)
		require.Error(t, err)

		whitespace := field.Value("   ")
		err = field.Rules()[0](whitespace)
		require.Error(t, err)
	})

	t.Run("WithFutureDate", func(t *testing.T) {
		field := crud.NewDateField("test", crud.WithFutureDate())
		assert.Len(t, field.Rules(), 1)

		// Test validation
		futureDate := field.Value(time.Now().Add(24 * time.Hour))
		err := field.Rules()[0](futureDate)
		require.NoError(t, err)

		pastDate := field.Value(time.Now().Add(-24 * time.Hour))
		err = field.Rules()[0](pastDate)
		require.Error(t, err)
	})

	t.Run("WithPastDate", func(t *testing.T) {
		field := crud.NewDateField("test", crud.WithPastDate())
		assert.Len(t, field.Rules(), 1)

		// Test validation
		pastDate := field.Value(time.Now().Add(-24 * time.Hour))
		err := field.Rules()[0](pastDate)
		require.NoError(t, err)

		futureDate := field.Value(time.Now().Add(24 * time.Hour))
		err = field.Rules()[0](futureDate)
		require.Error(t, err)
	})
}

func TestMultipleOptions(t *testing.T) {
	t.Run("Combining multiple options", func(t *testing.T) {
		field := crud.NewStringField("email",
			crud.WithKey(),
			crud.WithReadonly(),
			crud.WithSearchable(),
			crud.WithMinLen(5),
			crud.WithMaxLen(100),
			crud.WithPattern(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`),
			crud.WithTrim(),
			crud.WithLowercase(),
			crud.WithEmail(),
			crud.WithRequired(),
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
		field := crud.NewIntField("age",
			crud.WithMin(0),
			crud.WithMax(150),
			crud.WithStep(1),
			crud.WithMultipleOf(1),
			crud.WithNonNegative(),
			crud.WithRequired(),
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

		field := crud.NewDateField("appointment",
			crud.WithMinDate(minDate),
			crud.WithMaxDate(maxDate),
			crud.WithFormat("02/01/2006"),
			crud.WithWeekdaysOnly(),
			crud.WithFutureDate(),
			crud.WithRequired(),
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
		field := crud.NewStringField("test")
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
		field := crud.NewIntField("test")
		intField, err := field.AsIntField()
		require.NoError(t, err)

		assert.Equal(t, int64(-9223372036854775808), intField.Min()) // math.MinInt64
		assert.Equal(t, int64(9223372036854775807), intField.Max())  // math.MaxInt64
		assert.Equal(t, int64(1), intField.Step())
		assert.Equal(t, int64(1), intField.MultipleOf())
	})

	t.Run("FloatField defaults", func(t *testing.T) {
		field := crud.NewFloatField("test")
		floatField, err := field.AsFloatField()
		require.NoError(t, err)

		assert.InDelta(t, -1.7976931348623157e+308, floatField.Min(), 1e300) // -math.MaxFloat64
		assert.InDelta(t, 1.7976931348623157e+308, floatField.Max(), 1e300)  // math.MaxFloat64
		assert.Equal(t, 2, floatField.Precision())
		assert.InEpsilon(t, 0.01, floatField.Step(), 1e-9)
	})

	t.Run("BoolField defaults", func(t *testing.T) {
		field := crud.NewBoolField("test")
		boolField, err := field.AsBoolField()
		require.NoError(t, err)

		assert.False(t, boolField.DefaultValue())
		assert.Empty(t, boolField.TrueLabel())
		assert.Empty(t, boolField.FalseLabel())
	})

	t.Run("DateField defaults", func(t *testing.T) {
		field := crud.NewDateField("test")
		dateField, err := field.AsDateField()
		require.NoError(t, err)

		assert.Equal(t, time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC), dateField.MinDate())
		assert.Equal(t, time.Date(9999, 12, 31, 23, 59, 59, 999999999, time.UTC), dateField.MaxDate())
		assert.Equal(t, "2006-01-02", dateField.Format())
		assert.False(t, dateField.WeekdaysOnly())
	})

	t.Run("TimeField defaults", func(t *testing.T) {
		field := crud.NewTimeField("test")
		timeField, err := field.AsTimeField()
		require.NoError(t, err)

		assert.Equal(t, "15:04:05", timeField.Format())
	})

	t.Run("DateTimeField defaults", func(t *testing.T) {
		field := crud.NewDateTimeField("test")
		dateTimeField, err := field.AsDateTimeField()
		require.NoError(t, err)

		assert.Equal(t, time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC), dateTimeField.MinDateTime())
		assert.Equal(t, time.Date(9999, 12, 31, 23, 59, 59, 999999999, time.UTC), dateTimeField.MaxDateTime())
		assert.Equal(t, "2006-01-02 15:04:05", dateTimeField.Format())
		assert.Equal(t, "UTC", dateTimeField.Timezone())
		assert.False(t, dateTimeField.WeekdaysOnly())
	})

	t.Run("UUIDField defaults", func(t *testing.T) {
		field := crud.NewUUIDField("test")
		uuidField, err := field.AsUUIDField()
		require.NoError(t, err)

		assert.Equal(t, 4, uuidField.Version())
	})
}

func TestOptionValidation(t *testing.T) {
	t.Run("Options create proper field values", func(t *testing.T) {
		// String field with validation
		stringField := crud.NewStringField("username",
			crud.WithMinLen(3),
			crud.WithMaxLen(20),
			crud.WithPattern("^[a-zA-Z0-9_]+$"),
		)

		// Valid value
		validValue := stringField.Value("john_doe")
		for _, rule := range stringField.Rules() {
			err := rule(validValue)
			require.NoError(t, err)
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
		uuidField := crud.NewUUIDField("id", crud.WithUUIDVersion(4))

		// Valid UUID v4
		uuid4 := uuid.New() // Creates v4 by default
		validValue := uuidField.Value(uuid4)
		for _, rule := range uuidField.Rules() {
			err := rule(validValue)
			require.NoError(t, err)
		}
	})

	t.Run("Date field with weekday validation", func(t *testing.T) {
		dateField := crud.NewDateField("workday", crud.WithWeekdaysOnly())

		// Monday (weekday)
		monday := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC) // January 1, 2024 is Monday
		validValue := dateField.Value(monday)
		for _, rule := range dateField.Rules() {
			err := rule(validValue)
			require.NoError(t, err)
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
