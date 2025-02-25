package dialect_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/schema/dialect"
	"github.com/stretchr/testify/assert"
)

func TestRegisterAndGet(t *testing.T) {
	// Clear existing dialects for test
	dialect.ClearDialects()

	tests := []struct {
		name           string
		dialectName    string
		shouldRegister bool
		expectFound    bool
	}{
		{
			name:           "Register and get postgres dialect",
			dialectName:    "postgres",
			shouldRegister: true,
			expectFound:    true,
		},
		{
			name:           "Get unregistered dialect",
			dialectName:    "mysql",
			shouldRegister: false,
			expectFound:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.shouldRegister {
				dialect.Register(tt.dialectName, dialect.NewPostgresDialect())
			}

			d, found := dialect.Get(tt.dialectName)
			assert.Equal(t, tt.expectFound, found)
			if tt.expectFound {
				assert.NotNil(t, d)
			} else {
				assert.Nil(t, d)
			}
		})
	}
}

func TestPostgresDialect_GetDataTypeMapping(t *testing.T) {
	d := dialect.NewPostgresDialect()
	mapping := d.GetDataTypeMapping()

	// Verify essential type mappings
	expectedMappings := map[string]string{
		"int":       "integer",
		"varchar":   "character varying",
		"datetime":  "timestamp",
		"timestamp": "timestamp with time zone",
		"bool":      "boolean",
	}

	for sourceType, expectedType := range expectedMappings {
		actualType, exists := mapping[sourceType]
		assert.True(t, exists, "Expected mapping for type %s to exist", sourceType)
		assert.Equal(t, expectedType, actualType, "Expected type mapping %s -> %s, got %s",
			sourceType, expectedType, actualType)
	}
}