package ui

import (
	"bytes"
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
.dashboard-grid {
  display: grid;
  grid-template-columns: repeat({{.Columns}}, 1fr);
  grid-auto-rows: {{.RowHeight}}px;
  gap: 1rem;
  padding: 1rem;
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
}
`

// GenerateCSS generates the CSS for the dashboard using text templating
func GenerateCSS(gridConfig lens.GridConfig) string {
	tmpl, err := template.New("css").Parse(cssTemplate)
	if err != nil {
		return ""
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, gridConfig)
	if err != nil {
		return ""
	}

	return buf.String()
}

