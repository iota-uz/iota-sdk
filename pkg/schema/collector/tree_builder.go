package collector

import (
	"fmt"
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

type columnState struct {
	node      *tree.ColumnTableDef
	timestamp int64
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
				name := n.Table.String()
				s.tables[name] = n
			case *tree.AlterTable:
				s.applyAlterTable(n, timestamp, fileName)
			case *tree.CreateIndex:
				s.updateIndexState(n, timestamp, fileName)
			case *tree.DropTable:
				for _, name := range n.Names {
					tableName := name.String()
					s.drops[strings.ToLower(tableName)] = true
				}
			}
			return true
		},
	}
	_, _ = w.Walk(stmts, nil)
}

func (s *schemaState) applyAlterTable(node *tree.AlterTable, timestamp int64, fileName string) {
	name := node.Table.String()
	if s.drops[name] {
		return
	}

	for _, cmd := range node.Cmds {
		switch c := cmd.(type) {
		case *tree.AlterTableAddColumn:
			s.updateColumnState(name, c.ColumnDef, timestamp, fileName)
		case *tree.AlterTableDropColumn:
			if _, exists := s.tables[name]; exists {
				dropColumn(s.tables[name], strings.ToLower(c.Column.String()))
			} else {
				println("    Found unknown table for column:", name)
			}
		case *tree.AlterTableAlterColumnType:
			s.updateColumnState(name, c, timestamp, fileName)
		case *tree.AlterTableDropNotNull:
			s.updateColumnState(name, c, timestamp, fileName)
		case *tree.AlterTableSetDefault:
			s.updateColumnState(name, c, timestamp, fileName)
		case *tree.AlterTableDropConstraint:
			s.updateColumnState(name, c, timestamp, fileName)
		case *tree.AlterTableAddConstraint:
			s.updateColumnState(name, c, timestamp, fileName)
		default:
			println("    Found unknown alter table command for table:", name, "type:", fmt.Sprintf("%T", c))
		}
	}
}

func (s *schemaState) updateColumnState(tableName string, cmd interface{}, timestamp int64, fileName string) {
	if def, ok := cmd.(*tree.ColumnTableDef); ok {
		if _, exists := s.tables[tableName]; !exists {
			println(fmt.Sprintf("    Found unknown column %s for table %s", def.Name, tableName))
			return
		}
		s.tables[tableName].Defs = append(s.tables[tableName].Defs, def)
		return
	}
	table, ok := s.tables[tableName]
	if !ok {
		println("    Found unknown table for column:", tableName)
		return
	}
	switch c := cmd.(type) {
	case *tree.AlterTableDropConstraint:
		// TODO: handle constraints
	case *tree.AlterTableDropNotNull:
		colName := strings.ToLower(c.Column.String())
		idx := findColumnIndex(table.Defs, colName)
		if idx == -1 {
			println("    Found unknown column for table:", tableName, "column:", colName)
			return
		}
		col := table.Defs[idx].(*tree.ColumnTableDef)
		col.Nullable.Nullability = tree.Null
	case *tree.AlterTableAlterColumnType:
		colName := strings.ToLower(c.Column.String())
		idx := findColumnIndex(table.Defs, colName)
		if idx == -1 {
			println("    Found unknown column for table:", tableName, "column:", colName)
			return
		}
		col := table.Defs[idx].(*tree.ColumnTableDef)
		col.Type = c.ToType
	case *tree.AlterTableSetDefault:
		colName := strings.ToLower(c.Column.String())
		idx := findColumnIndex(table.Defs, colName)
		if idx == -1 {
			println("    Found unknown column for table:", tableName, "column:", colName)
			return
		}
		col := table.Defs[idx].(*tree.ColumnTableDef)
		col.DefaultExpr.Expr = c.Default
	case *tree.ForeignKeyConstraintTableDef:
		for _, col := range c.FromCols {
			idx := findColumnIndex(table.Defs, strings.ToLower(col.String()))
			if idx == -1 {
				println("    Found unknown column for table:", tableName, "column:", col)
				continue
			}
			col1 := table.Defs[idx].(*tree.ColumnTableDef)
			col1.References.Table = &c.Table
			col1.References.Table = &c.Table
			col1.References.Col = col
			col1.References.ConstraintName = c.Name
			col1.References.Actions = c.Actions
			col1.References.Match = c.Match
		}
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

	for _, t := range s.tables {
		tableName := t.Table.String()
		if s.drops[tableName] {
			continue
		}

		// Create the table in the schema
		schema.Tables[tableName] = t
	}

	for name, idx := range s.indexes {
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
	var filteredDefs tree.TableDefs
	for _, def := range node.Defs {
		if col, ok := def.(*tree.ColumnTableDef); ok {
			if col.Name.Normalize() == lex.NormalizeName(colName) {
				continue
			}
		}
		filteredDefs = append(filteredDefs, def)
	}
	node.Defs = filteredDefs
}
