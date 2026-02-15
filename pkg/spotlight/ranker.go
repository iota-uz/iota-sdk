package spotlight

import (
	"context"
	"sort"
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

func (r *DefaultRanker) Rank(_ context.Context, _ SearchRequest, hits []SearchHit) []SearchHit {
	if len(hits) == 0 {
		return []SearchHit{}
	}
	out := make([]SearchHit, 0, len(hits))
	for _, hit := range hits {
		scored := hit
		if scored.FinalScore == 0 {
			scored.FinalScore = scored.LexicalScore*DefaultLexicalWeight + scored.VectorScore*DefaultVectorWeight
		}
		out = append(out, scored)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].FinalScore > out[j].FinalScore
	})
	return out
}
