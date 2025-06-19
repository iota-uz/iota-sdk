package ui

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/a-h/templ"
	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/evaluation"
	"github.com/iota-uz/iota-sdk/pkg/lens/executor"
)

// Renderer renders lens dashboards to UI components
type Renderer interface {
	// Render evaluated dashboards (from evaluation package)
	RenderDashboard(dashboard *evaluation.EvaluatedDashboard) templ.Component
	RenderPanel(panel *evaluation.EvaluatedPanel) templ.Component
	RenderGrid(layout *evaluation.Layout) templ.Component

	// Render dashboards with executor results (simpler interface)
	RenderDashboardWithData(config lens.DashboardConfig, results *executor.DashboardResult) templ.Component
	RenderPanelWithData(config lens.PanelConfig, result *executor.ExecutionResult) templ.Component

	// Error handling
	RenderError(err error) templ.Component
	RenderPanelError(config lens.PanelConfig, message string) templ.Component
}

// Config contains UI rendering configuration
type Config struct {
	GridClasses     GridClassConfig
	RefreshStrategy RefreshStrategy
}

// PanelCSS represents panel-specific CSS (UI concern)
type PanelCSS struct {
	Classes []string
	Styles  map[string]string
}

// LayoutWithCSS represents a layout with CSS information for UI rendering
type LayoutWithCSS struct {
	lens.Layout // Embed the core layout
	CSS         LayoutCSS
}

// PanelLayoutWithCSS represents a panel layout with CSS information for UI rendering
type PanelLayoutWithCSS struct {
	lens.PanelLayout // Embed the core panel layout
	CSS              PanelCSS
}

// LayoutCSS represents layout-level CSS
type LayoutCSS struct {
	ContainerClasses []string
	ContainerStyles  map[string]string
}

// GridClassConfig contains grid CSS class configuration
type GridClassConfig struct {
	ContainerClass string
	PanelClass     string
}

// RefreshStrategy defines how panels refresh
type RefreshStrategy string

const (
	RefreshStrategyPolling RefreshStrategy = "polling"
	RefreshStrategyHTMX    RefreshStrategy = "htmx"
)

// renderer is the default implementation
type renderer struct {
	config Config
}

// NewRenderer creates a new renderer
func NewRenderer(config Config) Renderer {
	return &renderer{
		config: config,
	}
}

// RenderDashboard renders a complete dashboard
func (r *renderer) RenderDashboard(dashboard *evaluation.EvaluatedDashboard) templ.Component {
	return Dashboard(dashboard)
}

// RenderPanel renders a single panel
func (r *renderer) RenderPanel(panel *evaluation.EvaluatedPanel) templ.Component {
	return Panel(panel)
}

// RenderGrid renders the grid layout
func (r *renderer) RenderGrid(layout *evaluation.Layout) templ.Component {
	return Grid(layout)
}

// RenderError renders an error
func (r *renderer) RenderError(err error) templ.Component {
	return ErrorContent(err.Error())
}

// RenderDashboardWithData renders a dashboard using executor results
func (r *renderer) RenderDashboardWithData(config lens.DashboardConfig, results *executor.DashboardResult) templ.Component {
	return DashboardWithData(config, results)
}

// RenderPanelWithData renders a panel using executor results
func (r *renderer) RenderPanelWithData(config lens.PanelConfig, result *executor.ExecutionResult) templ.Component {
	return PanelWithData(config, result)
}

// RenderPanelError renders a panel error
func (r *renderer) RenderPanelError(config lens.PanelConfig, message string) templ.Component {
	return PanelError(config, message)
}

// DefaultConfig returns a default UI configuration
func DefaultConfig() Config {
	return Config{
		GridClasses: GridClassConfig{
			ContainerClass: "dashboard-grid",
			PanelClass:     "dashboard-panel",
		},
		RefreshStrategy: RefreshStrategyHTMX,
	}
}

// CSS template for dashboard styles
const cssTemplate = `
.dashboard-wrapper {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  padding: 1rem;
}

.dashboard-header {
  padding: 1rem 0;
  border-bottom: 1px solid #e5e7eb;
}

.dashboard-title {
  margin: 0 0 0.5rem 0;
  font-size: 1.5rem;
  font-weight: 700;
  color: #1f2937;
}

.dashboard-description {
  margin: 0;
  color: #6b7280;
  font-size: 0.875rem;
}

.dashboard-grid, .dashboard-panels {
  display: grid;
  grid-template-columns: repeat({{.Columns}}, 1fr);
  grid-auto-rows: {{.RowHeight}}px;
  gap: 1rem;
}

.dashboard-panel {
  background: white;
  border: 1px solid #e5e7eb;
  border-radius: 0.5rem;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.panel-header {
  padding: 0.75rem 1rem;
  border-bottom: 1px solid #e5e7eb;
  background: #f9fafb;
}

.panel-title {
  margin: 0;
  font-size: 1rem;
  font-weight: 600;
  color: #374151;
}

.panel-content {
  flex: 1;
  padding: 1rem;
  display: flex;
  flex-direction: column;
}

.chart-container {
  width: 100%;
  height: 100%;
  flex: 1;
}

.chart-error {
  color: #dc2626;
  text-align: center;
  padding: 2rem;
}

.table-container {
  overflow: auto;
  flex: 1;
}

.dashboard-table {
  width: 100%;
  border-collapse: collapse;
}

.dashboard-table th, .dashboard-table td {
  padding: 0.5rem;
  text-align: left;
  border-bottom: 1px solid #e5e7eb;
}

.dashboard-table th {
  background: #f9fafb;
  font-weight: 600;
  color: #374151;
}

.metric-container {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
}

.metric-card {
  background: white;
  border: 1px solid #e5e7eb;
  border-radius: 0.5rem;
  padding: 1.5rem;
  width: 100%;
  height: 100%;
  min-height: 140px;
  display: flex;
  flex-direction: column;
  justify-content: center;
  transition: all 0.2s ease;
  position: relative;
}

.metric-card:hover {
  box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1);
}

.metric-card--colored {
  border-left: 4px solid var(--metric-color, #3b82f6);
}

.metric-card--has-trend .metric-card__value {
  margin-bottom: 0.75rem;
}

.metric-card__header {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-bottom: 0.75rem;
}

.metric-card__icon {
  font-size: 1.25rem;
  opacity: 0.8;
}

.metric-card__label {
  font-size: 0.875rem;
  font-weight: 500;
  color: #6b7280;
  text-transform: uppercase;
  letter-spacing: 0.025em;
}

.metric-card__value {
  font-size: 2rem;
  font-weight: 700;
  color: #1f2937;
  line-height: 1.2;
  margin-bottom: 0.5rem;
}

.metric-card__trend {
  display: flex;
  align-items: center;
  gap: 0.25rem;
  font-size: 0.75rem;
  font-weight: 600;
}

.metric-card__trend--positive {
  color: #059669;
}

.metric-card__trend--negative {
  color: #dc2626;
}

.metric-card__trend-icon {
  font-size: 0.875rem;
}

.metric-card__trend-value {
  margin-left: 0.125rem;
}

.metric-error {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: #6b7280;
  font-size: 0.875rem;
  text-align: center;
  padding: 1rem;
}

@media (max-width: 768px) {
  .dashboard-grid {
    grid-template-columns: 1fr;
    gap: 0.5rem;
    padding: 0.5rem;
  }
  .dashboard-panel {
    grid-column: 1 / -1;
    min-height: 200px;
  }
  .metric-card {
    min-height: 120px;
    padding: 1rem;
  }
  .metric-card__value {
    font-size: 1.75rem;
  }
}
`

// GenerateCSS generates the CSS for the dashboard using text templating, wrapped in style tags
func GenerateCSS(gridConfig lens.GridConfig) string {
	tmpl, err := template.New("css").Parse(cssTemplate)
	if err != nil {
		return "<style>/* CSS generation error */</style>"
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, gridConfig)
	if err != nil {
		return "<style>/* CSS template execution error */</style>"
	}

	return "<style>\n" + buf.String() + "\n</style>"
}

// AddCSSToLayout enriches a core layout with CSS information for UI rendering
func AddCSSToLayout(coreLayout lens.Layout) LayoutWithCSS {
	layoutWithCSS := LayoutWithCSS{
		Layout: coreLayout,
		CSS: LayoutCSS{
			ContainerClasses: []string{"dashboard-grid", "grid-container"},
			ContainerStyles: map[string]string{
				"display":               "grid",
				"gap":                   "1rem",
				"padding":               "1rem",
				"grid-auto-rows":        fmt.Sprintf("%dpx", coreLayout.Grid.RowHeight),
				"grid-template-columns": fmt.Sprintf("repeat(%d, 1fr)", coreLayout.Grid.Columns),
			},
		},
	}

	return layoutWithCSS
}

// AddCSSToPanel enriches a core panel layout with CSS information for UI rendering
func AddCSSToPanel(corePanel lens.PanelLayout, panelType lens.ChartType) PanelLayoutWithCSS {
	// Calculate CSS grid area
	gridArea := fmt.Sprintf("%d / %d / %d / %d",
		corePanel.Position.Y+1,
		corePanel.Position.X+1,
		corePanel.Position.Y+corePanel.Dimensions.Height+1,
		corePanel.Position.X+corePanel.Dimensions.Width+1)

	panelWithCSS := PanelLayoutWithCSS{
		PanelLayout: corePanel,
		CSS: PanelCSS{
			Classes: []string{"dashboard-panel", fmt.Sprintf("panel-%s", panelType)},
			Styles: map[string]string{
				"grid-area":      gridArea,
				"display":        "flex",
				"flex-direction": "column",
				"border":         "1px solid #e5e7eb",
				"border-radius":  "0.5rem",
				"background":     "white",
				"overflow":       "hidden",
			},
		},
	}

	return panelWithCSS
}
