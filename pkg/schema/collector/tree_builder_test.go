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
