package spotlight

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
)

type IndexerPipeline struct {
	registry *ProviderRegistry
	engine   IndexEngine
}

func NewIndexerPipeline(registry *ProviderRegistry, engine IndexEngine) *IndexerPipeline {
	return &IndexerPipeline{registry: registry, engine: engine}
}

func (p *IndexerPipeline) Sync(ctx context.Context, tenantID uuid.UUID, language, query string, topK int, scope ScopeConfig) error {
	providers := p.registry.All()
	sort.Slice(providers, func(i, j int) bool {
		return providers[i].ProviderID() < providers[j].ProviderID()
	})

	all := make([]SearchDocument, 0, 256)
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
			return fmt.Errorf("provider %s failed: %w", provider.ProviderID(), err)
		}
		for i := range docs {
			docs[i].TenantID = tenantID
			docs[i].Provider = provider.ProviderID()
			if docs[i].UpdatedAt.IsZero() {
				docs[i].UpdatedAt = time.Now().UTC()
			}
		}
		all = append(all, docs...)
	}

	return p.engine.Upsert(ctx, all)
}
