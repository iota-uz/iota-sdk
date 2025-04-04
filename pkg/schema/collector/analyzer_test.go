package collector

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/schema/common"
	"github.com/iota-uz/psql-parser/sql/sem/tree"
	"github.com/iota-uz/psql-parser/sql/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

	// Collect upChanges
	upChanges, downChanges, err := CollectSchemaChanges(oldSchema, newSchema)
	assert.NoError(t, err)
	assert.NotNil(t, upChanges)

	// Verify changes
	assert.NotEmpty(t, upChanges.Changes, "Expected to find changes")
	assert.Len(t, upChanges.Changes, 1, "Expected one change for the new column")

	// Check downChanges
	require.NotNil(t, downChanges)
	require.Len(t, downChanges.Changes, 1, "Expected one change for the new table")
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

	// Collect upChanges
	upChanges, downChanges, err := CollectSchemaChanges(oldSchema, newSchema)

	// Assertions
	require.NoError(t, err)
	require.NotNil(t, upChanges)
	require.Len(t, upChanges.Changes, 1, "Expected one change for the new table")

	// Check if the change is a CreateTable
	createTable, ok := upChanges.Changes[0].(*tree.CreateTable)
	require.True(t, ok, "Expected a CreateTable change")
	assert.Equal(t, tableName.TableName.String(), createTable.Table.TableName.String(), "Table name should match")

	// Check downChanges
	require.NotNil(t, downChanges)
	require.Len(t, downChanges.Changes, 1, "Expected one change for the new table")
}

func TestCollectSchemaChanges_AddColumn(t *testing.T) {
	// Setup test schemas
	oldSchema := common.NewSchema()
	newSchema := common.NewSchema()

	// Create a table with a column for old schema
	tableName := tree.MakeTableName("", "users")
	oldTable := &tree.CreateTable{
		Table: tableName,
		Defs: tree.TableDefs{
			&tree.ColumnTableDef{
				Name: tree.Name("id"),
				Type: types.Int,
			},
		},
	}
	oldSchema.Tables[tableName.TableName.String()] = oldTable

	// Create same table with an additional column for new schema
	newTable := &tree.CreateTable{
		Table: tableName,
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
	newSchema.Tables[tableName.TableName.String()] = newTable

	// Collect changes
	upChanges, downChanges, err := CollectSchemaChanges(oldSchema, newSchema)

	// Assertions
	require.NoError(t, err)
	require.NotNil(t, upChanges)
	require.Equal(t, 1, len(upChanges.Changes), "Expected one change for the new column")

	// Check if the change is a AlterTable with AddColumn
	alterTable, ok := upChanges.Changes[0].(*tree.AlterTable)
	require.True(t, ok, "Expected an AlterTable change")

	addColumn, ok := alterTable.Cmds[0].(*tree.AlterTableAddColumn)
	require.True(t, ok, "Expected an AlterTableAddColumn command")
	assert.Equal(t, "email", addColumn.ColumnDef.Name.String(), "Column name should match")
	assert.Equal(t, types.String.String(), addColumn.ColumnDef.Type.String(), "Column type should match")

	// Check downChanges
	require.NotNil(t, downChanges)
	require.Equal(t, 1, len(downChanges.Changes), "Expected one change for column removal")

	dropTable, ok := downChanges.Changes[0].(*tree.AlterTable)
	require.True(t, ok, "Expected an AlterTable change for drop")

	dropColumn, ok := dropTable.Cmds[0].(*tree.AlterTableDropColumn)
	require.True(t, ok, "Expected an AlterTableDropColumn command")
	assert.Equal(t, "email", dropColumn.Column.String(), "Column name should match")
	assert.True(t, dropColumn.IfExists, "IfExists should be true")
}

func TestCollectSchemaChanges_AlterColumnType(t *testing.T) {
	// Setup test schemas
	oldSchema := common.NewSchema()
	newSchema := common.NewSchema()

	// Create a table with a column for old schema
	tableName := tree.MakeTableName("", "users")
	oldTable := &tree.CreateTable{
		Table: tableName,
		Defs: tree.TableDefs{
			&tree.ColumnTableDef{
				Name: tree.Name("id"),
				Type: types.Int,
			},
		},
	}
	oldSchema.Tables[tableName.TableName.String()] = oldTable

	// Create same table with different column type for new schema
	newTable := &tree.CreateTable{
		Table: tableName,
		Defs: tree.TableDefs{
			&tree.ColumnTableDef{
				Name: tree.Name("id"),
				Type: types.Int4,
			},
		},
	}
	newSchema.Tables[tableName.TableName.String()] = newTable

	// Collect changes
	upChanges, downChanges, err := CollectSchemaChanges(oldSchema, newSchema)

	// Assertions
	require.NoError(t, err)
	require.NotNil(t, upChanges)
	require.Equal(t, 1, len(upChanges.Changes), "Expected one change for the column type change")

	// Check if the change is a AlterTable with AlterColumnType
	alterTable, ok := upChanges.Changes[0].(*tree.AlterTable)
	require.True(t, ok, "Expected an AlterTable change")

	alterColumnType, ok := alterTable.Cmds[0].(*tree.AlterTableAlterColumnType)
	require.True(t, ok, "Expected an AlterTableAlterColumnType command")
	assert.Equal(t, "id", alterColumnType.Column.String(), "Column name should match")
	assert.Equal(t, types.Int4.String(), alterColumnType.ToType.String(), "New column type should match")

	// Check downChanges - should revert to original type
	require.NotNil(t, downChanges)
	require.Equal(t, 1, len(downChanges.Changes), "Expected one change for column type reversion")

	revertTable, ok := downChanges.Changes[0].(*tree.AlterTable)
	require.True(t, ok, "Expected an AlterTable change for type reversion")

	revertColumnType, ok := revertTable.Cmds[0].(*tree.AlterTableAlterColumnType)
	require.True(t, ok, "Expected an AlterTableAlterColumnType command")
	assert.Equal(t, "id", revertColumnType.Column.String(), "Column name should match")
	assert.Equal(t, types.Int.String(), revertColumnType.ToType.String(), "Reverted column type should match original")
}

func TestCompareTables_NoChanges(t *testing.T) {
	// Create identical tables
	tableName := tree.MakeTableName("", "users")
	oldTable := &tree.CreateTable{
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

	newTable := &tree.CreateTable{
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

	// Compare tables
	upChanges, downChanges, err := CompareTables(oldTable, newTable)

	// Assertions
	require.NoError(t, err)
	assert.Empty(t, upChanges, "Expected no up changes for identical tables")
	assert.Empty(t, downChanges, "Expected no down changes for identical tables")
}

func TestCompareTables_MultipleChanges(t *testing.T) {
	// Create tables with multiple differences
	tableName := tree.MakeTableName("", "users")
	oldTable := &tree.CreateTable{
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

	newTable := &tree.CreateTable{
		Table: tableName,
		Defs: tree.TableDefs{
			&tree.ColumnTableDef{
				Name: tree.Name("id"),
				Type: types.Int4, // Changed type
			},
			&tree.ColumnTableDef{
				Name: tree.Name("name"),
				Type: types.String,
			},
			&tree.ColumnTableDef{
				Name: tree.Name("email"),
				Type: types.String, // New column
			},
			&tree.ColumnTableDef{
				Name: tree.Name("created_at"),
				Type: types.TimestampTZ, // New column
			},
		},
	}

	// Compare tables
	upChanges, downChanges, err := CompareTables(oldTable, newTable)

	// Assertions
	require.NoError(t, err)

	// Just verify we have the expected number of changes
	// The actual content may change slightly based on implementation
	require.Len(t, upChanges, 3, "Should have exactly 3 changes")
	require.Len(t, downChanges, 3, "Should have exactly 3 down changes")

	// Check the first change
	change1 := upChanges[0].(*tree.AlterTable).Cmds[0].(*tree.AlterTableAlterColumnType)
	assert.Equal(t, "id", change1.Column.String(), "Column name should match")

	change2 := upChanges[1].(*tree.AlterTable).Cmds[0].(*tree.AlterTableAddColumn)
	assert.Equal(t, "email", change2.ColumnDef.Name.String(), "Column name should match")

	change3 := upChanges[2].(*tree.AlterTable).Cmds[0].(*tree.AlterTableAddColumn)
	assert.Equal(t, "created_at", change3.ColumnDef.Name.String(), "Column name should match")
}

func TestCompareTables_ColumnsRemoved(t *testing.T) {
	// Create tables where columns are removed in the new schema
	tableName := tree.MakeTableName("", "users")
	oldTable := &tree.CreateTable{
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
			&tree.ColumnTableDef{
				Name: tree.Name("email"),
				Type: types.String,
			},
			&tree.ColumnTableDef{
				Name: tree.Name("address"),
				Type: types.String,
			},
		},
	}

	newTable := &tree.CreateTable{
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
			// email and address columns removed
		},
	}

	// Compare tables
	upChanges, downChanges, err := CompareTables(oldTable, newTable)

	// Assertions
	require.NoError(t, err)
	require.Len(t, upChanges, 2, "Should have exactly 2 changes for removed columns")
	require.Len(t, downChanges, 2, "Should have exactly 2 down changes to re-add columns")

	// Verify first up change (drop email column)
	alterTable1 := upChanges[0].(*tree.AlterTable)
	dropColumn1 := alterTable1.Cmds[0].(*tree.AlterTableDropColumn)
	assert.Equal(t, "email", dropColumn1.Column.String(), "First column name should be 'email'")
	assert.True(t, dropColumn1.IfExists, "IfExists should be true")

	// Verify second up change (drop address column)
	alterTable2 := upChanges[1].(*tree.AlterTable)
	dropColumn2 := alterTable2.Cmds[0].(*tree.AlterTableDropColumn)
	assert.Equal(t, "address", dropColumn2.Column.String(), "Second column name should be 'address'")
	assert.True(t, dropColumn2.IfExists, "IfExists should be true")

	// Verify second down change (add email column back)
	downAlterTable1 := downChanges[0].(*tree.AlterTable)
	addColumn1 := downAlterTable1.Cmds[0].(*tree.AlterTableAddColumn)
	assert.Equal(t, "email", addColumn1.ColumnDef.Name.String(), "Second down change should add 'email'")
	assert.Equal(t, types.String.String(), addColumn1.ColumnDef.Type.String(), "Column type should be string")

	// Verify first down change (add address column back)
	downAlterTable2 := downChanges[1].(*tree.AlterTable)
	addColumn2 := downAlterTable2.Cmds[0].(*tree.AlterTableAddColumn)
	assert.Equal(t, "address", addColumn2.ColumnDef.Name.String(), "First down change should add 'address'")
	assert.Equal(t, types.String.String(), addColumn2.ColumnDef.Type.String(), "Column type should be string")
}

func TestCompareTables_UniqueConstraintChange(t *testing.T) {
	// Create tables with a change in unique constraints
	tableName := tree.MakeTableName("", "passports")
	oldTable := &tree.CreateTable{
		Table: tableName,
		Defs: tree.TableDefs{
			&tree.ColumnTableDef{
				Name: tree.Name("passport_number"),
				Type: types.String,
				// Old schema has inline unique constraint
				Nullable: struct {
					Nullability    tree.Nullability
					ConstraintName tree.Name
				}{tree.NotNull, ""},
				Unique: true,
			},
			&tree.ColumnTableDef{
				Name: tree.Name("series"),
				Type: types.String,
				Nullable: struct {
					Nullability    tree.Nullability
					ConstraintName tree.Name
				}{tree.NotNull, ""},
			},
		},
	}

	newTable := &tree.CreateTable{
		Table: tableName,
		Defs: tree.TableDefs{
			&tree.ColumnTableDef{
				Name: tree.Name("passport_number"),
				Type: types.String,
				Nullable: struct {
					Nullability    tree.Nullability
					ConstraintName tree.Name
				}{tree.NotNull, ""},
				Unique: false, // Removed inline unique
			},
			&tree.ColumnTableDef{
				Name: tree.Name("series"),
				Type: types.String,
				Nullable: struct {
					Nullability    tree.Nullability
					ConstraintName tree.Name
				}{tree.NotNull, ""},
			},
			// New schema has a table-level unique constraint
			&tree.UniqueConstraintTableDef{
				IndexTableDef: tree.IndexTableDef{
					Name: tree.Name("passports_passport_number_series_key"),
					Columns: []tree.IndexElem{
						{Column: tree.Name("passport_number")},
						{Column: tree.Name("series")}},
				},
			},
		},
	}

	// Compare tables
	upChanges, downChanges, err := CompareTables(oldTable, newTable)

	// Assertions
	require.NoError(t, err)
	require.Len(t, upChanges, 2, "Should have exactly 2 changes for unique constraint change")
	require.Len(t, downChanges, 2, "Should have exactly 2 down changes to revert unique constraint change")

	// Verify first up change (drop unique constraint on passport_number)
	alterTable1 := upChanges[0].(*tree.AlterTable)
	dropConstraint1, ok := alterTable1.Cmds[0].(*tree.AlterTableDropConstraint)
	require.True(t, ok, "Expected AlterTableDropConstraint")
	assert.Equal(t, "passports_passport_number_key", dropConstraint1.Constraint.String(), "First constraint name should be 'passports_passport_number_key'")
	assert.True(t, dropConstraint1.IfExists, "IfExists should be true")

	// Verify second up change (add unique constraint on passport_number and series)
	alterTable2 := upChanges[1].(*tree.AlterTable)
	addConstraint2, ok := alterTable2.Cmds[0].(*tree.AlterTableAddConstraint)
	require.True(t, ok, "Expected AlterTableAddConstraint")
	uniqueConstraint, ok := addConstraint2.ConstraintDef.(*tree.UniqueConstraintTableDef)
	require.True(t, ok, "Expected UniqueConstraintTableDef")
	assert.Equal(t, "passports_passport_number_series_key", uniqueConstraint.Name, "Second constraint name should be 'passports_passport_number_series_key'")
	require.Len(t, uniqueConstraint.Columns, 2, "Should have 2 columns in unique constraint")
	assert.Equal(t, "passport_number", uniqueConstraint.Columns[0].Column.String(), "First column in unique constraint should be 'passport_number'")
	assert.Equal(t, "series", uniqueConstraint.Columns[1].Column.String(), "Second column in unique constraint should be 'series'")

	// Verify first down change (drop unique constraint on passport_number and series)
	downAlterTable1 := downChanges[0].(*tree.AlterTable)
	downDropConstraint1, ok := downAlterTable1.Cmds[0].(*tree.AlterTableDropConstraint)
	require.True(t, ok, "Expected AlterTableDropConstraint")
	assert.Equal(t, "passports_passport_number_series_key", downDropConstraint1.Constraint.String(), "First down constraint name should be 'passports_passport_number_series_key'")
	assert.True(t, downDropConstraint1.IfExists, "IfExists should be true")

	// Verify second down change (add unique constraint on passport_number)
	downAlterTable2 := downChanges[1].(*tree.AlterTable)
	downAddConstraint2, ok := downAlterTable2.Cmds[0].(*tree.AlterTableAddConstraint)
	require.True(t, ok, "Expected AlterTableAddConstraint")
	downUniqueConstraint, ok := downAddConstraint2.ConstraintDef.(*tree.UniqueConstraintTableDef)
	require.True(t, ok, "Expected UniqueConstraintTableDef")
	assert.Equal(t, "passports_passport_number_key", downUniqueConstraint.Name, "Second down constraint name should be 'passports_passport_number_key'")
	require.Len(t, downUniqueConstraint.Columns, 1, "Should have 1 column in unique constraint")
	assert.Equal(t, "passport_number", downUniqueConstraint.Columns[0].Column.String(), "First column in unique constraint should be 'passport_number'")
}
