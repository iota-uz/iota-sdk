package templ

import (
	"bytes"
	"errors"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

func TestPanelFullscreenHeader_RendersVisibleMetricInfoText(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name                string
		spec                panel.Spec
		chartText           chartText
		expectedContains    []string
		expectedNotContains []string
	}{
		{
			name: "renders visible subtitle paragraph without redundant tooltip button",
			spec: panel.Bar("sales", "Sales Report", "sales").
				Info("Shows how revenue is aggregated for the selected period.").
				Description("Secondary chart description").
				Build(),
			chartText: chartText{
				CloseFullscreen: "Close fullscreen",
			},
			expectedContains: []string{
				`<p class="mt-2 max-w-3xl text-sm leading-6 text-slate-600">Shows how revenue is aggregated for the selected period.</p>`,
				`<p class="mt-1 text-sm text-slate-500">Secondary chart description</p>`,
				`aria-label="Close fullscreen"`,
			},
			expectedNotContains: []string{
				`x-tooltip.interactive.theme.light.html.raw=`,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var html bytes.Buffer
			err := PanelFullscreenHeader(tc.spec, tc.chartText).Render(metricInfoContext(t, language.English), &html)
			require.NoError(t, err)

			rendered := html.String()
			for _, expected := range tc.expectedContains {
				assert.Contains(t, rendered, expected)
			}
			for _, unexpected := range tc.expectedNotContains {
				assert.NotContains(t, rendered, unexpected)
			}
		})
	}
}

func TestErrorState_RendersPanelErrorActionWhenProvided(t *testing.T) {
	t.Parallel()

	spec := panel.Bar("sales", "Sales Report", "sales").Build()
	result := &runtime.PanelResult{
		Panel: spec,
		Error: errors.New("dataset failed"),
	}

	var html bytes.Buffer
	err := ErrorState(spec, result, func(panelSpec panel.Spec, panelResult *runtime.PanelResult) *PanelErrorAction {
		require.Equal(t, "sales", panelSpec.ID)
		require.Equal(t, result, panelResult)
		return &PanelErrorAction{
			Label:  "Fix with AI",
			URL:    "/dashboards/123/fix/sales",
			Method: "post",
			Target: "closest [data-lens-swap-target]",
			Swap:   "innerHTML",
		}
	}).Render(metricInfoContext(t, language.English), &html)
	require.NoError(t, err)

	rendered := html.String()
	assert.Contains(t, rendered, "Fix with AI")
	assert.Contains(t, rendered, `hx-post="/dashboards/123/fix/sales"`)
	assert.Contains(t, rendered, `hx-target="closest [data-lens-swap-target]"`)
	assert.Contains(t, rendered, `hx-swap="innerHTML"`)
}

func TestErrorState_RendersHTMXAttributesForGetPanelErrorAction(t *testing.T) {
	t.Parallel()

	spec := panel.Bar("sales", "Sales Report", "sales").Build()
	result := &runtime.PanelResult{
		Panel: spec,
		Error: errors.New("dataset failed"),
	}

	var html bytes.Buffer
	err := ErrorState(spec, result, func(panelSpec panel.Spec, panelResult *runtime.PanelResult) *PanelErrorAction {
		require.Equal(t, "sales", panelSpec.ID)
		require.Equal(t, result, panelResult)
		return &PanelErrorAction{
			Label:   "Retry",
			URL:     "/dashboards/123/panels/sales/retry",
			Method:  "get",
			Target:  "#panel",
			Swap:    "outerHTML",
			Include: "#filters",
			Confirm: "Retry panel?",
		}
	}).Render(metricInfoContext(t, language.English), &html)
	require.NoError(t, err)

	rendered := html.String()
	assert.Contains(t, rendered, `href="/dashboards/123/panels/sales/retry"`)
	assert.Contains(t, rendered, `hx-get="/dashboards/123/panels/sales/retry"`)
	assert.Contains(t, rendered, `hx-target="#panel"`)
	assert.Contains(t, rendered, `hx-swap="outerHTML"`)
	assert.Contains(t, rendered, `hx-include="#filters"`)
	assert.Contains(t, rendered, `hx-confirm="Retry panel?"`)
}

func TestDashboard_RendersVariableComponentOverrides(t *testing.T) {
	t.Parallel()

	spec := lens.DashboardSpec{
		ID:    "filters",
		Title: "Filters",
		Variables: []lens.VariableSpec{
			{
				Name:      "product",
				Label:     "Product",
				Kind:      lens.VariableSingleSelect,
				Component: lens.VariableComponentTextInput,
			},
		},
	}

	var html bytes.Buffer
	err := Dashboard(DashboardProps{Spec: spec}).Render(metricInfoContext(t, language.English), &html)
	require.NoError(t, err)

	rendered := html.String()
	assert.Contains(t, rendered, `type="text" name="product"`)
	assert.NotContains(t, rendered, `<select name="product"`)
}

func TestDashboard_RendersMultiSelectOverrideValueInTextInput(t *testing.T) {
	t.Parallel()

	spec := lens.DashboardSpec{
		ID:    "filters",
		Title: "Filters",
		Variables: []lens.VariableSpec{
			{
				Name:      "product",
				Label:     "Product",
				Kind:      lens.VariableMultiSelect,
				Component: lens.VariableComponentTextInput,
				Default:   []string{"sku-1", "sku-2"},
			},
		},
	}

	var html bytes.Buffer
	err := Dashboard(DashboardProps{Spec: spec}).Render(metricInfoContext(t, language.English), &html)
	require.NoError(t, err)

	rendered := html.String()
	assert.Contains(t, rendered, `type="text" name="product" value="sku-1,sku-2"`)
}
