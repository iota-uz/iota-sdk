package common

import (
	"github.com/iota-uz/psql-parser/sql/sem/tree"
)

// SchemaObject represents a generic schema object that can be different types
// from the postgresql-parser tree package
type SchemaObject interface{}

// Schema represents a database schema containing all objects
type Schema struct {
	Tables  map[string]*tree.CreateTable
	Indexes map[string]*tree.CreateIndex
	Columns map[string]map[string]*tree.ColumnTableDef
}

// NewSchema creates a new empty schema
func NewSchema() *Schema {
	return &Schema{
		Tables:  make(map[string]*tree.CreateTable),
		Indexes: make(map[string]*tree.CreateIndex),
		Columns: make(map[string]map[string]*tree.ColumnTableDef),
	}
}

// ChangeSet represents a collection of related schema changes
type ChangeSet struct {
	Changes   []interface{}
	Timestamp int64
	Version   string
	Hash      string
}

// NewChangeSet creates a new empty change set
func NewChangeSet() *ChangeSet {
	return &ChangeSet{
		Changes: make([]interface{}, 0),
	}
}

func HasReferences(table *tree.CreateTable) bool {
	for _, def := range table.Defs {
		if colDef, ok := def.(*tree.ColumnTableDef); ok {
			if colDef.References.Table != nil {
				return true
			}
		}
	}
	return false
}

func AllReferencesSatisfied(t *tree.CreateTable, tables []*tree.CreateTable) bool {
	for _, def := range t.Defs {
		colDef, ok := def.(*tree.ColumnTableDef)
		if !ok {
			continue
		}
		if colDef.References.Table == nil {
			continue
		}
		found := false
		for _, table := range tables {
			if table.Table.String() == colDef.References.Table.String() {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func SortTableDefs(tables []*tree.CreateTable) []*tree.CreateTable {
	var result []*tree.CreateTable
	processed := make(map[string]bool)

	for _, t := range tables {
		if !HasReferences(t) {
			result = append(result, t)
			processed[t.Table.String()] = true
		}
	}

	for {
		if len(processed) == len(tables) {
			break
		}

		for _, refTable := range tables {
			if processed[refTable.Table.String()] {
				continue
			}
			if AllReferencesSatisfied(refTable, result) {
				result = append(result, refTable)
				processed[refTable.Table.String()] = true
			}
		}

	}
	return result
}
