package crud_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/crud"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecimalField(t *testing.T) {
	t.Run("creates decimal field correctly", func(t *testing.T) {
		field := crud.NewDecimalField("price")

		assert.Equal(t, "price", field.Name())
		assert.Equal(t, crud.DecimalFieldType, field.Type())
		assert.Equal(t, 10, field.Precision())
		assert.Equal(t, 2, field.Scale())
	})

	t.Run("accepts precision and scale options", func(t *testing.T) {
		field := crud.NewDecimalField("commission",
			crud.WithPrecision(5),
			crud.WithScale(3),
		)

		assert.Equal(t, 5, field.Precision())
		assert.Equal(t, 3, field.Scale())
	})

	t.Run("accepts min and max options", func(t *testing.T) {
		field := crud.NewDecimalField("amount",
			crud.WithDecimalMin("0.00"),
			crud.WithDecimalMax("999999.99"),
		)

		assert.Equal(t, "0.00", field.Min())
		assert.Equal(t, "999999.99", field.Max())
	})

	t.Run("validates decimal values correctly", func(t *testing.T) {
		field := crud.NewDecimalField("price")

		require.NotPanics(t, func() {
			fieldValue := field.Value("123.45")
			assert.Equal(t, "123.45", fieldValue.Value())
		})

		require.NotPanics(t, func() {
			fieldValue := field.Value(123.45)
			assert.Equal(t, 123.45, fieldValue.Value())
		})
	})

	t.Run("can cast to DecimalField", func(t *testing.T) {
		field := crud.NewDecimalField("price")

		decimalField, err := field.AsDecimalField()
		require.NoError(t, err)
		assert.NotNil(t, decimalField)
		assert.Equal(t, "price", decimalField.Name())
	})

	t.Run("fails casting to other field types", func(t *testing.T) {
		field := crud.NewDecimalField("price")

		_, err := field.AsStringField()
		require.Error(t, err)
		require.ErrorIs(t, err, crud.ErrFieldTypeMismatch)

		_, err = field.AsIntField()
		require.Error(t, err)
		require.ErrorIs(t, err, crud.ErrFieldTypeMismatch)

		_, err = field.AsFloatField()
		require.Error(t, err)
		require.ErrorIs(t, err, crud.ErrFieldTypeMismatch)
	})
}

func TestDecimalFieldValue(t *testing.T) {
	t.Run("AsDecimal returns string representation", func(t *testing.T) {
		field := crud.NewDecimalField("price")

		fieldValue := field.Value("123.45")
		decimal, err := fieldValue.AsDecimal()
		require.NoError(t, err)
		assert.Equal(t, "123.45", decimal)

		fieldValue2 := field.Value(123.45)
		decimal2, err := fieldValue2.AsDecimal()
		require.NoError(t, err)
		assert.Equal(t, "123.45", decimal2)
	})

	t.Run("AsDecimal fails for non-decimal fields", func(t *testing.T) {
		stringField := crud.NewStringField("name")
		fieldValue := stringField.Value("test")

		_, err := fieldValue.AsDecimal()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "field 'name' has type 'string', expected 'decimal'")
	})
}
