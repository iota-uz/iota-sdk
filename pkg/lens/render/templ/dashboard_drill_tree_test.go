package templ

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func renderDashboardScripts(t *testing.T) string {
	t.Helper()

	var html bytes.Buffer
	require.NoError(t, DashboardScripts().Render(context.Background(), &html))
	return html.String()
}

func TestDashboardScripts_DrillTreeUsesStableKeyedState(t *testing.T) {
	t.Parallel()

	rendered := renderDashboardScripts(t)

	assert.Contains(t, rendered, "window.__lensDrillTreeMount")
	assert.Contains(t, rendered, "window.__lensDrillTreeClick")
	assert.Contains(t, rendered, "candidate.triggerKey === path[0]")
	assert.Contains(t, rendered, "candidate.key === path[depth]")
	assert.Contains(t, rendered, "JSON.stringify(Array.isArray(path) ? path : [])")
	assert.Contains(t, rendered, "scope.__lensDrillTreeSharedState")
	assert.Contains(t, rendered, "hiddenByLevel: lensDrillTreeHiddenSnapshot")
	assert.Contains(t, rendered, "return lensDrillTreeResolve(cfg, state, [])")
	// Drill updates must stay local to their chart. ApexCharts otherwise updates
	// every chart in the shared group and strips the sibling trees' enhanced
	// slice roles, stable keys, and keyboard bindings during the redraw.
	assert.Contains(t, rendered, "}, true, false, false);")
}

func TestDashboardScripts_DrillTreeRendersAccessibleNavigation(t *testing.T) {
	t.Parallel()

	rendered := renderDashboardScripts(t)

	assert.Contains(t, rendered, "lens-drill-tree__nav")
	assert.Contains(t, rendered, "data-lens-drill-tree-back")
	assert.Contains(t, rendered, "aria-current")
	assert.Contains(t, rendered, "data-lens-drill-tree-context")
	assert.Contains(t, rendered, "data-lens-drill-tree-live")
	assert.Contains(t, rendered, "aria-live")
	assert.Contains(t, rendered, "legend.setAttribute('role', 'checkbox')")
	assert.Contains(t, rendered, "legend.setAttribute('aria-checked'")
	assert.Contains(t, rendered, "slice.setAttribute('role', actionable ? 'button' : 'img')")
	assert.Contains(t, rendered, "!state.path.length && cfg.hasFallbackAction")
	assert.Contains(t, rendered, "container.__lensTotalBadgeUseDynamicSeries = true")
	assert.Contains(t, rendered, "delete container.__lensTotalBadgeUseDynamicSeries")
	assert.Contains(t, rendered, "const currentItem = (state.currentItems || [])[index]")
	assert.NotContains(t, rendered, "data-lens-drill-tree-action")
	assert.Contains(t, rendered, "event.key !== 'Escape'")
	assert.Contains(t, rendered, "event.key === 'Enter' || event.key === ' '")
	assert.Contains(t, rendered, "event.key === 'ArrowRight'")
}

func TestDashboardScripts_DrillTreeTransitionsRespectMotionPreference(t *testing.T) {
	t.Parallel()

	rendered := renderDashboardScripts(t)

	assert.Contains(t, rendered, "prefers-reduced-motion: reduce")
	assert.Contains(t, rendered, "matchMedia('(prefers-reduced-motion: reduce)').matches")
	assert.Contains(t, rendered, "lens-drill-tree--leaving-forward")
	assert.Contains(t, rendered, "lens-drill-tree--entering-back")
	assert.Contains(t, rendered, "container.setAttribute('aria-busy', 'true')")
	assert.Contains(t, rendered, "container.setAttribute('aria-busy', 'false')")
	assert.Contains(t, rendered, "setTimeout(finish, 140)")
}

func TestDashboardScripts_DrillControlsUseReusableStyles(t *testing.T) {
	t.Parallel()

	rendered := renderDashboardScripts(t)

	assert.Contains(t, rendered, ".lens-drill-back,")
	assert.Contains(t, rendered, "min-height: 44px")
	assert.Contains(t, rendered, "btn.className = 'lens-drill-back'")
	assert.NotContains(t, rendered, "btn.style.position = 'absolute'")
}
