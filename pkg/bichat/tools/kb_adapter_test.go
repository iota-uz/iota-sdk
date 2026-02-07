package tools

import (
	"context"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/bichat/kb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockCanonicalKBSearcher struct {
	results     []kb.SearchResult
	searchErr   error
	lastQuery   string
	lastOptions kb.SearchOptions
	available   bool
}

func (m *mockCanonicalKBSearcher) Search(ctx context.Context, query string, opts kb.SearchOptions) ([]kb.SearchResult, error) {
	m.lastQuery = query
	m.lastOptions = opts
	if m.searchErr != nil {
		return nil, m.searchErr
	}
	return m.results, nil
}

func (m *mockCanonicalKBSearcher) GetDocument(ctx context.Context, id string) (*kb.Document, error) {
	return nil, nil
}

func (m *mockCanonicalKBSearcher) IsAvailable() bool {
	return m.available
}

func TestKBSearcherAdapter_Search_MapsResultsAndLimit(t *testing.T) {
	t.Parallel()

	src := &mockCanonicalKBSearcher{
		available: true,
		results: []kb.SearchResult{
			{
				Document: kb.Document{
					ID:      "doc-1",
					Title:   "Revenue Rules",
					Content: "Use posted_at for revenue timing",
					Metadata: map[string]string{
						"source": "business",
					},
					UpdatedAt: time.Now(),
				},
				Score:   0.93,
				Excerpt: "posted_at for revenue timing",
			},
		},
	}

	adapter := NewKBSearcherAdapter(src)
	results, err := adapter.Search(context.Background(), "revenue timing", 7)
	require.NoError(t, err)
	require.Len(t, results, 1)

	assert.Equal(t, "revenue timing", src.lastQuery)
	assert.Equal(t, 7, src.lastOptions.TopK)
	assert.Equal(t, "doc-1", results[0].ID)
	assert.Equal(t, "Revenue Rules", results[0].Title)
	assert.Equal(t, 0.93, results[0].Score)
	assert.Equal(t, "business", results[0].Metadata["source"])
}

func TestKBSearcherAdapter_IsAvailable(t *testing.T) {
	t.Parallel()

	adapter := NewKBSearcherAdapter(&mockCanonicalKBSearcher{available: true})
	assert.True(t, adapter.IsAvailable())
}
