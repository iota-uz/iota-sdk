package kb

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/bichat/kb"
)

// KBSearcherAdapter adapts kb.KBSearcher to the tool-facing KBSearcher contract.
type KBSearcherAdapter struct {
	searcher kb.KBSearcher
}

// NewKBSearcherAdapter creates an adapter for using kb.KBSearcher with KBSearchTool.
func NewKBSearcherAdapter(searcher kb.KBSearcher) KBSearcher {
	return &KBSearcherAdapter{searcher: searcher}
}

// Search adapts search options and maps kb.SearchResult to tool SearchResult.
func (a *KBSearcherAdapter) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	opts := kb.SearchOptions{TopK: limit}
	results, err := a.searcher.Search(ctx, query, opts)
	if err != nil {
		return nil, err
	}

	mapped := make([]SearchResult, 0, len(results))
	for _, res := range results {
		metadata := make(map[string]interface{}, len(res.Document.Metadata))
		for k, v := range res.Document.Metadata {
			metadata[k] = v
		}

		mapped = append(mapped, SearchResult{
			ID:       res.Document.ID,
			Title:    res.Document.Title,
			Content:  res.Document.Content,
			Score:    res.Score,
			Excerpt:  res.Excerpt,
			Metadata: metadata,
		})
	}

	return mapped, nil
}

// IsAvailable proxies availability checks to the underlying KB searcher.
func (a *KBSearcherAdapter) IsAvailable() bool {
	return a.searcher != nil && a.searcher.IsAvailable()
}
