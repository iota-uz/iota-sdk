package templ

import (
	urlpkg "net/url"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/filter"
	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/stretchr/testify/assert"
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

func TestFilterModel_Scenarios(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		result *runtime.DashboardResult
		assert func(t *testing.T, model filter.Model)
	}{
		{
			name: "returns dashboard filters",
			result: &runtime.DashboardResult{
				Filters: filter.Model{
					Inputs: []filter.Input{{Name: "range"}},
				},
			},
			assert: func(t *testing.T, model filter.Model) {
				assert.Len(t, model.Inputs, 1)
				assert.Equal(t, "range", model.Inputs[0].Name)
			},
		},
		{
			name:   "returns empty model for nil result",
			result: nil,
			assert: func(t *testing.T, model filter.Model) {
				assert.Empty(t, model.Inputs)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			model := filterModel(tt.result)
			require.NotNil(t, &model)
			tt.assert(t, model)
		})
	}
}

func TestFormatValueReturnsEmptyStringForNil(t *testing.T) {
	t.Parallel()

	require.Empty(t, formatValue(nil, nil, "", ""))
}
