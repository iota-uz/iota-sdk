package crud

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewJsonField(t *testing.T) {
	t.Run("basic creation", func(t *testing.T) {
		field := NewJsonField("metadata")

		assert.Equal(t, "metadata", field.Name())
		assert.Equal(t, JsonFieldType, field.Type())
		assert.Equal(t, "", field.SchemaType())
	})

	t.Run("with MultiLang schema", func(t *testing.T) {
		field := NewJsonField("title", WithMultiLang())

		assert.Equal(t, "title", field.Name())
		assert.Equal(t, JsonFieldType, field.Type())
		assert.Equal(t, "multilang", field.SchemaType())
	})
}

func TestJsonField_AsJsonField(t *testing.T) {
	field := NewJsonField("test")

	jsonField, err := field.AsJsonField()
	require.NoError(t, err)
	assert.Equal(t, field, jsonField)
}

func TestJsonField_FormatJSON(t *testing.T) {
	tests := []struct {
		name     string
		data     interface{}
		expected string
	}{
		{
			name:     "object",
			data:     map[string]interface{}{"key": "value", "number": 42},
			expected: `{"key":"value","number":42}`,
		},
		{
			name:     "array",
			data:     []interface{}{"item1", "item2", 3},
			expected: `["item1","item2",3]`,
		},
		{
			name:     "string",
			data:     "hello",
			expected: `"hello"`,
		},
	}

	field := NewJsonField("test")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := field.FormatJSON(tt.data)
			require.NoError(t, err)
			assert.JSONEq(t, tt.expected, result)
		})
	}
}

func TestJsonField_ParseJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected interface{}
	}{
		{
			name:     "valid object",
			input:    `{"name": "test", "value": 123}`,
			expected: map[string]interface{}{"name": "test", "value": float64(123)},
		},
		{
			name:     "valid array",
			input:    `["item1", "item2", 3]`,
			expected: []interface{}{"item1", "item2", float64(3)},
		},
		{
			name:     "valid string",
			input:    `"hello world"`,
			expected: "hello world",
		},
		{
			name:     "valid number",
			input:    `42.5`,
			expected: 42.5,
		},
		{
			name:     "valid boolean",
			input:    `true`,
			expected: true,
		},
		{
			name:     "valid null",
			input:    `null`,
			expected: nil,
		},
	}

	field := NewJsonField("test")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := field.ParseJSON(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}

	t.Run("invalid JSON", func(t *testing.T) {
		_, err := field.ParseJSON(`{invalid json}`)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse JSON")
	})
}

func TestJsonField_ValidateJSON(t *testing.T) {
	field := NewJsonField("test")

	t.Run("valid JSON", func(t *testing.T) {
		tests := []string{
			`{"key": "value"}`,
			`["item1", "item2"]`,
			`"string"`,
			`123`,
			`true`,
			`null`,
		}

		for _, input := range tests {
			err := field.ValidateJSON(input)
			assert.NoError(t, err, "input: %s", input)
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		err := field.ValidateJSON(`{invalid}`)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid JSON")
	})
}
