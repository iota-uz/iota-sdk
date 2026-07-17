package apex

import (
	"math"
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

func TestDrillTreeConfig_UsesStableRootKeysAndSerializesContext(t *testing.T) {
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
		&runtime.PanelResult{Panel: panel.Spec{ID: "accumulated-premium"}},
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
	require.Contains(t, config, `"chartID":"accumulated-premium"`)
}

func TestDrillTreeConfig_RejectsInvalidRootIdentity(t *testing.T) {
	t.Parallel()

	tree := &panel.DrillTree{Branches: []panel.DrillBranch{{
		TriggerKey: "earned",
		Label:      "Earned",
		Children:   []panel.DrillNode{{Key: "2026", Label: "2026", Value: 100}},
	}}}

	tests := []struct {
		name    string
		ids     []any
		mapping panel.FieldMapping
	}{
		{name: "blank ID", ids: []any{"earned", ""}, mapping: panel.FieldMapping{ID: "id"}},
		{name: "ID with surrounding whitespace", ids: []any{"earned", " unearned"}, mapping: panel.FieldMapping{ID: "id"}},
		{name: "missing ID mapping", ids: []any{"earned", "unearned"}, mapping: panel.FieldMapping{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fr, err := frame.New("premium",
				frame.Field{Name: "id", Type: frame.FieldTypeString, Values: test.ids},
				frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{"Earned", "Unearned"}},
				frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{100.0, 40.0}},
			)
			require.NoError(t, err)
			_, ok := drillTreeConfigJS(tree, fr, test.mapping, nil, "en", "Premium", nil)
			require.False(t, ok)
		})
	}
}

func TestDrillTreeAction_UsesSafeResolvedActionSemantics(t *testing.T) {
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

func TestDrillTreeAction_RejectsUnsafeOrRowDependentActions(t *testing.T) {
	t.Parallel()

	unsafe := action.Navigate("https://outside.example/portfolio")
	fieldDependent := action.Navigate("/portfolio", action.FieldParam("year", "year"))
	fieldURL := action.Navigate("").WithFieldURL("action_url")
	cube := action.CubeDrill("/analytics", "year")
	tests := []struct {
		name string
		spec action.Spec
	}{
		{name: "external navigation", spec: unsafe},
		{name: "row-dependent parameter", spec: fieldDependent},
		{name: "row-dependent URL", spec: fieldURL},
		{name: "cube drill", spec: cube},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			require.Nil(t, drillTreeAction(&test.spec, nil))
		})
	}
}

func TestDrillTreeAction_SupportsHtmxAndEmitEvent(t *testing.T) {
	t.Parallel()

	htmx := action.HtmxSwap("/analytics/detail", "#drawer")
	emit := action.Spec{
		Kind:    action.KindEmitEvent,
		Event:   "premium:selected",
		Payload: map[string]action.ValueSource{"metric": action.LiteralValue("earned")},
	}
	tests := []struct {
		name     string
		spec     action.Spec
		expected *drillTreeActionConfig
	}{
		{
			name: "HTMX swap",
			spec: htmx,
			expected: &drillTreeActionConfig{
				Kind:    string(action.KindHtmxSwap),
				Method:  "GET",
				URL:     "/analytics/detail",
				Target:  "#drawer",
				Params:  []drillTreeActionParam{},
				Payload: map[string]any{},
			},
		},
		{
			name: "emitted event",
			spec: emit,
			expected: &drillTreeActionConfig{
				Kind:    string(action.KindEmitEvent),
				Method:  "GET",
				Event:   "premium:selected",
				Params:  []drillTreeActionParam{},
				Payload: map[string]any{"metric": "earned"},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, test.expected, drillTreeAction(&test.spec, nil))
		})
	}
}

func TestDrillTreeConfig_RejectsNonFiniteDataBeforeMarshalling(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("premium",
		frame.Field{Name: "id", Type: frame.FieldTypeString, Values: []any{"earned"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{100.0}},
	)
	require.NoError(t, err)
	tree := &panel.DrillTree{Branches: []panel.DrillBranch{{
		TriggerKey: "earned",
		Label:      "Earned",
		Children:   []panel.DrillNode{{Key: "bad", Label: "Bad", Value: math.Inf(1)}},
	}}}

	_, ok := drillTreeConfigJS(tree, fr, panel.FieldMapping{ID: "id", Value: "value"}, nil, "en", "Premium", nil)
	require.False(t, ok)
}

func TestDrillTreeConfig_FallsBackWhenConfigCannotBeMarshalled(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("premium",
		frame.Field{Name: "id", Type: frame.FieldTypeString, Values: []any{"earned"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{100.0}},
	)
	require.NoError(t, err)
	navigate := action.Navigate("/portfolio")
	navigate.Payload = map[string]action.ValueSource{"invalid": action.LiteralValue(math.NaN())}
	tree := &panel.DrillTree{Branches: []panel.DrillBranch{{
		TriggerKey: "earned",
		Label:      "Earned",
		Children:   []panel.DrillNode{{Key: "direct", Label: "Direct", Value: 100, Action: &navigate}},
	}}}

	_, ok := drillTreeConfigJS(tree, fr, panel.FieldMapping{ID: "id", Value: "value"}, nil, "en", "Premium", nil)
	require.False(t, ok)
}

func TestBuildDrillTreeJS_DelegatesThenFallsBack(t *testing.T) {
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

func TestOptions_UsesDrillTreeWithPanelFallbackWithoutLegacyHandler(t *testing.T) {
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
