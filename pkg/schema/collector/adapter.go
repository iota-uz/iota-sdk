package collector

import (
	"github.com/auxten/postgresql-parser/pkg/sql/sem/tree"
	"github.com/iota-uz/iota-sdk/pkg/schema/common"
)

func CompareTables(oldTable, newTable *tree.CreateTable) ([]interface{}, error) {
	changes := make([]interface{}, 0)

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

	for colName, newCol := range newColumns {
		if oldCol, exists := oldColumns[colName]; !exists {
			// Column was added
			changes = append(changes, &tree.AlterTableAddColumn{
				ColumnDef: newCol,
			})
		} else {
			// Column exists in both - compare types
			if oldCol.Type.String() != newCol.Type.String() {
				// Column type was changed
				changes = append(changes, &tree.AlterTableAlterColumnType{
					Column: newCol.Name,
					ToType: newCol.Type,
				})
			}
		}
	}

	// Removed columns would be detected here

	return changes, nil
}

// CollectSchemaChanges compares two schemas and generates a change set
func CollectSchemaChanges(oldSchema, newSchema *common.Schema) (*common.ChangeSet, error) {
	changes := &common.ChangeSet{
		Changes: []interface{}{},
	}

	// Check for tables in new schema
	for _, newTable := range newSchema.Tables {
		if oldTable, exists := oldSchema.Tables[newTable.Table.TableName.String()]; !exists {
			// New table was added
			changes.Changes = append(changes.Changes, &tree.CreateTable{
				IfNotExists: false,
				Table:       newTable.Table,
				Defs:        newTable.Defs,
			})
		} else {
			// Table exists in both - compare columns
			tableChanges, err := CompareTables(oldTable, newTable)
			if err != nil {
				return nil, err
			}

			changes.Changes = append(changes.Changes, tableChanges...)
		}
	}

	// Removed tables would be detected here

	return changes, nil
}

