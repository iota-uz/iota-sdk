package collector

import (
	"log"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/schema/common"
	"github.com/iota-uz/psql-parser/sql/lex"
	"github.com/iota-uz/psql-parser/sql/parser"
	"github.com/iota-uz/psql-parser/sql/sem/tree"
	"github.com/iota-uz/psql-parser/walk"
)

type schemaState struct {
	tables  map[string]*tree.CreateTable // table -> column -> state
	indexes map[string]*indexState
	drops   map[string]bool
}

type indexState struct {
	node      *tree.CreateIndex
	timestamp int64
	lastFile  string
}

func newSchemaState() *schemaState {
	return &schemaState{
		tables:  make(map[string]*tree.CreateTable),
		indexes: make(map[string]*indexState),
		drops:   make(map[string]bool),
	}
}

func (s *schemaState) update(stmts parser.Statements, timestamp int64, fileName string) {
	w := &walk.AstWalker{
		Fn: func(ctx interface{}, node interface{}) bool {
			switch n := node.(type) {
			case *tree.CreateTable:
				name := n.Table.TableName.Normalize()
				s.tables[name] = n
			case *tree.AlterTable:
				s.applyAlterTable(n, timestamp, fileName)
			case *tree.CreateIndex:
				s.updateIndexState(n, timestamp, fileName)
			case *tree.DropTable:
				for _, name := range n.Names {
					tableName := name.TableName.Normalize()
					s.drops[tableName] = true
					delete(s.tables, tableName)
				}
			}
			return true
		},
	}
	_, _ = w.Walk(stmts, nil)
}

func getConstraintName(def tree.TableDef) (string, bool) {
	switch d := def.(type) {
	case *tree.UniqueConstraintTableDef:
		if d.Name != "" {
			return d.Name.Normalize(), true
		}
		// TODO handle other constraint types
	}
	return "", false // not a recognized named constraint type
}

func (s *schemaState) applyAlterTable(node *tree.AlterTable, timestamp int64, fileName string) {
	// Use normalized table name for lookup
	tableNameKey := node.Table.ToTableName().TableName.Normalize()

	if s.drops[tableNameKey] {
		// Table was dropped earlier, ignore ALTER commands
		return
	}

	// Get the table state - IMPORTANT: Check existence *before* the loop
	table, tableExists := s.tables[tableNameKey]
	if !tableExists {
		// This can happen if the CREATE TABLE migration was missing or used a different name/qualification
		log.Printf("WARNING: Table '%s' not found in schema state while processing ALTER command from file: %s. Skipping command.", tableNameKey, fileName)
		return
	}
	// Process each command in the AlterTable
	for _, cmd := range node.Cmds {
		switch c := cmd.(type) {
		case *tree.AlterTableAddColumn:
			newColName := c.ColumnDef.Name.Normalize()
			colExists := false
			for _, def := range table.Defs {
				if colDef, ok := def.(*tree.ColumnTableDef); ok {
					if colDef.Name.Normalize() == newColName {
						colExists = true
						break
					}
				}
			}
			if !colExists {
				table.Defs = append(table.Defs, c.ColumnDef)
			} else {
				log.Printf("WARNING: Column '%s' already exists in table '%s' state when processing ADD COLUMN from %s. Skipping add.", newColName, tableNameKey, fileName)
			}
		case *tree.AlterTableDropColumn:
			dropColumn(table, c.Column.Normalize())
		case *tree.AlterTableAlterColumnType:
			s.updateColumnState(table, c, timestamp, fileName)
		case *tree.AlterTableDropNotNull:
			s.updateColumnState(table, c, timestamp, fileName)
		case *tree.AlterTableSetDefault:
			s.updateColumnState(table, c, timestamp, fileName)
		case *tree.AlterTableDropConstraint:
			constraintNameToDrop := c.Constraint.Normalize()
			found := false
			newDefs := make(tree.TableDefs, 0, len(table.Defs))
			for _, def := range table.Defs {
				name, isNamedConstraint := getConstraintName(def)
				if isNamedConstraint && name == constraintNameToDrop {
					// Found the constraint by name, skip adding it to newDefs
					found = true
				} else {
					// Keep this definition
					newDefs = append(newDefs, def)
				}
			}
			if !found {
				// This might happen if constraint was already dropped or name mismatch
				log.Printf("WARNING: Constraint '%s' to drop was not found in table '%s' state when processing %s.", constraintNameToDrop, tableNameKey, fileName)
			}
			table.Defs = newDefs // Update the definitions slice

		case *tree.AlterTableAddConstraint:
			table.Defs = append(table.Defs, c.ConstraintDef)
		default:
			log.Printf("Found unknown alter table command for table: %s type: %T", tableNameKey, c)
		}
	}
}

func (s *schemaState) updateColumnState(table *tree.CreateTable, cmd interface{}, timestamp int64, fileName string) {
	tableName := table.Table.TableName.Normalize()

	switch c := cmd.(type) {
	case *tree.AlterTableDropNotNull:
		colName := c.Column.Normalize()
		idx := findColumnIndex(table.Defs, colName)
		if idx == -1 {
			log.Printf("WARNING: Column '%s' not found in table '%s' state for DropNotNull from %s.", colName, tableName, fileName)
			return
		}
		// Ensure it's actually a ColumnTableDef before asserting type
		if col, ok := table.Defs[idx].(*tree.ColumnTableDef); ok {
			col.Nullable.Nullability = tree.Null
		} else {
			log.Printf("ERROR: Definition found for '%s' in table '%s' is not a ColumnTableDef (%T).", colName, tableName, table.Defs[idx])
		}

	case *tree.AlterTableAlterColumnType:
		colName := c.Column.Normalize()
		idx := findColumnIndex(table.Defs, colName)
		if idx == -1 {
			log.Printf("WARNING: Column '%s' not found in table '%s' state for AlterColumnType from %s.", colName, tableName, fileName)
			return
		}
		if col, ok := table.Defs[idx].(*tree.ColumnTableDef); ok {
			col.Type = c.ToType
		} else {
			log.Printf("ERROR: Definition found for '%s' in table '%s' is not a ColumnTableDef (%T).", colName, tableName, table.Defs[idx])
		}

	case *tree.AlterTableSetDefault:
		colName := c.Column.Normalize()
		idx := findColumnIndex(table.Defs, colName)
		if idx == -1 {
			log.Printf("WARNING: Column '%s' not found in table '%s' state for SetDefault from %s.", colName, tableName, fileName)
			return
		}
		if col, ok := table.Defs[idx].(*tree.ColumnTableDef); ok {
			col.DefaultExpr.Expr = c.Default
		} else {
			log.Printf("ERROR: Definition found for '%s' in table '%s' is not a ColumnTableDef (%T).", colName, tableName, table.Defs[idx])
		}

		// Removed ForeignKeyConstraintTableDef case as it's not an ALTER command type
		// FKs are added via AlterTableAddConstraint with a ForeignKeyConstraintTableDef inside
	default:
		// This case should ideally not be reached if routing in applyAlterTable is correct
		log.Printf("WARNING: Unhandled command type '%T' passed to updateColumnState for table '%s' from file %s.", c, tableName, fileName)
	}
}

func (s *schemaState) updateIndexState(node *tree.CreateIndex, timestamp int64, fileName string) {
	indexName := strings.ToLower(node.Name.String())
	s.indexes[indexName] = &indexState{
		node:      node,
		timestamp: timestamp,
		lastFile:  fileName,
	}
}

func (s *schemaState) buildSchema() *common.Schema {
	schema := common.NewSchema()

	for tableName, t := range s.tables {
		// Check against drops using the same normalized key
		if s.drops[tableName] {
			continue
		}

		// Add table to the final schema
		schema.Tables[tableName] = t
	}

	for name, idx := range s.indexes {
		// For now, assume indexes persist unless explicitly dropped (DROP INDEX not handled yet)
		schema.Indexes[name] = idx.node
	}

	return schema
}

func findColumnIndex(defs tree.TableDefs, colName string) int {
	for i, def := range defs {
		if col, ok := def.(*tree.ColumnTableDef); ok {
			if col.Name.Normalize() == lex.NormalizeName(colName) {
				return i
			}
		}
	}
	return -1
}

func dropColumn(node *tree.CreateTable, colName string) {
	newDefs := make(tree.TableDefs, 0, len(node.Defs))
	found := false
	for _, def := range node.Defs { // Iterate original
		if col, ok := def.(*tree.ColumnTableDef); ok {
			// If it's the target column...
			if col.Name.Normalize() == lex.NormalizeName(colName) {
				found = true
			} else {
				newDefs = append(newDefs, def)
			}
		} else {
			newDefs = append(newDefs, def)
		}
	}
	if found {
		node.Defs = newDefs
	} else {
		// Added a warning for safety
		log.Printf("WARNING: Column '%s' to drop was not found in table '%s' definition.", lex.NormalizeName(colName), node.Table.TableName.Normalize())
	}
}
