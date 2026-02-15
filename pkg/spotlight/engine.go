package spotlight

import (
	"context"

	"github.com/google/uuid"
)

type IndexEngine interface {
	Upsert(ctx context.Context, docs []SearchDocument) error
	Delete(ctx context.Context, refs []DocumentRef) error
	Search(ctx context.Context, req SearchRequest) ([]SearchHit, error)
	Health(ctx context.Context) error
}

type DocumentRef struct {
	TenantID uuid.UUID
	ID       string
}
