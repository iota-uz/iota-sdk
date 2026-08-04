// Package spotlight provides this package.
package spotlight

import (
	"context"

	"github.com/google/uuid"
)

// IndexStats holds runtime statistics about the search index.
type IndexStats struct {
	TotalDocuments         int64
	FieldDistribution      map[string]int64
	ProviderDocumentCounts map[string]int64
	SchemaVersion          string
	IsSearchable           bool
}

type IndexEngine interface {
	Upsert(ctx context.Context, docs []SearchDocument) error
	UpsertAsync(ctx context.Context, docs []SearchDocument) error
	WaitPending(ctx context.Context) error
	Delete(ctx context.Context, refs []DocumentRef) error
	DeleteTenant(ctx context.Context, tenantID uuid.UUID) error
	Search(ctx context.Context, req SearchRequest) ([]SearchHit, error)
	Health(ctx context.Context) error
	Stats(ctx context.Context) (*IndexStats, error)
}

type RebuildSession interface {
	Engine() IndexEngine
	Commit(ctx context.Context) error
	Abort(ctx context.Context) error
}

type RebuildableIndexEngine interface {
	IndexEngine
	StartRebuild(ctx context.Context) (RebuildSession, error)
}

type DocumentRef struct {
	TenantID uuid.UUID
	ID       string
}
