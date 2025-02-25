package dialect

import (
	"github.com/iota-uz/iota-sdk/pkg/schema/common"
)

// Dialect defines the interface for SQL dialect-specific operations
type Dialect interface {
	// GenerateCreate generates CREATE statements
	GenerateCreate(obj interface{}) (string, error)

	// GenerateAlter generates ALTER statements
	GenerateAlter(obj interface{}) (string, error)

	// ValidateSchema validates schema compatibility
	ValidateSchema(schema *common.Schema) error

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

// ClearDialects removes all registered dialects (primarily for testing)
func ClearDialects() {
	dialects = make(map[string]Dialect)
}
