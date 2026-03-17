package templ

import (
	"context"
	urlpkg "net/url"
	"testing"
	"time"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/cube"
	"github.com/iota-uz/iota-sdk/pkg/lens/filter"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
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
	}, &runtime.PanelResult{Variables: map[string]any{
		"active_only": true,
	}})

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
	}, &runtime.PanelResult{})

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
	}, map[string]any{}, &runtime.PanelResult{})

	require.Contains(t, onClick.Call, "lens:drilldown")
	require.Contains(t, onClick.Call, "default-product")
}

func TestActionOnClickPreservesTimePayloadValues(t *testing.T) {
	t.Parallel()

	timestamp := time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC)
	onClick := actionOnClick(&action.Spec{
		Kind:  action.KindEmitEvent,
		Event: "lens:drilldown",
		Payload: map[string]action.ValueSource{
			"from": {
				Kind:  action.SourceLiteral,
				Value: timestamp,
			},
		},
	}, nil, &runtime.PanelResult{})

	require.Contains(t, onClick.Call, "2026-03-09T00:00:00Z")
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
	}, nil, &runtime.PanelResult{})

	require.Contains(t, onClick.Call, "htmx.ajax")
	require.Contains(t, onClick.Call, "/contracts?scope=daily")
	require.Contains(t, onClick.Call, "#report")
}

func TestActionURLPreservesExistingCubeDrillFilters(t *testing.T) {
	t.Parallel()

	url := actionURL(&action.Spec{
		Kind: action.KindCubeDrill,
		URL:  "/insurance/sales-report",
		Drill: &action.DrillSpec{
			Dimension: "region",
			Value:     action.FieldValue("filter_value"),
		},
	}, map[string]any{
		"filter_value": "tashkent",
	}, &runtime.PanelResult{
		Request: urlpkg.Values{
			cube.QueryFilter: []string{"product:osago"},
		},
	})

	parsed, err := urlpkg.Parse(url)
	require.NoError(t, err)
	require.Equal(t, []string{"product:osago", "region:tashkent"}, parsed.Query()[cube.QueryFilter])
}

func TestFilterModel_Scenarios(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		result *runtime.Result
		assert func(t *testing.T, model filter.Model)
	}{
		{
			name: "returns dashboard filters",
			result: &runtime.Result{
				Filters: filter.Model{
					Inputs: []filter.Input{{Name: "range"}},
				},
			},
			assert: func(t *testing.T, model filter.Model) {
				t.Helper()
				assert.Len(t, model.Inputs, 1)
				assert.Equal(t, "range", model.Inputs[0].Name)
			},
		},
		{
			name:   "returns empty model for nil result",
			result: nil,
			assert: func(t *testing.T, model filter.Model) {
				t.Helper()
				assert.Empty(t, model.Inputs)
			},
		},
	}

	for _, tt := range tests {
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

func TestAssignQueryValuePreservesAllArrayItems(t *testing.T) {
	t.Parallel()

	values := urlpkg.Values{}
	assignQueryValue(values, "products", []any{"osago", "travel", "", 42})

	require.Equal(t, []string{"osago", "travel", "42"}, values["products"])
}

func TestJoinURLQueryReturnsEmptyForBlankBase(t *testing.T) {
	t.Parallel()

	url := joinURLQuery("", urlpkg.Values{"scope": []string{"sales"}})
	require.Empty(t, url)
}

func TestPanelUsesRadialActionSurface(t *testing.T) {
	t.Parallel()

	drillDonut := panel.Donut("payment-method", "Payment Method", "sales").
		LabelField("label").
		ValueField("value").
		Action(action.CubeDrill("/crm/reports/sales", "payment_method").WithDrillValue(action.FieldValue("filter_value"))).
		Build()
	plainBar := panel.Bar("daily-sales", "Daily Sales", "sales").
		CategoryField("day").
		ValueField("value").
		Build()

	require.True(t, panelUsesRadialActionSurface(drillDonut))
	require.False(t, panelUsesRadialActionSurface(plainBar))
}

func TestPanelChartClassAddsRadialActionClass(t *testing.T) {
	t.Parallel()

	drillPie := panel.Pie("product-share", "Product Share", "sales").
		LabelField("label").
		ValueField("value").
		Action(action.Navigate("/reports/drill")).
		Build()
	plainPie := panel.Pie("static-share", "Static Share", "sales").
		LabelField("label").
		ValueField("value").
		Build()

	require.Contains(t, panelChartClass(drillPie, false), "lens-chart--radial-action")
	require.NotContains(t, panelChartClass(plainPie, false), "lens-chart--radial-action")
	require.Contains(t, panelChartClass(drillPie, true), "min-h-[420px]")
}

func TestDrillNavigationModelPreservesBaseQueryAndDisplayLabels(t *testing.T) {
	t.Parallel()

	model := drillNavigationModel(metricInfoContext(t, language.English), &runtime.Result{
		Spec: lens.DashboardSpec{
			Drill: &lens.DrillMeta{
				BaseURL: "/insurance/sales-report",
				Dimensions: []lens.DrillDimensionMeta{
					{Name: "product", Label: "Product"},
					{Name: "region", Label: "Region"},
					{Name: "agency", Label: "Agency"},
				},
				Filters: []lens.DrillFilterMeta{
					{Dimension: "product", Value: "9d5877", Display: "OSAGO"},
					{Dimension: "region", Value: "d9206a", Display: "Tashkent"},
				},
				RemainingDimensions: []lens.DrillDimensionMeta{
					{Name: "agency", Label: "Agency"},
				},
			},
		},
		Drill: &cube.DrillContext{
			Filters: []cube.DimensionFilter{
				{Dimension: "product", Value: "9d5877"},
				{Dimension: "region", Value: "d9206a"},
			},
		},
		Request: urlpkg.Values{
			"ActualRangeStart": []string{"2026-02-14"},
			"ActualRangeEnd":   []string{"2026-03-15"},
			cube.QueryFilter:   []string{"product:9d5877", "region:d9206a"},
		},
	})

	require.True(t, model.HasNav)
	require.Equal(t, "Tashkent", model.CurrentDisplay)
	require.Len(t, model.Trail, 2)
	require.Equal(t, "All", model.Trail[0].Label)
	require.Contains(t, model.Trail[0].URL, "ActualRangeStart=2026-02-14")
	require.Contains(t, model.Trail[0].URL, "ActualRangeEnd=2026-03-15")
	require.Equal(t, "OSAGO", model.Trail[1].Label)
	require.Contains(t, model.Trail[1].URL, "_f=product%3A9d5877")
	require.Contains(t, model.Trail[1].URL, "ActualRangeStart=2026-02-14")
	require.Len(t, model.Remaining, 1)
	require.Contains(t, model.Remaining[0].URL, "_dim=agency")
	require.Contains(t, model.Remaining[0].URL, "ActualRangeEnd=2026-03-15")
	require.Equal(t, []drillSummaryItem{
		{Label: "Product", Value: "OSAGO"},
		{Label: "Region", Value: "Tashkent"},
	}, model.Summary)
}

func TestTablePaginationURLBuildsNextChunkRequest(t *testing.T) {
	t.Parallel()

	url := tablePaginationURL(&runtime.PanelResult{
		Panel:       panel.Spec{ID: "contracts-table"},
		RequestPath: "/insurance/sales-report/drill/contracts",
		Request: urlpkg.Values{
			"issue_at_from":  []string{"2026-03-01"},
			cube.QueryFilter: []string{"product:osago"},
		},
		TablePagination: &runtime.TablePagination{
			Page:    2,
			PerPage: 50,
			HasMore: true,
		},
	})

	parsed, err := urlpkg.Parse(url)
	require.NoError(t, err)
	require.Equal(t, "/insurance/sales-report/drill/contracts", parsed.Path)
	require.Equal(t, "2026-03-01", parsed.Query().Get("issue_at_from"))
	require.Equal(t, []string{"product:osago"}, parsed.Query()[cube.QueryFilter])
	require.Equal(t, "contracts-table", parsed.Query().Get(runtime.TablePaginationPanelQuery))
	require.Equal(t, "3", parsed.Query().Get(runtime.TablePaginationPageQuery))
	require.Equal(t, "50", parsed.Query().Get(runtime.TablePaginationLimitQuery))
}

func TestTablePaginationURLReturnsEmptyWithoutPathOrMorePages(t *testing.T) {
	t.Parallel()

	require.Empty(t, tablePaginationURL(&runtime.PanelResult{
		TablePagination: &runtime.TablePagination{Page: 1, PerPage: 50, HasMore: false},
	}))
	require.Empty(t, tablePaginationURL(&runtime.PanelResult{
		TablePagination: &runtime.TablePagination{Page: 1, PerPage: 50, HasMore: true},
	}))
}

func TestPanelMetricInfoTextPrefersExplicitInfo(t *testing.T) {
	t.Parallel()

	text := panelMetricInfoText(context.Background(), panel.Spec{
		Kind:        panel.KindBar,
		Description: "Chart subtitle",
		Info:        "Explicit metric info",
	})

	require.Equal(t, "Explicit metric info", text)
}

func TestPanelMetricInfoTextFallsBackForCharts(t *testing.T) {
	t.Parallel()

	ctx := metricInfoContext(t, language.English)

	require.Equal(t, `Shows how "Revenue" changes over time. Each point or bar aggregates records into its time period after applying the selected date range and active filters.`, panelMetricInfoText(ctx, panel.Spec{
		Kind:  panel.KindTimeSeries,
		Title: "Revenue",
	}))
	require.Equal(t, `This chart highlights how payment mix is aggregated for the current portfolio. Shows how the total is distributed across "Payment Method" segments. Each slice represents one category's aggregated share for the selected date range and active filters. Click a chart value to open the next level of detail for that segment.`, panelMetricInfoText(ctx, panel.Spec{
		Kind:        panel.KindDonut,
		Title:       "Payment Method",
		Description: "This chart highlights how payment mix is aggregated for the current portfolio",
		Action:      &action.Spec{Kind: action.KindCubeDrill},
	}))
}

func TestPanelMetricInfoTextLocalizesFallbackByContextLocale(t *testing.T) {
	t.Parallel()

	ctx := metricInfoContext(t, language.Russian)

	require.Equal(t, "Как рассчитывается метрика", localizedChartText(ctx).MetricInfo)
	require.Equal(t, "Показывает, как общий итог распределяется по сегментам «Способ оплаты». Каждый сектор отражает агрегированную долю одной категории за выбранный период и с учетом активных фильтров. Нажмите на значение на графике, чтобы открыть следующий уровень детализации по выбранному сегменту.", panelMetricInfoText(ctx, panel.Spec{
		Kind:   panel.KindDonut,
		Title:  "Способ оплаты",
		Action: &action.Spec{Kind: action.KindCubeDrill},
	}))
}

func TestPanelMetricInfoTextIncludesLogScaleGuidance(t *testing.T) {
	t.Parallel()

	ctx := metricInfoContext(t, language.English)

	require.Equal(t, `Compares totals by "Region" segment. Each bar or chart element aggregates the records that fall into one category after applying the selected date range and active filters. The axis uses a logarithmic scale so large and small categories remain readable on one chart.`, panelMetricInfoText(ctx, panel.Spec{
		Kind:      panel.KindBar,
		Title:     "Region",
		ValueAxis: panel.ValueAxis{Scale: panel.AxisScaleLogarithmic, LogBase: 10},
	}))
}

func TestPanelMetricInfoTextDoesNotFallbackForNonCharts(t *testing.T) {
	t.Parallel()

	require.Empty(t, panelMetricInfoText(context.Background(), panel.Spec{Kind: panel.KindTable}))
	require.Empty(t, panelMetricInfoText(context.Background(), panel.Spec{Kind: panel.KindStat}))
}

func metricInfoContext(t *testing.T, locale language.Tag) context.Context {
	t.Helper()

	bundle := i18n.NewBundle(language.English)
	bundle.AddMessages(language.English,
		&i18n.Message{ID: "Chart.MetricInfo", Other: "How this metric is calculated"},
		&i18n.Message{ID: "Lens.Drill.All", Other: "All"},
		&i18n.Message{ID: "Lens.Chart.Info.SubjectFallback", Other: "this metric"},
		&i18n.Message{ID: "Lens.Chart.Info.TimeSeries", Other: `Shows how "{{.Subject}}" changes over time. Each point or bar aggregates records into its time period after applying the selected date range and active filters.`},
		&i18n.Message{ID: "Lens.Chart.Info.Category", Other: `Compares totals by "{{.Subject}}" segment. Each bar or chart element aggregates the records that fall into one category after applying the selected date range and active filters.`},
		&i18n.Message{ID: "Lens.Chart.Info.Distribution", Other: `Shows how the total is distributed across "{{.Subject}}" segments. Each slice represents one category's aggregated share for the selected date range and active filters.`},
		&i18n.Message{ID: "Lens.Chart.Info.Gauge", Other: `Shows the current aggregated value for "{{.Subject}}" based on the selected date range and active filters.`},
		&i18n.Message{ID: "Lens.Chart.Info.Tabs", Other: `Lets you switch between several chart views for "{{.Subject}}" built from the same selected date range and active filters.`},
		&i18n.Message{ID: "Lens.Chart.Info.DrillHint", Other: "Click a chart value to open the next level of detail for that segment."},
		&i18n.Message{ID: "Lens.Chart.Info.LogScaleHint", Other: "The axis uses a logarithmic scale so large and small categories remain readable on one chart."},
	)
	bundle.AddMessages(language.Russian,
		&i18n.Message{ID: "Chart.MetricInfo", Other: "Как рассчитывается метрика"},
		&i18n.Message{ID: "Lens.Drill.All", Other: "Все"},
		&i18n.Message{ID: "Lens.Chart.Info.SubjectFallback", Other: "этот показатель"},
		&i18n.Message{ID: "Lens.Chart.Info.Distribution", Other: "Показывает, как общий итог распределяется по сегментам «{{.Subject}}». Каждый сектор отражает агрегированную долю одной категории за выбранный период и с учетом активных фильтров."},
		&i18n.Message{ID: "Lens.Chart.Info.DrillHint", Other: "Нажмите на значение на графике, чтобы открыть следующий уровень детализации по выбранному сегменту."},
	)

	pageCtx := types.NewPageContext(locale, &urlpkg.URL{Path: "/"}, i18n.NewLocalizer(bundle, locale.String()))
	return context.WithValue(context.Background(), constants.PageContext, pageCtx)
}
