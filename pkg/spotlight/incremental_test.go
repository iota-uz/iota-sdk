package spotlight

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type incrementalTestEngine struct {
	upserts [][]SearchDocument
	deletes [][]DocumentRef
}

func (e *incrementalTestEngine) Upsert(_ context.Context, docs []SearchDocument) error {
	batch := make([]SearchDocument, len(docs))
	copy(batch, docs)
	e.upserts = append(e.upserts, batch)
	return nil
}

func (e *incrementalTestEngine) Delete(_ context.Context, refs []DocumentRef) error {
	batch := make([]DocumentRef, len(refs))
	copy(batch, refs)
	e.deletes = append(e.deletes, batch)
	return nil
}

func (e *incrementalTestEngine) DeleteTenant(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (e *incrementalTestEngine) Search(_ context.Context, _ SearchRequest) ([]SearchHit, error) {
	return nil, nil
}

func (e *incrementalTestEngine) UpsertAsync(_ context.Context, docs []SearchDocument) error {
	return e.Upsert(context.Background(), docs)
}

func (e *incrementalTestEngine) WaitPending(_ context.Context) error {
	return nil
}

func (e *incrementalTestEngine) Health(_ context.Context) error {
	return nil
}

func (e *incrementalTestEngine) Stats(_ context.Context) (*IndexStats, error) {
	return &IndexStats{}, nil
}

type incrementalTestProvider struct {
	load IncrementalLoader
}

func (p *incrementalTestProvider) ProviderID() string {
	return "provider.test"
}

func (p *incrementalTestProvider) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{}
}

func (p *incrementalTestProvider) StreamDocuments(_ context.Context, _ ProviderScope, _ DocumentBatchEmitter) error {
	return nil
}

func (p *incrementalTestProvider) Watch(_ context.Context, _ ProviderScope) (<-chan DocumentEvent, error) {
	ch := make(chan DocumentEvent)
	close(ch)
	return ch, nil
}

func (p *incrementalTestProvider) LoadDocuments(ctx context.Context, scope ProviderScope, refs []DocumentRef, emit DocumentBatchEmitter) error {
	return p.load(ctx, scope, refs, emit)
}

func TestIncrementalSyncerSyncDocumentsDeletesMissingRefs(t *testing.T) {
	engine := &incrementalTestEngine{}
	tenantID := uuid.New()
	syncer := NewIncrementalSyncer(engine, tenantID, nil)

	err := syncer.SyncDocuments(context.Background(), "provider.test", []string{"doc-1", "doc-2"}, []SearchDocument{
		{ID: "doc-1"},
	})
	require.NoError(t, err)

	require.Len(t, engine.upserts, 1)
	require.Len(t, engine.upserts[0], 1)
	require.Equal(t, tenantID, engine.upserts[0][0].TenantID)

	require.Len(t, engine.deletes, 1)
	require.Equal(t, "doc-2", engine.deletes[0][0].ID)
	require.Equal(t, tenantID, engine.deletes[0][0].TenantID)
}

func TestIncrementalSyncerSyncProviderRefsLoadsThroughProvider(t *testing.T) {
	engine := &incrementalTestEngine{}
	tenantID := uuid.New()
	syncer := NewIncrementalSyncer(engine, tenantID, nil)
	scope := ProviderScope{TenantID: tenantID, Language: "en"}

	provider := &incrementalTestProvider{
		load: func(_ context.Context, gotScope ProviderScope, refs []DocumentRef, emit DocumentBatchEmitter) error {
			require.Equal(t, scope, gotScope)
			require.Equal(t, []DocumentRef{{TenantID: tenantID, ID: "doc-1"}}, refs)
			return emit([]SearchDocument{{ID: "doc-1"}})
		},
	}

	err := syncer.SyncProviderRefs(context.Background(), "provider.test", provider, scope, []DocumentRef{
		{TenantID: tenantID, ID: "doc-1"},
	})
	require.NoError(t, err)
	require.Len(t, engine.upserts, 1)
	require.Empty(t, engine.deletes)
}

func TestIncrementalSyncerSyncStreamReturnsStreamError(t *testing.T) {
	engine := &incrementalTestEngine{}
	syncer := NewIncrementalSyncer(engine, uuid.New(), nil)
	expected := errors.New("boom")

	err := syncer.SyncStream(context.Background(), "provider.test", ProviderScope{}, []string{"doc-1"}, func(DocumentBatchEmitter) error {
		return expected
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "boom")
}

func TestIncrementalBindingRegistryResolveMatchesAndDedupesRefs(t *testing.T) {
	registry := NewIncrementalBindingRegistry()
	tenantID := uuid.New()

	registry.Register(IncrementalBinding{
		Name:       "client",
		ProviderID: "crm.client",
		Match: func(event ProjectionEvent) bool {
			return event.EntityType == "client"
		},
		Refs: func(event ProjectionEvent) []DocumentRef {
			return []DocumentRef{
				{TenantID: event.TenantID, ID: "doc-1"},
				{TenantID: event.TenantID, ID: "doc-1"},
			}
		},
	})
	registry.Register(IncrementalBinding{
		Name:       "policy",
		ProviderID: "crm.policy",
		Match: func(event ProjectionEvent) bool {
			return event.EntityType == "policy"
		},
		Refs: func(event ProjectionEvent) []DocumentRef {
			return []DocumentRef{{TenantID: event.TenantID, ID: "doc-2"}}
		},
	})

	resolved := registry.Resolve(ProjectionEvent{
		TenantID:   tenantID,
		EntityType: "client",
	})

	require.Len(t, resolved, 1)
	require.Equal(t, "client", resolved[0].Name)
	require.Equal(t, "crm.client", resolved[0].ProviderID)
	require.Equal(t, []DocumentRef{{TenantID: tenantID, ID: "doc-1"}}, resolved[0].Refs)
}
