package comparison

import (
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/format"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/stretchr/testify/require"
)

func TestResolvePeriodUsesCalendarSpansTimezoneAndLeapClamp(t *testing.T) {
	t.Parallel()
	location := time.FixedZone("UTC+5", 5*60*60)
	start := time.Date(2026, time.January, 1, 0, 0, 0, 0, location)
	end := time.Date(2026, time.March, 31, 23, 59, 59, int(time.Second-time.Nanosecond), location)

	previous := ResolvePeriod(lens.ComparePreviousPeriod, lens.DateRangeValue{Start: &start, End: &end}, "", "", location)
	require.Equal(t, time.Date(2025, time.October, 1, 0, 0, 0, 0, location), *previous.Range.Start)
	require.Equal(t, time.Date(2025, time.December, 31, 23, 59, 59, int(time.Second-time.Nanosecond), location), *previous.Range.End)

	leapStart := time.Date(2024, time.February, 1, 0, 0, 0, 0, location)
	leapEnd := time.Date(2024, time.February, 29, 23, 59, 59, int(time.Second-time.Nanosecond), location)
	yearAgo := ResolvePeriod(lens.CompareYearAgo, lens.DateRangeValue{Start: &leapStart, End: &leapEnd}, "", "", location)
	require.Equal(t, time.Date(2023, time.February, 28, 23, 59, 59, int(time.Second-time.Nanosecond), location), *yearAgo.Range.End)

	custom := ResolvePeriod(lens.CompareCustom, lens.DateRangeValue{}, "2024-04-01", "2024-04-30", location)
	require.Equal(t, time.Date(2024, time.April, 1, 0, 0, 0, 0, location), *custom.Range.Start)
	require.Equal(t, time.Date(2024, time.April, 30, 23, 59, 59, int(time.Second-time.Nanosecond), location), *custom.Range.End)
}

func TestApplyStaticUsesStableIdentityAndAbsoluteBaseline(t *testing.T) {
	t.Parallel()

	current := lens.DashboardSpec{
		Datasets: []lens.DatasetSpec{staticDataset(t, "metrics", []string{"loss", "profit"}, []float64{-60, 30})},
		Rows: []lens.RowSpec{{Panels: []panel.Spec{{
			ID: "metrics", Kind: panel.KindHorizontalBar, Dataset: "metrics",
			Fields: panel.FieldMapping{Category: panel.Ref("key"), Value: panel.Ref("value")},
		}}}},
	}
	baseline := lens.DashboardSpec{Datasets: []lens.DatasetSpec{
		staticDataset(t, "metrics", []string{"profit", "loss"}, []float64{20, -100}),
	}}

	require.NoError(t, ApplyStatic(&current, &baseline, StaticOptions{}))
	primary := current.Datasets[0].Static.Primary()
	require.Equal(t, []any{-100.0, 20.0}, primary.MustField(PreviousField("value")).Values)
	require.Equal(t, []any{40.0, 10.0}, primary.MustField(DeltaField("value")).Values)
	require.Equal(t, []any{40.0, 50.0}, primary.MustField(DeltaPercentField("value")).Values)
	require.Equal(t, PreviousField("value"), current.Rows[0].Panels[0].Fields.Previous.Name())
}

func TestApplyStaticInferredIdentityIncludesSeries(t *testing.T) {
	t.Parallel()

	current := lens.DashboardSpec{Datasets: []lens.DatasetSpec{
		ordinalDataset(t, "trends", []string{"2025", "2025"}, []string{"premium", "claims"}, []float64{120, 80}),
	}}
	baseline := lens.DashboardSpec{Datasets: []lens.DatasetSpec{
		ordinalDataset(t, "trends", []string{"2025", "2025"}, []string{"claims", "premium"}, []float64{70, 100}),
	}}

	require.NoError(t, ApplyStatic(&current, &baseline, StaticOptions{}))
	primary := current.Datasets[0].Static.Primary()
	require.Equal(t, []any{100.0, 70.0}, primary.MustField(PreviousField("value")).Values)
}

func TestApplyStaticRejectsPositionalMultiRowJoin(t *testing.T) {
	t.Parallel()

	current := lens.DashboardSpec{Datasets: []lens.DatasetSpec{numericDataset(t, "ranked", []float64{120, 80})}}
	baseline := lens.DashboardSpec{Datasets: []lens.DatasetSpec{numericDataset(t, "ranked", []float64{100, 90})}}

	err := ApplyStatic(&current, &baseline, StaticOptions{})
	require.ErrorContains(t, err, "multi-row comparison has no stable identity field")
}

func TestApplyStaticOrdinalAlignsDifferentTimeCategories(t *testing.T) {
	t.Parallel()

	current := lens.DashboardSpec{Datasets: []lens.DatasetSpec{
		ordinalDataset(t, "trends", []string{"2025", "2026"}, []string{"premium", "premium"}, []float64{120, 150}),
	}}
	current.Datasets[0].ComparisonAlignment = lens.ComparisonAlignmentOrdinal
	baseline := lens.DashboardSpec{Datasets: []lens.DatasetSpec{
		ordinalDataset(t, "trends", []string{"2023", "2024"}, []string{"premium", "premium"}, []float64{100, 125}),
	}}

	require.NoError(t, ApplyStatic(&current, &baseline, StaticOptions{}))
	primary := current.Datasets[0].Static.Primary()
	require.Equal(t, []any{100.0, 125.0}, primary.MustField(PreviousField("value")).Values)
	require.Equal(t, []any{20.0, 25.0}, primary.MustField(DeltaField("value")).Values)
}

func TestApplyStaticOrdinalRejectsReorderedSeries(t *testing.T) {
	t.Parallel()

	current := lens.DashboardSpec{Datasets: []lens.DatasetSpec{
		ordinalDataset(t, "trends", []string{"2025", "2025"}, []string{"premium", "claims"}, []float64{120, 80}),
	}}
	current.Datasets[0].ComparisonAlignment = lens.ComparisonAlignmentOrdinal
	baseline := lens.DashboardSpec{Datasets: []lens.DatasetSpec{
		ordinalDataset(t, "trends", []string{"2024", "2024"}, []string{"claims", "premium"}, []float64{70, 100}),
	}}

	err := ApplyStatic(&current, &baseline, StaticOptions{})
	require.ErrorContains(t, err, `ordinal comparison field "series" mismatch at row 0`)
}

func TestApplyStaticOrdinalAlignsSharedPositionsWhenBaselineIsShorter(t *testing.T) {
	t.Parallel()

	current := lens.DashboardSpec{Datasets: []lens.DatasetSpec{
		ordinalDataset(t, "trends", []string{"2025", "2026"}, []string{"premium", "premium"}, []float64{120, 150}),
	}}
	current.Datasets[0].ComparisonAlignment = lens.ComparisonAlignmentOrdinal
	baseline := lens.DashboardSpec{Datasets: []lens.DatasetSpec{
		ordinalDataset(t, "trends", []string{"2024"}, []string{"premium"}, []float64{100}),
	}}

	require.NoError(t, ApplyStatic(&current, &baseline, StaticOptions{}))
	primary := current.Datasets[0].Static.Primary()
	require.Equal(t, []any{100.0, nil}, primary.MustField(PreviousField("value")).Values)
	require.Equal(t, []any{20.0, nil}, primary.MustField(DeltaField("value")).Values)
}

func TestApplyStaticOrdinalIgnoresBaselinePositionsWithoutCurrentRows(t *testing.T) {
	t.Parallel()

	current := lens.DashboardSpec{Datasets: []lens.DatasetSpec{
		ordinalDataset(t, "trends", []string{"2026"}, []string{"premium"}, []float64{120}),
	}}
	current.Datasets[0].ComparisonAlignment = lens.ComparisonAlignmentOrdinal
	baseline := lens.DashboardSpec{Datasets: []lens.DatasetSpec{
		ordinalDataset(t, "trends", []string{"2024", "2025"}, []string{"premium", "premium"}, []float64{100, 110}),
	}}

	require.NoError(t, ApplyStatic(&current, &baseline, StaticOptions{}))
	primary := current.Datasets[0].Static.Primary()
	require.Equal(t, []any{100.0}, primary.MustField(PreviousField("value")).Values)
	require.Equal(t, []any{20.0}, primary.MustField(DeltaField("value")).Values)
}

func TestApplyStaticOrdinalAllowsCategoryDerivedIDsAndLinks(t *testing.T) {
	t.Parallel()

	dataset := func(category, id, link string, value float64) lens.DatasetSpec {
		fr, err := frame.New("trends",
			frame.Field{Name: "id", Type: frame.FieldTypeString, Role: frame.RoleID, Values: []any{id}},
			frame.Field{Name: "category", Type: frame.FieldTypeString, Role: frame.RoleDimension, Values: []any{category}},
			frame.Field{Name: "series", Type: frame.FieldTypeString, Role: frame.RoleSeries, Values: []any{"premium"}},
			frame.Field{Name: "link", Type: frame.FieldTypeString, Role: frame.RoleLinkParam, Values: []any{link}},
			frame.Field{Name: "value", Type: frame.FieldTypeNumber, Role: frame.RoleMetric, Values: []any{value}},
		)
		require.NoError(t, err)
		set, err := frame.NewFrameSet(fr)
		require.NoError(t, err)
		return lens.DatasetSpec{
			Name: "trends", Kind: lens.DatasetKindStatic, Static: set,
			ComparisonAlignment: lens.ComparisonAlignmentOrdinal,
		}
	}

	current := lens.DashboardSpec{Datasets: []lens.DatasetSpec{
		dataset("2026-01", "2026-01|premium", "/drill?month=2026-01", 120),
	}}
	baseline := lens.DashboardSpec{Datasets: []lens.DatasetSpec{
		dataset("2025-01", "2025-01|premium", "/drill?month=2025-01", 100),
	}}

	require.NoError(t, ApplyStatic(&current, &baseline, StaticOptions{}))
	primary := current.Datasets[0].Static.Primary()
	require.Equal(t, []any{100.0}, primary.MustField(PreviousField("value")).Values)
	require.Equal(t, []any{20.0}, primary.MustField(DeltaField("value")).Values)
}

func TestApplyStaticOrdinalAllowsEmptyBaselineAsUnavailable(t *testing.T) {
	t.Parallel()

	current := lens.DashboardSpec{Datasets: []lens.DatasetSpec{
		ordinalDataset(t, "trends", []string{"2025", "2026"}, []string{"premium", "premium"}, []float64{120, 150}),
	}}
	current.Datasets[0].ComparisonAlignment = lens.ComparisonAlignmentOrdinal
	baseline := lens.DashboardSpec{Datasets: []lens.DatasetSpec{
		ordinalDataset(t, "trends", nil, nil, nil),
	}}

	require.NoError(t, ApplyStatic(&current, &baseline, StaticOptions{}))
	primary := current.Datasets[0].Static.Primary()
	require.Equal(t, []any{nil, nil}, primary.MustField(PreviousField("value")).Values)
	require.Equal(t, []any{nil, nil}, primary.MustField(DeltaField("value")).Values)
	require.Equal(t, []any{nil, nil}, primary.MustField(DeltaPercentField("value")).Values)
}

func TestApplyStaticOrdinalAllowsEmptyBaselineFrameSetAsUnavailable(t *testing.T) {
	t.Parallel()

	current := lens.DashboardSpec{Datasets: []lens.DatasetSpec{
		ordinalDataset(t, "trends", []string{"2025", "2026"}, []string{"premium", "premium"}, []float64{120, 150}),
	}}
	current.Datasets[0].ComparisonAlignment = lens.ComparisonAlignmentOrdinal
	empty, err := frame.NewFrameSet()
	require.NoError(t, err)
	baseline := lens.DashboardSpec{Datasets: []lens.DatasetSpec{{
		Name: "trends", Kind: lens.DatasetKindStatic, Static: empty,
	}}}

	require.NoError(t, ApplyStatic(&current, &baseline, StaticOptions{}))
	primary := current.Datasets[0].Static.Primary()
	require.Equal(t, []any{nil, nil}, primary.MustField(PreviousField("value")).Values)
}

// A producer that supplies the tri-state hook owns its silences: a field the
// hook does not name is neutral, rather than falling through to "higher is
// better" as the two-state InvertTrend forced it to.
func TestTrendPolarityHookOwnsItsSilences(t *testing.T) {
	options := StaticOptions{
		InvertTrend: func(string) bool { return true },
		TrendPolarity: func(fieldName string) panel.TrendPolarity {
			if fieldName == "earned_premium" {
				return panel.TrendHigherBetter
			}
			return panel.TrendNeutral
		},
	}
	require.Equal(t, panel.TrendHigherBetter, options.trendPolarity("earned_premium"))
	require.Equal(t, panel.TrendNeutral, options.trendPolarity("active_liability"),
		"the tri-state hook wins over InvertTrend even when it declines to judge")

	legacy := StaticOptions{InvertTrend: func(fieldName string) bool { return fieldName == "loss_ratio" }}
	require.Equal(t, panel.TrendLowerBetter, legacy.trendPolarity("loss_ratio"))
	require.Equal(t, panel.TrendNeutral, legacy.trendPolarity("earned_premium"))
}

func TestApplyStaticConfiguresPanelsWithSharedFieldContract(t *testing.T) {
	t.Parallel()

	current := lens.DashboardSpec{
		Datasets: []lens.DatasetSpec{staticDataset(t, "metrics", []string{"loss"}, []float64{-60})},
		Rows: []lens.RowSpec{{Panels: []panel.Spec{
			{ID: "stat", Kind: panel.KindStat, Dataset: "metrics", Fields: panel.FieldMapping{Value: panel.Ref("value")}},
			{ID: "table", Kind: panel.KindTable, Dataset: "metrics", Columns: []panel.TableColumn{
				{Field: panel.Ref("key"), Label: "Metric"}, {Field: panel.Ref("value"), Label: "Value"},
			}},
		}}},
	}
	baseline := lens.DashboardSpec{Datasets: []lens.DatasetSpec{staticDataset(t, "metrics", []string{"loss"}, []float64{-100})}}

	require.NoError(t, ApplyStatic(&current, &baseline, StaticOptions{
		Labels:      Labels{Before: "Before", After: "After", Delta: "Delta", Trend: "Comparison"},
		InvertTrend: func(fieldName string) bool { return fieldName == "value" },
		AbsoluteDeltaUnit: func(fieldName string) panel.TrendDeltaUnit {
			return panel.TrendDeltaPercentagePoints
		},
	}))
	stat := current.Rows[0].Panels[0]
	require.Equal(t, DeltaField("value"), stat.Trend.AbsoluteField.Name())
	require.Equal(t, DeltaPercentField("value"), stat.Trend.PercentField.Name())
	require.Equal(t, panel.TrendLowerBetter, stat.Trend.Polarity,
		"a producer that only declares the legacy InvertTrend hook keeps the colours it had")
	require.Equal(t, panel.TrendDeltaPercentagePoints, stat.Trend.AbsoluteDeltaUnit)
	table := current.Rows[0].Panels[1]
	require.Equal(t, PreviousField("value"), table.Columns[1].Field.Name())
	require.Equal(t, DeltaField("value"), table.Columns[3].Field.Name())
	require.Equal(t, DeltaPercentField("value"), table.Columns[3].Cell.PercentField.Name())
}

func TestConfigureProgressivePresentationDeclaresDeferredComparisonContract(t *testing.T) {
	dashboard := lens.DashboardSpec{Rows: []lens.RowSpec{{Panels: []panel.Spec{
		{ID: "chart", Kind: panel.KindTimeSeries, Fields: panel.FieldMapping{Value: panel.Ref("value")}},
		{ID: "stat", Kind: panel.KindStat, Fields: panel.FieldMapping{Value: panel.Ref("ratio")}},
		{ID: "map", Kind: panel.KindMap, Fields: panel.FieldMapping{Value: panel.Ref("value")}},
		{ID: "table", Kind: panel.KindTable, Columns: []panel.TableColumn{
			{Field: panel.Ref("product"), Label: "Product"},
			{Field: panel.Ref("margin"), Label: "Margin", Formatter: formatPtr(format.Money("UZS", 0))},
		}},
	}}}}

	ConfigureProgressivePresentation(&dashboard, true, StaticOptions{
		Labels: Labels{Trend: "vs baseline"},
		AbsoluteDeltaUnit: func(field string) panel.TrendDeltaUnit {
			if field == "ratio" {
				return panel.TrendDeltaPercentagePoints
			}
			return panel.TrendDeltaValue
		},
	})

	require.Equal(t, panel.Ref("previous_value"), dashboard.Rows[0].Panels[0].Fields.Previous)
	require.Equal(t, panel.Ref("ratio_delta"), dashboard.Rows[0].Panels[1].Trend.AbsoluteField)
	require.Equal(t, panel.TrendDeltaPercentagePoints, dashboard.Rows[0].Panels[1].Trend.AbsoluteDeltaUnit)
	require.True(t, dashboard.Rows[0].Panels[2].ComparisonUnsupported)
	require.Equal(t, []string{"product", "previous_margin", "margin", "margin_delta"}, tableFieldNames(dashboard.Rows[0].Panels[3].Columns))
	require.Equal(t, panel.Ref("margin_delta_percent"), dashboard.Rows[0].Panels[3].Columns[3].Cell.PercentField)

	ConfigureProgressivePresentation(&dashboard, false, StaticOptions{})
	require.Empty(t, dashboard.Rows[0].Panels[0].Fields.Previous)
	require.Nil(t, dashboard.Rows[0].Panels[1].Trend)
	require.False(t, dashboard.Rows[0].Panels[2].ComparisonUnsupported)
}

func formatPtr(value format.Spec) *format.Spec { return &value }

func tableFieldNames(columns []panel.TableColumn) []string {
	fields := make([]string, len(columns))
	for index, column := range columns {
		fields[index] = column.Field.Name()
	}
	return fields
}

func staticDataset(t *testing.T, name string, keys []string, values []float64) lens.DatasetSpec {
	t.Helper()
	keyValues := make([]any, len(keys))
	metricValues := make([]any, len(values))
	for index := range keys {
		keyValues[index] = keys[index]
	}
	for index := range values {
		metricValues[index] = values[index]
	}
	fr, err := frame.New(name,
		frame.Field{Name: "key", Type: frame.FieldTypeString, Role: frame.RoleID, Values: keyValues},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Role: frame.RoleMetric, Values: metricValues},
	)
	require.NoError(t, err)
	set, err := frame.NewFrameSet(fr)
	require.NoError(t, err)
	return lens.DatasetSpec{Name: name, Kind: lens.DatasetKindStatic, Static: set}
}

func numericDataset(t *testing.T, name string, values []float64) lens.DatasetSpec {
	t.Helper()
	items := make([]any, len(values))
	for index := range values {
		items[index] = values[index]
	}
	fr, err := frame.New(name, frame.Field{Name: "value", Type: frame.FieldTypeNumber, Role: frame.RoleMetric, Values: items})
	require.NoError(t, err)
	set, err := frame.NewFrameSet(fr)
	require.NoError(t, err)
	return lens.DatasetSpec{Name: name, Kind: lens.DatasetKindStatic, Static: set}
}

func ordinalDataset(t *testing.T, name string, categories, series []string, values []float64) lens.DatasetSpec {
	t.Helper()
	categoryValues := make([]any, len(categories))
	seriesValues := make([]any, len(series))
	metricValues := make([]any, len(values))
	for index := range categories {
		categoryValues[index] = categories[index]
	}
	for index := range series {
		seriesValues[index] = series[index]
	}
	for index := range values {
		metricValues[index] = values[index]
	}
	fr, err := frame.New(name,
		frame.Field{Name: "category", Type: frame.FieldTypeString, Role: frame.RoleDimension, Values: categoryValues},
		frame.Field{Name: "series", Type: frame.FieldTypeString, Role: frame.RoleSeries, Values: seriesValues},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Role: frame.RoleMetric, Values: metricValues},
	)
	require.NoError(t, err)
	set, err := frame.NewFrameSet(fr)
	require.NoError(t, err)
	return lens.DatasetSpec{Name: name, Kind: lens.DatasetKindStatic, Static: set}
}
