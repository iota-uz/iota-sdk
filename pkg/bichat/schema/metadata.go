package schema

import (
	"context"
	"errors"
)

// ErrTableMetadataNotFound is returned when GetTableMetadata is called for a table that has no metadata.
var ErrTableMetadataNotFound = errors.New("table metadata not found")

// TableMetadata represents structured semantic information about a database table.
// This includes business-friendly descriptions, use cases, data quality notes,
// column-level documentation, and calculated metrics.
type TableMetadata struct {
	TableName        string            `json:"table_name"`
	TableDescription string            `json:"table_description"`
	UseCases         []string          `json:"use_cases,omitempty"`
	DataQualityNotes []string          `json:"data_quality_notes,omitempty"`
	ColumnNotes      map[string]string `json:"column_notes,omitempty"`
	Metrics          []MetricDef       `json:"metrics,omitempty"`
}

// MetricDef represents a calculated metric or business formula.
// Example: "Average Order Value" = "SUM(total_amount) / COUNT(DISTINCT order_id)"
type MetricDef struct {
	Name       string `json:"name"`       // Human-readable metric name
	Formula    string `json:"formula"`    // SQL formula or calculation
	Definition string `json:"definition"` // Business definition/explanation
}

// MetadataProvider provides access to table metadata.
// Implementations can load from files, databases, or in-memory sources.
type MetadataProvider interface {
	// GetTableMetadata returns metadata for a specific table.
	// Returns nil if table metadata is not found.
	GetTableMetadata(ctx context.Context, tableName string) (*TableMetadata, error)

	// ListMetadata returns metadata for all available tables.
	ListMetadata(ctx context.Context) ([]TableMetadata, error)
}
