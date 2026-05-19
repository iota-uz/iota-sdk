// Package spotlight provides this package.
package spotlight

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/sirupsen/logrus"
)

// errTenantMismatch is returned when a provider emits a SearchDocument with
// a TenantID that does not match the pipeline scope tenant. Granite is
// currently single-tenant in production, but silently overwriting the
// field would mask any future cross-tenant leak.
var errTenantMismatch = errors.New("tenant mismatch")

type IndexerPipeline struct {
	registry *ProviderRegistry
	engine   IndexEngine
	logger   *logrus.Logger
}

// pipelineUpsertBatchSize is the default count-based batch size when a
// provider does not override it via ProviderCapabilities.BatchSize.
const pipelineUpsertBatchSize = 5000

// errProviderDocumentCap is returned by docBuffer.add when a provider has
// emitted more documents than its declared DocumentCap. The pipeline
// surfaces this as a per-provider stop, not a fatal sync failure.
var errProviderDocumentCap = errors.New("provider document cap reached")

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

		caps := provider.Capabilities()
		batchSize := pipelineUpsertBatchSize
		if caps.BatchSize > 0 {
			batchSize = caps.BatchSize
		}
		stats := &batchStats{}
		buf := &docBuffer{
			pipeline:    p,
			ctx:         ctx,
			stats:       stats,
			batchSize:   batchSize,
			documentCap: caps.DocumentCap,
		}
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
		// Document cap is a controlled stop, not a failure.
		capped := errors.Is(providerErr, errProviderDocumentCap)
		if capped {
			providerErr = nil
		}
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

		if capped && p.logger != nil {
			p.logger.WithFields(logrus.Fields{
				"provider_id":  provider.ProviderID(),
				"document_cap": caps.DocumentCap,
				"docs":         stats.docCount,
			}).Warn("Spotlight provider hit DocumentCap; results may be truncated")
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
// them to the engine in larger batches for efficiency. batchSize is the
// per-provider count ceiling (default pipelineUpsertBatchSize, override
// via ProviderCapabilities.BatchSize). documentCap, when positive, halts
// the provider once it emits that many docs.
type docBuffer struct {
	pipeline    *IndexerPipeline
	ctx         context.Context
	stats       *batchStats
	pending     []SearchDocument
	batchSize   int
	documentCap int
}

func (b *docBuffer) add(providerID string, tenantID uuid.UUID, docs []SearchDocument) error {
	if b.documentCap > 0 && b.stats.docCount >= b.documentCap {
		return errProviderDocumentCap
	}
	if b.documentCap > 0 {
		remaining := b.documentCap - b.stats.docCount
		if remaining < len(docs) {
			docs = docs[:remaining]
		}
	}
	now := time.Now().UTC()
	for i := range docs {
		if docs[i].TenantID != uuid.Nil && docs[i].TenantID != tenantID {
			return serrors.E("spotlight.IndexerPipeline.docBuffer.add",
				fmt.Sprintf("provider %s emitted document with tenant %s but pipeline scope is %s", providerID, docs[i].TenantID, tenantID),
				errTenantMismatch)
		}
		docs[i].TenantID = tenantID
		docs[i].Provider = providerID
		if docs[i].UpdatedAt.IsZero() {
			docs[i].UpdatedAt = now
		}
	}
	b.stats.docCount += len(docs)
	b.pending = append(b.pending, docs...)

	batch := b.batchSize
	if batch <= 0 {
		batch = pipelineUpsertBatchSize
	}
	// Flush full batches
	for len(b.pending) >= batch {
		flushBatch := b.pending[:batch]
		b.pending = b.pending[batch:]
		if err := b.upsert(flushBatch, providerID); err != nil {
			return err
		}
	}
	if b.documentCap > 0 && b.stats.docCount >= b.documentCap {
		return errProviderDocumentCap
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
