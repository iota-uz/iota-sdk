package sql

import (
	"context"
)

// SchemaLister lists all available tables and views in a schema.
type SchemaLister interface {
	// SchemaList returns a list of all available tables with basic metadata
	SchemaList(ctx context.Context) ([]TableInfo, error)
}

// SchemaDescriber provides detailed schema information for tables.
type SchemaDescriber interface {
	// SchemaDescribe returns detailed schema information for a specific table
	SchemaDescribe(ctx context.Context, tableName string) (*TableSchema, error)
}

// TableInfo provides basic metadata about a database table
type TableInfo struct {
	Name        string // Table name
	Schema      string // Database schema (e.g., "public", "finance")
	RowCount    int64  // Approximate row count
	Description string // Optional table description/comment
}

// TableSchema provides detailed schema information for a table
type TableSchema struct {
	Name        string
	Schema      string
	Description string
	Columns     []ColumnInfo
	Indexes     []IndexInfo
	PrimaryKey  []string // Column names in primary key
	ForeignKeys []ForeignKeyInfo
}

// ColumnInfo provides metadata about a table column
type ColumnInfo struct {
	Name         string
	Type         string // SQL data type (e.g., "integer", "varchar(255)")
	Nullable     bool
	DefaultValue *string
	Description  string
	IsPrimaryKey bool
	IsForeignKey bool
}

// IndexInfo provides metadata about a table index
type IndexInfo struct {
	Name      string
	Columns   []string
	IsUnique  bool
	IsPrimary bool
}

// ForeignKeyInfo provides metadata about a foreign key relationship
type ForeignKeyInfo struct {
	Name             string
	Column           string
	ReferencedTable  string
	ReferencedColumn string
}
