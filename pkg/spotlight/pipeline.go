// Package spotlight provides this package.
package spotlight

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/sirupsen/logrus"
)

type IndexerPipeline struct {
	registry *ProviderRegistry
	engine   IndexEngine
	logger   *logrus.Logger
}

const pipelineUpsertBatchSize = 500

type batchStats struct {
	docCount       int
	batchCount     int
	upsertDuration time.Duration
}

func NewIndexerPipeline(registry *ProviderRegistry, engine IndexEngine, logger *logrus.Logger) *IndexerPipeline {
	return &IndexerPipeline{registry: registry, engine: engine, logger: logger}
}

func (p *IndexerPipeline) Sync(ctx context.Context, tenantID uuid.UUID, language, query string, topK int, scope ScopeConfig) error {
	const op serrors.Op = "spotlight.IndexerPipeline.Sync"

	providers := p.registry.All()
	syncStart := time.Now()
	totalDocs := 0

	for _, provider := range providers {
		if provider.ProviderID() == "core.quick_links" {
			continue // searched in-memory via QuickLinks.FuzzySearch
		}
		enabled, ok := scope.EnabledProviders[provider.ProviderID()]
		if ok && !enabled {
			continue
		}

		stats := &batchStats{}
		providerStart := time.Now()

		providerScope := ProviderScope{
			TenantID: tenantID,
			Language: language,
			Query:    query,
			TopK:     topK,
		}
		if err := provider.StreamDocuments(ctx, providerScope, func(docs []SearchDocument) error {
			return p.processProviderBatch(ctx, provider.ProviderID(), tenantID, docs, stats)
		}); err != nil {
			return serrors.E(op, "provider "+provider.ProviderID()+" failed", err)
		}

		providerDuration := time.Since(providerStart)
		queryDuration := providerDuration - stats.upsertDuration
		totalDocs += stats.docCount

		if p.logger != nil {
			p.logger.WithFields(logrus.Fields{
				"provider_id": provider.ProviderID(),
				"docs":        stats.docCount,
				"batches":     stats.batchCount,
				"total_ms":    providerDuration.Milliseconds(),
				"upsert_ms":   stats.upsertDuration.Milliseconds(),
				"query_ms":    queryDuration.Milliseconds(),
			}).Info("Spotlight provider indexed")
		}
	}

	if p.logger != nil {
		p.logger.WithFields(logrus.Fields{
			"total_docs":     totalDocs,
			"total_ms":       time.Since(syncStart).Milliseconds(),
			"provider_count": len(providers),
		}).Info("Spotlight sync completed")
	}

	return nil
}

func (p *IndexerPipeline) processProviderBatch(ctx context.Context, providerID string, tenantID uuid.UUID, docs []SearchDocument, stats *batchStats) error {
	now := time.Now().UTC()
	for i := range docs {
		docs[i].TenantID = tenantID
		docs[i].Provider = providerID
		if docs[i].UpdatedAt.IsZero() {
			docs[i].UpdatedAt = now
		}
	}

	stats.docCount += len(docs)

	for start := 0; start < len(docs); start += pipelineUpsertBatchSize {
		end := start + pipelineUpsertBatchSize
		if end > len(docs) {
			end = len(docs)
		}
		upsertStart := time.Now()
		if err := p.engine.Upsert(ctx, docs[start:end]); err != nil {
			return serrors.E("spotlight.IndexerPipeline.processProviderBatch", fmt.Sprintf("provider %s upsert batch [%d:%d] failed", providerID, start, end), err)
		}
		stats.upsertDuration += time.Since(upsertStart)
		stats.batchCount++
	}
	return nil
}
