package crud

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelectField_Creation(t *testing.T) {
	t.Run("creates select field with default values", func(t *testing.T) {
		field := NewSelectField("status")

		assert.Equal(t, "status", field.Name())
		assert.Equal(t, StringFieldType, field.Type())
		assert.Equal(t, StringFieldType, field.ValueType())
		assert.Equal(t, SelectTypeStatic, field.SelectType())
		assert.Empty(t, field.Options())
		assert.Empty(t, field.Endpoint())
		assert.Empty(t, field.Placeholder())
		assert.False(t, field.Multiple())
		assert.True(t, field.Attrs()["isSelectField"].(bool))
	})
}

func TestSelectField_Options(t *testing.T) {
	t.Run("sets and gets static options", func(t *testing.T) {
		options := []SelectOption{
			{Value: "active", Label: "Active"},
			{Value: "inactive", Label: "Inactive"},
		}

		field := NewSelectField("status").SetOptions(options)

		assert.Equal(t, options, field.Options())
		assert.Equal(t, SelectTypeStatic, field.SelectType())
	})

	t.Run("sets options loader function", func(t *testing.T) {
		expectedOptions := []SelectOption{
			{Value: "1", Label: "Option 1"},
			{Value: "2", Label: "Option 2"},
		}

		loader := func() []SelectOption {
			return expectedOptions
		}

		field := NewSelectField("dynamic").SetOptionsLoader(loader)

		assert.NotNil(t, field.OptionsLoader())
		actualOptions := field.OptionsLoader()()
		assert.Equal(t, expectedOptions, actualOptions)
	})
}

func TestSelectField_ValueTypes(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(SelectField) SelectField
		wantType  FieldType
		wantValue FieldType
	}{
		{
			name:      "default string type",
			setup:     func(f SelectField) SelectField { return f },
			wantType:  StringFieldType,
			wantValue: StringFieldType,
		},
		{
			name:      "int type via SetValueType",
			setup:     func(f SelectField) SelectField { return f.SetValueType(IntFieldType) },
			wantType:  IntFieldType,
			wantValue: IntFieldType,
		},
		{
			name:      "int type via AsIntSelect",
			setup:     func(f SelectField) SelectField { return f.AsIntSelect() },
			wantType:  IntFieldType,
			wantValue: IntFieldType,
		},
		{
			name:      "bool type via AsBoolSelect",
			setup:     func(f SelectField) SelectField { return f.AsBoolSelect() },
			wantType:  BoolFieldType,
			wantValue: BoolFieldType,
		},
		{
			name:      "string type via AsStringSelect",
			setup:     func(f SelectField) SelectField { return f.AsStringSelect() },
			wantType:  StringFieldType,
			wantValue: StringFieldType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := NewSelectField("test")
			field = tt.setup(field)

			assert.Equal(t, tt.wantType, field.Type())
			assert.Equal(t, tt.wantValue, field.ValueType())
		})
	}
}

func TestSelectField_SelectTypes(t *testing.T) {
	t.Run("searchable select via SetEndpoint", func(t *testing.T) {
		field := NewSelectField("product").SetEndpoint("/api/products/search")

		assert.Equal(t, SelectTypeSearchable, field.SelectType())
		assert.Equal(t, "/api/products/search", field.Endpoint())
	})

	t.Run("searchable select via AsSearchable", func(t *testing.T) {
		field := NewSelectField("product").AsSearchable("/api/products/search")

		assert.Equal(t, SelectTypeSearchable, field.SelectType())
		assert.Equal(t, "/api/products/search", field.Endpoint())
	})

	t.Run("combobox via SetMultiple", func(t *testing.T) {
		field := NewSelectField("tags").SetMultiple(true)

		assert.Equal(t, SelectTypeCombobox, field.SelectType())
		assert.True(t, field.Multiple())
	})

	t.Run("combobox via AsCombobox", func(t *testing.T) {
		field := NewSelectField("tags").AsCombobox()

		assert.Equal(t, SelectTypeCombobox, field.SelectType())
		assert.True(t, field.Multiple())
	})

	t.Run("combobox via WithCombobox", func(t *testing.T) {
		field := NewSelectField("tags").WithCombobox("/api/tags", true)

		assert.Equal(t, SelectTypeCombobox, field.SelectType())
		assert.Equal(t, "/api/tags", field.Endpoint())
		assert.True(t, field.Multiple())
	})
}

func TestSelectField_FluentAPI(t *testing.T) {
	t.Run("chained configuration", func(t *testing.T) {
		options := []SelectOption{
			{Value: "1", Label: "One"},
			{Value: "2", Label: "Two"},
		}

		field := NewSelectField("category", WithReadonly()).
			SetValueType(IntFieldType).
			SetPlaceholder("Select category").
			SetOptions(options)

		assert.Equal(t, "category", field.Name())
		assert.Equal(t, IntFieldType, field.ValueType())
		assert.Equal(t, "Select category", field.Placeholder())
		assert.Equal(t, options, field.Options())
		assert.True(t, field.Readonly())
	})

	t.Run("with static options helper", func(t *testing.T) {
		field := NewSelectField("status").
			WithStaticOptions(
				SelectOption{Value: "1", Label: "Active"},
				SelectOption{Value: "0", Label: "Inactive"},
			)

		assert.Equal(t, SelectTypeStatic, field.SelectType())
		assert.Len(t, field.Options(), 2)
		assert.Equal(t, "1", field.Options()[0].Value)
		assert.Equal(t, "Active", field.Options()[0].Label)
	})

	t.Run("with search endpoint helper", func(t *testing.T) {
		field := NewSelectField("user").
			WithSearchEndpoint("/api/users/search")

		assert.Equal(t, SelectTypeSearchable, field.SelectType())
		assert.Equal(t, "/api/users/search", field.Endpoint())
	})
}

func TestSelectField_Placeholder(t *testing.T) {
	t.Run("sets and gets placeholder", func(t *testing.T) {
		field := NewSelectField("country").SetPlaceholder("Choose a country")

		assert.Equal(t, "Choose a country", field.Placeholder())
	})
}

func TestSelectField_ComplexScenarios(t *testing.T) {
	t.Run("searchable select with int values", func(t *testing.T) {
		field := NewSelectField("product_id").
			AsIntSelect().
			AsSearchable("/api/products/search").
			SetPlaceholder("Search products...")

		assert.Equal(t, IntFieldType, field.ValueType())
		assert.Equal(t, SelectTypeSearchable, field.SelectType())
		assert.Equal(t, "/api/products/search", field.Endpoint())
		assert.Equal(t, "Search products...", field.Placeholder())
	})

	t.Run("combobox with dynamic options", func(t *testing.T) {
		loader := func() []SelectOption {
			return []SelectOption{
				{Value: "tag1", Label: "Tag 1"},
				{Value: "tag2", Label: "Tag 2"},
			}
		}

		field := NewSelectField("tags").
			AsCombobox().
			SetOptionsLoader(loader).
			SetPlaceholder("Select tags")

		assert.Equal(t, SelectTypeCombobox, field.SelectType())
		assert.True(t, field.Multiple())
		assert.NotNil(t, field.OptionsLoader())
		assert.Equal(t, "Select tags", field.Placeholder())
	})

	t.Run("boolean select with yes/no options", func(t *testing.T) {
		field := NewSelectField("is_active").
			AsBoolSelect().
			WithStaticOptions(
				SelectOption{Value: "true", Label: "Yes"},
				SelectOption{Value: "false", Label: "No"},
			)

		assert.Equal(t, BoolFieldType, field.ValueType())
		assert.Equal(t, SelectTypeStatic, field.SelectType())
		assert.Len(t, field.Options(), 2)
	})
}

func TestSelectField_EdgeCases(t *testing.T) {
	t.Run("empty endpoint doesn't change select type", func(t *testing.T) {
		field := NewSelectField("test").SetEndpoint("")

		assert.Equal(t, SelectTypeStatic, field.SelectType())
		assert.Empty(t, field.Endpoint())
	})

	t.Run("setting multiple false doesn't change combobox type", func(t *testing.T) {
		field := NewSelectField("test").
			AsCombobox().
			SetMultiple(false)

		assert.Equal(t, SelectTypeCombobox, field.SelectType())
		assert.False(t, field.Multiple())
	})

	t.Run("can override select type after auto-setting", func(t *testing.T) {
		field := NewSelectField("test").
			SetEndpoint("/api/search"). // auto-sets to searchable
			SetSelectType(SelectTypeStatic)

		assert.Equal(t, SelectTypeStatic, field.SelectType())
		assert.Equal(t, "/api/search", field.Endpoint())
	})
}

func TestSelectField_MixedValueTypes(t *testing.T) {
	t.Run("int values in options", func(t *testing.T) {
		field := NewSelectField("priority").
			AsIntSelect().
			WithStaticOptions(
				SelectOption{Value: 1, Label: "Low"},
				SelectOption{Value: 2, Label: "Medium"},
				SelectOption{Value: 3, Label: "High"},
			)

		assert.Equal(t, IntFieldType, field.Type())
		assert.Equal(t, IntFieldType, field.ValueType())

		// Check options
		options := field.Options()
		assert.Len(t, options, 3)
		assert.Equal(t, 1, options[0].Value)
		assert.Equal(t, "Low", options[0].Label)
	})

	t.Run("bool values in options", func(t *testing.T) {
		field := NewSelectField("enabled").
			AsBoolSelect().
			WithStaticOptions(
				SelectOption{Value: true, Label: "Yes"},
				SelectOption{Value: false, Label: "No"},
			)

		assert.Equal(t, BoolFieldType, field.Type())
		assert.Equal(t, BoolFieldType, field.ValueType())

		// Check options
		options := field.Options()
		assert.Len(t, options, 2)
		assert.Equal(t, true, options[0].Value)
		assert.Equal(t, "Yes", options[0].Label)
	})

	t.Run("field value handling with correct types", func(t *testing.T) {
		// Int field should accept int values
		intField := NewSelectField("level").AsIntSelect()
		intValue := intField.Value(5)
		val, err := intValue.AsInt()
		require.NoError(t, err)
		assert.Equal(t, 5, val)

		// Bool field should accept bool values
		boolField := NewSelectField("active").AsBoolSelect()
		boolValue := boolField.Value(true)
		bval, err := boolValue.AsBool()
		require.NoError(t, err)
		assert.True(t, bval)
	})
}
