package collector

import (
	"fmt"
	"strings"
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

	// Extract constraints
	oldConstraints := extractConstraints(oldTable.Defs, oldTable.Table.Table())
	newConstraints := extractConstraints(newTable.Defs, newTable.Table.Table())

	// Compare constraints
	droppedConstraints, addedConstraints := compareConstraints(oldConstraints, newConstraints)

	// Generate ALTER TABLE commands for dropped constraints
	for _, constraint := range droppedConstraints {
		tableName, _ := tree.NewUnresolvedObjectName(
			1, /* number of parts */
			[3]string{oldTable.Table.Table()},
			0, /* no annotation */
		)
		dropConstraint := &tree.AlterTable{
			Table: tableName,
			Cmds: tree.AlterTableCmds{
				&tree.AlterTableDropConstraint{
					Constraint: tree.Name(constraint.Name),
					IfExists:   true,
				},
			},
		}
		upChanges = append(upChanges, dropConstraint)

		// Corresponding down operation: add the constraint back
		downTableName, _ := tree.NewUnresolvedObjectName(
			1, /* number of parts */
			[3]string{oldTable.Table.Table()},
			0, /* no annotation */
		)
		addConstraint := &tree.AlterTable{
			Table: downTableName,
			Cmds: tree.AlterTableCmds{
				&tree.AlterTableAddConstraint{
					ConstraintDef: constraint.Def,
				},
			},
		}
		downChanges = append(downChanges, addConstraint)
	}

	// Generate ALTER TABLE commands for added constraints
	for _, constraint := range addedConstraints {
		tableName, _ := tree.NewUnresolvedObjectName(
			1, /* number of parts */
			[3]string{newTable.Table.Table()},
			0, /* no annotation */
		)
		addConstraint := &tree.AlterTable{
			Table: tableName,
			Cmds: tree.AlterTableCmds{
				&tree.AlterTableAddConstraint{
					ConstraintDef: constraint.Def,
				},
			},
		}
		upChanges = append(upChanges, addConstraint)

		// Corresponding down operation: drop the constraint
		downTableName, _ := tree.NewUnresolvedObjectName(
			1, /* number of parts */
			[3]string{newTable.Table.Table()},
			0, /* no annotation */
		)
		dropConstraint := &tree.AlterTable{
			Table: downTableName,
			Cmds: tree.AlterTableCmds{
				&tree.AlterTableDropConstraint{
					Constraint: tree.Name(constraint.Name),
					IfExists:   true,
				},
			},
		}
		downChanges = append(downChanges, dropConstraint)
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
	var err error
	newSchemaTables, err = common.SortTableDefs(newSchemaTables)
	if err != nil {
		return nil, nil, err
	}

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

type constraintInfo struct {
	Name string
	Def  tree.ConstraintTableDef
}

func extractConstraints(defs tree.TableDefs, tableName string) map[string]constraintInfo {
	constraints := make(map[string]constraintInfo)
	for _, def := range defs {
		switch c := def.(type) {
		case *tree.UniqueConstraintTableDef:
			name := c.Name
			if name == "" {
				name = tree.Name(generateConstraintName(tableName, "key", c.Columns)) // !important need to be _key suffix in schemas
			}
			constraints[name.String()] = constraintInfo{Name: name.String(), Def: c}
		case *tree.ColumnTableDef:
			if c.Unique {
				name := fmt.Sprintf("%s_%s_key", tableName, c.Name)
				constraints[name] = constraintInfo{Name: name, Def: &tree.UniqueConstraintTableDef{
					IndexTableDef: tree.IndexTableDef{
						Name:    tree.Name(name),
						Columns: []tree.IndexElem{{Column: c.Name}},
					},
				}}
			}
			// TODO handle other constraints like PRIMARY SHARED, CHECK, FOREIGN KEY, etc.
			// etc c.PrimaryKey.IsPrimaryKey
		}
	}
	return constraints
}

func compareConstraints(oldConstraints, newConstraints map[string]constraintInfo) ([]constraintInfo, []constraintInfo) {
	dropped := make([]constraintInfo, 0, len(oldConstraints))
	added := make([]constraintInfo, 0, len(newConstraints))
	for name, constraint := range oldConstraints {
		if _, exists := newConstraints[name]; !exists {
			dropped = append(dropped, constraint)
		}
	}
	for name, constraint := range newConstraints {
		if _, exists := oldConstraints[name]; !exists {
			added = append(added, constraint)
		}
	}
	return dropped, added
}

func generateConstraintName(tableName, constraintType string, columns []tree.IndexElem) string {
	columnNames := make([]string, 0, len(columns))
	for _, col := range columns {
		columnNames = append(columnNames, col.Column.String())
	}
	return fmt.Sprintf("%s_%s_%s", tableName, strings.Join(columnNames, "_"), constraintType)
}
