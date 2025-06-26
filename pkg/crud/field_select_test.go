package crud

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelectField_Creation(t *testing.T) {
	t.Run("creates select field with default values", func(t *testing.T) {
		field := NewSelectField("status")
		selectField, ok := field.(SelectField)
		require.True(t, ok, "should implement SelectField interface")

		assert.Equal(t, "status", selectField.Name())
		assert.Equal(t, StringFieldType, selectField.Type())
		assert.Equal(t, StringFieldType, selectField.ValueType())
		assert.Equal(t, SelectTypeStatic, selectField.SelectType())
		assert.Empty(t, selectField.Options())
		assert.Empty(t, selectField.Endpoint())
		assert.Empty(t, selectField.Placeholder())
		assert.False(t, selectField.Multiple())
		assert.True(t, selectField.Attrs()["isSelectField"].(bool))
	})
}

func TestSelectField_Options(t *testing.T) {
	t.Run("sets and gets static options", func(t *testing.T) {
		options := []SelectOption{
			{Value: "active", Label: "Active"},
			{Value: "inactive", Label: "Inactive"},
		}

		field := NewSelectField("status").SetOptions(options)
		selectField := field.(SelectField)

		assert.Equal(t, options, selectField.Options())
		assert.Equal(t, SelectTypeStatic, selectField.SelectType())
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
		selectField := field.(SelectField)

		assert.NotNil(t, selectField.OptionsLoader())
		actualOptions := selectField.OptionsLoader()()
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
			selectField := tt.setup(field.(SelectField))

			assert.Equal(t, tt.wantType, selectField.Type())
			assert.Equal(t, tt.wantValue, selectField.ValueType())
		})
	}
}

func TestSelectField_SelectTypes(t *testing.T) {
	t.Run("searchable select via SetEndpoint", func(t *testing.T) {
		field := NewSelectField("product").SetEndpoint("/api/products/search")
		selectField := field.(SelectField)

		assert.Equal(t, SelectTypeSearchable, selectField.SelectType())
		assert.Equal(t, "/api/products/search", selectField.Endpoint())
	})

	t.Run("searchable select via AsSearchable", func(t *testing.T) {
		field := NewSelectField("product").(SelectField).AsSearchable("/api/products/search")

		assert.Equal(t, SelectTypeSearchable, field.SelectType())
		assert.Equal(t, "/api/products/search", field.Endpoint())
	})

	t.Run("combobox via SetMultiple", func(t *testing.T) {
		field := NewSelectField("tags").SetMultiple(true)
		selectField := field.(SelectField)

		assert.Equal(t, SelectTypeCombobox, selectField.SelectType())
		assert.True(t, selectField.Multiple())
	})

	t.Run("combobox via AsCombobox", func(t *testing.T) {
		field := NewSelectField("tags").(SelectField).AsCombobox()

		assert.Equal(t, SelectTypeCombobox, field.SelectType())
		assert.True(t, field.Multiple())
	})

	t.Run("combobox via WithCombobox", func(t *testing.T) {
		field := NewSelectField("tags").(SelectField).WithCombobox("/api/tags", true)

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

		selectField := field.(SelectField)

		assert.Equal(t, "category", selectField.Name())
		assert.Equal(t, IntFieldType, selectField.ValueType())
		assert.Equal(t, "Select category", selectField.Placeholder())
		assert.Equal(t, options, selectField.Options())
		assert.True(t, selectField.Readonly())
	})

	t.Run("with static options helper", func(t *testing.T) {
		field := NewSelectField("status").(SelectField).
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
		field := NewSelectField("user").(SelectField).
			WithSearchEndpoint("/api/users/search")

		assert.Equal(t, SelectTypeSearchable, field.SelectType())
		assert.Equal(t, "/api/users/search", field.Endpoint())
	})
}

func TestSelectField_Placeholder(t *testing.T) {
	t.Run("sets and gets placeholder", func(t *testing.T) {
		field := NewSelectField("country").SetPlaceholder("Choose a country")
		selectField := field.(SelectField)

		assert.Equal(t, "Choose a country", selectField.Placeholder())
	})
}

func TestSelectField_ComplexScenarios(t *testing.T) {
	t.Run("searchable select with int values", func(t *testing.T) {
		field := NewSelectField("product_id").
			AsIntSelect().
			AsSearchable("/api/products/search").
			SetPlaceholder("Search products...")

		selectField := field.(SelectField)

		assert.Equal(t, IntFieldType, selectField.ValueType())
		assert.Equal(t, SelectTypeSearchable, selectField.SelectType())
		assert.Equal(t, "/api/products/search", selectField.Endpoint())
		assert.Equal(t, "Search products...", selectField.Placeholder())
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

		selectField := field.(SelectField)

		assert.Equal(t, SelectTypeCombobox, selectField.SelectType())
		assert.True(t, selectField.Multiple())
		assert.NotNil(t, selectField.OptionsLoader())
		assert.Equal(t, "Select tags", selectField.Placeholder())
	})

	t.Run("boolean select with yes/no options", func(t *testing.T) {
		field := NewSelectField("is_active").
			AsBoolSelect().
			WithStaticOptions(
				SelectOption{Value: "true", Label: "Yes"},
				SelectOption{Value: "false", Label: "No"},
			)

		selectField := field.(SelectField)

		assert.Equal(t, BoolFieldType, selectField.ValueType())
		assert.Equal(t, SelectTypeStatic, selectField.SelectType())
		assert.Len(t, selectField.Options(), 2)
	})
}

func TestSelectField_EdgeCases(t *testing.T) {
	t.Run("empty endpoint doesn't change select type", func(t *testing.T) {
		field := NewSelectField("test").SetEndpoint("")
		selectField := field.(SelectField)

		assert.Equal(t, SelectTypeStatic, selectField.SelectType())
		assert.Empty(t, selectField.Endpoint())
	})

	t.Run("setting multiple false doesn't change combobox type", func(t *testing.T) {
		field := NewSelectField("test").
			AsCombobox().
			SetMultiple(false)

		selectField := field.(SelectField)

		assert.Equal(t, SelectTypeCombobox, selectField.SelectType())
		assert.False(t, selectField.Multiple())
	})

	t.Run("can override select type after auto-setting", func(t *testing.T) {
		field := NewSelectField("test").
			SetEndpoint("/api/search"). // auto-sets to searchable
			SetSelectType(SelectTypeStatic)

		selectField := field.(SelectField)

		assert.Equal(t, SelectTypeStatic, selectField.SelectType())
		assert.Equal(t, "/api/search", selectField.Endpoint())
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

		selectField := field.(SelectField)
		assert.Equal(t, IntFieldType, selectField.Type())
		assert.Equal(t, IntFieldType, selectField.ValueType())
		
		// Check options
		options := selectField.Options()
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

		selectField := field.(SelectField)
		assert.Equal(t, BoolFieldType, selectField.Type())
		assert.Equal(t, BoolFieldType, selectField.ValueType())
		
		// Check options
		options := selectField.Options()
		assert.Len(t, options, 2)
		assert.Equal(t, true, options[0].Value)
		assert.Equal(t, "Yes", options[0].Label)
	})

	t.Run("field value handling with correct types", func(t *testing.T) {
		// Int field should accept int values
		intField := NewSelectField("level").AsIntSelect()
		intValue := intField.Value(5)
		val, err := intValue.AsInt()
		assert.NoError(t, err)
		assert.Equal(t, 5, val)

		// Bool field should accept bool values
		boolField := NewSelectField("active").AsBoolSelect()
		boolValue := boolField.Value(true)
		bval, err := boolValue.AsBool()
		assert.NoError(t, err)
		assert.True(t, bval)
	})
}