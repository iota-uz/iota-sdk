package collector

import (
	"testing"

	"github.com/iota-uz/psql-parser/sql/sem/tree"
	"github.com/stretchr/testify/assert"
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
