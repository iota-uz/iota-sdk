package sources

import (
	"context"
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/bichat/kb"
)

// DocumentRepository provides access to documents stored in a database.
// Implementations should handle database-specific queries and transactions.
type DocumentRepository interface {
	// List retrieves all documents from the database.
	List(ctx context.Context) ([]kb.Document, error)

	// Watch returns a channel that emits document changes.
	// Implementations can use database triggers, polling, or CDC.
	// Return nil channel if watching is not supported.
	Watch(ctx context.Context) (<-chan kb.DocumentChange, error)
}

// databaseSource indexes documents from a database via DocumentRepository.
type databaseSource struct {
	repo DocumentRepository
}

// NewDatabaseSource creates a DocumentSource that pulls from a database.
func NewDatabaseSource(repo DocumentRepository) kb.DocumentSource {
	return &databaseSource{
		repo: repo,
	}
}

// List implements DocumentSource.
func (ds *databaseSource) List(ctx context.Context) ([]kb.Document, error) {
	docs, err := ds.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list documents from database: %w", err)
	}
	return docs, nil
}

// Watch implements DocumentSource.
func (ds *databaseSource) Watch(ctx context.Context) (<-chan kb.DocumentChange, error) {
	changes, err := ds.repo.Watch(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to watch database changes: %w", err)
	}
	return changes, nil
}
