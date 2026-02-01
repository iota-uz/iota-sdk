package kb

import (
	"context"
	"time"
)

// KBIndexer builds and maintains the knowledge base index.
// Implementations must be thread-safe.
type KBIndexer interface {
	// IndexDocument adds or updates a single document in the index.
	// If a document with the same ID exists, it will be replaced.
	IndexDocument(ctx context.Context, doc Document) error

	// IndexDocuments adds or updates multiple documents in a batch.
	// This is more efficient than calling IndexDocument multiple times.
	IndexDocuments(ctx context.Context, docs []Document) error

	// DeleteDocument removes a document from the index by ID.
	// Returns nil if the document doesn't exist.
	DeleteDocument(ctx context.Context, id string) error

	// Rebuild completely rebuilds the index from a document source.
	// This is a blocking operation that clears the existing index.
	Rebuild(ctx context.Context, source DocumentSource) error

	// GetStats returns current index statistics.
	GetStats() IndexStats

	// Close releases index resources.
	// The index cannot be used after Close is called.
	Close() error
}

// IndexStats contains statistics about the knowledge base index.
type IndexStats struct {
	// DocumentCount is the total number of indexed documents
	DocumentCount uint64
	// IndexSize is the approximate index size in bytes
	IndexSize int64
	// LastUpdated is the timestamp of the last index modification
	LastUpdated time.Time
}
