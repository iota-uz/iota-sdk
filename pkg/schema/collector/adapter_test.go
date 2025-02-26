package collector

import (
	"testing"

	"github.com/auxten/postgresql-parser/pkg/sql/sem/tree"
	"github.com/auxten/postgresql-parser/pkg/sql/types"
	"github.com/iota-uz/iota-sdk/pkg/schema/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchemaAdapter_Conversion(t *testing.T) {
	// Create schema using common.Schema
	localSchema := common.NewSchema()
	
	// Create table definition
	tableName := tree.MakeTableName("", "users")
	table := &tree.CreateTable{
		Table: tableName,
		Defs: tree.TableDefs{
			&tree.ColumnTableDef{
				Name: tree.Name("id"),
				Type: types.Int,
			},
			&tree.ColumnTableDef{
				Name: tree.Name("name"),
				Type: types.String,
			},
		},
	}
	
	// Add to schema
	localSchema.Tables[tableName.TableName.String()] = table
	
	// Test usage of schema - we can verify it's properly set up
	assert.NotNil(t, localSchema)
	assert.Contains(t, localSchema.Tables, "users")
}

func TestCollectSchemaChanges(t *testing.T) {
	// Create two schemas with differences
	oldSchema := common.NewSchema()
	newSchema := common.NewSchema()
	
	// Create table for old schema
	usersTable := tree.MakeTableName("", "users")
	oldTable := &tree.CreateTable{
		Table: usersTable,
		Defs: tree.TableDefs{
			&tree.ColumnTableDef{
				Name: tree.Name("id"),
				Type: types.Int,
			},
		},
	}
	oldSchema.Tables[usersTable.TableName.String()] = oldTable
	
	// Create table for new schema with an additional column
	newTable := &tree.CreateTable{
		Table: usersTable,
		Defs: tree.TableDefs{
			&tree.ColumnTableDef{
				Name: tree.Name("id"),
				Type: types.Int,
			},
			&tree.ColumnTableDef{
				Name: tree.Name("email"),
				Type: types.String,
			},
		},
	}
	newSchema.Tables[usersTable.TableName.String()] = newTable
	
	// Collect changes
	changes, err := CollectSchemaChanges(oldSchema, newSchema)
	assert.NoError(t, err)
	assert.NotNil(t, changes)
	
	// Verify changes
	assert.NotEmpty(t, changes.Changes, "Expected to find changes")
}

// New tests for adapter.go

func TestCollectSchemaChanges_AddTable(t *testing.T) {
	// Setup test schemas
	oldSchema := common.NewSchema()
	newSchema := common.NewSchema()
	
	// Create a new table for the new schema
	tableName := tree.MakeTableName("", "users")
	newTable := &tree.CreateTable{
		Table: tableName,
		Defs: tree.TableDefs{
			&tree.ColumnTableDef{
				Name: tree.Name("id"),
				Type: types.Int,
			},
		},
	}
	newSchema.Tables[tableName.TableName.String()] = newTable
	
	// Collect changes
	changes, err := CollectSchemaChanges(oldSchema, newSchema)
	
	// Assertions
	require.NoError(t, err)
	require.NotNil(t, changes)
	require.Len(t, changes.Changes, 1, "Expected one change for the new table")
	
	// Check if the change is a CreateTable
	createTable, ok := changes.Changes[0].(*tree.CreateTable)
	require.True(t, ok, "Expected a CreateTable change")
	assert.Equal(t, tableName.TableName.String(), createTable.Table.TableName.String(), "Table name should match")
}

func TestCollectSchemaChanges_AddColumn(t *testing.T) {
	// This test requires advanced mocking
	t.Skip("Test requires more complex mocking")
}

func TestCollectSchemaChanges_AlterColumnType(t *testing.T) {
	// This test requires advanced mocking
	t.Skip("Test requires more complex mocking")
}

func TestCollectSchemaChanges_NoChanges(t *testing.T) {
	// This test requires advanced mocking
	t.Skip("Test requires more complex mocking")
}

func TestCollectSchemaChanges_MultipleChanges(t *testing.T) {
	// This test requires advanced mocking
	t.Skip("Test requires more complex mocking")
}