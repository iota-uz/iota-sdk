package spotlight

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestDefaultGrouper_UsesDocumentMetadataForLookupGroup(t *testing.T) {
	tenantID := uuid.New()
	grouper := NewDefaultGrouper()

	resp := grouper.Group(context.Background(), SearchRequest{Query: "30833WAA"}, []SearchHit{
		{
			Document: SearchDocument{
				ID:       "policy-1",
				TenantID: tenantID,
				Domain:   ResultDomainLookup,
				Metadata: map[string]string{
					"group_key":       "policies",
					"group_title_key": "Spotlight.Group.Policies",
				},
			},
		},
	})

	require.Len(t, resp.Groups, 1)
	require.Equal(t, "policies", resp.Groups[0].Key)
	require.Equal(t, ResultDomainLookup, resp.Groups[0].Domain)
	require.Equal(t, "Spotlight.Group.Policies", resp.Groups[0].TitleKey)
}

func TestSearchResponse_HitsFlattensGroupsInOrder(t *testing.T) {
	resp := SearchResponse{
		Groups: []SearchGroup{
			{
				Key: "navigate",
				Hits: []SearchHit{
					{Document: SearchDocument{ID: "nav-1"}},
				},
			},
			{
				Key: "data",
				Hits: []SearchHit{
					{Document: SearchDocument{ID: "data-1"}},
					{Document: SearchDocument{ID: "data-2"}},
				},
			},
		},
	}

	hits := resp.Hits()
	require.Len(t, hits, 3)
	require.Equal(t, []string{"nav-1", "data-1", "data-2"}, []string{
		hits[0].Document.ID,
		hits[1].Document.ID,
		hits[2].Document.ID,
	})
}
