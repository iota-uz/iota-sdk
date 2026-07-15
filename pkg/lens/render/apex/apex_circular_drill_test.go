package apex

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/format"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/stretchr/testify/require"
)

func TestCircularDrillHierarchyConfigIncludesDetailAndLocalizedBackLabel(t *testing.T) {
	t.Parallel()

	spec := &panel.CircularDrillHierarchy{
		TriggerLabel: "Прочие",
		Detail: []panel.CircularDrillSlice{
			{Label: "Reinsurer A", Value: 12, Color: "#2563eb", ActionURL: "/reinsurance?Search=A"},
			{Label: "Reinsurer B", Value: 8, Color: "#7c3aed", ActionURL: "/reinsurance?Search=B"},
		},
	}
	formatter := format.Money("UZS", 0)

	config, ok := circularDrillHierarchyConfigJS(spec, &formatter, "ru", nil)
	require.True(t, ok)
	require.Contains(t, config, `"branches":[{"triggerLabel":"Прочие"`)
	require.Contains(t, config, `"backLabel":"Назад"`)
	require.Contains(t, config, `"detailTotal":"20 so’m"`)
	require.Contains(t, config, `/reinsurance?Search=A`)
}

func TestCircularDrillHierarchyConfigSupportsMultipleNestedBranches(t *testing.T) {
	t.Parallel()

	spec := &panel.CircularDrillHierarchy{Branches: []panel.CircularDrillBranch{
		{
			TriggerLabel: "Заработанная премия",
			Detail: []panel.CircularDrillSlice{{
				Label: "2025",
				Value: 100,
				Detail: []panel.CircularDrillSlice{{
					Label:  "Прямые продажи",
					Value:  100,
					Detail: []panel.CircularDrillSlice{{Label: "Q1", Value: 100, ActionURL: "/portfolio/policies?year=2025"}},
				}},
			}},
		},
		{
			TriggerLabel: "Незаработанная премия",
			Detail:       []panel.CircularDrillSlice{{Label: "Расчётный резерв", Value: 40}},
		},
	}}
	formatter := format.Money("UZS", 0)

	config, ok := circularDrillHierarchyConfigJS(spec, &formatter, "ru", nil)
	require.True(t, ok)
	require.Contains(t, config, `"triggerLabel":"Заработанная премия"`)
	require.Contains(t, config, `"triggerLabel":"Незаработанная премия"`)
	require.Contains(t, config, `"label":"2025","value":100`)
	require.Contains(t, config, `"label":"Прямые продажи","value":100`)
	require.Contains(t, config, `"label":"Q1","value":100`)
	require.Contains(t, config, `"actionUrl":"/portfolio/policies?year=2025"`)
}

func TestOptionsWiresCircularDrillBeforeFallbackAction(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("reinsurance",
		frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{"A", "Прочие"}},
		frame.Field{Name: "action_url", Type: frame.FieldTypeString, Values: []any{"/a", "/all"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{80.0, 20.0}},
	)
	require.NoError(t, err)
	frameSet, err := frame.NewFrameSet(fr)
	require.NoError(t, err)
	actionSpec := action.Navigate("").WithFieldURL("action_url")
	spec := panel.Pie("reinsurance-panel", "Reinsurance", "reinsurance").
		Action(actionSpec).
		CircularDrillHierarchy(panel.CircularDrillHierarchy{
			TriggerLabel: "Прочие",
			Detail: []panel.CircularDrillSlice{
				{Label: "B", Value: 12, ActionURL: "/b"},
				{Label: "C", Value: 8, ActionURL: "/c"},
			},
		}).
		Build()

	opts := Options(spec, &runtime.PanelResult{Frames: frameSet, Locale: "ru"})
	require.NotNil(t, opts.Chart.Events)
	click := string(opts.Chart.Events.DataPointSelection)
	require.Contains(t, click, "__lensCircularDrillClick")
	require.Contains(t, click, "window.location.href = nextURL")
	require.Contains(t, string(opts.Chart.Events.Mounted), "__lensCircularDrillMount")
}

func TestChainChartEventPreservesBothHandlers(t *testing.T) {
	t.Parallel()

	combined := string(chainChartEvent("function(){ window.first = true; }", "function(){ window.second = true; }"))
	require.Contains(t, combined, "window.first = true")
	require.Contains(t, combined, "window.second = true")
	require.Contains(t, combined, "first.apply(this, arguments)")
	require.Contains(t, combined, "second.apply(this, arguments)")
}

func TestPieSupportsRightLegendWithMobileBottomFallback(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("composition",
		frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{"A", "B"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{70.0, 30.0}},
	)
	require.NoError(t, err)
	frameSet, err := frame.NewFrameSet(fr)
	require.NoError(t, err)
	spec := panel.Pie("composition-panel", "Composition", "composition").
		LegendAt(panel.LegendRight).
		LegendWidth(300).
		Build()

	opts := Options(spec, &runtime.PanelResult{Frames: frameSet, Locale: "ru"})
	require.NotNil(t, opts.Legend)
	require.NotNil(t, opts.Legend.Position)
	require.Equal(t, "right", string(*opts.Legend.Position))
	require.NotNil(t, opts.Legend.Width)
	require.Equal(t, 300, *opts.Legend.Width)
	require.Len(t, opts.Responsive, 1)
	require.NotNil(t, opts.Responsive[0].Options.Legend)
	require.Equal(t, "bottom", string(*opts.Responsive[0].Options.Legend.Position))
}
