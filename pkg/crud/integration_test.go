package crud_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/crud"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEntity represents a test entity with select fields
type TestEntity struct {
	ID         string
	Status     string
	CategoryID int
	IsActive   bool
}

// TestSchema implements crud.Schema for testing
type TestSchema struct {
	fields crud.Fields
}

func NewTestSchema() *TestSchema {
	// Create fields array
	fieldList := []crud.Field{
		// Add ID field
		crud.NewStringField("ID", crud.WithKey()),

		// Add status select field with string values
		crud.NewSelectField("Status").
			WithStaticOptions(
				crud.SelectOption{Value: "active", Label: "Active"},
				crud.SelectOption{Value: "inactive", Label: "Inactive"},
				crud.SelectOption{Value: "pending", Label: "Pending"},
			).
			SetPlaceholder("Select status"),

		// Add category select field with int values
		crud.NewSelectField("CategoryID").
			AsIntSelect().
			WithStaticOptions(
				crud.SelectOption{Value: 1, Label: "Electronics"},
				crud.SelectOption{Value: 2, Label: "Clothing"},
				crud.SelectOption{Value: 3, Label: "Food"},
			).
			SetPlaceholder("Select category"),

		// Add boolean select field
		crud.NewSelectField("IsActive").
			AsBoolSelect().
			WithStaticOptions(
				crud.SelectOption{Value: true, Label: "Yes"},
				crud.SelectOption{Value: false, Label: "No"},
			),
	}

	fields := crud.NewFields(fieldList)
	return &TestSchema{fields: fields}
}

func (s *TestSchema) Name() string {
	return "TestEntity"
}

func (s *TestSchema) Fields() crud.Fields {
	return s.fields
}

func TestSelectField_Integration(t *testing.T) {
	t.Run("static select field renders correctly", func(t *testing.T) {
		schema := NewTestSchema()
		statusField, err := schema.Fields().Field("Status")
		require.NoError(t, err)

		selectField, ok := statusField.(crud.SelectField)
		require.True(t, ok, "Status field should be a SelectField")

		assert.Equal(t, crud.SelectTypeStatic, selectField.SelectType())
		assert.Len(t, selectField.Options(), 3)
		assert.Equal(t, "Select status", selectField.Placeholder())
	})

	t.Run("int select field parses values correctly", func(t *testing.T) {
		schema := NewTestSchema()
		categoryField, err := schema.Fields().Field("CategoryID")
		require.NoError(t, err)

		selectField, ok := categoryField.(crud.SelectField)
		require.True(t, ok, "CategoryID field should be a SelectField")

		assert.Equal(t, crud.IntFieldType, selectField.ValueType())

		// Test value parsing
		fieldValue := categoryField.Value(2)
		assert.NotNil(t, fieldValue)

		intVal, err := fieldValue.AsInt()
		require.NoError(t, err)
		assert.Equal(t, 2, intVal)
	})

	t.Run("bool select field handles values correctly", func(t *testing.T) {
		schema := NewTestSchema()
		activeField, err := schema.Fields().Field("IsActive")
		require.NoError(t, err)

		selectField, ok := activeField.(crud.SelectField)
		require.True(t, ok, "IsActive field should be a SelectField")

		assert.Equal(t, crud.BoolFieldType, selectField.ValueType())

		// Test value parsing
		fieldValue := activeField.Value(true)
		assert.NotNil(t, fieldValue)

		boolVal, err := fieldValue.AsBool()
		require.NoError(t, err)
		assert.True(t, boolVal)
	})
}

func TestSelectField_FormSubmission(t *testing.T) {
	t.Run("form submission with select fields", func(t *testing.T) {
		// Create form data
		form := url.Values{}
		form.Set("Status", "active")
		form.Set("CategoryID", "2")
		form.Set("IsActive", "true")

		// Create request with form data
		req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		// Parse form and extract values
		err := req.ParseForm()
		require.NoError(t, err)

		schema := NewTestSchema()

		// Test Status field value extraction
		statusValue := req.Form.Get("Status")
		assert.Equal(t, "active", statusValue)

		// Test CategoryID field value extraction and conversion
		categoryField, _ := schema.Fields().Field("CategoryID")
		categoryValue := req.Form.Get("CategoryID")
		assert.Equal(t, "2", categoryValue)

		// Verify the field handles the value correctly
		fieldValue := categoryField.Value(2) // Simulating parsed int value
		intVal, err := fieldValue.AsInt()
		require.NoError(t, err)
		assert.Equal(t, 2, intVal)

		// Test IsActive field value extraction
		activeField, _ := schema.Fields().Field("IsActive")
		activeValue := req.Form.Get("IsActive")
		assert.Equal(t, "true", activeValue)

		// Verify the field handles the value correctly
		boolFieldValue := activeField.Value(true) // Simulating parsed bool value
		boolVal, err := boolFieldValue.AsBool()
		require.NoError(t, err)
		assert.True(t, boolVal)
	})
}

func TestSelectField_DynamicOptions(t *testing.T) {
	t.Run("select field with options loader", func(t *testing.T) {
		// Create a select field with dynamic options
		dynamicOptions := []crud.SelectOption{
			{Value: "opt1", Label: "Option 1"},
			{Value: "opt2", Label: "Option 2"},
		}

		field := crud.NewSelectField("DynamicField").
			SetOptionsLoader(func(ctx context.Context) []crud.SelectOption {
				// Simulate loading options from a service
				return dynamicOptions
			})

		// Options should be nil initially
		assert.Nil(t, field.Options())

		// But loader should return options
		loader := field.OptionsLoader()
		require.NotNil(t, loader)

		options := loader(context.Background())
		assert.Equal(t, dynamicOptions, options)
	})
}

func TestSelectField_SearchableSelect(t *testing.T) {
	t.Run("searchable select configuration", func(t *testing.T) {
		field := crud.NewSelectField("ProductID").
			AsIntSelect().
			AsSearchable("/api/products/search").
			SetPlaceholder("Search products...")

		assert.Equal(t, crud.SelectTypeSearchable, field.SelectType())
		assert.Equal(t, "/api/products/search", field.Endpoint())
		assert.Equal(t, "Search products...", field.Placeholder())
		assert.Equal(t, crud.IntFieldType, field.ValueType())
	})
}

func TestSelectField_ComboboxSelect(t *testing.T) {
	t.Run("combobox configuration", func(t *testing.T) {
		field := crud.NewSelectField("Tags").
			WithCombobox("/api/tags", true).
			SetPlaceholder("Select tags")

		assert.Equal(t, crud.SelectTypeCombobox, field.SelectType())
		assert.Equal(t, "/api/tags", field.Endpoint())
		assert.True(t, field.Multiple())
		assert.Equal(t, "Select tags", field.Placeholder())
	})
}
