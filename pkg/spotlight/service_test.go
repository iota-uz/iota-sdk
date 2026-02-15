package spotlight

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type testEngine struct {
	mu          sync.Mutex
	searchCalls int
	searchHits  []SearchHit
	searchErr   error
}

func (e *testEngine) Upsert(context.Context, []SearchDocument) error { return nil }

func (e *testEngine) Delete(context.Context, []DocumentRef) error { return nil }

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

func (e *testEngine) calls() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.searchCalls
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

func TestSpotlightService_Search_UsesBatchACLAndCache(t *testing.T) {
	t.Helper()

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
	require.NotEmpty(t, resp1.Other)

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
