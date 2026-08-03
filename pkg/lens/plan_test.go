package lens

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/lens/explore"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/stretchr/testify/require"
)

func TestCompileExecutionPlanDerivesVisualOrderAndDependencyClosures(t *testing.T) {
	spec := DashboardSpec{
		Datasets: []DatasetSpec{
			{Name: "raw", Kind: DatasetKindQuery},
			{Name: "hero", Kind: DatasetKindTransform, DependsOn: []string{"raw"}},
			{Name: "detail", Kind: DatasetKindTransform, DependsOn: []string{"raw"}},
		},
		Rows: []RowSpec{
			{Panels: []panel.Spec{{ID: "hero", Kind: panel.KindStat, Dataset: "hero"}}},
			{Panels: []panel.Spec{{ID: "chart", Kind: panel.KindBar, Dataset: "raw"}}},
		},
		Explorers: []explore.Spec{{
			ID: "profit", HostPanelID: "chart", Branches: []explore.Branch{{
				Key: "claims", DefaultPerspective: "group", Perspectives: []explore.Perspective{{
					Key: "group", Semantics: explore.SemanticsPartition, RootNode: "root", Nodes: []explore.Node{
						{Key: "root", Panel: &panel.Spec{ID: "root-detail", Kind: panel.KindBar, Dataset: "detail"}, Edges: []explore.Edge{{PointKey: "A", ToNode: "leaf"}}},
						{Key: "leaf", Panel: &panel.Spec{ID: "leaf-detail", Kind: panel.KindTable, Dataset: "raw"}, DynamicEdges: true},
					},
				}},
			}},
		}},
	}

	plan, err := CompileExecutionPlan(spec)
	require.NoError(t, err)
	require.Len(t, plan.Surfaces, 4)
	hero, ok := plan.Surface("root:hero")
	require.True(t, ok)
	require.Equal(t, 0, hero.VisualOrder)
	require.Equal(t, 0, hero.Depth)
	require.Equal(t, []string{"raw", "hero"}, hero.DatasetClosure)
	chart, ok := plan.Surface("root:chart")
	require.True(t, ok)
	require.Equal(t, 1, chart.VisualOrder)
	require.Equal(t, 1, chart.Depth)
	leaf, ok := plan.Surface("explore:profit:claims:group:leaf")
	require.True(t, ok)
	require.Equal(t, 1, leaf.Depth)
	require.Equal(t, "explore:profit:claims:group:root", leaf.ParentKey)
	require.True(t, leaf.DynamicChildren)
	require.Equal(t, []string{"raw"}, plan.SharedDatasets("root:hero", leaf.Key))
}

func TestCompileExecutionPlanRejectsDependencyCycle(t *testing.T) {
	_, err := CompileExecutionPlan(DashboardSpec{
		Datasets: []DatasetSpec{
			{Name: "a", DependsOn: []string{"b"}},
			{Name: "b", DependsOn: []string{"a"}},
		},
		Rows: []RowSpec{{Panels: []panel.Spec{{ID: "panel", Kind: panel.KindStat, Dataset: "a"}}}},
	})
	require.ErrorContains(t, err, "dependency cycle")
}
