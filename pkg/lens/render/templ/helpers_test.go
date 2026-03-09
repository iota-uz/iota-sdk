package templ

import (
	urlpkg "net/url"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/stretchr/testify/require"
)

func TestActionURLIncludesVariableParams(t *testing.T) {
	t.Parallel()

	url := actionURL(&action.Spec{
		Kind: action.KindNavigate,
		URL:  "/contracts",
		Params: []action.Param{
			action.FieldParam("product", "product_id"),
			action.VariableParam("active_only", "active_only"),
			action.LiteralParam("scope", "report"),
		},
	}, map[string]any{
		"product_id": "osago",
	}, map[string]any{
		"active_only": true,
	})

	parsed, err := urlpkg.Parse(url)
	require.NoError(t, err)
	require.Equal(t, "/contracts", parsed.Path)
	require.Equal(t, "osago", parsed.Query().Get("product"))
	require.Equal(t, "true", parsed.Query().Get("active_only"))
	require.Equal(t, "report", parsed.Query().Get("scope"))
}

func TestVariableBoolHandlesMissingAndTruthyValues(t *testing.T) {
	t.Parallel()

	require.False(t, variableBool(nil, "enabled"))
	require.False(t, variableBool(map[string]any{"enabled": "false"}, "enabled"))
	require.True(t, variableBool(map[string]any{"enabled": "true"}, "enabled"))
	require.True(t, variableBool(map[string]any{"enabled": true}, "enabled"))
}
