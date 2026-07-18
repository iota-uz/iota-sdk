package templ

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/explore"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/stretchr/testify/require"
)

func TestRenderPanelFragment(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("sales",
		frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{"OSAGO"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{42.0}},
	)
	require.NoError(t, err)

	set, err := frame.NewFrameSet(fr)
	require.NoError(t, err)

	spec := panel.Bar("sales-panel", "Sales", "sales").
		LabelField("label").
		ValueField("value").
		Build()

	result := &runtime.Result{
		Spec: lens.DashboardSpec{
			Rows: []lens.RowSpec{{Panels: []panel.Spec{spec}}},
		},
		Panels: map[string]*runtime.PanelResult{
			"sales-panel": {
				Panel:  spec,
				Frames: set,
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/dashboards/panels/sales-panel", nil)
	rec := httptest.NewRecorder()

	ok := RenderPanelFragment(rec, req, result, "sales-panel")
	require.True(t, ok)
	require.Contains(t, rec.Body.String(), "ApexCharts(container, options)")
}

func TestRenderPanelFragmentReturnsFalseWhenPanelMissing(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/dashboards/panels/missing", nil)
	rec := httptest.NewRecorder()

	ok := RenderPanelFragment(rec, req, &runtime.Result{}, "missing")
	require.False(t, ok)
}

func TestRenderExplorationFragmentWiresStablePointEvent(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("products",
		frame.Field{Name: "id", Type: frame.FieldTypeString, Values: []any{"osago"}},
		frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{"OSAGO"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{42.0}},
	)
	require.NoError(t, err)
	set, err := frame.NewFrameSet(fr)
	require.NoError(t, err)
	panelSpec := panel.Pie("products-panel", "Products", "products").
		IDField("id").LabelField("label").ValueField("value").Build()
	node := explore.PanelNode("root", "Products", panelSpec, explore.ToAction("osago", action.Navigate("/portfolio")))
	view := explore.NewPerspective("products", "Products", explore.SemanticsPartition, "root", node)
	result := &runtime.ExplorationResult{
		Explorer: explore.Spec{ID: "premium-explorer"},
		View:     view,
		Path:     []string{"root"},
		Panel:    &runtime.PanelResult{Panel: panelSpec, Frames: set},
	}

	rec := httptest.NewRecorder()
	ok := RenderExplorationFragment(rec, httptest.NewRequest(http.MethodGet, "/explore", nil), result)

	require.True(t, ok)
	require.Contains(t, rec.Body.String(), "lens:explorer-point")
	require.Contains(t, rec.Body.String(), "premium-explorer")
	require.Contains(t, rec.Body.String(), "pointKey")
	require.Contains(t, rec.Body.String(), "data-lens-explorer-resolved-edges")
	require.Contains(t, rec.Body.String(), "/portfolio")
}

func TestRenderExplorationFragmentSerializesResolvedDynamicEdges(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("groups",
		frame.Field{Name: "id", Type: frame.FieldTypeString, Values: []any{"other"}},
		frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{"Other"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{7.0}},
	)
	require.NoError(t, err)
	set, err := frame.NewFrameSet(fr)
	require.NoError(t, err)
	panelSpec := panel.Pie("groups-panel", "Groups", "groups").
		IDField("id").LabelField("label").ValueField("value").Build()
	root := explore.PanelNode("root", "Groups", panelSpec).WithDynamicEdges("detail")
	detail := explore.LazyNode("detail", "Details", "/explore")
	view := explore.NewPerspective("groups", "Groups", explore.SemanticsPartition, "root", root, detail)
	result := &runtime.ExplorationResult{
		Explorer: explore.Spec{ID: "metric-explorer"},
		View:     view,
		Path:     []string{"root"},
		Steps:    []explore.PathStep{{NodeKey: "root"}},
		Panel:    &runtime.PanelResult{Panel: panelSpec, Frames: set},
		Edges:    []explore.Edge{explore.ToNode("other", "detail")},
	}

	rec := httptest.NewRecorder()
	ok := RenderExplorationFragment(rec, httptest.NewRequest(http.MethodGet, "/explore", nil), result)

	require.True(t, ok)
	require.Contains(t, rec.Body.String(), "data-lens-explorer-resolved-edges")
	require.Contains(t, rec.Body.String(), "other")
	require.Contains(t, rec.Body.String(), "detail")
	require.Contains(t, rec.Body.String(), "lens:explorer-point")
}
