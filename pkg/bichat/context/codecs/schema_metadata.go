package codecs

import (
	"fmt"
	"sort"

	"github.com/iota-uz/iota-sdk/pkg/bichat/context"
	"github.com/iota-uz/iota-sdk/pkg/bichat/schema"
)

// SchemaMetadataPayload represents structured table metadata for the context.
// This provides business-friendly documentation about database tables,
// including use cases, data quality notes, and calculated metrics.
type SchemaMetadataPayload struct {
	Tables []schema.TableMetadata `json:"tables"`
}

// SchemaMetadataCodec handles schema metadata blocks.
// This codec is used to inject structured table documentation into the agent context.
type SchemaMetadataCodec struct {
	*context.BaseCodec
}

// NewSchemaMetadataCodec creates a new schema metadata codec.
func NewSchemaMetadataCodec() *SchemaMetadataCodec {
	return &SchemaMetadataCodec{
		BaseCodec: context.NewBaseCodec("schema-metadata", "1.0.0"),
	}
}

// Validate validates the schema metadata payload.
func (c *SchemaMetadataCodec) Validate(payload any) error {
	switch v := payload.(type) {
	case SchemaMetadataPayload:
		if len(v.Tables) == 0 {
			return fmt.Errorf("schema metadata must have at least one table")
		}
		for i, table := range v.Tables {
			if table.TableName == "" {
				return fmt.Errorf("table %d missing table_name", i)
			}
		}
		return nil
	case []schema.TableMetadata:
		if len(v) == 0 {
			return fmt.Errorf("schema metadata must have at least one table")
		}
		for i, table := range v {
			if table.TableName == "" {
				return fmt.Errorf("table %d missing table_name", i)
			}
		}
		return nil
	case map[string]any:
		tables, ok := v["tables"]
		if !ok {
			return fmt.Errorf("tables field not found")
		}
		if tbls, ok := tables.([]any); !ok || len(tbls) == 0 {
			return fmt.Errorf("tables must be a non-empty array")
		}
		return nil
	default:
		return fmt.Errorf("invalid schema metadata payload type: %T", payload)
	}
}

// Canonicalize converts the payload to canonical form.
// Tables are sorted by table_name for deterministic hashing.
func (c *SchemaMetadataCodec) Canonicalize(payload any) ([]byte, error) {
	var metadata SchemaMetadataPayload

	switch v := payload.(type) {
	case SchemaMetadataPayload:
		metadata = v
	case []schema.TableMetadata:
		metadata = SchemaMetadataPayload{Tables: v}
	case map[string]any:
		// Extract tables array
		if tables, ok := v["tables"].([]any); ok {
			for _, tbl := range tables {
				if tblMap, ok := tbl.(map[string]any); ok {
					table := extractTableMetadata(tblMap)
					metadata.Tables = append(metadata.Tables, table)
				}
			}
		}
	default:
		return nil, fmt.Errorf("invalid schema metadata payload type: %T", payload)
	}

	// Sort tables by name for deterministic ordering
	sort.Slice(metadata.Tables, func(i, j int) bool {
		return metadata.Tables[i].TableName < metadata.Tables[j].TableName
	})

	return context.SortedJSONBytes(metadata)
}

// extractTableMetadata extracts a TableMetadata from a map[string]any.
func extractTableMetadata(m map[string]any) schema.TableMetadata {
	table := schema.TableMetadata{}

	if name, ok := m["table_name"].(string); ok {
		table.TableName = name
	}
	if desc, ok := m["table_description"].(string); ok {
		table.TableDescription = desc
	}

	// Extract use cases (array of strings)
	if useCases, ok := m["use_cases"].([]any); ok {
		for _, uc := range useCases {
			if ucStr, ok := uc.(string); ok {
				table.UseCases = append(table.UseCases, ucStr)
			}
		}
	}

	// Extract data quality notes (array of strings)
	if notes, ok := m["data_quality_notes"].([]any); ok {
		for _, note := range notes {
			if noteStr, ok := note.(string); ok {
				table.DataQualityNotes = append(table.DataQualityNotes, noteStr)
			}
		}
	}

	// Extract column notes (map of string -> string)
	if colNotes, ok := m["column_notes"].(map[string]any); ok {
		table.ColumnNotes = make(map[string]string)
		for col, note := range colNotes {
			if noteStr, ok := note.(string); ok {
				table.ColumnNotes[col] = noteStr
			}
		}
	}

	// Extract metrics (array of objects)
	if metrics, ok := m["metrics"].([]any); ok {
		for _, metric := range metrics {
			if metricMap, ok := metric.(map[string]any); ok {
				metricDef := schema.MetricDef{}
				if name, ok := metricMap["name"].(string); ok {
					metricDef.Name = name
				}
				if formula, ok := metricMap["formula"].(string); ok {
					metricDef.Formula = formula
				}
				if def, ok := metricMap["definition"].(string); ok {
					metricDef.Definition = def
				}
				table.Metrics = append(table.Metrics, metricDef)
			}
		}
	}

	return table
}
