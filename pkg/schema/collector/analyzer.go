package collector

import (
	"time"

	"github.com/iota-uz/iota-sdk/pkg/schema/common"
	"github.com/iota-uz/psql-parser/sql/sem/tree"
)

func CompareTables(oldTable, newTable *tree.CreateTable) ([]interface{}, []interface{}, error) {
	upChanges := make([]interface{}, 0)
	downChanges := make([]interface{}, 0)

	// Compare columns
	oldColumns := make(map[string]*tree.ColumnTableDef)
	for _, colNode := range oldTable.Defs {
		if colDef, ok := colNode.(*tree.ColumnTableDef); ok {
			oldColumns[colDef.Name.String()] = colDef
		}
	}

	newColumns := make(map[string]*tree.ColumnTableDef)
	for _, colNode := range newTable.Defs {
		if colDef, ok := colNode.(*tree.ColumnTableDef); ok {
			newColumns[colDef.Name.String()] = colDef
		}
	}

	// iterate over the array instead of map to preserve order
	for _, def := range newTable.Defs {
		newCol, ok := def.(*tree.ColumnTableDef)
		if !ok {
			continue
		}
		colName := newCol.Name.String()
		if oldCol, exists := oldColumns[colName]; !exists {
			// Column was added (up operation)
			tableName, _ := tree.NewUnresolvedObjectName(
				1, /* number of parts */
				[3]string{newTable.Table.Table()},
				0, /* no annotation */
			)
			addColumn := &tree.AlterTable{
				Table: tableName,
				Cmds: tree.AlterTableCmds{
					&tree.AlterTableAddColumn{
						ColumnDef: newCol,
					},
				},
			}
			upChanges = append(upChanges, addColumn)

			// Corresponding down operation: drop the column
			dropTableName, _ := tree.NewUnresolvedObjectName(
				1, /* number of parts */
				[3]string{newTable.Table.Table()},
				0, /* no annotation */
			)
			dropColumn := &tree.AlterTable{
				Table: dropTableName,
				Cmds: tree.AlterTableCmds{
					&tree.AlterTableDropColumn{
						Column:   newCol.Name,
						IfExists: true,
					},
				},
			}
			downChanges = append(downChanges, dropColumn)
		} else {
			// Column exists in both - compare types
			if oldCol.Type.String() != newCol.Type.String() {
				// Column type was changed (up operation)
				upTableName, _ := tree.NewUnresolvedObjectName(
					1, /* number of parts */
					[3]string{newTable.Table.Table()},
					0, /* no annotation */
				)
				alterTypeUp := &tree.AlterTable{
					Table: upTableName,
					Cmds: tree.AlterTableCmds{
						&tree.AlterTableAlterColumnType{
							Column: newCol.Name,
							ToType: newCol.Type,
						},
					},
				}
				upChanges = append(upChanges, alterTypeUp)

				// Corresponding down operation: revert to original type
				downTableName, _ := tree.NewUnresolvedObjectName(
					1, /* number of parts */
					[3]string{oldTable.Table.Table()},
					0, /* no annotation */
				)
				alterTypeDown := &tree.AlterTable{
					Table: downTableName,
					Cmds: tree.AlterTableCmds{
						&tree.AlterTableAlterColumnType{
							Column: oldCol.Name,
							ToType: oldCol.Type,
						},
					},
				}
				downChanges = append(downChanges, alterTypeDown)
			}
		}
	}

	// Detect removed columns
	for _, def := range oldTable.Defs {
		oldCol, ok := def.(*tree.ColumnTableDef)
		if !ok {
			continue
		}
		if _, exists := newColumns[oldCol.Name.String()]; !exists {
			// Column was removed (up operation)
			tableName, _ := tree.NewUnresolvedObjectName(
				1, /* number of parts */
				[3]string{oldTable.Table.Table()},
				0, /* no annotation */
			)
			dropColumn := &tree.AlterTable{
				Table: tableName,
				Cmds: tree.AlterTableCmds{
					&tree.AlterTableDropColumn{
						Column:   oldCol.Name,
						IfExists: true,
					},
				},
			}
			upChanges = append(upChanges, dropColumn)

			// Corresponding down operation: add the column back
			downTableName, _ := tree.NewUnresolvedObjectName(
				1, /* number of parts */
				[3]string{oldTable.Table.Table()},
				0, /* no annotation */
			)
			addColumn := &tree.AlterTable{
				Table: downTableName,
				Cmds: tree.AlterTableCmds{
					&tree.AlterTableAddColumn{
						ColumnDef: oldCol,
					},
				},
			}
			downChanges = append(downChanges, addColumn)
		}
	}

	return upChanges, downChanges, nil
}

// CollectSchemaChanges compares two schemas and generates both up and down change sets
func CollectSchemaChanges(oldSchema, newSchema *common.Schema) (*common.ChangeSet, *common.ChangeSet, error) {
	timestamp := time.Now().Unix()

	upChanges := &common.ChangeSet{
		Changes:   []interface{}{},
		Timestamp: timestamp,
	}

	downChanges := &common.ChangeSet{
		Changes:   []interface{}{},
		Timestamp: timestamp,
	}

	newSchemaTables := make([]*tree.CreateTable, 0, len(newSchema.Tables))
	for _, t := range newSchema.Tables {
		newSchemaTables = append(newSchemaTables, t)
	}
	newSchemaTables = common.SortTableDefs(newSchemaTables)

	// Check for tables in new schema
	for _, newTable := range newSchemaTables {
		tableName := newTable.Table.Table()
		if oldTable, exists := oldSchema.Tables[tableName]; !exists {
			// New table was added (up operation)
			createTable := &tree.CreateTable{
				IfNotExists: false,
				Table:       newTable.Table,
				Defs:        newTable.Defs,
			}
			upChanges.Changes = append(upChanges.Changes, createTable)

			// Corresponding down operation: drop the table
			dropTable := &tree.DropTable{
				Names:        tree.TableNames{newTable.Table},
				IfExists:     true,
				DropBehavior: tree.DropCascade,
			}
			downChanges.Changes = append(downChanges.Changes, dropTable)
		} else {
			// Table exists in both - compare columns
			tableUpChanges, tableDownChanges, err := CompareTables(oldTable, newTable)
			if err != nil {
				return nil, nil, err
			}
			upChanges.Changes = append(upChanges.Changes, tableUpChanges...)
			downChanges.Changes = append(downChanges.Changes, tableDownChanges...)
		}
	}

	// Check for new indexes
	for indexName, newIndex := range newSchema.Indexes {
		if _, exists := oldSchema.Indexes[indexName]; !exists {
			upChanges.Changes = append(upChanges.Changes, newIndex)

			downChanges.Changes = append(downChanges.Changes, &tree.DropIndex{
				IndexList: tree.TableIndexNames{
					&tree.TableIndexName{
						Table: newIndex.Table,
						Index: tree.UnrestrictedName(newIndex.Name),
					},
				},
			})
		}
	}

	// Check for removed tables
	for tableName, oldTable := range oldSchema.Tables {
		if _, exists := newSchema.Tables[tableName]; !exists {
			upChanges.Changes = append(upChanges.Changes, &tree.DropTable{
				Names:        tree.TableNames{oldTable.Table},
				IfExists:     true,
				DropBehavior: tree.DropCascade,
			})

			downChanges.Changes = append(downChanges.Changes, &tree.CreateTable{
				IfNotExists: false,
				Table:       oldTable.Table,
				Defs:        oldTable.Defs,
			})
		}
	}

	// Ensure the down migrations are in the reverse order of the up migrations
	reversedDownChanges := make([]interface{}, len(downChanges.Changes))
	for i, change := range downChanges.Changes {
		reversedDownChanges[len(downChanges.Changes)-1-i] = change
	}
	downChanges.Changes = reversedDownChanges

	return upChanges, downChanges, nil
}
