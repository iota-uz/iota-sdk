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
		field := crud.NewField("id", crud.UUIDFieldType, crud.WithKey(true))

		assert.Equal(t, "id", field.Name())
		assert.Equal(t, crud.UUIDFieldType, field.Type())
		assert.True(t, field.Key())
	})

	t.Run("validates UUID values correctly", func(t *testing.T) {
		field := crud.NewField("id", crud.UUIDFieldType)
		testUUID := uuid.New()

		// Should not panic with valid UUID
		require.NotPanics(t, func() {
			fieldValue := field.Value(testUUID)
			assert.Equal(t, testUUID, fieldValue.Value())
		})
	})

	t.Run("panics with invalid UUID value", func(t *testing.T) {
		field := crud.NewField("id", crud.UUIDFieldType)

		// Should panic with invalid type
		require.Panics(t, func() {
			field.Value("not-a-uuid")
		})

		require.Panics(t, func() {
			field.Value(123)
		})
	})
}

func TestValidationRules(t *testing.T) {
	t.Run("RequiredRule", func(t *testing.T) {
		field := crud.NewField("name", crud.StringFieldType, crud.WithRule(crud.RequiredRule()))
		
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
		field := crud.NewField("name", crud.StringFieldType, crud.WithRule(crud.MinLengthRule(5)))
		
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
		field := crud.NewField("name", crud.StringFieldType, crud.WithRule(crud.MaxLengthRule(5)))
		
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
		field := crud.NewField("email", crud.StringFieldType, crud.WithRule(crud.EmailRule()))
		
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
		field := crud.NewField("age", crud.IntFieldType, crud.WithRule(crud.MinValueRule(18)))
		
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
		field := crud.NewField("score", crud.IntFieldType, crud.WithRule(crud.MaxValueRule(100)))
		
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
		field := crud.NewField("amount", crud.IntFieldType, crud.WithRule(crud.PositiveRule()))
		
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
		field := crud.NewField("count", crud.IntFieldType, crud.WithRule(crud.NonNegativeRule()))
		
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
		field := crud.NewField("status", crud.StringFieldType, crud.WithRule(crud.InRule("active", "inactive", "pending")))
		
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
		field := crud.NewField("description", crud.StringFieldType, crud.WithRule(crud.NotEmptyRule()))
		
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
		field := crud.NewField("expiry", crud.DateTimeFieldType, crud.WithRule(crud.FutureDateRule()))
		
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
		field := crud.NewField("birthdate", crud.DateTimeFieldType, crud.WithRule(crud.PastDateRule()))
		
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
		field := crud.NewField("phone", crud.StringFieldType, crud.WithRule(crud.PatternRule(`^\+?[1-9]\d{1,14}$`)))
		
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
