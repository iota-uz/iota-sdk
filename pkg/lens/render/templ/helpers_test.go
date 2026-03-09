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

func TestActionURLSupportsHtmxActions(t *testing.T) {
	t.Parallel()

	url := actionURL(&action.Spec{
		Kind: action.KindHtmxSwap,
		URL:  "/contracts",
		Params: []action.Param{
			action.FieldParam("product", "product_id"),
		},
	}, map[string]any{
		"product_id": "osago",
	}, nil)

	require.Equal(t, "/contracts?product=osago", url)
}

func TestActionOnClickSupportsEmitEventFallbacks(t *testing.T) {
	t.Parallel()

	onClick := actionOnClick(&action.Spec{
		Kind:  action.KindEmitEvent,
		Event: "lens:drilldown",
		Payload: map[string]action.ValueSource{
			"product": {
				Kind:     action.SourceField,
				Name:     "product_id",
				Fallback: "default-product",
			},
		},
	}, map[string]any{}, nil)

	require.Contains(t, onClick.Call, "lens:drilldown")
	require.Contains(t, onClick.Call, "default-product")
}

func TestActionOnClickSupportsHtmxSwap(t *testing.T) {
	t.Parallel()

	onClick := actionOnClick(&action.Spec{
		Kind:   action.KindHtmxSwap,
		URL:    "/contracts",
		Target: "#report",
		Params: []action.Param{
			action.LiteralParam("scope", "daily"),
		},
	}, nil, nil)

	require.Contains(t, onClick.Call, "htmx.ajax")
	require.Contains(t, onClick.Call, "/contracts?scope=daily")
	require.Contains(t, onClick.Call, "#report")
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

func TestFormatValueReturnsEmptyStringForNil(t *testing.T) {
	t.Parallel()

	require.Empty(t, formatValue(nil, nil, "", ""))
}
