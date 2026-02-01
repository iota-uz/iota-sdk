package codecs

import (
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/bichat/context"
)

// TableColumn represents a database table column.
type TableColumn struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
}

// TableSchema represents a database table schema.
type TableSchema struct {
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	Columns     []TableColumn `json:"columns"`
}

// DatabaseSchemaPayload represents a database schema block (BI-specific).
type DatabaseSchemaPayload struct {
	SchemaName string        `json:"schema_name"`
	Tables     []TableSchema `json:"tables"`
}

// DatabaseSchemaCodec handles database schema blocks for BI use cases.
type DatabaseSchemaCodec struct {
	*context.BaseCodec
}

// NewDatabaseSchemaCodec creates a new database schema codec.
func NewDatabaseSchemaCodec() *DatabaseSchemaCodec {
	return &DatabaseSchemaCodec{
		BaseCodec: context.NewBaseCodec("database-schema", "1.0.0"),
	}
}

// Validate validates the database schema payload.
func (c *DatabaseSchemaCodec) Validate(payload any) error {
	switch v := payload.(type) {
	case DatabaseSchemaPayload:
		if v.SchemaName == "" {
			return fmt.Errorf("schema name cannot be empty")
		}
		if len(v.Tables) == 0 {
			return fmt.Errorf("schema must have at least one table")
		}
		for i, table := range v.Tables {
			if table.Name == "" {
				return fmt.Errorf("table %d missing name", i)
			}
			if len(table.Columns) == 0 {
				return fmt.Errorf("table %s must have at least one column", table.Name)
			}
		}
		return nil
	case map[string]any:
		if name, ok := v["schema_name"].(string); !ok || name == "" {
			return fmt.Errorf("schema name cannot be empty")
		}
		if tables, ok := v["tables"].([]any); !ok || len(tables) == 0 {
			return fmt.Errorf("schema must have at least one table")
		}
		return nil
	default:
		return fmt.Errorf("invalid database schema payload type: %T", payload)
	}
}

// Canonicalize converts the payload to canonical form.
func (c *DatabaseSchemaCodec) Canonicalize(payload any) ([]byte, error) {
	var schema DatabaseSchemaPayload

	switch v := payload.(type) {
	case DatabaseSchemaPayload:
		schema = v
	case map[string]any:
		if name, ok := v["schema_name"].(string); ok {
			schema.SchemaName = name
		}
		if tables, ok := v["tables"].([]any); ok {
			for _, t := range tables {
				if tableMap, ok := t.(map[string]any); ok {
					table := TableSchema{}
					if name, ok := tableMap["name"].(string); ok {
						table.Name = name
					}
					if desc, ok := tableMap["description"].(string); ok {
						table.Description = normalizeWhitespace(desc)
					}
					if cols, ok := tableMap["columns"].([]any); ok {
						for _, c := range cols {
							if colMap, ok := c.(map[string]any); ok {
								col := TableColumn{}
								if name, ok := colMap["name"].(string); ok {
									col.Name = name
								}
								if typ, ok := colMap["type"].(string); ok {
									col.Type = typ
								}
								if nullable, ok := colMap["nullable"].(bool); ok {
									col.Nullable = nullable
								}
								table.Columns = append(table.Columns, col)
							}
						}
					}
					schema.Tables = append(schema.Tables, table)
				}
			}
		}
	default:
		return nil, fmt.Errorf("invalid database schema payload type: %T", payload)
	}

	return context.SortedJSONBytes(schema)
}
