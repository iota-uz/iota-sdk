// Package spotlight provides this package.
package spotlight

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type IndexerPipeline struct {
	registry *ProviderRegistry
	engine   IndexEngine
}

const pipelineUpsertBatchSize = 500

func NewIndexerPipeline(registry *ProviderRegistry, engine IndexEngine) *IndexerPipeline {
	return &IndexerPipeline{registry: registry, engine: engine}
}

func (p *IndexerPipeline) Sync(ctx context.Context, tenantID uuid.UUID, language, query string, topK int, scope ScopeConfig) error {
	const op serrors.Op = "spotlight.IndexerPipeline.Sync"

	providers := p.registry.All()

	for _, provider := range providers {
		if provider.ProviderID() == "core.quick_links" {
			continue // searched in-memory via QuickLinks.FuzzySearch
		}
		enabled, ok := scope.EnabledProviders[provider.ProviderID()]
		if ok && !enabled {
			continue
		}
		providerScope := ProviderScope{
			TenantID: tenantID,
			Language: language,
			Query:    query,
			TopK:     topK,
		}
		if err := provider.StreamDocuments(ctx, providerScope, func(docs []SearchDocument) error {
			return p.processProviderBatch(ctx, provider.ProviderID(), tenantID, docs)
		}); err != nil {
			return serrors.E(op, "provider "+provider.ProviderID()+" failed", err)
		}
	}
	return nil
}

func (p *IndexerPipeline) processProviderBatch(ctx context.Context, providerID string, tenantID uuid.UUID, docs []SearchDocument) error {
	now := time.Now().UTC()
	for i := range docs {
		docs[i].TenantID = tenantID
		docs[i].Provider = providerID
		if docs[i].UpdatedAt.IsZero() {
			docs[i].UpdatedAt = now
		}
	}

	for start := 0; start < len(docs); start += pipelineUpsertBatchSize {
		end := start + pipelineUpsertBatchSize
		if end > len(docs) {
			end = len(docs)
		}
		if err := p.engine.Upsert(ctx, docs[start:end]); err != nil {
			return serrors.E("spotlight.IndexerPipeline.processProviderBatch", fmt.Sprintf("provider %s upsert batch [%d:%d] failed", providerID, start, end), err)
		}
	}
	return nil
}
