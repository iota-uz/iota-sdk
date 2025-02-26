package common

import (
	"github.com/auxten/postgresql-parser/pkg/sql/sem/tree"
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
	Metadata  map[string]interface{}
}

// NewChangeSet creates a new empty change set
func NewChangeSet() *ChangeSet {
	return &ChangeSet{
		Changes:  make([]interface{}, 0),
		Metadata: make(map[string]interface{}),
	}
}

