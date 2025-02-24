package dialect

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/schema/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterAndGet(t *testing.T) {
	// Clear existing dialects for test
	dialects = make(map[string]Dialect)

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
				Register(tt.dialectName, NewPostgresDialect())
			}

			dialect, found := Get(tt.dialectName)
			assert.Equal(t, tt.expectFound, found)
			if tt.expectFound {
				assert.NotNil(t, dialect)
			} else {
				assert.Nil(t, dialect)
			}
		})
	}
}

func TestPostgresDialect_GenerateCreate(t *testing.T) {
	d := NewPostgresDialect()

	tests := []struct {
		name        string
		node        *types.Node
		expectedSQL string
		expectError bool
	}{
		{
			name: "Generate simple create table",
			node: &types.Node{
				Type: types.NodeTable,
				Name: "users",
				Children: []*types.Node{
					{
						Type: types.NodeColumn,
						Name: "id",
						Metadata: map[string]interface{}{
							"type":       "SERIAL",
							"definition": "id SERIAL PRIMARY KEY",
						},
					},
					{
						Type: types.NodeColumn,
						Name: "name",
						Metadata: map[string]interface{}{
							"type":       "VARCHAR",
							"definition": "name VARCHAR(255) NOT NULL",
						},
					},
				},
			},
			expectedSQL: `CREATE TABLE IF NOT EXISTS users (
  id SERIAL PRIMARY KEY,
  name VARCHAR(255) NOT NULL
);`,
			expectError: false,
		},
		{
			name: "Invalid node type",
			node: &types.Node{
				Type: types.NodeColumn,
				Name: "invalid",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, err := d.GenerateCreate(tt.node)
			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedSQL, sql)
		})
	}
}

func TestPostgresDialect_GenerateAlter(t *testing.T) {
	d := NewPostgresDialect()

	tests := []struct {
		name        string
		node        *types.Node
		expectedSQL string
		expectError bool
	}{
		{
			name: "Generate add column",
			node: &types.Node{
				Type: types.NodeTable,
				Name: "users",
				Children: []*types.Node{
					{
						Type: types.NodeColumn,
						Name: "email",
						Metadata: map[string]interface{}{
							"type":       "VARCHAR",
							"definition": "email VARCHAR(255) NOT NULL",
						},
					},
				},
				Metadata: map[string]interface{}{
					"alteration": "ADD COLUMN email VARCHAR(255) NOT NULL",
				},
			},
			expectedSQL: "ALTER TABLE users ADD COLUMN email VARCHAR(255) NOT NULL;",
			expectError: false,
		},
		{
			name: "Invalid node",
			node: &types.Node{
				Type: types.NodeColumn,
				Name: "invalid",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, err := d.GenerateAlter(tt.node)
			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedSQL, sql)
		})
	}
}

func TestPostgresDialect_GetDataTypeMapping(t *testing.T) {
	d := NewPostgresDialect()
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

func TestPostgresDialect_ValidateSchema(t *testing.T) {
	d := NewPostgresDialect()

	tests := []struct {
		name        string
		schema      *types.SchemaTree
		expectError bool
	}{
		{
			name: "Valid schema",
			schema: &types.SchemaTree{
				Root: &types.Node{
					Type: types.NodeRoot,
					Children: []*types.Node{
						{
							Type: types.NodeTable,
							Name: "users",
							Children: []*types.Node{
								{
									Type: types.NodeColumn,
									Name: "id",
									Metadata: map[string]interface{}{
										"type": "SERIAL",
									},
								},
							},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "Empty schema",
			schema: &types.SchemaTree{
				Root: &types.Node{
					Type:     types.NodeRoot,
					Children: []*types.Node{},
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := d.ValidateSchema(tt.schema)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
