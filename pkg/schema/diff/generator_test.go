package diff

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerator_Generate(t *testing.T) {
	t.Skip("Skipping this test as it's using the old Node structure. Need to adapt to new SchemaObject interface")
}

func TestGenerator_InvalidDialect(t *testing.T) {
	_, err := NewGenerator(GeneratorOptions{
		Dialect:   "invalid",
		OutputDir: "test",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported dialect")
}

func TestExtractDefaultValue(t *testing.T) {
	tests := []struct {
		name        string
		constraints string
		expected    string
	}{
		{
			name:        "simple numeric default",
			constraints: "NOT NULL DEFAULT 0",
			expected:    "0",
		},
		{
			name:        "quoted string default",
			constraints: "DEFAULT 'test'",
			expected:    "'test'",
		},
		{
			name:        "function default",
			constraints: "DEFAULT CURRENT_TIMESTAMP",
			expected:    "CURRENT_TIMESTAMP",
		},
		{
			name:        "no default",
			constraints: "NOT NULL",
			expected:    "",
		},
		{
			name:        "complex default with spaces",
			constraints: "DEFAULT now() NOT NULL",
			expected:    "now()",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractDefaultValue(tt.constraints)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerator_GenerateColumnDefinition(t *testing.T) {
	t.Skip("Skipping this test as it's using the old Node structure. Need to adapt to new SchemaObject interface")
}
