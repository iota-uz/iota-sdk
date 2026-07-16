package apex

import (
	"net/url"
	"testing"

	"github.com/a-h/templ"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/format"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/stretchr/testify/require"
)

func TestDrillTreeConfigUsesStableRootKeysAndSerializesContext(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("premium",
		frame.Field{Name: "id", Type: frame.FieldTypeString, Values: []any{"earned", "unearned"}},
		frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{"Премия", "Премия"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{100.0, 40.0}},
	)
	require.NoError(t, err)
	formatter := format.Money("UZS", 0)
	tree := &panel.DrillTree{Branches: []panel.DrillBranch{{
		TriggerKey: "earned",
		Label:      "Заработанная премия",
		Children: []panel.DrillNode{{
			Key:   "2026",
			Label: "2026",
			Value: 100,
			Children: []panel.DrillNode{{
				Key:   "direct",
				Label: "Прямые продажи",
				Value: 100,
			}},
		}},
	}}}

	config, ok := drillTreeConfigJS(
		tree,
		fr,
		panel.FieldMapping{ID: "id", Label: "label", Value: "value"},
		&formatter,
		"ru",
		"Накопленная премия",
		&runtime.PanelResult{},
	)

	require.True(t, ok)
	require.Contains(t, config, `"rootKeys":["earned","unearned"]`)
	require.Contains(t, config, `"rootValuesFormatted":["100 so’m","40 so’m"]`)
	require.Contains(t, config, `"rootLabel":"Накопленная премия"`)
	require.Contains(t, config, `"triggerKey":"earned"`)
	require.Contains(t, config, `"label":"Заработанная премия"`)
	require.Contains(t, config, `"key":"2026"`)
	require.Contains(t, config, `"key":"direct"`)
	require.Contains(t, config, `"total":100`)
	require.Contains(t, config, `"totalFormatted":"100 so’m"`)
	require.Contains(t, config, `"backLabel":"Назад"`)
}

func TestDrillTreeConfigRequiresCompleteIDField(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("premium",
		frame.Field{Name: "id", Type: frame.FieldTypeString, Values: []any{"earned", ""}},
		frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{"Earned", "Unearned"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{100.0, 40.0}},
	)
	require.NoError(t, err)
	tree := &panel.DrillTree{Branches: []panel.DrillBranch{{
		TriggerKey: "earned",
		Label:      "Earned",
		Children:   []panel.DrillNode{{Key: "2026", Label: "2026", Value: 100}},
	}}}

	_, ok := drillTreeConfigJS(tree, fr, panel.FieldMapping{ID: "id"}, nil, "en", "Premium", nil)
	require.False(t, ok)
	_, ok = drillTreeConfigJS(tree, fr, panel.FieldMapping{}, nil, "en", "Premium", nil)
	require.False(t, ok)
}

func TestDrillTreeActionUsesSafeResolvedActionSemantics(t *testing.T) {
	t.Parallel()

	spec := action.Navigate(
		"/portfolio/policies",
		action.LiteralParam("year", 2026),
		action.VariableParam("source", "source"),
	).WithPreservedQuery()
	spec.Payload = map[string]action.ValueSource{"metric": action.LiteralValue("earned")}
	result := &runtime.PanelResult{
		Variables: map[string]any{"source": "direct"},
		Request:   url.Values{"period": []string{"year"}},
	}

	configured := drillTreeAction(&spec, result)

	require.NotNil(t, configured)
	require.Equal(t, string(action.KindNavigate), configured.Kind)
	require.Equal(t, "GET", configured.Method)
	require.Equal(t, "/portfolio/policies", configured.URL)
	require.Equal(t, []drillTreeActionParam{
		{Name: "year", Value: 2026},
		{Name: "source", Value: "direct"},
	}, configured.Params)
	require.Equal(t, map[string]any{"metric": "earned"}, configured.Payload)
	require.Equal(t, map[string][]string{"period": {"year"}}, configured.BaseQuery)
	require.True(t, configured.PreserveQuery)
}

func TestDrillTreeActionRejectsUnsafeOrRowDependentActions(t *testing.T) {
	t.Parallel()

	unsafe := action.Navigate("https://outside.example/portfolio")
	require.Nil(t, drillTreeAction(&unsafe, nil))

	fieldDependent := action.Navigate("/portfolio", action.FieldParam("year", "year"))
	require.Nil(t, drillTreeAction(&fieldDependent, nil))

	fieldURL := action.Navigate("").WithFieldURL("action_url")
	require.Nil(t, drillTreeAction(&fieldURL, nil))

	cube := action.CubeDrill("/analytics", "year")
	require.Nil(t, drillTreeAction(&cube, nil))
}

func TestDrillTreeActionSupportsHtmxAndEmitEvent(t *testing.T) {
	t.Parallel()

	htmx := action.HtmxSwap("/analytics/detail", "#drawer")
	htmxConfig := drillTreeAction(&htmx, nil)
	require.NotNil(t, htmxConfig)
	require.Equal(t, string(action.KindHtmxSwap), htmxConfig.Kind)
	require.Equal(t, "#drawer", htmxConfig.Target)
	require.Equal(t, "/analytics/detail", htmxConfig.URL)

	emit := action.Spec{
		Kind:    action.KindEmitEvent,
		Event:   "premium:selected",
		Payload: map[string]action.ValueSource{"metric": action.LiteralValue("earned")},
	}
	emitConfig := drillTreeAction(&emit, nil)
	require.NotNil(t, emitConfig)
	require.Equal(t, string(action.KindEmitEvent), emitConfig.Kind)
	require.Equal(t, "premium:selected", emitConfig.Event)
	require.Equal(t, map[string]any{"metric": "earned"}, emitConfig.Payload)
}

func TestBuildDrillTreeJSDelegatesThenFallsBack(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("premium",
		frame.Field{Name: "id", Type: frame.FieldTypeString, Values: []any{"earned"}},
		frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{"Earned"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{100.0}},
	)
	require.NoError(t, err)
	tree := &panel.DrillTree{Branches: []panel.DrillBranch{{
		TriggerKey: "earned",
		Label:      "Earned",
		Children:   []panel.DrillNode{{Key: "2026", Label: "2026", Value: 100}},
	}}}
	fallback := templ.JSExpression(`function(){ window.fallback = true; }`)

	click := string(buildDrillTreeJS(tree, fr, panel.FieldMapping{ID: "id"}, nil, "en", "Premium", nil, fallback))
	mount := string(buildDrillTreeMountJS(tree, fr, panel.FieldMapping{ID: "id"}, nil, "en", "Premium", nil, true))

	require.Contains(t, click, "__lensDrillTreeClick")
	require.Contains(t, click, "window.fallback = true")
	require.Contains(t, click, "cfg.hasFallbackAction = true")
	require.Contains(t, click, `"rootKeys":["earned"]`)
	require.Contains(t, mount, "__lensDrillTreeMount")
	require.Contains(t, mount, "cfg.hasFallbackAction = true")
}

func TestOptionsUsesDrillTreeWithPanelFallbackWithoutLegacyHandler(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("premium",
		frame.Field{Name: "id", Type: frame.FieldTypeString, Values: []any{"earned", "legacy"}},
		frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{"Earned", "Legacy"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{100.0, 20.0}},
	)
	require.NoError(t, err)
	frameSet, err := frame.NewFrameSet(fr)
	require.NoError(t, err)
	spec := panel.Pie("premium-panel", "Premium", "premium").
		IDField("id").
		Action(action.Navigate("/fallback")).
		DrillTree(panel.DrillTree{Branches: []panel.DrillBranch{{
			TriggerKey: "earned",
			Label:      "Earned",
			Children:   []panel.DrillNode{{Key: "2026", Label: "2026", Value: 100}},
		}}}).
		Build()

	opts := Options(spec, &runtime.PanelResult{Frames: frameSet, Locale: "en"})
	require.NotNil(t, opts.Chart.Events)
	click := string(opts.Chart.Events.DataPointSelection)
	mount := string(opts.Chart.Events.Mounted)
	require.Contains(t, click, "__lensDrillTreeClick")
	require.Contains(t, click, "window.location.href = nextURL")
	require.Contains(t, click, "cfg.hasFallbackAction = true")
	require.Contains(t, mount, "__lensDrillTreeMount")
	require.Contains(t, mount, "cfg.hasFallbackAction = true")
}
