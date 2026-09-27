// Package spotlight provides this package.
package spotlight

import (
	"context"
	"time"

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

// RebuildArtifactPruner removes abandoned rebuild indexes. Implementations
// must leave a grace period large enough for an active rebuild to finish.
// Callers are responsible for serializing this operation with StartRebuild.
type RebuildArtifactPruner interface {
	PruneOrphanBuildIndexes(ctx context.Context, minAge time.Duration) ([]PrunedIndex, error)
}

type rebuildRunIDKey struct{}

// WithRebuildRunID attaches a caller-controlled run identifier to a full
// rebuild. Meilisearch engines include it in the temporary build-index name,
// which makes overlapping rebuilds non-destructive and lets operators relate
// logs to the corresponding Meili artifact. An empty or invalid identifier is
// replaced with a generated UUID by the engine.
func WithRebuildRunID(ctx context.Context, runID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, rebuildRunIDKey{}, runID)
}

func rebuildRunID(ctx context.Context) string {
	if ctx != nil {
		if runID, ok := ctx.Value(rebuildRunIDKey{}).(string); ok {
			if parsed, err := uuid.Parse(runID); err == nil {
				return parsed.String()
			}
		}
	}
	return uuid.NewString()
}

type DocumentRef struct {
	TenantID uuid.UUID
	ID       string
}
