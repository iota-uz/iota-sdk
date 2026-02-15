package spotlight

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/kb"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type BIChatAgent struct {
	searcher kb.KBSearcher
}

func NewBIChatAgent(searcher kb.KBSearcher) *BIChatAgent {
	return &BIChatAgent{searcher: searcher}
}

func (a *BIChatAgent) Answer(ctx context.Context, req SearchRequest, hits []SearchHit) (*AgentAnswer, error) {
	const op serrors.Op = "spotlight.BIChatAgent.Answer"

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
		tenantID := req.TenantID
		if tenantID == uuid.Nil {
			if resolvedTenantID, tenantErr := composables.UseTenantID(ctx); tenantErr == nil {
				tenantID = resolvedTenantID
			}
		}
		results, err := a.searcher.Search(ctx, query, kb.SearchOptions{TopK: 2})
		if err != nil {
			return nil, serrors.E(op, err)
		}
		for _, result := range results {
			access := AccessPolicy{Visibility: VisibilityRestricted}
			if req.UserID != "" {
				access.AllowedUsers = []string{req.UserID}
			}
			citations = append(citations, SearchDocument{
				ID:         result.Document.ID,
				TenantID:   tenantID,
				Provider:   "bichat.kb",
				EntityType: "knowledge",
				Title:      result.Document.Title,
				Body:       result.Excerpt,
				URL:        result.Document.Path,
				Language:   "",
				Metadata: map[string]string{
					"source": "bichat.kb",
				},
				Access:    access,
				UpdatedAt: result.Document.UpdatedAt,
			})
		}
	}

	if len(actions) == 0 && len(citations) == 0 {
		return nil, nil
	}

	summary := "Best matches found for your request"
	if req.Intent == SearchIntentHelp || IsHowQuery(query) {
		summary = "Here are the best matching pages and knowledge entries"
	}

	return &AgentAnswer{Summary: summary, Citations: citations, Actions: actions}, nil
}
