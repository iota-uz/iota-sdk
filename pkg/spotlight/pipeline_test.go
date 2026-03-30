package spotlight

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type pipelineTestProvider struct {
	id   string
	docs []SearchDocument
}

func (p *pipelineTestProvider) ProviderID() string {
	return p.id
}

func (p *pipelineTestProvider) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{}
}

func (p *pipelineTestProvider) ListDocuments(_ context.Context, _ ProviderScope) ([]SearchDocument, error) {
	out := make([]SearchDocument, len(p.docs))
	copy(out, p.docs)
	return out, nil
}

func (p *pipelineTestProvider) Watch(_ context.Context, _ ProviderScope) (<-chan DocumentEvent, error) {
	ch := make(chan DocumentEvent)
	close(ch)
	return ch, nil
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

func (e *pipelineTestEngine) Delete(_ context.Context, _ []DocumentRef) error {
	return nil
}

func (e *pipelineTestEngine) Search(_ context.Context, _ SearchRequest) ([]SearchHit, error) {
	return nil, nil
}

func (e *pipelineTestEngine) Health(_ context.Context) error {
	return nil
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
	pipeline := NewIndexerPipeline(registry, engine)

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
