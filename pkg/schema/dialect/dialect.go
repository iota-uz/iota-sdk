package dialect

import "github.com/iota-uz/iota-sdk/pkg/schema/types"

// Dialect defines the interface for SQL dialect-specific operations
type Dialect interface {
	// ParseCreateTable parses CREATE TABLE statements
	ParseCreateTable(sql string) (*types.Node, error)

	// ParseAlterTable parses ALTER TABLE statements
	ParseAlterTable(sql string) (*types.Node, error)

	// GenerateCreate generates CREATE statements from nodes
	GenerateCreate(node *types.Node) (string, error)

	// GenerateAlter generates ALTER statements from changes
	GenerateAlter(change *types.Node) (string, error)

	// ValidateSchema validates schema compatibility
	ValidateSchema(schema *types.SchemaTree) error

	// GetDataTypeMapping returns dialect-specific type mappings
	GetDataTypeMapping() map[string]string
}

// Register registers a new SQL dialect
func Register(name string, d Dialect) {
	dialects[name] = d
}

// Get returns a dialect by name
func Get(name string) (Dialect, bool) {
	d, ok := dialects[name]
	return d, ok
}

var dialects = make(map[string]Dialect)
