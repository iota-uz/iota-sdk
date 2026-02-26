package spotlight

import (
	"context"
	"sort"
	"strings"
)

const (
	DefaultLexicalWeight = 0.75
	DefaultVectorWeight  = 0.25
)

type Ranker interface {
	Rank(ctx context.Context, req SearchRequest, hits []SearchHit) []SearchHit
}

type DefaultRanker struct{}

func NewDefaultRanker() *DefaultRanker {
	return &DefaultRanker{}
}

func (r *DefaultRanker) Rank(_ context.Context, req SearchRequest, hits []SearchHit) []SearchHit {
	if len(hits) == 0 {
		return []SearchHit{}
	}
	out := make([]SearchHit, 0, len(hits))
	for _, hit := range hits {
		scored := hit
		if scored.FinalScore == 0 {
			scored.FinalScore = scored.LexicalScore*DefaultLexicalWeight + scored.VectorScore*DefaultVectorWeight
		}
		scored.FinalScore += titleMatchBonus(req.Query, scored.Document.Title)
		out = append(out, scored)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].FinalScore > out[j].FinalScore
	})
	return out
}

// titleMatchBonus gives a small score boost to hits whose title closely matches the query.
// This ensures "Users" ranks above "New User" when searching "user".
func titleMatchBonus(query, title string) float64 {
	q := strings.ToLower(strings.TrimSpace(query))
	t := strings.ToLower(strings.TrimSpace(title))
	if q == "" || t == "" {
		return 0
	}
	if t == q {
		return 0.05
	}
	if strings.HasPrefix(t, q) {
		return 0.03
	}
	if strings.Contains(t, q) {
		return 0.02 * float64(len(q)) / float64(len(t))
	}
	return 0
}
