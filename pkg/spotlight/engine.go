// Package spotlight provides this package.
package spotlight

import (
	"context"

	"github.com/google/uuid"
)

type IndexEngine interface {
	Upsert(ctx context.Context, docs []SearchDocument) error
	Delete(ctx context.Context, refs []DocumentRef) error
	DeleteTenant(ctx context.Context, tenantID uuid.UUID) error
	Search(ctx context.Context, req SearchRequest) ([]SearchHit, error)
	Health(ctx context.Context) error
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
