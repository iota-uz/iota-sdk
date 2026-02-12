package kb

import "context"

// KBSearcher searches the knowledge base index.
// Implementations must be thread-safe.
type KBSearcher interface {
	// Search performs a full-text search and returns matching documents.
	// Results are ordered by relevance score (highest first).
	// Returns empty slice if no matches are found.
	Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error)

	// GetDocument retrieves a single document by ID.
	// Returns nil if the document doesn't exist.
	GetDocument(ctx context.Context, id string) (*Document, error)

	// IsAvailable checks if the search index is ready for queries.
	// Returns false if the index is not initialized or has been closed.
	IsAvailable() bool
}
