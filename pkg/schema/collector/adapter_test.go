package collector

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/schema/common"
	"github.com/stretchr/testify/assert"
)

func TestSchemaAdapter_Conversion(t *testing.T) {
	// Create a local schema tree
	localTree := &SchemaTree{
		Root: &Node{
			Type: NodeRoot,
			Children: []*Node{
				{
					Type: NodeTable,
					Name: "users",
					Children: []*Node{
						{
							Type: NodeColumn,
							Name: "id",
							Metadata: map[string]interface{}{
								"type":        "SERIAL",
								"fullType":    "SERIAL",
								"constraints": "PRIMARY KEY",
								"definition":  "id SERIAL PRIMARY KEY",
							},
						},
						{
							Type: NodeColumn,
							Name: "name",
							Metadata: map[string]interface{}{
								"type":        "VARCHAR(255)",
								"fullType":    "VARCHAR(255)",
								"constraints": "NOT NULL",
								"definition":  "name VARCHAR(255) NOT NULL",
							},
						},
					},
					Metadata: map[string]interface{}{
						"schema": "public",
					},
				},
			},
			Metadata: make(map[string]interface{}),
		},
		Metadata: map[string]interface{}{
			"version": "1.0",
		},
	}

	// Convert to common.Schema
	adapter := NewSchemaAdapter(localTree)
	commonSchema := adapter.ToSchema()

	// Verify conversion - we can only check basic things because the structure is different
	assert.NotNil(t, commonSchema)
	assert.Contains(t, commonSchema.Tables, "users")
}

func TestCollectSchemaChanges(t *testing.T) {
	// Create two schema trees with differences
	oldSchema := &SchemaTree{
		Root: &Node{
			Type: NodeRoot,
			Children: []*Node{
				{
					Type: NodeTable,
					Name: "users",
					Children: []*Node{
						{
							Type: NodeColumn,
							Name: "id",
							Metadata: map[string]interface{}{
								"type":        "SERIAL",
								"fullType":    "SERIAL",
								"constraints": "PRIMARY KEY",
								"definition":  "id SERIAL PRIMARY KEY",
							},
						},
					},
				},
			},
		},
	}

	newSchema := &SchemaTree{
		Root: &Node{
			Type: NodeRoot,
			Children: []*Node{
				{
					Type: NodeTable,
					Name: "users",
					Children: []*Node{
						{
							Type: NodeColumn,
							Name: "id",
							Metadata: map[string]interface{}{
								"type":        "SERIAL",
								"fullType":    "SERIAL",
								"constraints": "PRIMARY KEY",
								"definition":  "id SERIAL PRIMARY KEY",
							},
						},
						{
							Type: NodeColumn,
							Name: "email",
							Metadata: map[string]interface{}{
								"type":        "VARCHAR(255)",
								"fullType":    "VARCHAR(255)",
								"constraints": "NOT NULL",
								"definition":  "email VARCHAR(255) NOT NULL",
							},
						},
					},
				},
			},
		},
	}

	// Collect changes
	changes, err := CollectSchemaChanges(oldSchema, newSchema)
	assert.NoError(t, err)
	assert.NotNil(t, changes)

	// Verify changes
	found := false
	for _, change := range changes.Changes {
		if change.Type == common.AddColumn && change.ObjectName == "email" {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected to find ADD_COLUMN email change")
}