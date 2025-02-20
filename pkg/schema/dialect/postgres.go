package dialect

import (
	"fmt"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/schema/types"
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

func (d *PostgresDialect) GenerateCreate(node *types.Node) (string, error) {
	if node.Type != types.NodeTable {
		return "", fmt.Errorf("expected table node, got %s", node.Type)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "CREATE TABLE IF NOT EXISTS %s (\n", node.Name)

	// Generate columns
	for i, col := range node.Children {
		if col.Type != types.NodeColumn {
			continue
		}
		if i > 0 {
			b.WriteString(",\n")
		}
		colDef := d.generateColumnDefinition(col)
		b.WriteString("  " + colDef)
	}

	// Generate constraints
	constraints := d.generateConstraints(node)
	if constraints != "" {
		b.WriteString(",\n  " + constraints)
	}

	b.WriteString("\n);")
	return b.String(), nil
}

func (d *PostgresDialect) GenerateAlter(node *types.Node) (string, error) {
	if node.Type != types.NodeTable {
		return "", fmt.Errorf("expected table node, got %s", node.Type)
	}

	var statements []string
	tableName := node.Name

	// Get alteration type from metadata
	alterationType, hasAlteration := node.Metadata["alteration"].(string)
	if !hasAlteration {
		return "", fmt.Errorf("no alteration type specified in metadata")
	}

	// Handle different types of alterations
	if strings.Contains(strings.ToUpper(alterationType), "ADD COLUMN") {
		// Process column additions
		for _, child := range node.Children {
			if child.Type == types.NodeColumn {
				colDef := d.generateColumnDefinition(child)
				if colDef != "" {
					stmt := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s", tableName, colDef)
					statements = append(statements, stmt)
				}
			}
		}
	} else if strings.Contains(strings.ToUpper(alterationType), "ALTER COLUMN") {
		// Process column modifications
		for _, child := range node.Children {
			if child.Type == types.NodeColumn {
				colDef := d.generateColumnDefinition(child)
				if colDef != "" {
					stmt := fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s", tableName, colDef)
					statements = append(statements, stmt)
				}
			}
		}
	}

	// Join all statements and add semicolons
	if len(statements) > 0 {
		return strings.Join(statements, ";\n") + ";", nil
	}

	return "", nil
}

func (d *PostgresDialect) ValidateSchema(schema *types.SchemaTree) error {
	// Validate PostgreSQL specific constraints
	// Check type compatibility
	// Verify constraint definitions
	return nil
}

func (d *PostgresDialect) GetDataTypeMapping() map[string]string {
	return d.typeMapping
}

func (d *PostgresDialect) generateColumnDefinition(col *types.Node) string {
	if col == nil {
		return ""
	}

	// Use the full definition if available
	if def, ok := col.Metadata["definition"].(string); ok && def != "" {
		return def
	}

	// Fallback to constructing the definition
	var b strings.Builder
	b.WriteString(col.Name)
	b.WriteString(" ")

	dataType := col.Metadata["type"].(string)
	if mappedType, ok := d.typeMapping[strings.ToLower(dataType)]; ok {
		dataType = mappedType
	}
	b.WriteString(dataType)

	// Add any constraints
	if constraints, ok := col.Metadata["constraints"].(string); ok && constraints != "" {
		b.WriteString(" ")
		b.WriteString(constraints)
	}

	return b.String()
}

func (d *PostgresDialect) generateConstraints(node *types.Node) string {
	if node == nil {
		return ""
	}

	constraints := []string{}
	for _, child := range node.Children {
		if child.Type == types.NodeConstraint {
			constraints = append(constraints, child.Name)
		}
	}

	return strings.Join(constraints, ",\n")
}

func init() {
	Register("postgres", NewPostgresDialect())
}
