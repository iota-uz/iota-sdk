package document

import (
	"context"
	"encoding/json"
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	lensbuild "github.com/iota-uz/iota-sdk/pkg/lens/build"
	"github.com/iota-uz/iota-sdk/pkg/lens/explore"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/stretchr/testify/require"
)

func TestBuild_ExistingExploreSpec(t *testing.T) {
	t.Parallel()
	spec, result := executeExploreDashboard(t)
	doc, err := Build(spec, result, BuildOptions{
		SnapshotID: "snapshot-generated", GeneratedAt: time.Date(2026, time.July, 19, 10, 0, 0, 0, time.UTC),
		Locale: "en", InlineDepth: 1,
	})
	require.NoError(t, err)
	require.Equal(t, ContractVersion, doc.Version)
	require.Len(t, doc.Panels, 1)
	require.Equal(t, SemanticsPartition, doc.Panels[0].Semantics)
	require.NotNil(t, doc.Panels[0].DrillRoot)
	require.Contains(t, doc.Drill.Edges, *doc.Panels[0].DrillRoot)
	require.Len(t, doc.Perspectives, 1)
	require.Equal(t, NodeKey("metric/focus/composition/root"), doc.Perspectives[0].Root)

	payload, err := json.MarshalIndent(doc, "", "  ")
	require.NoError(t, err)
	require.Equal(t, golden(t, "generated_explore.json"), string(payload)+"\n")
}

func TestBuild_NodeKeysIgnoreLabelsAndDefinitionOrder(t *testing.T) {
	t.Parallel()
	spec, result := executeExploreDashboard(t)
	first, err := Build(spec, result, BuildOptions{SnapshotID: "one", GeneratedAt: time.Unix(1, 0), InlineDepth: 1})
	require.NoError(t, err)

	view := &spec.Explorers[0].Branches[0].Perspectives[0]
	view.Label = "Localized label"
	for index := range view.Nodes {
		view.Nodes[index].Label = "Localized " + view.Nodes[index].Key
	}
	slices.Reverse(view.Nodes)
	second, err := Build(spec, result, BuildOptions{SnapshotID: "two", GeneratedAt: time.Unix(2, 0), InlineDepth: 1})
	require.NoError(t, err)

	firstKeys := maps.Keys(first.Drill.Edges)
	secondKeys := maps.Keys(second.Drill.Edges)
	require.ElementsMatch(t, slices.Collect(firstKeys), slices.Collect(secondKeys))
	for index := range first.Perspectives {
		require.Equal(t, first.Perspectives[index].Root, second.Perspectives[index].Root)
	}
}

func TestBuild_ReusesRuntimeExploreValidation(t *testing.T) {
	t.Parallel()
	spec, result := executeExploreDashboard(t)
	spec.Explorers[0].Branches[0].Perspectives[0].Semantics = "unsupported"
	_, err := Build(spec, result, BuildOptions{SnapshotID: "invalid"})
	require.ErrorContains(t, err, "unsupported semantics")
}

func TestBuild_InlineDepthIncludesOnlyMaterializedAggregateLevels(t *testing.T) {
	t.Parallel()
	spec, result := executeExploreDashboard(t)
	view := &spec.Explorers[0].Branches[0].Perspectives[0]
	rootPanel := panel.Pie("explore-root", "Root", "premium").IDField("id").Build()
	detailPanel := panel.Pie("explore-detail", "Detail", "premium").IDField("id").Build()
	view.Nodes[0].Load = nil
	view.Nodes[0].Panel = &rootPanel
	view.Nodes[1].Load = nil
	view.Nodes[1].Panel = &detailPanel
	result.Panels[rootPanel.ID] = &runtime.PanelResult{Panel: rootPanel, Frames: result.Panels["host"].Frames}
	result.Panels[detailPanel.ID] = &runtime.PanelResult{Panel: detailPanel, Frames: result.Panels["host"].Frames}

	doc, err := Build(spec, result, BuildOptions{SnapshotID: "inline", GeneratedAt: time.Unix(1, 0), InlineDepth: 0})
	require.NoError(t, err)
	require.Equal(t, FrameRef("explore:metric/focus/composition:root"), doc.Drill.Edges["metric/focus/composition/root"].Frame)
	require.Empty(t, doc.Drill.Edges["metric/focus/composition/detail"].Frame)
	require.NotContains(t, doc.Frames, FrameRef("explore:metric/focus/composition:detail"))
}

func executeExploreDashboard(t *testing.T) (lens.DashboardSpec, *runtime.Result) {
	t.Helper()
	primary, err := frame.New("premium",
		frame.Field{Name: "id", Type: frame.FieldTypeString, Values: []any{"a", "b"}},
		frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{"Alpha", "Beta"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{60.0, 40.0}},
	)
	require.NoError(t, err)
	frames, err := frame.NewFrameSet(primary)
	require.NoError(t, err)
	explorerSpec, err := explore.New("metric", "host",
		explore.NewBranch("focus", "Focus", "composition",
			explore.NewPerspective("composition", "Composition", explore.SemanticsPartition, "root",
				explore.LazyNode("root", "Root", "/explore", explore.ToNode("a", "detail")),
				explore.LazyNode("detail", "Detail", "/explore", explore.ToAction("leaf", action.Navigate("/policies/{policyId}", action.LiteralParam("policyId", "selected")))),
			),
		),
	).Build()
	require.NoError(t, err)
	spec := lensbuild.Dashboard("overview", "Overview",
		lensbuild.Row(panel.Pie("host", "Premium", "premium").IDField("id").Build()),
	).Datasets(lensbuild.StaticDataset("premium", frames)).Explorers(explorerSpec).Build()
	executed, err := runtime.New(runtime.Options{}).Execute(context.Background(), spec, runtime.Request{Locale: "en", DataScope: "tenant:1"}, runtime.DashboardScope())
	require.NoError(t, err)
	return spec, executed
}
