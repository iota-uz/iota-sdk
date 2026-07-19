package runtime

import (
	"context"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	lensbuild "github.com/iota-uz/iota-sdk/pkg/lens/build"
	"github.com/iota-uz/iota-sdk/pkg/lens/explore"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/stretchr/testify/require"
)

type explorationLoaderStub struct {
	definition ExplorationDefinition
	request    ExplorationLoadRequest
}

func (s *explorationLoaderStub) LoadExploration(_ context.Context, req ExplorationLoadRequest) (ExplorationDefinition, error) {
	s.request = req
	return s.definition, nil
}

func TestExecuteExploration_LoadsOnlyRequestedPanel(t *testing.T) {
	t.Parallel()

	frames := testFrames(t, "detail-data")
	detailDashboard := lensbuild.Dashboard("detail", "Detail",
		lensbuild.Row(
			panel.Pie("requested", "Requested", "detail-data").Build(),
			panel.Pie("not-requested", "Not requested", "detail-data").Build(),
		),
	).Datasets(lensbuild.StaticDataset("detail-data", frames)).Build()
	loader := &explorationLoaderStub{definition: ExplorationDefinition{Dashboard: detailDashboard, PanelID: "requested"}}
	dashboard := explorerDashboard(t)

	result, err := New(Options{}).ExecuteExploration(context.Background(), dashboard, loader, ExplorationLoadRequest{
		ExplorerID:     "metric",
		BranchKey:      "focus",
		PerspectiveKey: "composition",
		Path:           []string{"root"},
		Variables:      map[string]any{"period": "2026"},
	}, Request{DataScope: "tenant:one"})
	require.NoError(t, err)
	require.Equal(t, "requested", result.Panel.Panel.ID)
	require.Equal(t, "2026", loader.request.Variables["period"])
}

func TestExplorationFragmentHandler_ReturnsScopedPanelResult(t *testing.T) {
	t.Parallel()

	frames := testFrames(t, "detail-data")
	detailDashboard := lensbuild.Dashboard("detail", "Detail",
		lensbuild.Row(panel.Pie("requested", "Requested", "detail-data").Build()),
	).Datasets(lensbuild.StaticDataset("detail-data", frames)).Build()
	handler := ExplorationFragmentHandler{
		Runtime: New(Options{}),
		Loader:  &explorationLoaderStub{definition: ExplorationDefinition{Dashboard: detailDashboard, PanelID: "requested"}},
	}

	response, err := handler.Handle(context.Background(), ExplorationFragmentRequest{
		Dashboard: explorerDashboard(t),
		Load:      ExplorationLoadRequest{ExplorerID: "metric", BranchKey: "focus"},
		Runtime:   Request{DataScope: "tenant:one"},
	})
	require.NoError(t, err)
	require.Same(t, response.Result.Panel, response.Panel)
	require.Equal(t, "requested", response.Panel.Panel.ID)
	require.Equal(t, []string{"root"}, response.Result.Path)
}

func TestValidate_RejectsExplorerWithMissingHostPanel(t *testing.T) {
	t.Parallel()

	dashboard := explorerDashboard(t)
	dashboard.Explorers[0].HostPanelID = "missing"
	err := Validate(dashboard)
	require.Error(t, err)
	require.ErrorContains(t, err, "missing host panel")
}

func TestValidate_AcceptsExploreActionWithoutURL(t *testing.T) {
	t.Parallel()

	dashboard := explorerDashboard(t)
	dashboard.Rows[0].Panels[0].Action = actionSpecRuntime(action.Explore("metric", "focus"))
	require.NoError(t, Validate(dashboard))
}

func TestExecuteExploration_RejectsEdgeBearingLazyPanelWithoutStablePointID(t *testing.T) {
	t.Parallel()

	frames := testFrames(t, "detail-data")
	loadedPanel := panel.Pie("requested", "Requested", "detail-data").Build()
	loadedPanel.Fields.ID = ""
	detailDashboard := lensbuild.Dashboard("detail", "Detail", lensbuild.Row(loadedPanel)).
		Datasets(lensbuild.StaticDataset("detail-data", frames)).Build()
	loader := &explorationLoaderStub{definition: ExplorationDefinition{Dashboard: detailDashboard, PanelID: "requested"}}

	_, err := New(Options{}).ExecuteExploration(context.Background(), explorerDashboard(t), loader, ExplorationLoadRequest{
		ExplorerID: "metric",
		BranchKey:  "focus",
		Path:       []string{"root"},
	}, Request{DataScope: "test"})
	require.Error(t, err)
	require.ErrorContains(t, err, "requires loaded panel")
}

func TestExecuteExploration_RejectsResolvedEdgeForUnknownPoint(t *testing.T) {
	t.Parallel()

	loader := &explorationLoaderStub{definition: resolvedExplorationDefinition(t,
		explore.ToNode("missing", "detail"),
	)}
	_, err := New(Options{}).ExecuteExploration(context.Background(), dynamicExplorerDashboard(t), loader, ExplorationLoadRequest{
		ExplorerID: "metric", BranchKey: "focus",
	}, Request{DataScope: "test"})

	require.Error(t, err)
	require.ErrorContains(t, err, "missing from loaded panel")
}

func TestExecuteExploration_RejectsDuplicateResolvedPoint(t *testing.T) {
	t.Parallel()

	loader := &explorationLoaderStub{definition: resolvedExplorationDefinition(t,
		explore.ToNode("a", "detail"),
		explore.ToAction("a", action.Navigate("/portfolio")),
	)}
	_, err := New(Options{}).ExecuteExploration(context.Background(), dynamicExplorerDashboard(t), loader, ExplorationLoadRequest{
		ExplorerID: "metric", BranchKey: "focus",
	}, Request{DataScope: "test"})

	require.Error(t, err)
	require.ErrorContains(t, err, "duplicate resolved edge point")
}

func TestExecuteExploration_DynamicChildPathPreservesSelectedPoint(t *testing.T) {
	t.Parallel()

	rootLoader := &explorationLoaderStub{definition: resolvedExplorationDefinition(t, explore.ToNode("a", "detail"))}
	root, err := New(Options{}).ExecuteExploration(context.Background(), dynamicExplorerDashboard(t), rootLoader, ExplorationLoadRequest{
		ExplorerID: "metric", BranchKey: "focus",
	}, Request{DataScope: "test"})
	require.NoError(t, err)
	require.Equal(t, []explore.Edge{explore.ToNode("a", "detail")}, root.Edges)

	detailDefinition := resolvedExplorationDefinition(t)
	detailLoader := &explorationLoaderStub{definition: detailDefinition}
	steps := []explore.PathStep{{NodeKey: "root"}, {NodeKey: "detail", PointKey: "a"}}
	detail, err := New(Options{}).ExecuteExploration(context.Background(), dynamicExplorerDashboard(t), detailLoader, ExplorationLoadRequest{
		ExplorerID: "metric", BranchKey: "focus", Steps: steps,
	}, Request{DataScope: "test"})

	require.NoError(t, err)
	require.Equal(t, []string{"root", "detail"}, detail.Path)
	require.Equal(t, steps, detail.Steps)
	require.Equal(t, steps, detailLoader.request.Steps)
	require.Equal(t, []string{"root", "detail"}, detailLoader.request.Path)
}

func TestExecuteExploration_ReturnsResolvedTerminalAction(t *testing.T) {
	t.Parallel()

	edge := explore.ToAction("a", action.Navigate("/portfolio"))
	loader := &explorationLoaderStub{definition: resolvedExplorationDefinition(t, edge)}
	result, err := New(Options{}).ExecuteExploration(context.Background(), dynamicExplorerDashboard(t), loader, ExplorationLoadRequest{
		ExplorerID: "metric", BranchKey: "focus",
	}, Request{DataScope: "test"})

	require.NoError(t, err)
	require.Equal(t, []explore.Edge{edge}, result.Edges)
	require.Equal(t, action.KindNavigate, result.Edges[0].Action.Kind)
}

func TestValidate_RejectsExploreActionWithMissingBranch(t *testing.T) {
	t.Parallel()

	dashboard := explorerDashboard(t)
	dashboard.Rows[0].Panels[0].Action = actionSpecRuntime(action.Explore("metric", "missing"))
	err := Validate(dashboard)
	require.Error(t, err)
	require.ErrorContains(t, err, "missing explorer branch")
}

func explorerDashboard(t *testing.T) lens.DashboardSpec {
	t.Helper()
	frames := testFrames(t, "host-data")
	explorerSpec, err := explore.New("metric", "host",
		explore.NewBranch("focus", "Focus", "composition",
			explore.NewPerspective("composition", "Composition", explore.SemanticsPartition, "root",
				explore.LazyNode("root", "Root", "/analytics/explore", explore.ToNode("a", "detail")),
				explore.LazyNode("detail", "Detail", "/analytics/explore"),
			),
		),
	).ExpandedSpan(12).Build()
	require.NoError(t, err)
	return lensbuild.Dashboard("overview", "Overview",
		lensbuild.Row(panel.Pie("host", "Host", "host-data").Build()),
	).Datasets(lensbuild.StaticDataset("host-data", frames)).Explorers(explorerSpec).Build()
}

func dynamicExplorerDashboard(t *testing.T) lens.DashboardSpec {
	t.Helper()
	frames := testFrames(t, "host-data")
	explorerSpec, err := explore.New("metric", "host",
		explore.NewBranch("focus", "Focus", "composition",
			explore.NewPerspective("composition", "Composition", explore.SemanticsPartition, "root",
				explore.LazyNode("root", "Root", "/analytics/explore").WithDynamicEdges("detail"),
				explore.LazyNode("detail", "Detail", "/analytics/explore"),
			),
		),
	).Build()
	require.NoError(t, err)
	return lensbuild.Dashboard("overview", "Overview",
		lensbuild.Row(panel.Pie("host", "Host", "host-data").Build()),
	).Datasets(lensbuild.StaticDataset("host-data", frames)).Explorers(explorerSpec).Build()
}

func resolvedExplorationDefinition(t *testing.T, edges ...explore.Edge) ExplorationDefinition {
	t.Helper()
	frames := testFrames(t, "detail-data")
	loadedPanel := panel.Pie("requested", "Requested", "detail-data").IDField("id").Build()
	dashboard := lensbuild.Dashboard("detail", "Detail", lensbuild.Row(loadedPanel)).
		Datasets(lensbuild.StaticDataset("detail-data", frames)).Build()
	return ExplorationDefinition{Dashboard: dashboard, PanelID: "requested", ResolvedEdges: edges}
}

func testFrames(t *testing.T, name string) *frame.FrameSet {
	t.Helper()
	fr, err := frame.New(name,
		frame.Field{Name: "id", Type: frame.FieldTypeString, Values: []any{"a"}},
		frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{"A"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{1.0}},
	)
	require.NoError(t, err)
	set, err := frame.NewFrameSet(fr)
	require.NoError(t, err)
	return set
}

func actionSpecRuntime(spec action.Spec) *action.Spec { return &spec }
