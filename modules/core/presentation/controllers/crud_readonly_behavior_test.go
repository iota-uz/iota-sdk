package controllers_test

import (
	"context"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/crud"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReadonlyFieldBehavior demonstrates how readonly fields work in the CRUD system
func TestReadonlyFieldBehavior(t *testing.T) {
	t.Run("Field with WithReadonly option should be marked as readonly", func(t *testing.T) {
		// Create a field with readonly option
		field := crud.NewStringField("test_field", crud.WithReadonly())

		// Verify it's marked as readonly
		assert.True(t, field.Readonly(), "Field should be marked as readonly")
	})

	t.Run("Different field types can be readonly", func(t *testing.T) {
		testCases := []struct {
			name  string
			field crud.Field
		}{
			{
				name:  "String field",
				field: crud.NewStringField("str", crud.WithReadonly()),
			},
			{
				name:  "Int field",
				field: crud.NewIntField("num", crud.WithReadonly()),
			},
			{
				name:  "Bool field",
				field: crud.NewBoolField("flag", crud.WithReadonly()),
			},
			{
				name:  "DateTime field",
				field: crud.NewDateTimeField("created", crud.WithReadonly()),
			},
			{
				name:  "Select field",
				field: crud.NewSelectField("status", crud.WithReadonly()),
			},
			{
				name:  "Timestamp field",
				field: crud.NewTimestampField("timestamp", crud.WithReadonly()),
			},
			{
				name:  "UUIDField field",
				field: crud.NewUUIDField("uuid", crud.WithReadonly()),
			},
			{
				name:  "Date field",
				field: crud.NewDateField("date", crud.WithReadonly()),
			},
			{
				name:  "Float field",
				field: crud.NewFloatField("float", crud.WithReadonly()),
			},
			{
				name:  "Decimal field",
				field: crud.NewDecimalField("decimal", crud.WithReadonly()),
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				assert.True(t, tc.field.Readonly(), "%s should be readonly", tc.name)
			})
		}
	})

	t.Run("Readonly fields can have initial values", func(t *testing.T) {
		now := time.Now()
		field := crud.NewDateTimeField("created_at",
			crud.WithReadonly(),
			crud.WithInitialValue(func(ctx context.Context) any {
				return now
			}),
		)

		assert.True(t, field.Readonly())
		assert.Equal(t, now, field.InitialValue(context.Background()))
	})

	t.Run("Fields without WithReadonly are editable", func(t *testing.T) {
		field := crud.NewStringField("editable_field")
		assert.False(t, field.Readonly(), "Field without WithReadonly should be editable")
	})

	t.Run("Key fields with readonly are still readonly", func(t *testing.T) {
		field := crud.NewStringField("id",
			crud.WithKey(),
			crud.WithReadonly(),
		)

		assert.True(t, field.Key())
		assert.True(t, field.Readonly())
	})

	t.Run("Schema preserves readonly status", func(t *testing.T) {
		fields := crud.NewFields([]crud.Field{
			crud.NewStringField("id", crud.WithKey()),
			crud.NewStringField("name"),
			crud.NewStringField("status", crud.WithReadonly()),
			crud.NewDateTimeField("created_at", crud.WithReadonly()),
		})

		// Check each field
		idField, err := fields.Field("id")
		require.NoError(t, err)
		assert.False(t, idField.Readonly(), "id should not be readonly")

		nameField, err := fields.Field("name")
		require.NoError(t, err)
		assert.False(t, nameField.Readonly(), "name should not be readonly")

		statusField, err := fields.Field("status")
		require.NoError(t, err)
		assert.True(t, statusField.Readonly(), "status should be readonly")

		createdField, err := fields.Field("created_at")
		require.NoError(t, err)
		assert.True(t, createdField.Readonly(), "created_at should be readonly")
	})
}

// TestReadonlyFieldUsageExample shows example usage of readonly fields
func TestReadonlyFieldUsageExample(t *testing.T) {
	// Example: Creating a typical entity schema with readonly audit fields
	fields := crud.NewFields([]crud.Field{
		// Primary key - often readonly after creation
		crud.NewStringField("id", crud.WithKey()),

		// Editable fields
		crud.NewStringField("name", crud.WithRequired()),
		crud.NewStringField("description"),
		crud.NewIntField("quantity"),

		// Readonly status field - set by system
		crud.NewStringField("status",
			crud.WithReadonly(),
			crud.WithInitialValue(func(ctx context.Context) any { return "pending" }),
		),

		// Readonly audit fields
		crud.NewDateTimeField("created_at",
			crud.WithReadonly(),
			crud.WithInitialValue(func(ctx context.Context) any { return time.Now() }),
		),
		crud.NewDateTimeField("updated_at",
			crud.WithReadonly(),
			crud.WithInitialValue(func(ctx context.Context) any { return time.Now() }),
		),
		crud.NewStringField("created_by", crud.WithReadonly()),
		crud.NewStringField("updated_by", crud.WithReadonly()),
	})

	// Verify the readonly fields
	readonlyFields := []string{"status", "created_at", "updated_at", "created_by", "updated_by"}
	for _, fieldName := range readonlyFields {
		field, err := fields.Field(fieldName)
		require.NoError(t, err)
		assert.True(t, field.Readonly(), "%s should be readonly", fieldName)
	}

	// Verify editable fields
	editableFields := []string{"name", "description", "quantity"}
	for _, fieldName := range editableFields {
		field, err := fields.Field(fieldName)
		require.NoError(t, err)
		assert.False(t, field.Readonly(), "%s should be editable", fieldName)
	}
}
