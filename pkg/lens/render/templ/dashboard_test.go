package templ

import (
	"bytes"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

func TestPanelFullscreenHeaderRendersVisibleMetricInfoText(t *testing.T) {
	t.Parallel()

	var html bytes.Buffer
	err := PanelFullscreenHeader(
		panel.Bar("sales", "Sales Report", "sales").
			Info("Shows how revenue is aggregated for the selected period.").
			Description("Secondary chart description").
			Build(),
		chartText{
			CloseFullscreen: "Close fullscreen",
		},
	).Render(metricInfoContext(t, language.English), &html)
	require.NoError(t, err)

	rendered := html.String()
	require.Contains(t, rendered, "Shows how revenue is aggregated for the selected period.")
	require.Contains(t, rendered, "Secondary chart description")
	require.Contains(t, rendered, "x-tooltip.interactive.theme.light.html.raw")
}
