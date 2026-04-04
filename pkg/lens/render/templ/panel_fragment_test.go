package templ

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/lens"
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
