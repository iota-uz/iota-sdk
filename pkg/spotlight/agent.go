package spotlight

import (
	"context"
	"errors"
	"strings"
)

var ErrNoAgentAnswer = errors.New("spotlight: no agent answer")

type HeuristicAgent struct{}

func NewHeuristicAgent() *HeuristicAgent {
	return &HeuristicAgent{}
}

func (a *HeuristicAgent) Answer(_ context.Context, req SearchRequest, hits []SearchHit) (*AgentAnswer, error) {
	query := strings.TrimSpace(req.Query)
	if query == "" || len(hits) == 0 {
		return nil, ErrNoAgentAnswer
	}

	top := hits[0]
	summary := "Best match found"
	if IsHowQuery(query) || req.Intent == SearchIntentHelp {
		summary = "Here is the best matching page for your request"
	}

	answer := &AgentAnswer{
		Summary:   summary,
		Citations: []SearchDocument{top.Document},
		Actions: []AgentAction{
			{
				Type:              ActionTypeNavigate,
				Label:             "Open " + top.Document.Title,
				TargetURL:         top.Document.URL,
				NeedsConfirmation: true,
			},
		},
	}

	return answer, nil
}
