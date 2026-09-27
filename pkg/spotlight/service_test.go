package spotlight

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type reindexEngine struct {
	deleteTenantCalls []uuid.UUID
	upserts           [][]SearchDocument
}

func (e *reindexEngine) Upsert(_ context.Context, docs []SearchDocument) error {
	copied := make([]SearchDocument, len(docs))
	copy(copied, docs)
	e.upserts = append(e.upserts, copied)
	return nil
}

func (e *reindexEngine) Delete(context.Context, []DocumentRef) error { return nil }

func (e *reindexEngine) DeleteTenant(_ context.Context, tenantID uuid.UUID) error {
	e.deleteTenantCalls = append(e.deleteTenantCalls, tenantID)
	return nil
}

func (e *reindexEngine) Search(context.Context, SearchRequest) ([]SearchHit, error) { return nil, nil }

func (e *reindexEngine) Health(context.Context) error { return nil }

func (e *reindexEngine) Stats(context.Context) (*IndexStats, error) { return &IndexStats{}, nil }

func (e *reindexEngine) UpsertAsync(ctx context.Context, docs []SearchDocument) error {
	return e.Upsert(ctx, docs)
}

func (e *reindexEngine) WaitPending(context.Context) error { return nil }

type testEngine struct {
	mu          sync.Mutex
	searchCalls int
	searchHits  []SearchHit
	searchErr   error
}

func (e *testEngine) Upsert(context.Context, []SearchDocument) error { return nil }

func (e *testEngine) UpsertAsync(context.Context, []SearchDocument) error { return nil }

func (e *testEngine) WaitPending(context.Context) error { return nil }

func (e *testEngine) Delete(context.Context, []DocumentRef) error { return nil }

func (e *testEngine) DeleteTenant(context.Context, uuid.UUID) error { return nil }

func (e *testEngine) Search(_ context.Context, _ SearchRequest) ([]SearchHit, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.searchCalls++
	if e.searchErr != nil {
		return nil, e.searchErr
	}
	out := make([]SearchHit, len(e.searchHits))
	copy(out, e.searchHits)
	return out, nil
}

func (e *testEngine) Health(context.Context) error { return nil }

func (e *testEngine) Stats(context.Context) (*IndexStats, error) { return &IndexStats{}, nil }

func (e *testEngine) calls() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.searchCalls
}

type scriptedCall struct {
	delay time.Duration
	hits  []SearchHit
}

type scriptedEngine struct {
	mu     sync.Mutex
	calls  int
	script []scriptedCall
}

func (e *scriptedEngine) Upsert(context.Context, []SearchDocument) error { return nil }

func (e *scriptedEngine) UpsertAsync(context.Context, []SearchDocument) error { return nil }

func (e *scriptedEngine) WaitPending(context.Context) error { return nil }

func (e *scriptedEngine) Delete(context.Context, []DocumentRef) error { return nil }

func (e *scriptedEngine) DeleteTenant(context.Context, uuid.UUID) error { return nil }

func (e *scriptedEngine) Search(ctx context.Context, _ SearchRequest) ([]SearchHit, error) {
	e.mu.Lock()
	idx := e.calls
	e.calls++
	var call scriptedCall
	if idx < len(e.script) {
		call = e.script[idx]
	}
	e.mu.Unlock()

	if call.delay > 0 {
		timer := time.NewTimer(call.delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
		}
	}

	out := make([]SearchHit, len(call.hits))
	copy(out, call.hits)
	return out, nil
}

func (e *scriptedEngine) Health(context.Context) error { return nil }

func (e *scriptedEngine) Stats(context.Context) (*IndexStats, error) { return &IndexStats{}, nil }

func (e *scriptedEngine) callsCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.calls
}

type fakeProvider struct {
	id string
}

func (p *fakeProvider) ProviderID() string { return p.id }

func (p *fakeProvider) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{EntityTypes: []string{"client"}}
}

func (p *fakeProvider) StreamDocuments(_ context.Context, _ ProviderScope, _ DocumentBatchEmitter) error {
	return nil
}

type testBatchACL struct {
	mu          sync.Mutex
	batchCalls  int
	canReadCall int
}

func (a *testBatchACL) CanRead(_ context.Context, _ SearchRequest, _ SearchHit) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.canReadCall++
	return true
}

func (a *testBatchACL) FilterAuthorized(_ context.Context, _ SearchRequest, hits []SearchHit) []SearchHit {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.batchCalls++
	out := make([]SearchHit, len(hits))
	copy(out, hits)
	return out
}

func (a *testBatchACL) stats() (int, int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.batchCalls, a.canReadCall
}

type capturingEngine struct {
	mu       sync.Mutex
	requests []SearchRequest
	hits     []SearchHit
}

func (e *capturingEngine) Upsert(context.Context, []SearchDocument) error      { return nil }
func (e *capturingEngine) UpsertAsync(context.Context, []SearchDocument) error { return nil }
func (e *capturingEngine) WaitPending(context.Context) error                   { return nil }
func (e *capturingEngine) Delete(context.Context, []DocumentRef) error         { return nil }
func (e *capturingEngine) DeleteTenant(context.Context, uuid.UUID) error       { return nil }
func (e *capturingEngine) Health(context.Context) error                        { return nil }
func (e *capturingEngine) Stats(context.Context) (*IndexStats, error)          { return &IndexStats{}, nil }

func (e *capturingEngine) Search(_ context.Context, req SearchRequest) ([]SearchHit, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.requests = append(e.requests, req)
	out := make([]SearchHit, len(e.hits))
	copy(out, e.hits)
	return out, nil
}

type denyAllACL struct {
	denyFirst int
}

func (a *denyAllACL) CanRead(_ context.Context, _ SearchRequest, _ SearchHit) bool {
	return false
}

func (a *denyAllACL) FilterAuthorized(_ context.Context, _ SearchRequest, hits []SearchHit) []SearchHit {
	if len(hits) <= a.denyFirst {
		return nil
	}
	return hits[a.denyFirst:]
}

func TestSpotlightService_Search_FansOutTopKForACLDrops(t *testing.T) {
	tenantID := uuid.New()
	hits := make([]SearchHit, 0, 60)
	for i := 0; i < 60; i++ {
		hits = append(hits, SearchHit{
			Document: SearchDocument{
				ID:       string(rune('a'+i)) + "-doc",
				TenantID: tenantID,
				Access:   AccessPolicy{Visibility: VisibilityPublic},
			},
			LexicalScore: float64(60 - i),
			FinalScore:   float64(60 - i),
		})
	}
	engine := &capturingEngine{hits: hits}
	cfg := DefaultServiceConfig()
	cfg.SearchCacheTTL = 0 // bypass cache to verify engine req
	svc := NewService(engine, nil, cfg, WithACLEvaluator(&denyAllACL{denyFirst: 50}))

	req := SearchRequest{TenantID: tenantID, Query: "anything", TopK: 10}
	resp, err := svc.Search(context.Background(), req)
	require.NoError(t, err)

	engine.mu.Lock()
	require.Len(t, engine.requests, 1, "engine should be called exactly once per search")
	require.Equal(t, 50, engine.requests[0].TopK,
		"engine fetch must be topK * ACLFanOutFactor = 10*5 (not the caller's 10)")
	engine.mu.Unlock()

	// 60 hits returned, ACL denies first 50, leaving 10 — exactly topK.
	require.Equal(t, 10, totalHitsAcrossGroups(resp))
}

func totalHitsAcrossGroups(resp SearchResponse) int {
	total := 0
	for _, g := range resp.Groups {
		total += len(g.Hits)
	}
	return total
}

func TestSpotlightService_Search_ClampsEngineTopKAtMax(t *testing.T) {
	tenantID := uuid.New()
	hits := make([]SearchHit, 0, 50)
	for i := 0; i < 50; i++ {
		hits = append(hits, SearchHit{
			Document: SearchDocument{
				ID:       string(rune('a'+i%26)) + "-doc",
				TenantID: tenantID,
				Access:   AccessPolicy{Visibility: VisibilityPublic},
			},
			LexicalScore: float64(50 - i),
			FinalScore:   float64(50 - i),
		})
	}
	engine := &capturingEngine{hits: hits}
	cfg := DefaultServiceConfig()
	cfg.SearchCacheTTL = 0
	cfg.ACLFanOutFactor = 5
	cfg.ACLEngineMaxTopK = 200
	svc := NewService(engine, nil, cfg, WithACLEvaluator(&denyAllACL{denyFirst: 0}))

	// Caller asks for TopK=300 — above ACLEngineMaxTopK (200). The engine
	// must still receive the cap, not 300 or 300*5=1500.
	req := SearchRequest{TenantID: tenantID, Query: "many", TopK: 300}
	_, err := svc.Search(context.Background(), req)
	require.NoError(t, err)

	engine.mu.Lock()
	require.Equal(t, 200, engine.requests[0].TopK,
		"engine must be clamped to ACLEngineMaxTopK even when caller TopK exceeds the cap")
	engine.mu.Unlock()
}

func TestSpotlightService_Search_UsesBatchACLAndCache(t *testing.T) {
	tenantID := uuid.New()
	engine := &testEngine{
		searchHits: []SearchHit{
			{
				Document: SearchDocument{
					ID:       "doc-1",
					TenantID: tenantID,
					Access:   AccessPolicy{Visibility: VisibilityPublic},
				},
				LexicalScore: 1,
				FinalScore:   1,
			},
		},
	}
	acl := &testBatchACL{}
	svc := NewService(engine, nil, DefaultServiceConfig(), WithACLEvaluator(acl))

	req := SearchRequest{
		Query:    "roles",
		TenantID: tenantID,
		UserID:   "7",
		TopK:     10,
		Intent:   SearchIntentMixed,
	}

	resp1, err := svc.Search(context.Background(), req)
	require.NoError(t, err)
	require.NotEmpty(t, resp1.Groups)
	require.Equal(t, ResultDomainOther, resp1.Groups[0].Domain)

	resp2, err := svc.Search(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, resp1, resp2)

	require.Equal(t, 1, engine.calls())
	batchCalls, canReadCalls := acl.stats()
	require.Equal(t, 1, batchCalls)
	require.Equal(t, 0, canReadCalls)

	stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, svc.Stop(stopCtx))
}

func TestSpotlightService_CreateSession_StreamsAndPromotesBetterLateMatches(t *testing.T) {
	tenantID := uuid.New()
	engine := &scriptedEngine{
		script: []scriptedCall{
			{
				hits: []SearchHit{
					{
						Document: SearchDocument{
							ID:         "fast",
							TenantID:   tenantID,
							Title:      "Fast match",
							EntityType: "client",
							Domain:     ResultDomainLookup,
							Access:     AccessPolicy{Visibility: VisibilityPublic},
						},
						FinalScore: 10,
					},
				},
			},
			{
				delay: 50 * time.Millisecond,
				hits: []SearchHit{
					{
						Document: SearchDocument{
							ID:         "better",
							TenantID:   tenantID,
							Title:      "Better match",
							EntityType: "client",
							Domain:     ResultDomainLookup,
							Access:     AccessPolicy{Visibility: VisibilityPublic},
						},
						FinalScore: 100,
					},
				},
			},
			{
				delay: 50 * time.Millisecond,
				hits: []SearchHit{
					{
						Document: SearchDocument{
							ID:         "provider",
							TenantID:   tenantID,
							Title:      "Provider match",
							EntityType: "client",
							Domain:     ResultDomainLookup,
							Access:     AccessPolicy{Visibility: VisibilityPublic},
						},
						FinalScore: 20,
					},
				},
			},
		},
	}
	svc := NewService(engine, nil, DefaultServiceConfig())
	svc.RegisterProvider(&fakeProvider{id: "provider.fake"})

	snapshot, err := svc.CreateSession(context.Background(), SearchRequest{
		Query:    "1234567",
		TenantID: tenantID,
		UserID:   "7",
		TopK:     10,
		Intent:   SearchIntentMixed,
	})
	require.NoError(t, err)
	require.True(t, snapshot.Loading)
	require.False(t, snapshot.Completed)
	require.Equal(t, "Fast match", topSnapshotTitle(snapshot))

	updates, err := svc.SubscribeSession(context.Background(), snapshot.ID, SearchSessionAccess{
		TenantID: tenantID,
		UserID:   "7",
	})
	require.NoError(t, err)

	var finalSnapshot SearchSessionSnapshot
	require.Eventually(t, func() bool {
		select {
		case finalSnapshot = <-updates:
			return finalSnapshot.Completed
		default:
			return false
		}
	}, 2*time.Second, 20*time.Millisecond)

	require.Equal(t, "Better match", topSnapshotTitle(finalSnapshot))
	require.GreaterOrEqual(t, engine.callsCount(), 3)
}

func TestSpotlightService_ReindexTenant_RefreshesOnlyRequestedTenant(t *testing.T) {
	tenantID := uuid.New()
	engine := &reindexEngine{}
	svc := NewService(engine, nil, DefaultServiceConfig())
	svc.RegisterProvider(&pipelineStreamingTestProvider{
		id: "provider.streaming",
		batches: [][]SearchDocument{{
			{ID: "doc-1"},
			{ID: "doc-2"},
		}},
	})

	err := svc.ReindexTenant(context.Background(), tenantID, "ru")
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{tenantID}, engine.deleteTenantCalls)
	require.Len(t, engine.upserts, 1)
	require.Equal(t, tenantID, engine.upserts[0][0].TenantID)
	require.Equal(t, "provider.streaming", engine.upserts[0][0].Provider)
}

func TestSearchResponse_Hits_ReturnsScoreOrderedHitsAcrossGroups(t *testing.T) {
	resp := SearchResponse{
		Groups: []SearchGroup{
			{
				Domain: ResultDomainNavigate,
				Hits: []SearchHit{{
					Document:   SearchDocument{ID: "navigate"},
					FinalScore: 1,
				}},
			},
			{
				Domain: ResultDomainLookup,
				Hits: []SearchHit{{
					Document:   SearchDocument{ID: "lookup"},
					FinalScore: 10,
				}},
			},
		},
	}

	hits := resp.Hits()
	require.Len(t, hits, 2)
	require.Equal(t, "lookup", hits[0].Document.ID)
	require.Equal(t, "navigate", hits[1].Document.ID)
}

func TestCreateSession_BindsSessionToPrincipal(t *testing.T) {
	tenantID := uuid.New()
	engine := &scriptedEngine{
		script: []scriptedCall{
			{
				hits: []SearchHit{
					{
						Document: SearchDocument{
							ID:         "fast",
							TenantID:   tenantID,
							Title:      "Fast match",
							EntityType: "client",
							Domain:     ResultDomainLookup,
							Access:     AccessPolicy{Visibility: VisibilityPublic},
						},
						FinalScore: 10,
					},
				},
			},
		},
	}
	svc := NewService(engine, nil, DefaultServiceConfig())

	snapshot, err := svc.CreateSession(context.Background(), SearchRequest{
		Query:    "1234567",
		TenantID: tenantID,
		UserID:   "7",
		TopK:     10,
		Intent:   SearchIntentMixed,
	})
	require.NoError(t, err)

	_, err = svc.SubscribeSession(context.Background(), snapshot.ID, SearchSessionAccess{
		TenantID: tenantID,
		UserID:   "8",
	})
	require.ErrorIs(t, err, ErrSessionNotFound)
}

func topSnapshotTitle(snapshot SearchSessionSnapshot) string {
	for _, group := range snapshot.Response.Groups {
		if len(group.Hits) > 0 {
			return group.Hits[0].Document.Title
		}
	}
	return ""
}
