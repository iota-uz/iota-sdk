package spotlight

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestDefaultRanker_PrefersPolicyForExactLookupMatches(t *testing.T) {
	ranker := NewDefaultRanker()
	tenantID := uuid.New()

	ranked := ranker.Rank(context.Background(), SearchRequest{
		Query:    "30833WAA",
		TenantID: tenantID,
		Mode:     QueryModeLookup,
	}, []SearchHit{
		{
			Document: SearchDocument{
				ID:         "client",
				TenantID:   tenantID,
				Title:      "Gulchehra Shukurova",
				EntityType: "client",
			},
			FinalScore: 10,
			WhyMatched: "exact_terms",
		},
		{
			Document: SearchDocument{
				ID:         "policy",
				TenantID:   tenantID,
				Title:      "EEIL0388215",
				EntityType: "policy",
			},
			FinalScore: 10,
			WhyMatched: "exact_terms",
		},
	})

	require.Len(t, ranked, 2)
	require.Equal(t, "policy", ranked[0].Document.ID)
	require.Equal(t, "client", ranked[1].Document.ID)
}
