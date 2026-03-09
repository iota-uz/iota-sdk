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

	cases := []struct {
		name string
		vars map[string]any
		want bool
	}{
		{name: "missing_map", vars: nil, want: false},
		{name: "string_false", vars: map[string]any{"enabled": "false"}, want: false},
		{name: "string_true", vars: map[string]any{"enabled": "true"}, want: true},
		{name: "bool_true", vars: map[string]any{"enabled": true}, want: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, variableBool(tc.vars, "enabled"))
		})
	}
}
