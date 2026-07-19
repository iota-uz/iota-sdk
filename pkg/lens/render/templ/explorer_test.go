package templ

import (
	"context"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/stretchr/testify/require"
)

func TestClientActionPreservesConcreteTerminalSemantics(t *testing.T) {
	t.Parallel()

	spec := action.Navigate("/portfolio", action.LiteralParam("product", "osago"))
	spec.PreserveQuery = true
	spec.Payload = map[string]action.ValueSource{
		"source":  action.LiteralValue("explorer"),
		"ignored": action.FieldValue("row_value"),
	}

	client := clientAction(&spec)

	require.Equal(t, "navigate", client.Kind)
	require.Equal(t, "/portfolio", client.URL)
	require.True(t, client.PreserveQuery)
	require.Equal(t, "osago", client.Params["product"])
	require.Equal(t, map[string]any{"source": "explorer"}, client.Payload)
}

func TestLocalizedExplorerTextHasGenericFallbacks(t *testing.T) {
	t.Parallel()

	text := localizedExplorerText(context.Background())

	require.Equal(t, "Back", text.Back)
	require.Equal(t, "Back to start", text.Home)
	require.Equal(t, "Expand chart", text.Expand)
	require.Equal(t, "Close fullscreen", text.Collapse)
	require.Equal(t, "Loading…", text.Loading)
	require.Equal(t, "This view has no data.", text.Unavailable)
	require.Equal(t, "Retry", text.Retry)
	require.Equal(t, "Unable to load this view.", text.Error)
}
