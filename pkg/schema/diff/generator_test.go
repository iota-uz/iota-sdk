package diff

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/schema/types"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerator_Generate(t *testing.T) {
	// Create temporary directory for test output
	tmpDir, err := os.MkdirTemp("", "generator-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create test logger
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	tests := []struct {
		name           string
		changes        *ChangeSet
		opts           GeneratorOptions
		expectedUp     string
		expectedDown   string
		expectError    bool
		validateOutput func(t *testing.T, upContent, downContent string)
	}{
		{
			name: "create table with columns and constraints",
			changes: &ChangeSet{
				Changes: []*Change{
					{
						Type:       CreateTable,
						ObjectName: "users",
						Object: &types.Node{
							Type: types.NodeTable,
							Name: "users",
							Children: []*types.Node{
								{
									Type: types.NodeColumn,
									Name: "id",
									Metadata: map[string]interface{}{
										"definition": "id SERIAL PRIMARY KEY",
									},
								},
								{
									Type: types.NodeColumn,
									Name: "email",
									Metadata: map[string]interface{}{
										"definition": "email VARCHAR(255) NOT NULL UNIQUE",
									},
								},
							},
						},
						Reversible: true,
					},
				},
			},
			opts: GeneratorOptions{
				Dialect:     "postgres",
				OutputDir:   tmpDir,
				IncludeDown: true,
			},
			validateOutput: func(t *testing.T, upContent, downContent string) {
				assert.Contains(t, upContent, "CREATE TABLE IF NOT EXISTS users")
				assert.Contains(t, upContent, "id SERIAL PRIMARY KEY")
				assert.Contains(t, upContent, "email VARCHAR(255) NOT NULL UNIQUE")
				assert.Contains(t, downContent, "DROP TABLE IF EXISTS users")
			},
		},
		{
			name: "add column with constraints",
			changes: &ChangeSet{
				Changes: []*Change{
					{
						Type:       AddColumn,
						ParentName: "users",
						ObjectName: "created_at",
						Object: &types.Node{
							Type: types.NodeColumn,
							Name: "created_at",
							Metadata: map[string]interface{}{
								"definition": "created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP",
							},
						},
						Reversible: true,
					},
				},
			},
			opts: GeneratorOptions{
				Dialect:     "postgres",
				OutputDir:   tmpDir,
				IncludeDown: true,
			},
			validateOutput: func(t *testing.T, upContent, downContent string) {
				assert.Contains(t, upContent, "ALTER TABLE users ADD COLUMN created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP")
				assert.Contains(t, downContent, "ALTER TABLE users DROP COLUMN IF EXISTS created_at")
			},
		},
		{
			name: "modify column type and constraints",
			changes: &ChangeSet{
				Changes: []*Change{
					{
						Type:       ModifyColumn,
						ParentName: "users",
						ObjectName: "email",
						Object: &types.Node{
							Type: types.NodeColumn,
							Name: "email",
							Metadata: map[string]interface{}{
								"definition":  "email TEXT NOT NULL",
								"fullType":    "TEXT",
								"constraints": "NOT NULL",
							},
						},
					},
				},
			},
			opts: GeneratorOptions{
				Dialect:     "postgres",
				OutputDir:   tmpDir,
				IncludeDown: true,
			},
			validateOutput: func(t *testing.T, upContent, downContent string) {
				assert.Contains(t, upContent, "ALTER TABLE users ALTER COLUMN email TYPE TEXT")
				assert.Contains(t, upContent, "SET NOT NULL")
			},
		},
		{
			name: "empty change set",
			changes: &ChangeSet{
				Changes: []*Change{},
			},
			opts: GeneratorOptions{
				Dialect:     "postgres",
				OutputDir:   tmpDir,
				IncludeDown: true,
			},
			validateOutput: func(t *testing.T, upContent, downContent string) {
				assert.Empty(t, upContent)
				assert.Empty(t, downContent)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator, err := NewGenerator(tt.opts)
			require.NoError(t, err)

			err = generator.Generate(tt.changes)
			if tt.expectError {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Find and read the generated files
			files, err := os.ReadDir(tmpDir)
			require.NoError(t, err)

			var upContent, downContent string
			for _, file := range files {
				if strings.HasSuffix(file.Name(), ".down.sql") {
					content, err := os.ReadFile(filepath.Join(tmpDir, file.Name()))
					require.NoError(t, err)
					downContent = string(content)
				} else if strings.HasSuffix(file.Name(), ".sql") {
					content, err := os.ReadFile(filepath.Join(tmpDir, file.Name()))
					require.NoError(t, err)
					upContent = string(content)
				}
			}

			if tt.validateOutput != nil {
				tt.validateOutput(t, upContent, downContent)
			}

			// Clean up files after each test
			for _, file := range files {
				os.Remove(filepath.Join(tmpDir, file.Name()))
			}
		})
	}
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
	generator, err := NewGenerator(GeneratorOptions{
		Dialect: "postgres",
	})
	require.NoError(t, err)

	tests := []struct {
		name     string
		node     *types.Node
		expected string
	}{
		{
			name: "simple column",
			node: &types.Node{
				Name: "id",
				Metadata: map[string]interface{}{
					"definition": "id SERIAL PRIMARY KEY",
				},
			},
			expected: "id SERIAL PRIMARY KEY",
		},
		{
			name: "column with type and constraints",
			node: &types.Node{
				Name: "email",
				Metadata: map[string]interface{}{
					"type":        "varchar(255)",
					"constraints": "NOT NULL UNIQUE",
				},
			},
			expected: "email varchar(255) NOT NULL UNIQUE",
		},
		{
			name: "column with mapped type",
			node: &types.Node{
				Name: "description",
				Metadata: map[string]interface{}{
					"type": "text",
				},
			},
			expected: "description text",
		},
		{
			name:     "nil node",
			node:     nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generator.generateColumnDefinition(tt.node)
			assert.Equal(t, tt.expected, strings.TrimSpace(result))
		})
	}
}
