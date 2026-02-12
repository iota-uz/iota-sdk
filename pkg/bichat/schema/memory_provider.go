package schema

import (
	"context"
	"sort"
)

// MemoryMetadataProvider is an in-memory metadata provider.
// Useful for testing or when metadata is loaded from a database.
type MemoryMetadataProvider struct {
	metadata map[string]*TableMetadata
}

// NewMemoryMetadataProvider creates a new in-memory metadata provider.
//
// Example:
//
//	metadata := []TableMetadata{
//	    {TableName: "orders", TableDescription: "Customer orders"},
//	    {TableName: "customers", TableDescription: "Customer information"},
//	}
//	provider := schema.NewMemoryMetadataProvider(metadata)
func NewMemoryMetadataProvider(metadata []TableMetadata) *MemoryMetadataProvider {
	provider := &MemoryMetadataProvider{
		metadata: make(map[string]*TableMetadata, len(metadata)),
	}

	// Build lookup map
	for i := range metadata {
		// Use pointer to original slice element to avoid copying
		provider.metadata[metadata[i].TableName] = &metadata[i]
	}

	return provider
}

// GetTableMetadata returns metadata for a specific table.
// Returns nil if table metadata is not found.
func (p *MemoryMetadataProvider) GetTableMetadata(ctx context.Context, tableName string) (*TableMetadata, error) {
	if metadata, ok := p.metadata[tableName]; ok {
		return metadata, nil
	}
	return nil, ErrTableMetadataNotFound
}

// ListMetadata returns metadata for all available tables.
func (p *MemoryMetadataProvider) ListMetadata(ctx context.Context) ([]TableMetadata, error) {
	result := make([]TableMetadata, 0, len(p.metadata))
	for _, metadata := range p.metadata {
		result = append(result, *metadata)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].TableName < result[j].TableName
	})
	return result, nil
}
