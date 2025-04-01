package common

import (
	"testing"

	"github.com/iota-uz/psql-parser/sql/sem/tree"
	"github.com/iota-uz/psql-parser/sql/types"
	"github.com/stretchr/testify/assert"
)

func newColumn(t *testing.T, name string, typ *types.T, qualifications ...tree.ColumnQualification) *tree.ColumnTableDef {
	t.Helper()
	qs := make([]tree.NamedColumnQualification, 0, len(qualifications))
	for _, q := range qualifications {
		qs = append(qs, tree.NamedColumnQualification{
			Name:          "",
			Qualification: q,
		})
	}
	col, err := tree.NewColumnTableDef(tree.Name(name), typ, false, qs)
	assert.NoError(t, err)
	return col
}

func TestSortTableDefs(t *testing.T) {
	tests := []struct {
		name     string
		tables   func() []*tree.CreateTable
		expected func() []*tree.CreateTable
	}{
		{
			name: "tables without references",
			tables: func() []*tree.CreateTable {
				table1 := &tree.CreateTable{
					Table: tree.MakeUnqualifiedTableName(tree.Name("users")),
					Defs: tree.TableDefs{
						newColumn(t, "id", types.Int),
					},
				}
				table2 := &tree.CreateTable{
					Table: tree.MakeUnqualifiedTableName(tree.Name("roles")),
					Defs: tree.TableDefs{
						newColumn(t, "id", types.Int),
					},
				}
				return []*tree.CreateTable{table1, table2}
			},
			expected: func() []*tree.CreateTable {
				table1 := &tree.CreateTable{
					Table: tree.MakeUnqualifiedTableName(tree.Name("users")),
					Defs: tree.TableDefs{
						newColumn(t, "id", types.Int),
					},
				}
				table2 := &tree.CreateTable{
					Table: tree.MakeUnqualifiedTableName(tree.Name("roles")),
					Defs: tree.TableDefs{
						newColumn(t, "id", types.Int),
					},
				}
				return []*tree.CreateTable{table1, table2}
			},
		},
		{
			name: "tables with references",
			tables: func() []*tree.CreateTable {
				// Table with a foreign key to users
				userRoles := &tree.CreateTable{
					Table: tree.MakeUnqualifiedTableName(tree.Name("user_roles")),
					Defs: tree.TableDefs{
						newColumn(t, "id", types.Int),
						newColumn(t, "user_id", types.Int, &tree.ColumnFKConstraint{
							Table: tree.MakeUnqualifiedTableName(tree.Name("users")),
							Col:   tree.Name("id"),
						}),
						newColumn(t, "role_id", types.Int, &tree.ColumnFKConstraint{
							Table: tree.MakeUnqualifiedTableName(tree.Name("roles")),
							Col:   tree.Name("id"),
						}),
					},
				}

				// Tables without foreign keys
				users := &tree.CreateTable{
					Table: tree.MakeUnqualifiedTableName(tree.Name("users")),
					Defs: tree.TableDefs{
						newColumn(t, "id", types.Int),
					},
				}
				roles := &tree.CreateTable{
					Table: tree.MakeUnqualifiedTableName(tree.Name("roles")),
					Defs: tree.TableDefs{
						newColumn(t, "id", types.Int),
					},
				}

				// Return in a different order than expected
				return []*tree.CreateTable{userRoles, users, roles}
			},
			expected: func() []*tree.CreateTable {
				// Expected order: first tables without references, then tables with references
				users := &tree.CreateTable{
					Table: tree.MakeUnqualifiedTableName(tree.Name("users")),
					Defs: tree.TableDefs{
						newColumn(t, "id", types.Int),
					},
				}
				roles := &tree.CreateTable{
					Table: tree.MakeUnqualifiedTableName(tree.Name("roles")),
					Defs: tree.TableDefs{
						newColumn(t, "id", types.Int),
					},
				}
				userRoles := &tree.CreateTable{
					Table: tree.MakeUnqualifiedTableName(tree.Name("user_roles")),
					Defs: tree.TableDefs{
						newColumn(t, "id", types.Int),
						newColumn(t, "user_id", types.Int, &tree.ColumnFKConstraint{
							Table: tree.MakeUnqualifiedTableName(tree.Name("users")),
							Col:   tree.Name("id"),
						}),
						newColumn(t, "role_id", types.Int, &tree.ColumnFKConstraint{
							Table: tree.MakeUnqualifiedTableName(tree.Name("roles")),
							Col:   tree.Name("id"),
						}),
					},
				}

				return []*tree.CreateTable{users, roles, userRoles}
			},
		},
		{
			name: "complex dependency chain",
			tables: func() []*tree.CreateTable {
				// Create a chain of dependencies: D -> C -> B -> A && D -> A
				tableA := &tree.CreateTable{
					Table: tree.MakeUnqualifiedTableName(tree.Name("table_a")),
					Defs: tree.TableDefs{
						&tree.ColumnTableDef{
							Name: tree.Name("id"),
							Type: types.Int,
						},
					},
				}

				tableB := &tree.CreateTable{
					Table: tree.MakeUnqualifiedTableName(tree.Name("table_b")),
					Defs: tree.TableDefs{
						newColumn(t, "id", types.Int),
						newColumn(t, "a_id", types.Int, &tree.ColumnFKConstraint{
							Table: tree.MakeUnqualifiedTableName(tree.Name("table_a")),
							Col:   tree.Name("id"),
						}),
					},
				}

				tableC := &tree.CreateTable{
					Table: tree.MakeUnqualifiedTableName(tree.Name("table_c")),
					Defs: tree.TableDefs{
						newColumn(t, "id", types.Int),
						newColumn(t, "b_id", types.Int, &tree.ColumnFKConstraint{
							Table: tree.MakeUnqualifiedTableName(tree.Name("table_b")),
							Col:   tree.Name("id"),
						}),
					},
				}

				tableD := &tree.CreateTable{
					Table: tree.MakeUnqualifiedTableName(tree.Name("table_d")),
					Defs: tree.TableDefs{
						newColumn(t, "id", types.Int),
						newColumn(t, "c_id", types.Int, &tree.ColumnFKConstraint{
							Table: tree.MakeUnqualifiedTableName(tree.Name("table_c")),
							Col:   tree.Name("id"),
						}),
						newColumn(t, "a_id", types.Int, &tree.ColumnFKConstraint{
							Table: tree.MakeUnqualifiedTableName(tree.Name("table_a")),
							Col:   tree.Name("id"),
						}),
					},
				}

				// Return in reverse order
				return []*tree.CreateTable{tableD, tableC, tableB, tableA}
			},
			expected: func() []*tree.CreateTable {
				// Expected order: A, B, C, D (dependency order)
				tableA := &tree.CreateTable{
					Table: tree.MakeUnqualifiedTableName(tree.Name("table_a")),
					Defs: tree.TableDefs{
						&tree.ColumnTableDef{
							Name: tree.Name("id"),
							Type: types.Int,
						},
					},
				}

				tableB := &tree.CreateTable{
					Table: tree.MakeUnqualifiedTableName(tree.Name("table_b")),
					Defs: tree.TableDefs{
						newColumn(t, "id", types.Int),
						newColumn(t, "a_id", types.Int, &tree.ColumnFKConstraint{
							Table: tree.MakeUnqualifiedTableName(tree.Name("table_a")),
							Col:   tree.Name("id"),
						}),
					},
				}

				tableC := &tree.CreateTable{
					Table: tree.MakeUnqualifiedTableName(tree.Name("table_c")),
					Defs: tree.TableDefs{
						newColumn(t, "id", types.Int),
						newColumn(t, "b_id", types.Int, &tree.ColumnFKConstraint{
							Table: tree.MakeUnqualifiedTableName(tree.Name("table_b")),
							Col:   tree.Name("id"),
						}),
					},
				}

				tableD := &tree.CreateTable{
					Table: tree.MakeUnqualifiedTableName(tree.Name("table_d")),
					Defs: tree.TableDefs{
						newColumn(t, "id", types.Int),
						newColumn(t, "c_id", types.Int, &tree.ColumnFKConstraint{
							Table: tree.MakeUnqualifiedTableName(tree.Name("table_c")),
							Col:   tree.Name("id"),
						}),
					},
				}

				return []*tree.CreateTable{tableA, tableB, tableC, tableD}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tables := tt.tables()
			expected := tt.expected()

			result := SortTableDefs(tables)

			// We need to verify the order is correct for dependency resolution
			assert.Equal(t, len(expected), len(result), "Result should have the same number of tables")

			// Verify the order matches our expectations
			for i, table := range result {
				assert.Equal(t, expected[i].Table.String(), table.Table.String(),
					"Table at position %d should be %s but was %s",
					i, expected[i].Table.String(), table.Table.String())
			}
		})
	}
}
