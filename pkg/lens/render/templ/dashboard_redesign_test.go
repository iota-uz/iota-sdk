package templ

import (
	"bytes"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

// A row carrying a Heading renders as a section band (label + hairline rule),
// not a panel grid.
func TestDashboard_RendersSectionHeading(t *testing.T) {
	t.Parallel()

	spec := lens.DashboardSpec{
		ID: "sectioned",
		Rows: []lens.RowSpec{
			{Heading: "Премии"},
		},
	}

	var html bytes.Buffer
	err := Dashboard(DashboardProps{Spec: spec}).Render(metricInfoContext(t, language.English), &html)
	require.NoError(t, err)

	rendered := html.String()
	assert.Contains(t, rendered, "Премии")
	assert.Contains(t, rendered, "uppercase tracking-wider")
}

// A Stat with an AccentColor but no Icon renders the icon-less accent chrome: a
// solid family-color bar, the value, and NO colored icon badge.
func TestStatPanel_AccentChromeWithoutIcon(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("kpis",
		frame.Field{Name: "earned_premium", Type: frame.FieldTypeNumber, Values: []any{1000.0}},
	)
	require.NoError(t, err)
	set, err := frame.NewFrameSet(fr)
	require.NoError(t, err)

	spec := panel.Stat("kpi-earned", "Заработанная премия", "kpis").
		AccentColor("#2563eb").
		ValueField("earned_premium").
		Build()

	result := &runtime.PanelResult{Panel: spec, Frames: set, Locale: "en"}

	var html bytes.Buffer
	err = StatPanel(spec, result, nil).Render(metricInfoContext(t, language.English), &html)
	require.NoError(t, err)

	rendered := html.String()
	assert.Contains(t, rendered, "Заработанная премия")
	// solid family-color accent bar (statAccentStyle)
	assert.Contains(t, rendered, "background-color: #2563eb;")
	// no translucent icon-badge chrome (badgeStyle uses rgba(...))
	assert.NotContains(t, rendered, "rgba(37, 101, 235")
	assert.NotContains(t, rendered, "h-10 w-10")
}

// The skeleton mirrors the prepared layout: a heading band plus card-shaped
// shimmer per panel, all under one pulse.
func TestDashboardSkeleton_MirrorsLayout(t *testing.T) {
	t.Parallel()

	statSpec := panel.Stat("kpi", "KPI", "ds").AccentColor("#2563eb").Build()
	chartSpec := panel.Bar("bar", "Bar", "ds").Build()
	spec := lens.DashboardSpec{
		ID: "skeleton",
		Rows: []lens.RowSpec{
			{Heading: "Премии"},
			{Panels: []panel.Spec{statSpec, chartSpec}},
		},
	}

	var html bytes.Buffer
	err := DashboardSkeleton(spec).Render(metricInfoContext(t, language.English), &html)
	require.NoError(t, err)

	rendered := html.String()
	assert.Contains(t, rendered, "lens-skeleton-shimmer")
	// stat-shaped skeleton card (min height of the accent stat card)
	assert.Contains(t, rendered, "min-height:136px")
	// chart/table-shaped skeleton block
	assert.Contains(t, rendered, "min-height:260px")
	// no live spinner
	assert.NotContains(t, rendered, "animate-spin")
}
