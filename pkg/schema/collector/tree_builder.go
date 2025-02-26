package collector

import (
	"strings"

	"github.com/auxten/postgresql-parser/pkg/sql/parser"
	"github.com/auxten/postgresql-parser/pkg/sql/sem/tree"
	"github.com/auxten/postgresql-parser/pkg/walk"
	"github.com/iota-uz/iota-sdk/pkg/schema/common"
)

type schemaState struct {
	tables  map[string]map[string]*columnState // table -> column -> state
	indexes map[string]*indexState
	drops   map[string]bool
}

type columnState struct {
	node      *tree.ColumnTableDef
	timestamp int64
	type_     string
	lastFile  string
}

type indexState struct {
	node      *tree.CreateIndex
	timestamp int64
	lastFile  string
}

func newSchemaState() *schemaState {
	return &schemaState{
		tables:  make(map[string]map[string]*columnState),
		indexes: make(map[string]*indexState),
		drops:   make(map[string]bool),
	}
}

func (s *schemaState) update(stmts parser.Statements, timestamp int64, fileName string) {
	w := &walk.AstWalker{
		Fn: func(ctx interface{}, node interface{}) bool {
			switch n := node.(type) {
			case *tree.CreateTable:
				s.updateTableState(n, timestamp, fileName)
			case *tree.CreateIndex:
				s.updateIndexState(n, timestamp, fileName)
			case *tree.DropTable:
				for _, name := range n.Names {
					s.drops[strings.ToLower(name.String())] = true
				}
			}
			return true
		},
	}
	_, _ = w.Walk(stmts, nil)
}

func (s *schemaState) updateTableState(node *tree.CreateTable, timestamp int64, fileName string) {
	tableName := strings.ToLower(node.Table.String())

	// Handle dropped tables
	if s.drops[tableName] {
		return
	}

	if _, exists := s.tables[tableName]; !exists {
		s.tables[tableName] = make(map[string]*columnState)
	}

	for _, def := range node.Defs {
		switch d := def.(type) {
		case *tree.ColumnTableDef:
			s.updateColumnState(tableName, d, timestamp, fileName)
		}
	}
}

func (s *schemaState) updateColumnState(tableName string, col *tree.ColumnTableDef, timestamp int64, fileName string) {
	colName := strings.ToLower(col.Name.String())
	currentState := s.tables[tableName][colName]

	newType := col.Type.String()

	if shouldUpdateColumn(currentState, timestamp, newType) {
		s.tables[tableName][colName] = &columnState{
			node:      col,
			timestamp: timestamp,
			type_:     newType,
			lastFile:  fileName,
		}
	}
}

func (s *schemaState) updateIndexState(node *tree.CreateIndex, timestamp int64, fileName string) {
	indexName := strings.ToLower(node.Name.String())
	currentState := s.indexes[indexName]

	if shouldUpdateIndex(currentState, timestamp) {
		s.indexes[indexName] = &indexState{
			node:      node,
			timestamp: timestamp,
			lastFile:  fileName,
		}
	}
}

func (s *schemaState) buildSchema() *common.Schema {
	schema := common.NewSchema()

	// Add tables
	for tableName, columns := range s.tables {
		if s.drops[tableName] {
			continue
		}

		defs := make(tree.TableDefs, 0, len(columns))
		for _, state := range columns {
			defs = append(defs, state.node)
		}

		schema.Tables[tableName] = &tree.CreateTable{
			IfNotExists: false,
			Table:       tree.MakeUnqualifiedTableName(tree.Name(tableName)),
			Defs:        defs,
		}
	}

	// Add indexes
	for _, state := range s.indexes {
		schema.Indexes[strings.ToLower(state.node.Name.String())] = state.node
	}

	return schema
}

// Helper functions

func shouldUpdateColumn(current *columnState, newTimestamp int64, newType string) bool {
	return current == nil || (newTimestamp > current.timestamp && newType != current.type_)
}

func shouldUpdateIndex(current *indexState, newTimestamp int64) bool {
	return current == nil || newTimestamp > current.timestamp
}
