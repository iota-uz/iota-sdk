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

const pipelineUpsertBatchSize = 5000

type batchStats struct {
	docCount       int
	batchCount     int
	upsertDuration time.Duration
}

func NewIndexerPipeline(registry *ProviderRegistry, engine IndexEngine, logger *logrus.Logger) *IndexerPipeline {
	return &IndexerPipeline{registry: registry, engine: engine, logger: logger}
}

func (p *IndexerPipeline) Sync(ctx context.Context, tenantID uuid.UUID, language, query string, topK int, scope ScopeConfig) error {
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
		buf := &docBuffer{pipeline: p, ctx: ctx, stats: stats}
		providerStart := time.Now()

		providerScope := ProviderScope{
			TenantID: tenantID,
			Language: language,
			Query:    query,
			TopK:     topK,
		}
		providerErr := provider.StreamDocuments(ctx, providerScope, func(docs []SearchDocument) error {
			return buf.add(provider.ProviderID(), tenantID, docs)
		})
		// Flush remaining buffered docs
		if providerErr == nil {
			providerErr = buf.flush()
		}

		providerDuration := time.Since(providerStart)
		queryDuration := providerDuration - stats.upsertDuration
		totalDocs += stats.docCount

		if providerErr != nil {
			if p.logger != nil {
				p.logger.WithFields(logrus.Fields{
					"provider_id": provider.ProviderID(),
					"docs":        stats.docCount,
					"batches":     stats.batchCount,
					"total_ms":    providerDuration.Milliseconds(),
					"upsert_ms":   stats.upsertDuration.Milliseconds(),
					"query_ms":    queryDuration.Milliseconds(),
					"error":       providerErr.Error(),
				}).Error("Spotlight provider failed")
			}
			continue
		}

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

	waitStart := time.Now()
	if err := p.engine.WaitPending(ctx); err != nil {
		if p.logger != nil {
			p.logger.WithError(err).Error("Spotlight WaitPending failed")
		}
	}

	if p.logger != nil {
		p.logger.WithFields(logrus.Fields{
			"total_docs":     totalDocs,
			"total_ms":       time.Since(syncStart).Milliseconds(),
			"wait_ms":        time.Since(waitStart).Milliseconds(),
			"provider_count": len(providers),
		}).Info("Spotlight sync completed")
	}

	return nil
}

// docBuffer accumulates documents from provider emit calls and flushes
// to the engine in larger batches (pipelineUpsertBatchSize) for efficiency.
type docBuffer struct {
	pipeline *IndexerPipeline
	ctx      context.Context
	stats    *batchStats
	pending  []SearchDocument
}

func (b *docBuffer) add(providerID string, tenantID uuid.UUID, docs []SearchDocument) error {
	now := time.Now().UTC()
	for i := range docs {
		docs[i].TenantID = tenantID
		docs[i].Provider = providerID
		if docs[i].UpdatedAt.IsZero() {
			docs[i].UpdatedAt = now
		}
	}
	b.stats.docCount += len(docs)
	b.pending = append(b.pending, docs...)

	// Flush full batches
	for len(b.pending) >= pipelineUpsertBatchSize {
		batch := b.pending[:pipelineUpsertBatchSize]
		b.pending = b.pending[pipelineUpsertBatchSize:]
		if err := b.upsert(batch, providerID); err != nil {
			return err
		}
	}
	return nil
}

func (b *docBuffer) flush() error {
	if len(b.pending) == 0 {
		return nil
	}
	providerID := ""
	if len(b.pending) > 0 {
		providerID = b.pending[0].Provider
	}
	batch := b.pending
	b.pending = nil
	return b.upsert(batch, providerID)
}

func (b *docBuffer) upsert(batch []SearchDocument, providerID string) error {
	upsertStart := time.Now()
	if err := b.pipeline.engine.UpsertAsync(b.ctx, batch); err != nil {
		return serrors.E("spotlight.IndexerPipeline.processProviderBatch",
			fmt.Sprintf("provider %s upsert batch of %d failed", providerID, len(batch)), err)
	}
	b.stats.upsertDuration += time.Since(upsertStart)
	b.stats.batchCount++
	return nil
}
