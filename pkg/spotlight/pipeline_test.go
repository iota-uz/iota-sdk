package spotlight

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type pipelineTestProvider struct {
	id       string
	priority int
	docs     []SearchDocument
}

func (p *pipelineTestProvider) ProviderID() string {
	return p.id
}

func (p *pipelineTestProvider) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{IndexPriority: p.priority}
}

func (p *pipelineTestProvider) StreamDocuments(_ context.Context, _ ProviderScope, emit DocumentBatchEmitter) error {
	docs := make([]SearchDocument, len(p.docs))
	copy(docs, p.docs)
	return emit(docs)
}

type pipelineStreamingTestProvider struct {
	id      string
	batches [][]SearchDocument
}

func (p *pipelineStreamingTestProvider) ProviderID() string {
	return p.id
}

func (p *pipelineStreamingTestProvider) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{}
}

func (p *pipelineStreamingTestProvider) StreamDocuments(_ context.Context, _ ProviderScope, emit DocumentBatchEmitter) error {
	for _, batch := range p.batches {
		docs := make([]SearchDocument, len(batch))
		copy(docs, batch)
		if err := emit(docs); err != nil {
			return err
		}
	}
	return nil
}

type pipelineTestEngine struct {
	batches [][]SearchDocument
}

func (e *pipelineTestEngine) Upsert(_ context.Context, docs []SearchDocument) error {
	batch := make([]SearchDocument, len(docs))
	copy(batch, docs)
	e.batches = append(e.batches, batch)
	return nil
}

func (e *pipelineTestEngine) UpsertAsync(_ context.Context, docs []SearchDocument) error {
	return e.Upsert(context.Background(), docs)
}

func (e *pipelineTestEngine) WaitPending(_ context.Context) error {
	return nil
}

func (e *pipelineTestEngine) Delete(_ context.Context, _ []DocumentRef) error {
	return nil
}

func (e *pipelineTestEngine) DeleteTenant(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (e *pipelineTestEngine) Search(_ context.Context, _ SearchRequest) ([]SearchHit, error) {
	return nil, nil
}

func (e *pipelineTestEngine) Health(_ context.Context) error {
	return nil
}

func (e *pipelineTestEngine) Stats(_ context.Context) (*IndexStats, error) {
	return &IndexStats{}, nil
}

func TestIndexerPipelineSync_UpsertsPerProviderBatch(t *testing.T) {
	registry := NewProviderRegistry()
	tenantID := uuid.New()
	now := time.Now().UTC()

	registry.Register(&pipelineTestProvider{
		id: "provider.alpha",
		docs: []SearchDocument{
			{ID: "a-1", UpdatedAt: now},
			{ID: "a-2"},
		},
	})
	registry.Register(&pipelineTestProvider{
		id: "provider.beta",
		docs: []SearchDocument{
			{ID: "b-1"},
		},
	})

	engine := &pipelineTestEngine{}
	pipeline := NewIndexerPipeline(registry, engine, nil)

	err := pipeline.Sync(context.Background(), tenantID, "en", "", 10, ScopeConfig{})
	require.NoError(t, err)
	require.Len(t, engine.batches, 2)

	require.Len(t, engine.batches[0], 2)
	require.Equal(t, "a-1", engine.batches[0][0].ID)
	require.Equal(t, "provider.alpha", engine.batches[0][0].Provider)
	require.Equal(t, tenantID, engine.batches[0][0].TenantID)
	require.False(t, engine.batches[0][1].UpdatedAt.IsZero())

	require.Len(t, engine.batches[1], 1)
	require.Equal(t, "b-1", engine.batches[1][0].ID)
	require.Equal(t, "provider.beta", engine.batches[1][0].Provider)
	require.Equal(t, tenantID, engine.batches[1][0].TenantID)
	require.False(t, engine.batches[1][0].UpdatedAt.IsZero())
}

func TestIndexerPipelineSync_StreamingProviderUpsertsIncrementally(t *testing.T) {
	registry := NewProviderRegistry()
	tenantID := uuid.New()

	registry.Register(&pipelineStreamingTestProvider{
		id: "provider.streaming",
		batches: [][]SearchDocument{
			{
				{ID: "s-1"},
				{ID: "s-2"},
			},
			{
				{ID: "s-3"},
			},
		},
	})

	engine := &pipelineTestEngine{}
	pipeline := NewIndexerPipeline(registry, engine, nil)

	err := pipeline.Sync(context.Background(), tenantID, "en", "", 10, ScopeConfig{})
	require.NoError(t, err)

	// Buffer merges small emit batches into a single upsert (all 3 docs < pipelineUpsertBatchSize)
	require.Len(t, engine.batches, 1)
	require.Len(t, engine.batches[0], 3)
	require.Equal(t, []string{"s-1", "s-2", "s-3"}, []string{
		engine.batches[0][0].ID, engine.batches[0][1].ID, engine.batches[0][2].ID,
	})

	for _, doc := range engine.batches[0] {
		require.Equal(t, "provider.streaming", doc.Provider)
		require.Equal(t, tenantID, doc.TenantID)
		require.False(t, doc.UpdatedAt.IsZero())
	}
}

func TestIndexerPipelineSync_UsesRegistryPriorityOrder(t *testing.T) {
	registry := NewProviderRegistry()
	tenantID := uuid.New()

	registry.Register(&pipelineTestProvider{id: "provider.high", priority: 200, docs: []SearchDocument{{ID: "high-1"}}})
	registry.Register(&pipelineTestProvider{id: "provider.mid", priority: 100, docs: []SearchDocument{{ID: "mid-1"}}})
	registry.Register(&pipelineTestProvider{id: "provider.low", priority: 10, docs: []SearchDocument{{ID: "low-1"}}})

	engine := &pipelineTestEngine{}
	pipeline := NewIndexerPipeline(registry, engine, nil)

	err := pipeline.Sync(context.Background(), tenantID, "en", "", 10, ScopeConfig{})
	require.NoError(t, err)

	require.Len(t, engine.batches, 3)
	require.Equal(t, "provider.high", engine.batches[0][0].Provider)
	require.Equal(t, "provider.mid", engine.batches[1][0].Provider)
	require.Equal(t, "provider.low", engine.batches[2][0].Provider)
}
