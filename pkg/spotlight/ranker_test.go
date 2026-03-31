package spotlight

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestDefaultRanker_UsesMetadataBoostForExactLookupMatches(t *testing.T) {
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
				ID:         "priority",
				TenantID:   tenantID,
				Title:      "Priority record",
				EntityType: "custom_record",
				Metadata: map[string]string{
					"rank_boost": "0.08",
				},
			},
			FinalScore: 10,
			WhyMatched: "exact_terms",
		},
	})

	require.Len(t, ranked, 2)
	require.Equal(t, "priority", ranked[0].Document.ID)
	require.Equal(t, "client", ranked[1].Document.ID)
}

func TestDefaultRanker_IgnoresNonFiniteMetadataBoost(t *testing.T) {
	ranker := NewDefaultRanker()
	tenantID := uuid.New()

	ranked := ranker.Rank(context.Background(), SearchRequest{
		Query:    "30833WAA",
		TenantID: tenantID,
		Mode:     QueryModeLookup,
	}, []SearchHit{
		{
			Document: SearchDocument{
				ID:         "nan-boost",
				TenantID:   tenantID,
				Title:      "NaN record",
				EntityType: "custom_record",
				Metadata: map[string]string{
					"rank_boost": "NaN",
				},
			},
			FinalScore: 10,
			WhyMatched: "exact_terms",
		},
		{
			Document: SearchDocument{
				ID:         "baseline",
				TenantID:   tenantID,
				Title:      "Baseline",
				EntityType: "custom_record",
			},
			FinalScore: 10,
			WhyMatched: "exact_terms",
		},
	})

	require.Len(t, ranked, 2)
	require.InDelta(t, 10.0, ranked[0].FinalScore, 1e-9)
	require.InDelta(t, 10.0, ranked[1].FinalScore, 1e-9)
}
