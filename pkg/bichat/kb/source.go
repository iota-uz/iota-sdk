package kb

import "context"

// DocumentSource provides documents for indexing.
// Implementations can pull from various sources (filesystem, database, API, etc.).
type DocumentSource interface {
	// List returns all documents available from this source.
	// This is typically used for initial indexing or rebuilding.
	List(ctx context.Context) ([]Document, error)

	// Watch returns a channel that emits document changes as they occur.
	// This enables live indexing - new/updated/deleted documents are
	// automatically reflected in the index.
	//
	// The channel is closed when the context is cancelled or an error occurs.
	// Implementations that don't support watching can return a nil channel.
	Watch(ctx context.Context) (<-chan DocumentChange, error)
}
