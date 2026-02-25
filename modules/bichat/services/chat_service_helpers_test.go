package services

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/stretchr/testify/require"
)

func TestMapsValues_DeterministicOrderByKey(t *testing.T) {
	in := map[string]types.ToolArtifact{
		"b|artifact-b|https://example.com/b": {
			Type: "code_output",
			Name: "artifact-b",
			URL:  "https://example.com/b",
		},
		"a|artifact-a|https://example.com/a": {
			Type: "code_output",
			Name: "artifact-a",
			URL:  "https://example.com/a",
		},
	}

	got := mapsValues(in)
	require.Len(t, got, 2)
	require.Equal(t, "artifact-a", got[0].Name)
	require.Equal(t, "artifact-b", got[1].Name)
}
