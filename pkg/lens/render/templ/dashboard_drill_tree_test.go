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
	assert.Contains(t, rendered, "scope.__lensDrillTreeSharedState[key]")
	assert.Contains(t, rendered, "cfg.chartID")
	assert.Contains(t, rendered, "hiddenByLevel: lensDrillTreeHiddenSnapshot")
	assert.Contains(t, rendered, "return lensDrillTreeResolve(cfg, state, [])")
	// Drill updates must stay local to their chart. ApexCharts otherwise updates
	// every chart in the shared group and strips the sibling trees' enhanced
	// slice roles, stable keys, and keyboard bindings during the redraw.
	assert.Contains(t, rendered, "}, true, false, false);")
}

func TestDashboardScripts_MetricExplorerOwnsPerspectiveAndLazyViewState(t *testing.T) {
	t.Parallel()

	rendered := renderDashboardScripts(t)

	assert.Contains(t, rendered, "window.__lensExploreOpen")
	assert.Contains(t, rendered, "lensExploreSelectPerspective")
	assert.Contains(t, rendered, "pathByPerspective")
	assert.Contains(t, rendered, "data-lens-explorer-tabs")
	assert.Contains(t, rendered, "data-lens-explorer-crumbs")
	assert.Contains(t, rendered, "state.requestVersion !== version")
	assert.Contains(t, rendered, "new AbortController()")
	assert.Contains(t, rendered, "state.abortController.abort()")
	assert.Contains(t, rendered, "error.name === 'AbortError'")
	assert.Contains(t, rendered, "data-lens-explorer-retry")
	assert.Contains(t, rendered, "lensExploreCaptureHidden")
	assert.Contains(t, rendered, "hiddenByView[lensExploreHiddenKey")
	assert.Contains(t, rendered, "apexcharts-inactive-legend")
	assert.Contains(t, rendered, "setTimeout(function()")
	assert.Contains(t, rendered, "}, 150)")
	assert.Contains(t, rendered, "prefers-reduced-motion: reduce")
	assert.Contains(t, rendered, "window.__lensExploreToggleFullscreen")
	assert.Contains(t, rendered, "const lensExploreSetFullscreen")
	assert.Contains(t, rendered, "button.setAttribute('aria-label', label)")
	assert.Contains(t, rendered, "button.setAttribute('title', label)")
	assert.Contains(t, rendered, "lensExploreSetFullscreen(fullscreen, false)")
	assert.Contains(t, rendered, "lensExploreEnableVisibleTotals")
	assert.Contains(t, rendered, "container.__lensTotalBadgeUseDynamicSeries = true")
	assert.Contains(t, rendered, "chart.updateOptions({}, false, false, false)")
	assert.Contains(t, rendered, "metadata.manualLogScale")
	assert.Contains(t, rendered, "lens_explorer")
	assert.Contains(t, rendered, "lens_path")
}

func TestDashboardScripts_MetricExplorerSameViewActivationIsNoOp(t *testing.T) {
	t.Parallel()

	rendered := renderDashboardScripts(t)

	assert.Contains(t, rendered, "if (state.perspectiveKey === key) { return; }")
	assert.Contains(t, rendered, "const resolvedViewKey = lensExploreResolvedViewKey(state, steps)")
	assert.Contains(t, rendered, "if (state.activeViewKey === resolvedViewKey && (state.status === 'loading' || state.status === 'skeleton' || state.status === 'ready')) { return; }")
	assert.Contains(t, rendered, "const requestKey = lensExploreResolvedViewKey(state, steps)")
	assert.Contains(t, rendered, "if (state.pendingViewKey === requestKey && (state.status === 'loading' || state.status === 'skeleton')) { return; }")
	assert.Contains(t, rendered, "state.pendingViewKey = requestKey")
}

func TestDashboardScripts_MetricExplorerAppliesResolvedDynamicEdges(t *testing.T) {
	t.Parallel()

	rendered := renderDashboardScripts(t)

	assert.Contains(t, rendered, "data-lens-explorer-resolved-edges")
	assert.Contains(t, rendered, "const lensExploreApplyResolvedEdges")
	assert.Contains(t, rendered, "node.edges = Array.isArray(edges) ? edges : []")
	assert.Contains(t, rendered, "lensExploreApplyResolvedEdges(content, node)")
	assert.Contains(t, rendered, "const edge = node && (node.edges || []).find")
	assert.Contains(t, rendered, "state.stepsByPerspective[key] = steps.concat({nodeKey: edge.toNode, pointKey: clickedKey})")
	assert.Contains(t, rendered, "if (edge.toNode)")
	assert.Contains(t, rendered, "const action = edge.action")
	assert.Contains(t, rendered, "if (action.kind === 'navigate')")
}

func TestDashboardScripts_MetricExplorerRestoresTypedPathAndExportsPoints(t *testing.T) {
	t.Parallel()

	rendered := renderDashboardScripts(t)

	assert.Contains(t, rendered, "stepsByPerspective: {}")
	assert.Contains(t, rendered, "state.stepsByPerspective[key] = (state.stepsByPerspective[key] || []).slice(0, -1)")
	assert.Contains(t, rendered, "points: params.getAll('lens_explore_point')")
	assert.Contains(t, rendered, "state.stepsByPerspective[key] = requestedPath.map")
	assert.Contains(t, rendered, "next.searchParams.append('lens_path', step.nodeKey)")
	assert.Contains(t, rendered, "next.searchParams.append('lens_explore_point', step.pointKey || '')")
	assert.Contains(t, rendered, "url.searchParams.append('lens_explore_path', step.nodeKey)")
	assert.Contains(t, rendered, "url.searchParams.append('lens_explore_point', step.pointKey || '')")
}

func TestDashboardScripts_MetricExplorerExportTracksActiveViewAndRestoresRoot(t *testing.T) {
	t.Parallel()

	rendered := renderDashboardScripts(t)

	assert.Contains(t, rendered, "const lensExploreSyncExport")
	assert.Contains(t, rendered, "button.dataset.lensExplorerRootHref = button.getAttribute('href')")
	assert.Contains(t, rendered, "button.setAttribute('href', button.dataset.lensExplorerRootHref)")
	assert.Contains(t, rendered, "url.searchParams.set('lens_explore_export', 'current_view')")
	assert.Contains(t, rendered, "url.searchParams.set('lens_explorer', host.__lensExplorerCfg.id)")
	assert.Contains(t, rendered, "url.searchParams.set('lens_explore_branch', state.branchKey)")
	assert.Contains(t, rendered, "url.searchParams.set('lens_explore_perspective', state.perspectiveKey)")
	assert.Contains(t, rendered, "url.searchParams.append('lens_explore_path', step.nodeKey)")
	assert.Contains(t, rendered, "url.searchParams.append('lens_explore_point', step.pointKey || '')")
	assert.Contains(t, rendered, "url.searchParams.set('lens_explore_node', steps[steps.length - 1].nodeKey)")
	assert.Contains(t, rendered, "lensExploreSyncExport(host)")
	assert.Contains(t, rendered, "const card = host.closest('.lens-card')")
	assert.Contains(t, rendered, "card.classList.toggle('lens-explorer--fullscreen', active)")
}

func TestDashboardScripts_DrillTreeKeepsTabbedChartStateIndependent(t *testing.T) {
	t.Parallel()

	rendered := renderDashboardScripts(t)

	assert.Contains(t, rendered, "lensDrillTreeStateKey")
	assert.Contains(t, rendered, "Object.create(null)")
	assert.Contains(t, rendered, "scope.__lensDrillTreeSharedState[key] = {")
	assert.Contains(t, rendered, "scope.__lensDrillTreeSharedState[key] : null")
}

func TestDashboardScripts_DrillTreeClearsApexSelectionBetweenLevels(t *testing.T) {
	t.Parallel()

	rendered := renderDashboardScripts(t)

	// Apex replays the clicked pie slice's explode transform on redraw. A drill
	// level owns a different series, so that selection must never cross levels.
	assert.Contains(t, rendered, "chartContext.w.globals.selectedDataPoints = []")
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
	assert.Contains(t, rendered, "let view = branch.view || null")
	assert.Contains(t, rendered, "if (node.view)")
	assert.Contains(t, rendered, "legend: drilledLegend")
	assert.Contains(t, rendered, "plotOptions: { pie: drilledPie }")
	assert.NotContains(t, rendered, "lens-drill-tree__shortcut")
	assert.Contains(t, rendered, "const currentItem = (state.currentItems || [])[currentIndex]")
	assert.NotContains(t, rendered, "data-lens-drill-tree-action")
	assert.Contains(t, rendered, "event.key !== 'Escape'")
	assert.Contains(t, rendered, "event.key === 'Enter' || event.key === ' '")
	assert.Contains(t, rendered, "event.key === 'ArrowRight'")
	assert.Contains(t, rendered, "const lensDrillTreeActionableIndexes")
	assert.Contains(t, rendered, "actionableIndexes[0] === index")
	assert.Contains(t, rendered, "const position = liveActionableIndexes.indexOf(currentIndex)")
	assert.Contains(t, rendered, "chartLabels.some(function(label, index)")
	assert.NotContains(t, rendered, "chartLabels.join(")
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
	assert.Contains(t, rendered, "setTimeout(finish, 220)")
}

func TestDashboardScripts_DrillControlsUseReusableStyles(t *testing.T) {
	t.Parallel()

	rendered := renderDashboardScripts(t)

	assert.Contains(t, rendered, ".lens-drill-back,")
	// Desktop controls are compact; mobile restores 44px touch targets via an
	// expanded ::after hit area inside the max-width media block.
	assert.Contains(t, rendered, "inset: -8px")
	assert.Contains(t, rendered, "btn.className = 'lens-drill-back'")
	assert.NotContains(t, rendered, "btn.style.position = 'absolute'")
}

func TestDashboardScripts_DrillTreeChromeSurvivesApexRerenders(t *testing.T) {
	t.Parallel()

	rendered := renderDashboardScripts(t)

	// Apex re-renders wipe the chart element's children; the sync hook and the
	// enhance-side re-establish keep the toolbar and SR nodes alive mid-drill.
	assert.Contains(t, rendered, "window.__lensDrillTreeSync")
	assert.Contains(t, rendered, "lensDrillTreeEnsureA11y(container, cfg)")
	assert.Contains(t, rendered, "data-lens-drill-nav-active")
}

func TestDashboardScripts_DrillTreeNavUsesIconTemplates(t *testing.T) {
	t.Parallel()

	rendered := renderDashboardScripts(t)

	// The runtime-built toolbar clones server-rendered Phosphor glyphs instead
	// of unicode arrows.
	assert.Contains(t, rendered, "data-lens-drill-icons")
	assert.Contains(t, rendered, `data-lens-icon="back"`)
	assert.Contains(t, rendered, `data-lens-icon="home"`)
	assert.Contains(t, rendered, `data-lens-icon="sep"`)
	// The DrillTree toolbar must not fall back to text glyphs (the legacy bar
	// DrillHierarchy keeps its own back-button path with a text fallback).
	assert.NotContains(t, rendered, "back.textContent = '← '")
	assert.NotContains(t, rendered, "home.textContent = '⌂'")
	assert.Contains(t, rendered, "lensDrillTreeControlIcon(back, 'back'")
	assert.Contains(t, rendered, "lensDrillTreeControlIcon(home, 'home'")
}
