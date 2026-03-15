package runtime

import (
	"context"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	lensbuild "github.com/iota-uz/iota-sdk/pkg/lens/build"
	"github.com/iota-uz/iota-sdk/pkg/lens/datasource"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubDataSource struct {
	calls atomic.Int32
}

func (s *stubDataSource) Run(_ context.Context, req datasource.QueryRequest) (*frame.FrameSet, error) {
	s.calls.Add(1)
	fr, err := frame.New(req.Source,
		frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{"a", "b"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{1.0, 2.0}},
	)
	if err != nil {
		return nil, err
	}
	return frame.NewFrameSet(fr)
}

func (s *stubDataSource) Capabilities() datasource.CapabilitySet {
	return datasource.CapabilitySet{datasource.CapabilityParameterizedQueries: true}
}

func TestExecuteReusesDatasetAcrossPanels(t *testing.T) {
	t.Parallel()

	ds := &stubDataSource{}
	spec := lensbuild.Dashboard("shared", "Shared Dataset",
		lensbuild.Row(
			panel.Bar("p1", "Panel 1", "shared-data").Build(),
			panel.Table("p2", "Panel 2", "shared-data").Build(),
		),
	).Datasets(
		lensbuild.QueryDataset("shared-data", "primary", "select 1"),
	).Build()

	result, err := Execute(context.Background(), spec, Runtime{
		DataSources: map[string]datasource.DataSource{"primary": ds},
	})
	require.NoError(t, err)
	require.Len(t, result.Panels, 2)
	require.Equal(t, int32(1), ds.calls.Load())
}

func TestBuildPlan_Scenarios(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		spec   lens.DashboardSpec
		assert func(t *testing.T, plan ExecutionPlan)
	}{
		{
			name: "includes only required datasets",
			spec: lensbuild.Dashboard("planned", "Planned",
				lensbuild.Row(
					panel.Bar("sales", "Sales", "daily_sales").Build(),
				),
			).Datasets(
				lensbuild.StaticDataset("source_lookup", mustFrameSet(t, "source_lookup")),
				lens.DatasetSpec{
					Name:       "daily_sales",
					Kind:       lens.DatasetKindTransform,
					DependsOn:  []string{"raw_sales", "source_lookup"},
					Transforms: nil,
				},
				lensbuild.QueryDataset("raw_sales", "primary", "select 1"),
				lensbuild.StaticDataset("unused_dataset", mustFrameSet(t, "unused_dataset")),
			).Build(),
			assert: func(t *testing.T, plan ExecutionPlan) {
				t.Helper()
				assert.Len(t, plan.DatasetStages, 2)
				assert.ElementsMatch(t, []string{"raw_sales", "source_lookup"}, plan.DatasetStages[0].Datasets)
				assert.Equal(t, []string{"daily_sales"}, plan.DatasetStages[1].Datasets)
				assert.NotContains(t, plan.DatasetStages[0].Datasets, "unused_dataset")
				assert.Equal(t, []string{"sales"}, plan.Panels)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := BuildPlan(tt.spec)
			require.NoError(t, err)
			tt.assert(t, plan)
		})
	}
}

func TestExecute_Scenarios(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		spec   lens.DashboardSpec
		assert func(t *testing.T, result *DashboardResult, ds *stubDataSource)
	}{
		{
			name: "skips unused datasets",
			spec: lensbuild.Dashboard("planned", "Planned",
				lensbuild.Row(
					panel.Bar("sales", "Sales", "shared-data").Build(),
				),
			).Datasets(
				lensbuild.QueryDataset("shared-data", "primary", "select 1"),
				lensbuild.QueryDataset("unused-data", "primary", "select 2"),
			).Build(),
			assert: func(t *testing.T, result *DashboardResult, ds *stubDataSource) {
				t.Helper()
				assert.Equal(t, int32(1), ds.calls.Load())
				assert.Contains(t, result.Datasets, "shared-data")
				assert.NotContains(t, result.Datasets, "unused-data")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ds := &stubDataSource{}
			result, err := Execute(context.Background(), tt.spec, Runtime{
				DataSources: map[string]datasource.DataSource{"primary": ds},
			})
			require.NoError(t, err)
			require.NotNil(t, result)
			tt.assert(t, result, ds)
		})
	}
}

func TestValidateRejectsDatasetCycles(t *testing.T) {
	t.Parallel()

	spec := lensbuild.Dashboard("cycle", "Cycle").Datasets(
		lens.DatasetSpec{Name: "a", Kind: lens.DatasetKindTransform, DependsOn: []string{"b"}},
		lens.DatasetSpec{Name: "b", Kind: lens.DatasetKindTransform, DependsOn: []string{"a"}},
	).Build()

	err := Validate(spec)
	require.Error(t, err)
}

func TestDateRangeVariableSupportsAllTimeAndDefaults(t *testing.T) {
	t.Parallel()

	spec := lensbuild.Dashboard("variables", "Variables").Variables(
		lensbuild.DateRangeVariable("range", "Range", 24*time.Hour),
	).Build()

	defaults, err := resolveVariables(spec.Variables, Runtime{Request: url.Values{}})
	require.NoError(t, err)
	defaultRange := defaults["range"].(lens.DateRangeValue)
	require.Equal(t, "default", defaultRange.Mode)
	require.NotNil(t, defaultRange.Start)
	require.NotNil(t, defaultRange.End)

	allTime, err := resolveVariables(spec.Variables, Runtime{Request: url.Values{"range": []string{"all"}}})
	require.NoError(t, err)
	allRange := allTime["range"].(lens.DateRangeValue)
	require.Equal(t, "all", allRange.Mode)
}

func TestDateRangeVariableUsesStartAndEndRequestKeysWhenModeKeyIsPresent(t *testing.T) {
	t.Parallel()

	spec := lensbuild.Dashboard("variables", "Variables").Variables(
		lensbuild.DateRangeVariable("range", "Range", 24*time.Hour),
	).Build()

	values := url.Values{
		"range":       []string{"bounded"},
		"range_start": []string{"2026-03-01"},
		"range_end":   []string{"2026-03-15"},
	}

	resolved, err := resolveVariables(spec.Variables, Runtime{Request: values})
	require.NoError(t, err)

	bounded := resolved["range"].(lens.DateRangeValue)
	require.Equal(t, "bounded", bounded.Mode)
	require.NotNil(t, bounded.Start)
	require.NotNil(t, bounded.End)
	assert.Equal(t, time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), bounded.Start.UTC())
	assert.Equal(t, time.Date(2026, 3, 15, 23, 59, 59, int(time.Second-time.Nanosecond), time.UTC), bounded.End.UTC())
}

func TestValidateRejectsDuplicatePanels(t *testing.T) {
	t.Parallel()

	spec := lensbuild.Dashboard("duplicates", "Duplicates",
		lensbuild.Row(
			panel.Bar("same", "Panel 1", "dataset-a").
				LabelField("label").
				ValueField("value").
				Build(),
			panel.Bar("same", "Panel 2", "dataset-b").
				LabelField("label").
				ValueField("value").
				Build(),
		),
	).Datasets(
		lensbuild.StaticDataset("dataset-a", mustFrameSet(t, "dataset-a")),
		lensbuild.StaticDataset("dataset-b", mustFrameSet(t, "dataset-b")),
	).Build()

	err := Validate(spec)
	require.Error(t, err)
}

func TestValidateRejectsDuplicateDatasets(t *testing.T) {
	t.Parallel()

	spec := lensbuild.Dashboard("duplicate-datasets", "Duplicate Datasets",
		lensbuild.Row(
			panel.Bar("panel-a", "Panel 1", "dataset-a").
				LabelField("label").
				ValueField("value").
				Build(),
		),
	).Datasets(
		lensbuild.StaticDataset("dataset-a", mustFrameSet(t, "dataset-a")),
		lensbuild.StaticDataset("dataset-a", mustFrameSet(t, "dataset-b")),
	).Build()

	err := Validate(spec)
	require.Error(t, err)
}

func TestValidateRejectsMissingStaticFramesAndQuerySpec(t *testing.T) {
	t.Parallel()

	staticErr := Validate(lensbuild.Dashboard("static", "Static").Datasets(
		lens.DatasetSpec{Name: "missing-static", Kind: lens.DatasetKindStatic},
	).Build())
	require.Error(t, staticErr)

	queryErr := Validate(lensbuild.Dashboard("query", "Query").Datasets(
		lens.DatasetSpec{Name: "missing-query", Kind: lens.DatasetKindQuery, Source: "primary"},
	).Build())
	require.Error(t, queryErr)
}

func TestValidateRejectsMissingActionFieldSource(t *testing.T) {
	t.Parallel()

	spec := lensbuild.Dashboard("actions", "Actions",
		lensbuild.Row(
			panel.Bar("sales", "Sales", "dataset").
				LabelField("label").
				ValueField("value").
				Action(action.Navigate("/contracts", action.FieldParam("source", ""))).
				Build(),
		),
	).Datasets(
		lensbuild.StaticDataset("dataset", mustFrameSet(t, "dataset")),
	).Build()

	err := Validate(spec)
	require.Error(t, err)
}

func TestExecuteMarksMissingPanelFieldsAsPanelError(t *testing.T) {
	t.Parallel()

	spec := lensbuild.Dashboard("frames", "Frames",
		lensbuild.Row(
			panel.Bar("sales", "Sales", "dataset").
				LabelField("missing_label").
				ValueField("value").
				Build(),
		),
	).Datasets(
		lensbuild.StaticDataset("dataset", mustFrameSet(t, "dataset")),
	).Build()

	result, err := Execute(context.Background(), spec, Runtime{})
	require.NoError(t, err)
	require.Error(t, result.Panels["sales"].Error)
	require.Contains(t, result.Panels["sales"].Error.Error(), "missing field")
}

func TestResolveVariablesPreservesAllMultiSelectValues(t *testing.T) {
	t.Parallel()

	spec := lensbuild.Dashboard("variables", "Variables").Variables(
		lens.VariableSpec{
			Name:    "products",
			Label:   "Products",
			Kind:    lens.VariableMultiSelect,
			Default: []string{"default"},
		},
	).Build()

	values, err := resolveVariables(spec.Variables, Runtime{
		Request: url.Values{"products": []string{"osago", "travel"}},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"osago", "travel"}, values["products"])
}

func TestResolveVariablesSplitsCommaSeparatedMultiSelectValues(t *testing.T) {
	t.Parallel()

	spec := lensbuild.Dashboard("variables", "Variables").Variables(
		lens.VariableSpec{
			Name:    "products",
			Label:   "Products",
			Kind:    lens.VariableMultiSelect,
			Default: []string{"default"},
		},
	).Build()

	values, err := resolveVariables(spec.Variables, Runtime{
		Request: url.Values{"products": []string{"osago, travel", "kasko"}},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"osago", "travel", "kasko"}, values["products"])
}

func TestResolveVariablesParsesNumberValues(t *testing.T) {
	t.Parallel()

	spec := lensbuild.Dashboard("variables", "Variables").Variables(
		lens.VariableSpec{Name: "limit", Label: "Limit", Kind: lens.VariableNumber, Default: 10.0},
	).Build()

	values, err := resolveVariables(spec.Variables, Runtime{
		Request: url.Values{"limit": []string{"25.5"}},
	})
	require.NoError(t, err)
	require.InDelta(t, 25.5, values["limit"].(float64), 0.001)
}

func TestResolveVariablesUsesToggleDefaultWhenRequestMissing(t *testing.T) {
	t.Parallel()

	spec := lensbuild.Dashboard("variables", "Variables").Variables(
		lens.VariableSpec{Name: "active_only", Label: "Active Only", Kind: lens.VariableToggle, Default: true},
	).Build()

	values, err := resolveVariables(spec.Variables, Runtime{Request: url.Values{}})
	require.NoError(t, err)
	require.Equal(t, true, values["active_only"])
}

func TestValidateAllowsUngroupedTimeSeriesPanels(t *testing.T) {
	t.Parallel()

	spec := lensbuild.Dashboard("sales", "Sales").Datasets(
		lensbuild.StaticDataset("daily_sales", mustFrameSet(t, "daily_sales")),
	).Build()
	spec.Rows = []lens.RowSpec{
		lensbuild.Row(
			panel.TimeSeries("daily", "Daily Sales", "daily_sales").
				CategoryField("category").
				ValueField("value").
				Build(),
		),
	}

	require.NoError(t, Validate(spec))
}

func mustFrameSet(t *testing.T, name string) *frame.FrameSet {
	t.Helper()

	set, err := frame.FromRows(name, frame.Row{
		"label": "row",
		"value": 1.0,
	})
	require.NoError(t, err)
	return set
}
