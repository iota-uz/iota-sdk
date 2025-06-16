package crud_test

import (
	"github.com/iota-uz/iota-sdk/pkg/crud"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUUIDField(t *testing.T) {
	t.Run("creates UUID field correctly", func(t *testing.T) {
		field := crud.NewUUIDField("id", crud.WithKey(true))

		assert.Equal(t, "id", field.Name())
		assert.Equal(t, crud.UUIDFieldType, field.Type())
		assert.True(t, field.Key())
	})

	t.Run("validates UUID values correctly", func(t *testing.T) {
		field := crud.NewUUIDField("id")
		testUUID := uuid.New()

		// Should not panic with valid UUID
		require.NotPanics(t, func() {
			fieldValue := field.Value(testUUID)
			assert.Equal(t, testUUID, fieldValue.Value())
		})
	})

	t.Run("panics with invalid UUID value", func(t *testing.T) {
		field := crud.NewUUIDField("id")

		// Should panic with invalid type
		require.Panics(t, func() {
			field.Value("not-a-uuid")
		})

		require.Panics(t, func() {
			field.Value(123)
		})
	})
}

func TestFieldCasting(t *testing.T) {
	t.Run("StringField casting", func(t *testing.T) {
		field := crud.NewStringField("name")

		// Should cast to StringField successfully
		stringField, err := field.AsStringField()
		assert.NoError(t, err)
		assert.NotNil(t, stringField)
		assert.Equal(t, "name", stringField.Name())

		// Should fail casting to other types
		_, err = field.AsIntField()
		assert.Error(t, err)
		assert.ErrorIs(t, err, crud.ErrFieldTypeMismatch)
		assert.Contains(t, err.Error(), "field \"name\" is string, not int")

		_, err = field.AsBoolField()
		assert.Error(t, err)
		assert.ErrorIs(t, err, crud.ErrFieldTypeMismatch)

		_, err = field.AsFloatField()
		assert.Error(t, err)
		assert.ErrorIs(t, err, crud.ErrFieldTypeMismatch)

		_, err = field.AsDateField()
		assert.Error(t, err)
		assert.ErrorIs(t, err, crud.ErrFieldTypeMismatch)

		_, err = field.AsTimeField()
		assert.Error(t, err)
		assert.ErrorIs(t, err, crud.ErrFieldTypeMismatch)

		_, err = field.AsDateTimeField()
		assert.Error(t, err)
		assert.ErrorIs(t, err, crud.ErrFieldTypeMismatch)

		_, err = field.AsTimestampField()
		assert.Error(t, err)
		assert.ErrorIs(t, err, crud.ErrFieldTypeMismatch)

		_, err = field.AsUUIDField()
		assert.Error(t, err)
		assert.ErrorIs(t, err, crud.ErrFieldTypeMismatch)
	})

	t.Run("IntField casting", func(t *testing.T) {
		field := crud.NewIntField("age")

		// Should cast to IntField successfully
		intField, err := field.AsIntField()
		assert.NoError(t, err)
		assert.NotNil(t, intField)
		assert.Equal(t, "age", intField.Name())

		// Should fail casting to other types
		_, err = field.AsStringField()
		assert.Error(t, err)
		assert.ErrorIs(t, err, crud.ErrFieldTypeMismatch)

		_, err = field.AsBoolField()
		assert.Error(t, err)
		assert.ErrorIs(t, err, crud.ErrFieldTypeMismatch)
	})

	t.Run("BoolField casting", func(t *testing.T) {
		field := crud.NewBoolField("active")

		// Should cast to BoolField successfully
		boolField, err := field.AsBoolField()
		assert.NoError(t, err)
		assert.NotNil(t, boolField)
		assert.Equal(t, "active", boolField.Name())

		// Should fail casting to other types
		_, err = field.AsStringField()
		assert.Error(t, err)
		assert.ErrorIs(t, err, crud.ErrFieldTypeMismatch)

		_, err = field.AsIntField()
		assert.Error(t, err)
		assert.ErrorIs(t, err, crud.ErrFieldTypeMismatch)
	})

	t.Run("FloatField casting", func(t *testing.T) {
		field := crud.NewFloatField("price")

		// Should cast to FloatField successfully
		floatField, err := field.AsFloatField()
		assert.NoError(t, err)
		assert.NotNil(t, floatField)
		assert.Equal(t, "price", floatField.Name())
	})

	t.Run("DateField casting", func(t *testing.T) {
		field := crud.NewDateField("birthdate")

		// Should cast to DateField successfully
		dateField, err := field.AsDateField()
		assert.NoError(t, err)
		assert.NotNil(t, dateField)
		assert.Equal(t, "birthdate", dateField.Name())
	})

	t.Run("TimeField casting", func(t *testing.T) {
		field := crud.NewTimeField("start_time")

		// Should cast to TimeField successfully
		timeField, err := field.AsTimeField()
		assert.NoError(t, err)
		assert.NotNil(t, timeField)
		assert.Equal(t, "start_time", timeField.Name())
	})

	t.Run("DateTimeField casting", func(t *testing.T) {
		field := crud.NewDateTimeField("created_at")

		// Should cast to DateTimeField successfully
		dateTimeField, err := field.AsDateTimeField()
		assert.NoError(t, err)
		assert.NotNil(t, dateTimeField)
		assert.Equal(t, "created_at", dateTimeField.Name())
	})

	t.Run("TimestampField casting", func(t *testing.T) {
		field := crud.NewTimestampField("updated_at")

		// Should cast to TimestampField successfully
		timestampField, err := field.AsTimestampField()
		assert.NoError(t, err)
		assert.NotNil(t, timestampField)
		assert.Equal(t, "updated_at", timestampField.Name())
	})

	t.Run("UUIDField casting", func(t *testing.T) {
		field := crud.NewUUIDField("id")

		// Should cast to UUIDField successfully
		uuidField, err := field.AsUUIDField()
		assert.NoError(t, err)
		assert.NotNil(t, uuidField)
		assert.Equal(t, "id", uuidField.Name())
	})

	t.Run("Casting through Field interface", func(t *testing.T) {
		// Test that casting works when field is referenced through Field interface
		var field crud.Field = crud.NewStringField("description", crud.WithMaxLen(500))

		// Should cast to StringField successfully
		stringField, err := field.AsStringField()
		assert.NoError(t, err)
		assert.NotNil(t, stringField)
		assert.Equal(t, 500, stringField.MaxLen())

		// Test with IntField
		field = crud.NewIntField("count", crud.WithMin(0), crud.WithMax(100))
		intField, err := field.AsIntField()
		assert.NoError(t, err)
		assert.NotNil(t, intField)
		assert.Equal(t, int64(0), intField.Min())
		assert.Equal(t, int64(100), intField.Max())
	})
}

func TestValidationRules(t *testing.T) {
	t.Run("RequiredRule", func(t *testing.T) {
		field := crud.NewStringField("name", crud.WithRule(crud.RequiredRule()))

		// Should fail for empty string
		fieldValue := field.Value("")
		err := field.Rules()[0](fieldValue)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "is required")

		// Should pass for non-empty string
		fieldValue = field.Value("test")
		err = field.Rules()[0](fieldValue)
		assert.NoError(t, err)
	})

	t.Run("MinLengthRule", func(t *testing.T) {
		field := crud.NewStringField("name", crud.WithRule(crud.MinLengthRule(5)))

		// Should fail for short string
		fieldValue := field.Value("abc")
		err := field.Rules()[0](fieldValue)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be at least 5 characters")

		// Should pass for long enough string
		fieldValue = field.Value("abcdef")
		err = field.Rules()[0](fieldValue)
		assert.NoError(t, err)
	})

	t.Run("MaxLengthRule", func(t *testing.T) {
		field := crud.NewStringField("name", crud.WithRule(crud.MaxLengthRule(5)))

		// Should fail for long string
		fieldValue := field.Value("abcdefgh")
		err := field.Rules()[0](fieldValue)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be at most 5 characters")

		// Should pass for short enough string
		fieldValue = field.Value("abc")
		err = field.Rules()[0](fieldValue)
		assert.NoError(t, err)
	})

	t.Run("EmailRule", func(t *testing.T) {
		field := crud.NewStringField("email", crud.WithRule(crud.EmailRule()))

		// Should fail for invalid email
		fieldValue := field.Value("invalid-email")
		err := field.Rules()[0](fieldValue)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be a valid email")

		// Should pass for valid email
		fieldValue = field.Value("test@example.com")
		err = field.Rules()[0](fieldValue)
		assert.NoError(t, err)
	})

	t.Run("MinValueRule", func(t *testing.T) {
		field := crud.NewIntField("age", crud.WithRule(crud.MinValueRule(18)))

		// Should fail for value below minimum
		fieldValue := field.Value(16)
		err := field.Rules()[0](fieldValue)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be at least 18")

		// Should pass for value above minimum
		fieldValue = field.Value(25)
		err = field.Rules()[0](fieldValue)
		assert.NoError(t, err)
	})

	t.Run("MaxValueRule", func(t *testing.T) {
		field := crud.NewIntField("score", crud.WithRule(crud.MaxValueRule(100)))

		// Should fail for value above maximum
		fieldValue := field.Value(150)
		err := field.Rules()[0](fieldValue)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be at most 100")

		// Should pass for value below maximum
		fieldValue = field.Value(85)
		err = field.Rules()[0](fieldValue)
		assert.NoError(t, err)
	})

	t.Run("PositiveRule", func(t *testing.T) {
		field := crud.NewIntField("amount", crud.WithRule(crud.PositiveRule()))

		// Should fail for zero and negative values
		fieldValue := field.Value(0)
		err := field.Rules()[0](fieldValue)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be positive")

		fieldValue = field.Value(-5)
		err = field.Rules()[0](fieldValue)
		assert.Error(t, err)

		// Should pass for positive value
		fieldValue = field.Value(10)
		err = field.Rules()[0](fieldValue)
		assert.NoError(t, err)
	})

	t.Run("NonNegativeRule", func(t *testing.T) {
		field := crud.NewIntField("count", crud.WithRule(crud.NonNegativeRule()))

		// Should fail for negative values
		fieldValue := field.Value(-1)
		err := field.Rules()[0](fieldValue)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be non-negative")

		// Should pass for zero and positive values
		fieldValue = field.Value(0)
		err = field.Rules()[0](fieldValue)
		assert.NoError(t, err)

		fieldValue = field.Value(5)
		err = field.Rules()[0](fieldValue)
		assert.NoError(t, err)
	})

	t.Run("InRule", func(t *testing.T) {
		field := crud.NewStringField("status", crud.WithRule(crud.InRule("active", "inactive", "pending")))

		// Should fail for value not in allowed list
		fieldValue := field.Value("unknown")
		err := field.Rules()[0](fieldValue)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be one of")

		// Should pass for value in allowed list
		fieldValue = field.Value("active")
		err = field.Rules()[0](fieldValue)
		assert.NoError(t, err)
	})

	t.Run("NotEmptyRule", func(t *testing.T) {
		field := crud.NewStringField("description", crud.WithRule(crud.NotEmptyRule()))

		// Should fail for empty string and whitespace
		fieldValue := field.Value("")
		err := field.Rules()[0](fieldValue)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be empty")

		fieldValue = field.Value("   ")
		err = field.Rules()[0](fieldValue)
		assert.Error(t, err)

		// Should pass for non-empty string
		fieldValue = field.Value("test description")
		err = field.Rules()[0](fieldValue)
		assert.NoError(t, err)
	})

	t.Run("FutureDateRule", func(t *testing.T) {
		field := crud.NewDateTimeField("expiry", crud.WithRule(crud.FutureDateRule()))

		// Should fail for past date
		pastDate := time.Now().Add(-24 * time.Hour)
		fieldValue := field.Value(pastDate)
		err := field.Rules()[0](fieldValue)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be a future date")

		// Should pass for future date
		futureDate := time.Now().Add(24 * time.Hour)
		fieldValue = field.Value(futureDate)
		err = field.Rules()[0](fieldValue)
		assert.NoError(t, err)
	})

	t.Run("PastDateRule", func(t *testing.T) {
		field := crud.NewDateTimeField("birthdate", crud.WithRule(crud.PastDateRule()))

		// Should fail for future date
		futureDate := time.Now().Add(24 * time.Hour)
		fieldValue := field.Value(futureDate)
		err := field.Rules()[0](fieldValue)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be a past date")

		// Should pass for past date
		pastDate := time.Now().Add(-24 * time.Hour)
		fieldValue = field.Value(pastDate)
		err = field.Rules()[0](fieldValue)
		assert.NoError(t, err)
	})

	t.Run("PatternRule", func(t *testing.T) {
		// Phone number pattern
		field := crud.NewStringField("phone", crud.WithRule(crud.PatternRule(`^\+?[1-9]\d{1,14}$`)))

		// Should fail for invalid pattern
		fieldValue := field.Value("invalid-phone")
		err := field.Rules()[0](fieldValue)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must match pattern")

		// Should pass for valid pattern
		fieldValue = field.Value("+1234567890")
		err = field.Rules()[0](fieldValue)
		assert.NoError(t, err)
	})
}
