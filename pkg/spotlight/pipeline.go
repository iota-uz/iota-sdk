// Package spotlight provides this package.
package spotlight

import (
	"context"
	"fmt"
	"sort"
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
	sort.Slice(providers, func(i, j int) bool {
		return providers[i].ProviderID() < providers[j].ProviderID()
	})

	for _, provider := range providers {
		enabled, ok := scope.EnabledProviders[provider.ProviderID()]
		if ok && !enabled {
			continue
		}
		docs, err := provider.ListDocuments(ctx, ProviderScope{
			TenantID: tenantID,
			Language: language,
			Query:    query,
			TopK:     topK,
		})
		if err != nil {
			return serrors.E(op, "provider "+provider.ProviderID()+" failed", err)
		}
		for i := range docs {
			docs[i].TenantID = tenantID
			docs[i].Provider = provider.ProviderID()
			if docs[i].UpdatedAt.IsZero() {
				docs[i].UpdatedAt = time.Now().UTC()
			}
		}

		for start := 0; start < len(docs); start += pipelineUpsertBatchSize {
			end := start + pipelineUpsertBatchSize
			if end > len(docs) {
				end = len(docs)
			}
			if err := p.engine.Upsert(ctx, docs[start:end]); err != nil {
				return serrors.E(op, fmt.Sprintf("provider %s upsert batch [%d:%d] failed", provider.ProviderID(), start, end), err)
			}
		}
	}
	return nil
}
