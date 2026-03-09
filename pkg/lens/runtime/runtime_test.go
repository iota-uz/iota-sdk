package runtime

import (
	"context"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/datasource"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
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
	spec := lens.Dashboard("shared", "Shared Dataset",
		lens.Row(
			panel.Bar("p1", "Panel 1", "shared-data").Build(),
			panel.Table("p2", "Panel 2", "shared-data").Build(),
		),
	).WithDatasets(
		lens.QueryDataset("shared-data", "primary", "select 1"),
	)

	result, err := Execute(context.Background(), spec, Runtime{
		DataSources: map[string]datasource.DataSource{"primary": ds},
	})
	require.NoError(t, err)
	require.Len(t, result.Panels, 2)
	require.Equal(t, int32(1), ds.calls.Load())
}

func TestValidateRejectsDatasetCycles(t *testing.T) {
	t.Parallel()

	spec := lens.Dashboard("cycle", "Cycle").WithDatasets(
		lens.DatasetSpec{Name: "a", Kind: lens.DatasetKindTransform, DependsOn: []string{"b"}},
		lens.DatasetSpec{Name: "b", Kind: lens.DatasetKindTransform, DependsOn: []string{"a"}},
	)

	err := Validate(spec)
	require.Error(t, err)
}

func TestDateRangeVariableSupportsAllTimeAndDefaults(t *testing.T) {
	t.Parallel()

	spec := lens.Dashboard("variables", "Variables").WithVariables(
		lens.DateRangeVariable("range", "Range", 24*time.Hour),
	)

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

func TestValidateRejectsDuplicateDatasetsAndPanels(t *testing.T) {
	t.Parallel()

	spec := lens.Dashboard("duplicates", "Duplicates",
		lens.Row(
			panel.Bar("same", "Panel 1", "dataset-a").
				LabelField("label").
				ValueField("value").
				Build(),
			panel.Bar("same", "Panel 2", "dataset-a").
				LabelField("label").
				ValueField("value").
				Build(),
		),
	).WithDatasets(
		lens.StaticDataset("dataset-a", mustFrameSet(t, "dataset-a")),
		lens.StaticDataset("dataset-a", mustFrameSet(t, "dataset-b")),
	)

	err := Validate(spec)
	require.Error(t, err)
}

func TestValidateRejectsMissingActionFieldSource(t *testing.T) {
	t.Parallel()

	spec := lens.Dashboard("actions", "Actions",
		lens.Row(
			panel.Bar("sales", "Sales", "dataset").
				LabelField("label").
				ValueField("value").
				Action(action.Navigate("/contracts", action.FieldParam("source", ""))).
				Build(),
		),
	).WithDatasets(
		lens.StaticDataset("dataset", mustFrameSet(t, "dataset")),
	)

	err := Validate(spec)
	require.Error(t, err)
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
