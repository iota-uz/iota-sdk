package dialect

import (
	"fmt"

	"github.com/auxten/postgresql-parser/pkg/sql/sem/tree"
	"github.com/iota-uz/iota-sdk/pkg/schema/common"
)

// PostgresDialect implements the Dialect interface for PostgreSQL
type PostgresDialect struct {
	typeMapping map[string]string
}

func NewPostgresDialect() *PostgresDialect {
	return &PostgresDialect{
		typeMapping: map[string]string{
			"int":       "integer",
			"varchar":   "character varying",
			"datetime":  "timestamp",
			"timestamp": "timestamp with time zone",
			"bool":      "boolean",
		},
	}
}

func (d *PostgresDialect) GenerateCreate(obj interface{}) (string, error) {
	switch node := obj.(type) {
	case *tree.CreateTable:
		// Use the SQL string representation directly
		return node.String(), nil
		
	case *tree.CreateIndex:
		// Use the SQL string representation directly
		return node.String(), nil
		
	default:
		return "", fmt.Errorf("unsupported object type for GenerateCreate: %T", obj)
	}
}

func (d *PostgresDialect) GenerateAlter(obj interface{}) (string, error) {
	switch node := obj.(type) {
	case *tree.AlterTable:
		// Use the SQL string representation directly
		return node.String(), nil
		
	case *tree.ColumnTableDef:
		// We're dealing with a column definition for an ALTER TABLE statement
		return node.String(), nil
		
	default:
		return "", fmt.Errorf("unsupported object type for GenerateAlter: %T", obj)
	}
}

func (d *PostgresDialect) ValidateSchema(schema *common.Schema) error {
	// Validate PostgreSQL specific constraints
	// Check type compatibility
	// Verify constraint definitions
	return nil
}

func (d *PostgresDialect) GetDataTypeMapping() map[string]string {
	return d.typeMapping
}

func init() {
	Register("postgres", NewPostgresDialect())
}
