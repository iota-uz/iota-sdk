package spotlight

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestToViewResponse_UsesDocumentGroupMetadataWhenPresent(t *testing.T) {
	view := ToViewResponse(SearchResponse{
		Groups: []SearchGroup{
			{
				Domain: ResultDomainLookup,
				Hits: []SearchHit{
					{Document: SearchDocument{ID: "policy-1", EntityType: "policy", Metadata: map[string]string{"group_key": "policies", "group_title_key": "Spotlight.Group.Policies"}}},
					{Document: SearchDocument{ID: "vehicle-1", EntityType: "vehicle", Metadata: map[string]string{"group_key": "vehicles", "group_title_key": "Spotlight.Group.Vehicles"}}},
					{Document: SearchDocument{ID: "client-1", EntityType: "client", Metadata: map[string]string{"group_key": "people", "group_title_key": "Spotlight.Group.People"}}},
				},
			},
		},
	})

	require.Len(t, view.Groups, 3)
	require.Equal(t, "policies", view.Groups[0].Key)
	require.Equal(t, "Spotlight.Group.Policies", view.Groups[0].TitleKey)
	require.Equal(t, "vehicles", view.Groups[1].Key)
	require.Equal(t, "Spotlight.Group.Vehicles", view.Groups[1].TitleKey)
	require.Equal(t, "people", view.Groups[2].Key)
	require.Equal(t, "Spotlight.Group.People", view.Groups[2].TitleKey)
}
