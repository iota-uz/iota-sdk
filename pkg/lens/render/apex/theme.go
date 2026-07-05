package apex

import (
	"strconv"

	"github.com/iota-uz/iota-sdk/pkg/lens/theme"
)

// chartTheme carries the Apex-facing slice of the Lens design system v2
// tokens (pkg/lens/theme). Everything in options() and the raw-JS/raw-HTML
// builders reads styling values from apexTheme instead of scattering hex
// literals; the CSS side of the same tokens lives in
// render/templ.LensThemeStyles().
type chartTheme struct {
	// FontFamily is the Lens font stack used for axis labels, legends and
	// tooltip text.
	FontFamily string
	// AxisFontSize is the axis label font size (px CSS string).
	AxisFontSize string
	// AxisLabelColor colors x/y axis labels.
	AxisLabelColor string
	// GridColor is the gridline/hairline color inside the plot.
	GridColor string
	// LegendFontSize is the legend label font size (px CSS string).
	LegendFontSize string
	// LegendColor colors legend labels.
	LegendColor string
	// Text is the default body text color (raw-HTML tooltip rows, badges).
	Text string
	// TextStrong is the high-emphasis text color (tooltip totals).
	TextStrong string
	// TextFaint is the lowest-emphasis color, used as the fallback tooltip
	// marker swatch when a series has no resolved color.
	TextFaint string
	// Border is the hairline for floating chips (total badge).
	Border string
	// Surface is the light chip/tooltip background.
	Surface string
	// TooltipDividerColor separates the totals row in raw-HTML tooltips
	// (light-theme hairline).
	TooltipDividerColor string
	// TooltipCSSClass is added to the Apex tooltip container so the
	// .lens-tooltip light skin (render/templ/theme.templ) applies.
	TooltipCSSClass string
	// NumeralCSSClass is set on axis label <text> elements so tabular
	// numerals apply (.apexcharts-text.lens-num, render/templ/theme.templ).
	NumeralCSSClass string
}

var apexTheme = chartTheme{
	FontFamily:          theme.FontFamily,
	AxisFontSize:        strconv.Itoa(theme.AxisFontSizePx) + "px",
	AxisLabelColor:      theme.TextMuted,
	GridColor:           theme.Divider,
	LegendFontSize:      strconv.Itoa(theme.AxisFontSizePx) + "px",
	LegendColor:         theme.TextMuted,
	Text:                theme.Text,
	TextStrong:          theme.TextStrong,
	TextFaint:           theme.TextFaint,
	Border:              theme.Border,
	Surface:             theme.BgCard,
	TooltipDividerColor: "rgba(15,23,42,0.08)",
	TooltipCSSClass:     "lens-tooltip",
	NumeralCSSClass:     "lens-num",
}

// Bar geometry.
const (
	// barBorderRadius is the corner radius applied to the outer end of bars.
	barBorderRadius = 3
	// horizontalBarHeight is the plotOptions.bar.barHeight for horizontal bars.
	horizontalBarHeight = "55%"
	// donutSize is the plotOptions.pie.donut.size hollow ratio.
	donutSize = "78%"
)

// adaptiveColumnWidth picks a plotOptions.bar.columnWidth so sparse category
// sets don't render as slabs and dense ones don't collapse into slivers.
func adaptiveColumnWidth(categoryCount int) string {
	switch {
	case categoryCount <= 4:
		return "32%"
	case categoryCount <= 12:
		return "48%"
	default:
		return "62%"
	}
}
