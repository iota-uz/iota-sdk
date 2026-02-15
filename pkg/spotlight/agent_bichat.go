package spotlight

import (
	"context"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/bichat/kb"
)

type BIChatAgent struct {
	searcher kb.KBSearcher
}

func NewBIChatAgent(searcher kb.KBSearcher) *BIChatAgent {
	return &BIChatAgent{searcher: searcher}
}

func (a *BIChatAgent) Answer(ctx context.Context, req SearchRequest, hits []SearchHit) (*AgentAnswer, error) {
	query := strings.TrimSpace(req.Query)
	if query == "" {
		return nil, nil
	}

	citations := make([]SearchDocument, 0, 4)
	actions := make([]AgentAction, 0, 4)

	for i := 0; i < len(hits) && i < 3; i++ {
		citations = append(citations, hits[i].Document)
		actions = append(actions, AgentAction{
			Type:              ActionTypeNavigate,
			Label:             "Open " + hits[i].Document.Title,
			TargetURL:         hits[i].Document.URL,
			NeedsConfirmation: true,
		})
	}

	if a.searcher != nil && a.searcher.IsAvailable() {
		results, err := a.searcher.Search(ctx, query, kb.SearchOptions{TopK: 2})
		if err == nil {
			for _, result := range results {
				citations = append(citations, SearchDocument{
					ID:         result.Document.ID,
					Provider:   "bichat.kb",
					EntityType: "knowledge",
					Title:      result.Document.Title,
					Body:       result.Excerpt,
					URL:        result.Document.Path,
					Language:   "",
					Metadata: map[string]string{
						"source": "bichat.kb",
					},
					Access:    AccessPolicy{Visibility: VisibilityPublic},
					UpdatedAt: result.Document.UpdatedAt,
				})
			}
		}
	}

	if len(actions) == 0 && len(citations) == 0 {
		return nil, nil
	}

	summary := "Best matches found for your request"
	if req.Intent == SearchIntentHelp || strings.Contains(strings.ToLower(query), "how") {
		summary = "Here are the best matching pages and knowledge entries"
	}

	return &AgentAnswer{Summary: summary, Citations: citations, Actions: actions}, nil
}
