package apex

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/iota-uz/iota-sdk/components/charts"
	"github.com/iota-uz/iota-sdk/pkg/lens/format"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
)

// buildDrillHierarchyJS builds the sole DataPointSelection handler for a Bar
// panel carrying panel.DrillHierarchy: a client-side "zoom" state machine
// (year -> quarter, and expand a trailing-years "Others" bucket) that
// requires zero further server round-trips. Axis planning (log-vs-linear
// decision, min/max/tick config) happens here, once per level, by reusing
// buildLogarithmicAxisPlan/logarithmicSeriesData/supportsManualLogScale — the
// exact same functions that plan the panel's own initial (level 0) axis —
// rather than duplicating that logic client-side. The actual zoom/back state
// machine is a single shared function (window.__lensDrillHierarchyClick,
// defined once in render/templ's shared script block) that every
// DrillHierarchy panel's generated handler delegates to; this function only
// prepares the per-panel JSON config blob.
func buildDrillHierarchyJS(spec *panel.DrillHierarchy, formatterSpec *format.Spec, locale string) templ.JSExpression {
	configJS, ok := drillHierarchyConfigJS(spec, formatterSpec, locale)
	if !ok {
		return ""
	}
	return templ.JSExpression(fmt.Sprintf(`function(event, chartContext, opts) {
		var cfg = %s;
		if (window.__lensDrillHierarchyClick) {
			window.__lensDrillHierarchyClick(chartContext, opts, cfg);
		}
	}`, configJS))
}

// buildDrillHierarchyMountJS re-derives the exact same cfg blob as
// buildDrillHierarchyJS and hands it to the shared JS state machine on every
// (re)mount — including the lazy fullscreen chart instance, which has no
// click history of its own yet. This is what lets the fullscreen chart pick
// up whatever zoom level its sibling (in the same [data-lens-rerender-scope])
// last left behind, without requiring a click first.
func buildDrillHierarchyMountJS(spec *panel.DrillHierarchy, formatterSpec *format.Spec, locale string) templ.JSExpression {
	configJS, ok := drillHierarchyConfigJS(spec, formatterSpec, locale)
	if !ok {
		return ""
	}
	return templ.JSExpression(fmt.Sprintf(`function(chartContext) {
		var cfg = %s;
		if (window.__lensDrillHierarchyMount) {
			window.__lensDrillHierarchyMount(chartContext, cfg);
		}
	}`, configJS))
}

// drillHierarchyConfigJS builds the JSON+formatter-closures config blob
// shared by the click and mount handlers. Axis planning (log-vs-linear
// decision, min/max/tick config) happens here, once per level, by reusing
// buildLogarithmicAxisPlan/logarithmicSeriesData/supportsManualLogScale — the
// exact same functions that plan the panel's own initial (level 0) axis —
// rather than duplicating that logic client-side.
func drillHierarchyConfigJS(spec *panel.DrillHierarchy, formatterSpec *format.Spec, locale string) (string, bool) {
	if spec == nil {
		return "", false
	}
	locale = normalizedChartLocale(locale)
	axisFormatter, tooltipFormatter := chartValueFormatters(formatterSpec, locale)

	cfg := drillHierarchyConfig{
		OthersLabel: spec.OthersLabel,
		Sources:     spec.Sources,
		BackLabel:   drillBackLabel(locale),
		Quarters:    map[string]drillQuarterConfig{},
	}
	if spec.OthersLabel != "" && len(spec.OthersYears) > 0 {
		cfg.Others = buildDrillLevelConfig(spec, spec.OthersYears)
	}
	for key, qb := range spec.Quarters {
		cfg.Quarters[key] = buildDrillQuarterConfig(qb)
	}

	axisFormatterJS := "null"
	if axisFormatter != "" {
		axisFormatterJS = "(" + string(axisFormatter) + ")"
	}
	tooltipFormatterJS := "null"
	if tooltipFormatter != "" {
		tooltipFormatterJS = "(" + string(tooltipFormatter) + ")"
	}

	configJS := fmt.Sprintf(`(function() {
		var cfg = %s;
		cfg.axisFormatter = %s;
		cfg.tooltipFormatter = %s;
		return cfg;
	})()`, mustJSONJS(cfg), axisFormatterJS, tooltipFormatterJS)
	return configJS, true
}

// buildDrillLevelConfig builds the "expand Others" level's dataset: one
// category per bucketed year, one series per source, floored + (if
// applicable) log-transformed exactly like the panel's own level-0 series.

func buildDrillLevelConfig(spec *panel.DrillHierarchy, years []int) *drillLevelConfig {
	categories := make([]string, len(years))
	for i, year := range years {
		categories[i] = strconv.Itoa(year)
	}
	series := make([]charts.Series, len(spec.Sources))
	for i, source := range spec.Sources {
		data := make([]any, len(years))
		for j, year := range years {
			data[j] = spec.Years[fmt.Sprintf("%d|%s", year, source)]
		}
		series[i] = charts.Series{Name: source, Data: data}
	}
	series, axis := floorAndScale(series)
	return &drillLevelConfig{
		Categories: categories,
		Series:     series,
		Axis:       axis,
	}
}

// buildDrillQuarterConfig builds one (year, source) pair's Q1..Q4 dataset:
// a single series, floored + (if applicable) log-transformed independently
// of every other quarter dataset — a single year's four quarters may span a
// far narrower magnitude range than the top-level chart's years, so each one
// gets its own log-vs-linear decision rather than inheriting the panel's.
func buildDrillQuarterConfig(qb panel.QuarterBreakdown) drillQuarterConfig {
	data := make([]any, len(qb.Amounts))
	for i, amount := range qb.Amounts {
		data[i] = amount
	}
	series, axis := floorAndScale([]charts.Series{{Data: data}})
	return drillQuarterConfig{
		Values:       series[0].Data,
		NavigateURLs: qb.NavigateURLs,
		Axis:         axis,
	}
}

// floorAndScale floors every non-positive/zero point to a small positive
// epsilon (one order of magnitude below the smallest positive value in the
// set, mirroring EAI's dashboards.buildPremiumBySourceYearChart) and, if the
// resulting series qualifies (see supportsManualLogScale), log-transforms it
// and returns a log axis plan; otherwise returns the floored-but-untransformed
// series with a linear axis. Never mutates the input slice's underlying data.
func floorAndScale(series []charts.Series) ([]charts.Series, drillAxisConfig) {
	minPositive := 0.0
	for _, s := range series {
		for _, point := range s.Data {
			value := numericValue(point)
			if value > 0 && (minPositive == 0 || value < minPositive) {
				minPositive = value
			}
		}
	}
	floor := 1.0
	if minPositive > 0 {
		floor = minPositive / 10
	}

	floored := make([]charts.Series, len(series))
	for i, s := range series {
		data := make([]any, len(s.Data))
		for j, point := range s.Data {
			data[j] = math.Max(numericValue(point), floor)
		}
		floored[i] = charts.Series{Name: s.Name, Data: data}
	}

	if !supportsManualLogScale(panel.Spec{Kind: panel.KindBar}, floored) {
		return floored, drillAxisConfig{Scale: "linear"}
	}
	plan, ok := buildLogarithmicAxisPlan(floored, 10)
	if !ok {
		return floored, drillAxisConfig{Scale: "linear"}
	}
	scaled := make([]charts.Series, len(floored))
	for i, s := range floored {
		scaled[i] = charts.Series{Name: s.Name, Data: logarithmicSeriesData(s.Data, 10)}
	}
	return scaled, drillAxisConfig{
		Scale:      "log",
		Base:       plan.Base,
		Min:        plan.MinExponent,
		Max:        plan.MaxExponent,
		Step:       plan.Step,
		TickAmount: plan.TickAmount,
	}
}

// drillBackLabel mirrors stackedBarTotalLabel's locale-prefix idiom (this
// render layer only has a locale string, not the page i18n localizer, so
// translations are hardcoded here rather than resolved via pageCtx.T). Values
// match the existing (previously unused) Lens.Drill.Back locale key in
// back/shared/presentation/lenssupport/locales/*.toml.
func drillBackLabel(locale string) string {
	switch {
	case strings.HasPrefix(locale, "ru"):
		return "Назад"
	case strings.HasPrefix(locale, "uz-Cyrl"):
		return "Орқага"
	case strings.HasPrefix(locale, "uz"):
		return "Orqaga"
	default:
		return "Back"
	}
}

type drillHierarchyConfig struct {
	Sources     []string                      `json:"sources"`
	OthersLabel string                        `json:"othersLabel,omitempty"`
	Others      *drillLevelConfig             `json:"others,omitempty"`
	Quarters    map[string]drillQuarterConfig `json:"quarters"`
	BackLabel   string                        `json:"backLabel"`
}

type drillLevelConfig struct {
	Categories []string        `json:"categories"`
	Series     []charts.Series `json:"series"`
	Axis       drillAxisConfig `json:"axis"`
}

type drillQuarterConfig struct {
	Values       []any           `json:"values"`
	NavigateURLs [4]string       `json:"navigateUrls"`
	Axis         drillAxisConfig `json:"axis"`
}

type drillAxisConfig struct {
	Scale      string  `json:"scale"`
	Base       int     `json:"base,omitempty"`
	Min        float64 `json:"min,omitempty"`
	Max        float64 `json:"max,omitempty"`
	Step       float64 `json:"step,omitempty"`
	TickAmount int     `json:"tickAmount,omitempty"`
}
