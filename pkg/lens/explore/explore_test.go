package explore

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/stretchr/testify/require"
)

func TestSpecValidate_AcceptsMultiPerspectiveGraph(t *testing.T) {
	t.Parallel()

	spec := testSpec()
	require.NoError(t, spec.Validate())
}

func TestSpecValidate_RejectsInvalidGraphs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*Spec)
		wantErr string
	}{
		{
			name: "missing default perspective",
			mutate: func(spec *Spec) {
				spec.Branches[0].DefaultPerspective = "missing"
			},
			wantErr: "missing default perspective",
		},
		{
			name: "edge panel missing stable id field",
			mutate: func(spec *Spec) {
				spec.Branches[0].Perspectives[0].Nodes[0].Panel.Fields.ID = ""
			},
			wantErr: "requires panel id field",
		},
		{
			name: "node cycle",
			mutate: func(spec *Spec) {
				spec.Branches[0].Perspectives[0].Nodes[1].Edges = []Edge{ToNode("return", "root")}
			},
			wantErr: "node cycle",
		},
		{
			name: "unbalanced check",
			mutate: func(spec *Spec) {
				spec.Branches[0].Perspectives[0].Nodes[0].Check = &BalanceCheck{Expected: 100, Actual: 90, Tolerance: 1}
			},
			wantErr: "out of balance",
		},
		{
			name: "edge with target and action",
			mutate: func(spec *Spec) {
				spec.Branches[0].Perspectives[0].Nodes[0].Edges[0].Action = actionSpec(action.Navigate("/items"))
			},
			wantErr: "exactly one",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			spec := testSpec()
			test.mutate(&spec)
			err := spec.Validate()
			require.Error(t, err)
			require.ErrorContains(t, err, test.wantErr)
		})
	}
}

func TestReduce_PreservesIndependentPerspectivePathsAndRejectsStaleRequests(t *testing.T) {
	t.Parallel()

	spec := testSpec()
	state, err := Reduce(spec, InitialState(spec), Event{Kind: EventOpen, BranchKey: "focus"})
	require.NoError(t, err)
	require.Equal(t, "composition", state.PerspectiveKey)

	state, err = Reduce(spec, state, Event{Kind: EventDrill, NodeKey: "detail"})
	require.NoError(t, err)
	require.Equal(t, []string{"root", "detail"}, state.PathByPerspective["focus/composition"])

	state, err = Reduce(spec, state, Event{Kind: EventSelectPerspective, PerspectiveKey: "trend"})
	require.NoError(t, err)
	require.Equal(t, []string{"trend-root"}, state.PathByPerspective["focus/trend"])

	state, err = Reduce(spec, state, Event{Kind: EventSelectPerspective, PerspectiveKey: "composition"})
	require.NoError(t, err)
	require.Equal(t, []string{"root", "detail"}, state.PathByPerspective["focus/composition"])

	state, err = Reduce(spec, state, Event{Kind: EventRequestStarted, RequestVersion: 2})
	require.NoError(t, err)
	state, err = Reduce(spec, state, Event{Kind: EventRequestFailed, RequestVersion: 1, Error: "stale"})
	require.NoError(t, err)
	require.Equal(t, StatusLoading, state.Status)
	require.Empty(t, state.Error)
}

func TestReduce_BackReturnsToRootChartAtPerspectiveRoot(t *testing.T) {
	t.Parallel()

	spec := testSpec()
	state, err := Reduce(spec, InitialState(spec), Event{Kind: EventOpen, BranchKey: "focus"})
	require.NoError(t, err)
	state, err = Reduce(spec, state, Event{Kind: EventBack})
	require.NoError(t, err)
	require.Equal(t, StatusIdle, state.Status)
	require.Empty(t, state.BranchKey)
}

func TestReduce_DynamicDrillPreservesSelectedPoint(t *testing.T) {
	t.Parallel()

	spec := testSpec()
	root := &spec.Branches[0].Perspectives[0].Nodes[0]
	root.Edges = nil
	root.DynamicEdges = true
	root.DynamicTargets = []string{"detail"}
	state, err := Reduce(spec, InitialState(spec), Event{Kind: EventOpen, BranchKey: "focus"})
	require.NoError(t, err)
	state, err = Reduce(spec, state, Event{Kind: EventDrill, NodeKey: "detail", PointKey: "other"})
	require.NoError(t, err)

	selection, err := ActiveSelection(spec, state)
	require.NoError(t, err)
	require.Equal(t, []PathStep{{NodeKey: "root"}, {NodeKey: "detail", PointKey: "other"}}, selection.Steps)
	require.Equal(t, []string{"root", "detail"}, selection.Path)

	request, err := ExportRequestFromState(spec, state, ExportCurrentView)
	require.NoError(t, err)
	require.Equal(t, selection.Steps, request.Steps)
}

func TestResolveExportRequest_ResolvesStableLabelsAndPath(t *testing.T) {
	t.Parallel()

	resolved, err := ResolveExportRequest(testSpec(), ExportRequest{
		Mode:           ExportCurrentView,
		ExplorerID:     "metric-explorer",
		BranchKey:      "focus",
		PerspectiveKey: "composition",
		Path:           []string{"root", "detail"},
		NodeKey:        "detail",
	})
	require.NoError(t, err)
	require.Equal(t, "Focus", resolved.Labels.Branch)
	require.Equal(t, "Composition", resolved.Labels.Perspective)
	require.Equal(t, "Detail", resolved.Labels.Node)
}

func TestResolveExportRequest_RepresentsFullExplorationWithoutViewPath(t *testing.T) {
	t.Parallel()

	resolved, err := ResolveExportRequest(testSpec(), ExportRequest{
		Mode:       ExportFull,
		ExplorerID: "metric-explorer",
		BranchKey:  "focus",
	})
	require.NoError(t, err)
	require.Equal(t, "Focus", resolved.Labels.Branch)
	require.Empty(t, resolved.PerspectiveKey)
}

func TestExportRequestFromState_CapturesCurrentNode(t *testing.T) {
	t.Parallel()

	spec := testSpec()
	state, err := Reduce(spec, InitialState(spec), Event{Kind: EventOpen, BranchKey: "focus"})
	require.NoError(t, err)
	state, err = Reduce(spec, state, Event{Kind: EventDrill, NodeKey: "detail"})
	require.NoError(t, err)

	request, err := ExportRequestFromState(spec, state, ExportCurrentView)
	require.NoError(t, err)
	require.Equal(t, []string{"root", "detail"}, request.Path)
	require.Equal(t, []PathStep{{NodeKey: "root"}, {NodeKey: "detail", PointKey: "detail-point"}}, request.Steps)
	require.Equal(t, "detail", request.NodeKey)
	require.Equal(t, "Detail", request.Labels.Node)
}

func testSpec() Spec {
	rootPanel := panel.Pie("root-panel", "Root", "root-data").IDField("id").Build()
	detailPanel := panel.Bar("detail-panel", "Detail", "detail-data").Build()
	trendPanel := panel.TimeSeries("trend-panel", "Trend", "trend-data").Build()
	return Spec{
		ID:           "metric-explorer",
		HostPanelID:  "host-panel",
		ExpandedSpan: 12,
		Branches: []Branch{
			NewBranch("focus", "Focus", "composition",
				NewPerspective("composition", "Composition", SemanticsPartition, "root",
					PanelNode("root", "Root", rootPanel, ToNode("detail-point", "detail")).WithBalance(100, 100, 0),
					PanelNode("detail", "Detail", detailPanel, ToAction("leaf", action.Navigate("/items"))),
				),
				NewPerspective("trend", "Trend", SemanticsSeries, "trend-root",
					PanelNode("trend-root", "Trend", trendPanel),
				),
			),
		},
	}
}

func actionSpec(spec action.Spec) *action.Spec { return &spec }
