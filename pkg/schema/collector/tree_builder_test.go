package collector

import (
	"fmt"
	"testing"

	"github.com/iota-uz/psql-parser/sql/lex"
	"github.com/iota-uz/psql-parser/sql/sem/tree"
	"github.com/iota-uz/psql-parser/sql/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchemaState_buildSchema(t *testing.T) {
	tests := []struct {
		name      string
		setup     func() *schemaState
		wantTable string
	}{
		{
			name: "should create table with unqualified name",
			setup: func() *schemaState {
				state := newSchemaState()

				// Create a test table with a column
				tableName := "test_table"

				// Create a CreateTable node
				createTable := &tree.CreateTable{
					Table: tree.MakeUnqualifiedTableName(tree.Name(tableName)),
					Defs: tree.TableDefs{
						&tree.ColumnTableDef{
							Name: tree.Name("id"),
						},
					},
				}

				state.tables[tableName] = createTable

				return state
			},
			wantTable: "test_table",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := tt.setup()
			schema := state.buildSchema()

			// Verify the table exists in the schema
			table, exists := schema.Tables[tt.wantTable]
			assert.True(t, exists, "Table should exist in schema")

			// Skip additional checks if table doesn't exist to avoid nil pointer dereference
			if !exists {
				return
			}

			// Verify the table name is unqualified (not public.public.tablename)
			tableName := table.Table.String()
			assert.Equal(t, tt.wantTable, tableName,
				"Table name should be unqualified and match expected name")

			// Ensure there is no schema qualification
			assert.NotContains(t, tableName, "public.",
				"Table name should not contain schema qualification")
		})
	}
}

func Test_findColumnIndex(t *testing.T) {
	tests := []struct {
		name      string
		defs      tree.TableDefs
		colName   string
		wantIndex int
	}{
		{
			name:      "empty defs",
			defs:      tree.TableDefs{},
			colName:   "id",
			wantIndex: -1,
		},
		{
			name: "column exists",
			defs: tree.TableDefs{
				&tree.ColumnTableDef{Name: tree.Name("id")},
				&tree.ColumnTableDef{Name: tree.Name("name")},
			},
			colName:   "name",
			wantIndex: 1,
		},
		{
			name: "column does not exist",
			defs: tree.TableDefs{
				&tree.ColumnTableDef{Name: tree.Name("id")},
				&tree.ColumnTableDef{Name: tree.Name("name")},
			},
			colName:   "age",
			wantIndex: -1,
		},
		{
			name: "case-insensitive match",
			defs: tree.TableDefs{
				&tree.ColumnTableDef{Name: tree.Name("ID")},
				&tree.ColumnTableDef{Name: tree.Name("NAME")},
			},
			colName:   "id",
			wantIndex: 0,
		},
		{
			name: "case-insensitive match 2",
			defs: tree.TableDefs{
				&tree.ColumnTableDef{Name: tree.Name("ID")},
				&tree.ColumnTableDef{Name: tree.Name("NAME")},
			},
			colName:   "Name",
			wantIndex: 1,
		},
		{
			name: "mixed defs",
			defs: tree.TableDefs{
				&tree.ColumnTableDef{Name: tree.Name("id")},
				&tree.ColumnTableDef{Name: tree.Name("name")},
				&tree.UniqueConstraintTableDef{IndexTableDef: tree.IndexTableDef{Name: tree.Name("unique_name")}},
				&tree.ColumnTableDef{Name: tree.Name("age")},
			},
			colName:   "age",
			wantIndex: 3,
		},
		{
			name: "mixed defs not found",
			defs: tree.TableDefs{
				&tree.ColumnTableDef{Name: tree.Name("id")},
				&tree.ColumnTableDef{Name: tree.Name("name")},
				&tree.UniqueConstraintTableDef{IndexTableDef: tree.IndexTableDef{Name: tree.Name("unique_name")}},
				&tree.ColumnTableDef{Name: tree.Name("age")},
			},
			colName:   "address",
			wantIndex: -1,
		},
		{
			name: "column with spaces",
			defs: tree.TableDefs{
				&tree.ColumnTableDef{Name: tree.Name("id")},
				&tree.ColumnTableDef{Name: tree.Name("first name")},
			},
			colName:   "first name",
			wantIndex: 1,
		},
		{
			name: "column with special characters",
			defs: tree.TableDefs{
				&tree.ColumnTableDef{Name: tree.Name("id")},
				&tree.ColumnTableDef{Name: tree.Name("user-name")},
			},
			colName:   "user-name",
			wantIndex: 1,
		},
		{
			name: "column with mixed case",
			defs: tree.TableDefs{
				&tree.ColumnTableDef{Name: tree.Name("Id")},
				&tree.ColumnTableDef{Name: tree.Name("Name")},
			},
			colName:   "id",
			wantIndex: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotIndex := findColumnIndex(tt.defs, tt.colName)
			assert.Equal(t, tt.wantIndex, gotIndex)
		})
	}
}

func TestApplyAlterTable(t *testing.T) {
	const testTblName = "test_table"
	const testTblNameNorm = "test_table" // Assuming normalization doesn't change it

	// Default columns for initial state
	colID := makeTestColumn("id", types.Int, false, nil)
	colName := makeTestColumn("name", types.String, true, nil)
	colValue := makeTestColumn("value", types.Decimal, false, tree.NewDString("0.0")) // Note: string literal needed for parser

	// Define test cases
	testCases := []struct {
		name           string
		initialState   *schemaState
		alterTableNode *tree.AlterTable
		// Assertions to run after applying the alter command
		assertions func(t *testing.T, finalState *schemaState)
	}{
		// --- Add Column ---
		{
			name: "Add Column - Simple",
			initialState: makeTestSchemaState(testTblName,
				colID,
				colName,
			),
			alterTableNode: &tree.AlterTable{
				Table: makeUnresolvedObjectName(testTblName),
				Cmds: tree.AlterTableCmds{
					&tree.AlterTableAddColumn{
						ColumnDef: makeTestColumn("new_col", types.Bool, true, tree.DBoolTrue), // Add new column
					},
				},
			},
			assertions: func(t *testing.T, finalState *schemaState) {
				t.Helper()
				t.Helper()
				table, ok := finalState.tables[testTblNameNorm]
				require.True(t, ok, "Table should still exist")
				assert.Len(t, table.Defs, 3, "Should have 3 definitions (2 initial + 1 added)")
				_, exists := findDefByName[*tree.ColumnTableDef](table.Defs, "new_col")
				assert.True(t, exists, "new_col should be present")
			},
		},
		{
			name: "Add Column - Already Exists",
			initialState: makeTestSchemaState(testTblName,
				colID,
				colName, // 'name' column exists
			),
			alterTableNode: &tree.AlterTable{
				Table: makeUnresolvedObjectName(testTblName),
				Cmds: tree.AlterTableCmds{
					&tree.AlterTableAddColumn{
						ColumnDef: makeTestColumn("name", types.String, true, tree.NewDString("name_value")), // Try adding 'name' again
					},
				},
			},
			assertions: func(t *testing.T, finalState *schemaState) {
				t.Helper()
				t.Helper()
				table, ok := finalState.tables[testTblNameNorm]
				require.True(t, ok, "Table should still exist")
				assert.Len(t, table.Defs, 2, "Should still have 2 definitions (add should be skipped)")
				// We could potentially check logs for the warning here if logging was captured
			},
		},

		// --- Drop Column ---
		{
			name: "Drop Column - Exists",
			initialState: makeTestSchemaState(testTblName,
				colID,
				colName, // Will drop this one
				colValue,
			),
			alterTableNode: &tree.AlterTable{
				Table: makeUnresolvedObjectName(testTblName),
				Cmds: tree.AlterTableCmds{
					&tree.AlterTableDropColumn{
						Column: "name", // Drop 'name'
					},
				},
			},
			assertions: func(t *testing.T, finalState *schemaState) {
				t.Helper()
				table, ok := finalState.tables[testTblNameNorm]
				require.True(t, ok, "Table should still exist")
				assert.Len(t, table.Defs, 2, "Should have 2 definitions left (id, value)")
				_, exists := findDefByName[*tree.ColumnTableDef](table.Defs, "name")
				assert.False(t, exists, "'name' column should be dropped")
				_, exists = findDefByName[*tree.ColumnTableDef](table.Defs, "id")
				assert.True(t, exists, "'id' column should remain")
				_, exists = findDefByName[*tree.ColumnTableDef](table.Defs, "value")
				assert.True(t, exists, "'value' column should remain")
			},
		},
		{
			name: "Drop Column - Not Exists",
			initialState: makeTestSchemaState(testTblName,
				colID,
				colName,
			),
			alterTableNode: &tree.AlterTable{
				Table: makeUnresolvedObjectName(testTblName),
				Cmds: tree.AlterTableCmds{
					&tree.AlterTableDropColumn{
						Column: "non_existent_col", // Try dropping a column that isn't there
					},
				},
			},
			assertions: func(t *testing.T, finalState *schemaState) {
				t.Helper()
				table, ok := finalState.tables[testTblNameNorm]
				require.True(t, ok, "Table should still exist")
				assert.Len(t, table.Defs, 2, "Should still have 2 definitions")
				// Check log for warning ideally
			},
		},

		// --- Add Constraint ---
		{
			name: "Add Constraint - Unique",
			initialState: makeTestSchemaState(testTblName,
				colID,
				colName,
				colValue,
			),
			alterTableNode: &tree.AlterTable{
				Table: makeUnresolvedObjectName(testTblName),
				Cmds: tree.AlterTableCmds{
					&tree.AlterTableAddConstraint{
						ConstraintDef: makeTestUniqueConstraint("test_table_name_uq", "name"),
					},
				},
			},
			assertions: func(t *testing.T, finalState *schemaState) {
				t.Helper()
				table, ok := finalState.tables[testTblNameNorm]
				require.True(t, ok, "Table should still exist")
				assert.Len(t, table.Defs, 4, "Should have 4 definitions (3 cols + 1 constraint)")
				constraint, exists := findDefByName[*tree.UniqueConstraintTableDef](table.Defs, "test_table_name_uq")
				assert.True(t, exists, "Unique constraint 'test_table_name_uq' should be present")
				require.NotNil(t, constraint)
				require.Len(t, constraint.Columns, 1)
				assert.Equal(t, "name", constraint.Columns[0].Column.Normalize())
			},
		},

		// --- Drop Constraint ---
		{
			name: "Drop Constraint - Exists",
			initialState: makeTestSchemaState(testTblName,
				colID,
				colName,
				makeTestUniqueConstraint("test_table_name_uq", "name"), // Constraint exists
			),
			alterTableNode: &tree.AlterTable{
				Table: makeUnresolvedObjectName(testTblName),
				Cmds: tree.AlterTableCmds{
					&tree.AlterTableDropConstraint{
						Constraint: "test_table_name_uq",
					},
				},
			},
			assertions: func(t *testing.T, finalState *schemaState) {
				t.Helper()
				table, ok := finalState.tables[testTblNameNorm]
				require.True(t, ok, "Table should still exist")
				assert.Len(t, table.Defs, 2, "Should have 2 definitions left (2 cols)")
				_, exists := findDefByName[*tree.UniqueConstraintTableDef](table.Defs, "test_table_name_uq")
				assert.False(t, exists, "Unique constraint 'test_table_name_uq' should be dropped")
			},
		},
		{
			name: "Drop Constraint - Not Exists",
			initialState: makeTestSchemaState(testTblName,
				colID,
				colName, // No constraint defined initially
			),
			alterTableNode: &tree.AlterTable{
				Table: makeUnresolvedObjectName(testTblName),
				Cmds: tree.AlterTableCmds{
					&tree.AlterTableDropConstraint{
						Constraint: "non_existent_constraint",
					},
				},
			},
			assertions: func(t *testing.T, finalState *schemaState) {
				t.Helper()
				table, ok := finalState.tables[testTblNameNorm]
				require.True(t, ok, "Table should still exist")
				assert.Len(t, table.Defs, 2, "Should still have 2 definitions")
				// Check log for warning ideally
			},
		},

		// --- Alter Column Properties (via updateColumnState) ---
		{
			name: "Alter Column Type",
			initialState: makeTestSchemaState(testTblName,
				colID,
				makeTestColumn("name", types.VarChar, true, tree.NewDString("varchar_value")), // Start as VarChar
			),
			alterTableNode: &tree.AlterTable{
				Table: makeUnresolvedObjectName(testTblName),
				Cmds: tree.AlterTableCmds{
					&tree.AlterTableAlterColumnType{
						Column: "name",
						ToType: types.String, // Change to String
					},
				},
			},
			assertions: func(t *testing.T, finalState *schemaState) {
				t.Helper()
				table, ok := finalState.tables[testTblNameNorm]
				require.True(t, ok, "Table should still exist")
				col, exists := findDefByName[*tree.ColumnTableDef](table.Defs, "name")
				require.True(t, exists, "'name' column should exist")
				assert.Equal(t, types.String.SQLString(), col.Type.SQLString(), "Column type should be updated to STRING")
			},
		},
		{
			name: "Drop Not Null",
			initialState: makeTestSchemaState(testTblName,
				colID,
				makeTestColumn("name", types.String, false, tree.NewDString("string_value")), // Start as NOT NULL
			),
			alterTableNode: &tree.AlterTable{
				Table: makeUnresolvedObjectName(testTblName),
				Cmds: tree.AlterTableCmds{
					&tree.AlterTableDropNotNull{
						Column: "name",
					},
				},
			},
			assertions: func(t *testing.T, finalState *schemaState) {
				t.Helper()
				table, ok := finalState.tables[testTblNameNorm]
				require.True(t, ok, "Table should still exist")
				col, exists := findDefByName[*tree.ColumnTableDef](table.Defs, "name")
				require.True(t, exists, "'name' column should exist")
				assert.Equal(t, tree.Null, col.Nullable.Nullability, "Column should become nullable")
			},
		},
		{
			name: "Set Default",
			initialState: makeTestSchemaState(testTblName,
				colID,
				makeTestColumn("name", types.String, true, tree.NewDString("")), // Start with no default
			),
			alterTableNode: &tree.AlterTable{
				Table: makeUnresolvedObjectName(testTblName),
				Cmds: tree.AlterTableCmds{
					&tree.AlterTableSetDefault{
						Column:  "name",
						Default: tree.NewDString("DEFAULT_VAL"), // Set default
					},
				},
			},
			assertions: func(t *testing.T, finalState *schemaState) {
				t.Helper()
				table, ok := finalState.tables[testTblNameNorm]
				require.True(t, ok, "Table should still exist")
				col, exists := findDefByName[*tree.ColumnTableDef](table.Defs, "name")
				require.True(t, exists, "'name' column should exist")
				require.NotNil(t, col.DefaultExpr, "DefaultExpr should not be nil")
				assert.Equal(t, "'DEFAULT_VAL'", col.DefaultExpr.Expr.String(), "Default expression should be set")
			},
		},

		// --- Edge Cases ---
		{
			name: "Alter on Dropped Table",
			initialState: func() *schemaState {
				s := makeTestSchemaState(testTblName, colID)
				s.drops[testTblNameNorm] = true // Mark table as dropped
				return s
			}(),
			alterTableNode: &tree.AlterTable{
				Table: makeUnresolvedObjectName(testTblName),
				Cmds: tree.AlterTableCmds{
					&tree.AlterTableDropColumn{Column: "id"}, // Try to drop column
				},
			},
			assertions: func(t *testing.T, finalState *schemaState) {
				t.Helper()
				// State should be unchanged because the function should return early
				assert.True(t, finalState.drops[testTblNameNorm], "Table should remain marked as dropped")
			},
		},
		{
			name:         "Alter on Non-Existent Table",
			initialState: makeTestSchemaState(""), // No tables initially
			alterTableNode: &tree.AlterTable{
				Table: makeUnresolvedObjectName(testTblName), // Target table doesn't exist
				Cmds: tree.AlterTableCmds{
					&tree.AlterTableAddColumn{ColumnDef: makeTestColumn("new_col", types.Bool, true, tree.DBoolTrue)},
				},
			},
			assertions: func(t *testing.T, finalState *schemaState) {
				t.Helper()
				assert.Empty(t, finalState.tables, "Tables map should remain empty")
				// Check log for warning ideally
			},
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Apply the ALTER command to the initial state
			// Use a fixed timestamp and filename for testing
			tc.initialState.applyAlterTable(tc.alterTableNode, 1234567890, "test_migration.sql")

			// Run the specific assertions for this test case
			tc.assertions(t, tc.initialState)
		})
	}
}

// Creates a basic schema state, optionally with an initial table.
func makeTestSchemaState(tableName tree.Name, initialDefs ...tree.TableDef) *schemaState {
	s := newSchemaState()
	if tableName != "" {
		normalizedTableName := tableName.Normalize()
		createTable := &tree.CreateTable{
			Table: tree.MakeUnqualifiedTableName(tableName),
			Defs:  make(tree.TableDefs, 0, len(initialDefs)),
		}
		createTable.Defs = append(createTable.Defs, initialDefs...)
		s.tables[normalizedTableName] = createTable
	}
	return s
}

// Creates a simple column definition.
func makeTestColumn(name tree.Name, typ *types.T, nullable bool, defaultValExpr tree.Expr) *tree.ColumnTableDef {
	def := &tree.ColumnTableDef{
		Name: name,
		Type: typ,
		Nullable: struct {
			Nullability    tree.Nullability
			ConstraintName tree.Name
		}{
			Nullability: tree.SilentNull, // Default assumption
		},
	}
	if !nullable {
		def.Nullable.Nullability = tree.NotNull
	} else {
		def.Nullable.Nullability = tree.Null // Explicitly Null if nullable=true
	}

	// Assign the provided default expression directly if it's not nil
	if defaultValExpr != nil {
		def.DefaultExpr.Expr = defaultValExpr
	}

	return def
}

// Creates a simple unique constraint definition.
func makeTestUniqueConstraint(name tree.Name, cols ...tree.Name) *tree.UniqueConstraintTableDef {
	idxParams := make([]tree.IndexElem, len(cols))
	for i, colName := range cols {
		idxParams[i] = tree.IndexElem{Column: colName}
	}
	return &tree.UniqueConstraintTableDef{
		IndexTableDef: tree.IndexTableDef{
			Name:    name,
			Columns: tree.IndexElemList(idxParams),
		},
		PrimaryKey: false,
	}
}

// Helper to find a column definition by normalized name.
func findDefByName[T tree.TableDef](defs tree.TableDefs, normalizedName string) (T, bool) {
	for _, def := range defs {
		if typedDef, ok := def.(T); ok {
			// Need to get the name based on the actual type T
			var currentName string
			switch specificDef := any(typedDef).(type) {
			case *tree.ColumnTableDef:
				currentName = specificDef.Name.Normalize()
			case *tree.UniqueConstraintTableDef:
				currentName = specificDef.Name.Normalize()
			// Add cases for other constraint types (Check, FK, PK) as needed
			default:
				continue // Skip types we can't get a name from easily
			}

			if currentName == normalizedName {
				return typedDef, true
			}
		}
	}
	var zero T // Create a zero value for the type T
	return zero, false
}

func makeUnresolvedObjectName(testTblName string) *tree.UnresolvedObjectName {
	tableName, err := tree.NewUnresolvedObjectName(
		1, // Number of name parts (1 for unqualified)
		[3]string{lex.NormalizeName(testTblName)}, // Use normalized name string in the parts array
		tree.NoAnnotation,                         // Assuming NoAnnotation constant exists, otherwise use 0
	)
	if err != nil {
		panic(fmt.Sprintf("Setup error: Failed to create UnresolvedObjectName for testing: %v", err))
	}
	return tableName
}
