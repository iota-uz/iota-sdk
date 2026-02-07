package schema

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// FileMetadataProvider reads table metadata from JSON files in a directory.
// Each file should be named {table_name}.json and contain a single TableMetadata object.
//
// Example directory structure:
//
//	/metadata/
//	  orders.json
//	  customers.json
//	  products.json
type FileMetadataProvider struct {
	dirPath string
	cache   map[string]*TableMetadata
}

// NewFileMetadataProvider creates a new file-based metadata provider.
// The directory path should contain JSON files named {table_name}.json.
//
// Example:
//
//	provider, err := schema.NewFileMetadataProvider("/var/lib/bichat/metadata")
func NewFileMetadataProvider(dirPath string) (*FileMetadataProvider, error) {
	const op serrors.Op = "FileMetadataProvider.New"

	// Validate directory exists
	info, err := os.Stat(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, serrors.E(op, serrors.NotFound, fmt.Sprintf("directory not found: %s", dirPath))
		}
		return nil, serrors.E(op, err)
	}

	if !info.IsDir() {
		return nil, serrors.E(op, serrors.KindValidation, fmt.Sprintf("path is not a directory: %s", dirPath))
	}

	provider := &FileMetadataProvider{
		dirPath: dirPath,
		cache:   make(map[string]*TableMetadata),
	}

	// Preload all metadata files into cache
	if err := provider.loadAll(); err != nil {
		return nil, serrors.E(op, err)
	}

	return provider, nil
}

// loadAll loads all JSON files from the directory into the cache.
func (p *FileMetadataProvider) loadAll() error {
	const op serrors.Op = "FileMetadataProvider.loadAll"

	entries, err := os.ReadDir(p.dirPath)
	if err != nil {
		return serrors.E(op, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Only process .json files
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		// Extract table name from filename (remove .json extension)
		tableName := strings.TrimSuffix(entry.Name(), ".json")

		// Load and parse file
		filePath := filepath.Join(p.dirPath, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			return serrors.E(op, err, fmt.Sprintf("failed to read %s", entry.Name()))
		}

		var metadata TableMetadata
		if err := json.Unmarshal(data, &metadata); err != nil {
			return serrors.E(op, err, fmt.Sprintf("failed to parse %s", entry.Name()))
		}

		// Validate table_name matches filename
		if metadata.TableName == "" {
			metadata.TableName = tableName
		} else if metadata.TableName != tableName {
			return serrors.E(op, serrors.KindValidation,
				fmt.Sprintf("table_name mismatch in %s: expected %s, got %s",
					entry.Name(), tableName, metadata.TableName))
		}

		p.cache[tableName] = &metadata
	}

	return nil
}

// GetTableMetadata returns metadata for a specific table.
// Returns nil if table metadata is not found.
func (p *FileMetadataProvider) GetTableMetadata(ctx context.Context, tableName string) (*TableMetadata, error) {
	// Lookup in cache (case-sensitive)
	if metadata, ok := p.cache[tableName]; ok {
		return metadata, nil
	}

	// Not found
	return nil, nil
}

// ListMetadata returns metadata for all available tables.
func (p *FileMetadataProvider) ListMetadata(ctx context.Context) ([]TableMetadata, error) {
	result := make([]TableMetadata, 0, len(p.cache))
	for _, metadata := range p.cache {
		result = append(result, *metadata)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].TableName < result[j].TableName
	})
	return result, nil
}
