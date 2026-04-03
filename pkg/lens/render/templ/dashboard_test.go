package templ

import (
	"bytes"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

func TestPanelFullscreenHeader_RendersVisibleMetricInfoText(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name             string
		spec             panel.Spec
		chartText        chartText
		expectedContains []string
	}{
		{
			name: "renders visible subtitle paragraph and tooltip button",
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
		})
	}
}
