package ui

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/a-h/templ"
	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/evaluation"
	"github.com/iota-uz/iota-sdk/pkg/lens/executor"
	"github.com/iota-uz/iota-sdk/pkg/lens/layout"
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
	config           Config
	responsiveEngine layout.ResponsiveEngine
}

// NewRenderer creates a new renderer
func NewRenderer(config Config) Renderer {
	return &renderer{
		config:           config,
		responsiveEngine: layout.NewResponsiveEngine(),
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

// CSS template for dashboard styles with comprehensive responsive design
const cssTemplate = `
/* Base styles - Mobile first approach */
.dashboard-wrapper {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  padding: 0.75rem;
}

.dashboard-header {
  padding: 0.75rem 0;
  border-bottom: 1px solid #e5e7eb;
}

.dashboard-title {
  margin: 0 0 0.5rem 0;
  font-size: 1.25rem;
  font-weight: 700;
  color: #1f2937;
  line-height: 1.2;
}

.dashboard-description {
  margin: 0;
  color: #6b7280;
  font-size: 0.8125rem;
  line-height: 1.4;
}

.dashboard-grid, .dashboard-panels {
  display: grid;
  grid-template-columns: 1fr;
  grid-auto-rows: minmax(180px, auto);
  gap: 0.75rem;
}

.dashboard-panel {
  background: white;
  border: 1px solid #e5e7eb;
  border-radius: 0.5rem;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  min-height: 180px;
  /* Mobile: panels stack naturally in single column, no grid-area */
}

.dashboard-panel--metric {
  /* Remove panel wrapper styling for metrics */
  background: transparent;
  border: none;
  border-radius: 0;
  box-shadow: none;
  padding: 0;
}

.panel-header {
  padding: 0.75rem;
  border-bottom: 1px solid #e5e7eb;
  background: #f9fafb;
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 44px;
}

.panel-title {
  margin: 0;
  font-size: 0.875rem;
  font-weight: 600;
  color: #374151;
  line-height: 1.2;
}

.panel-actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-left: auto;
}

.panel-actions button {
  min-width: 44px;
  min-height: 44px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: none;
  background: transparent;
  border-radius: 0.375rem;
  transition: background-color 0.2s;
}

.panel-actions button:hover {
  background-color: rgba(0, 0, 0, 0.05);
}

.panel-expanded {
  position: fixed !important;
  top: 0 !important;
  left: 0 !important;
  width: 100vw !important;
  height: 100vh !important;
  z-index: 1000 !important;
  grid-area: unset !important;
  margin: 0 !important;
  border-radius: 0 !important;
}

.panel-expanded .panel-content {
  height: calc(100vh - 65px);
  overflow: auto;
}

.cache-indicator {
  background: #dbeafe;
  color: #1e40af;
  font-size: 0.6875rem;
  padding: 0.25rem 0.5rem;
  border-radius: 0.375rem;
  font-weight: 500;
}

.panel-content {
  flex: 1;
  padding: 0.75rem;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.chart-container {
  width: 100%;
  height: 100%;
  flex: 1;
  min-height: 120px;
}

.chart-error {
  color: #dc2626;
  text-align: center;
  padding: 1.5rem;
  font-size: 0.875rem;
}

.table-container {
  overflow: auto;
  flex: 1;
  -webkit-overflow-scrolling: touch;
}

.dashboard-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.8125rem;
}

.dashboard-table th, .dashboard-table td {
  padding: 0.375rem 0.5rem;
  text-align: left;
  border-bottom: 1px solid #e5e7eb;
  white-space: nowrap;
}

.dashboard-table th {
  background: #f9fafb;
  font-weight: 600;
  color: #374151;
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.metric-container {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0.25rem;
}

.metric-card {
  background: #ffffff;
  border: 1px solid #f1f5f9;
  border-radius: 0.75rem;
  padding: 1rem;
  width: 100%;
  height: 100%;
  min-height: 140px;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  position: relative;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.02), 0 1px 2px rgba(0, 0, 0, 0.04);
}

.metric-card:hover {
  box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.06), 0 2px 4px -1px rgba(0, 0, 0, 0.04);
  transform: translateY(-1px);
  border-color: #e2e8f0;
}

.metric-card--colored {
  border-left: 4px solid var(--metric-color, #3b82f6);
  background: linear-gradient(135deg, #ffffff 0%, #fafbfc 100%);
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
  font-size: 1.125rem;
  opacity: 0.9;
  width: 2rem;
  height: 2rem;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(59, 130, 246, 0.08);
  border-radius: 0.5rem;
  color: var(--metric-color, #3b82f6);
  flex-shrink: 0;
}

.metric-card__label {
  font-size: 0.75rem;
  font-weight: 600;
  color: #64748b;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  line-height: 1.2;
}

.metric-card__value {
  font-size: 1.5rem;
  font-weight: 800;
  color: #1e293b;
  line-height: 1.1;
  margin-bottom: 0.5rem;
  letter-spacing: -0.025em;
  word-break: break-word;
}

.metric-card__trend {
  display: flex;
  align-items: center;
  gap: 0.25rem;
  font-size: 0.6875rem;
  font-weight: 600;
  padding: 0.25rem 0.5rem;
  border-radius: 0.375rem;
  background: rgba(0, 0, 0, 0.02);
  width: fit-content;
}

.metric-card__trend--positive {
  color: #059669;
  background: rgba(5, 150, 105, 0.08);
}

.metric-card__trend--negative {
  color: #dc2626;
  background: rgba(220, 38, 38, 0.08);
}

.metric-card__trend-icon {
  font-size: 0.875rem;
  line-height: 1;
}

.metric-card__trend-value {
  margin-left: 0.125rem;
  font-weight: 700;
}

.metric-error {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: #6b7280;
  font-size: 0.8125rem;
  text-align: center;
  padding: 1rem;
}

.error-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  text-align: center;
  padding: 1rem;
  color: #6b7280;
}

.error-icon {
  font-size: 1.5rem;
}

.error-message {
  font-size: 0.8125rem;
  line-height: 1.4;
}

/* Small tablets and large phones - 576px and up */
@media (min-width: 576px) {
  .dashboard-wrapper {
    gap: 1rem;
    padding: 1rem;
  }
  
  .dashboard-title {
    font-size: 1.5rem;
  }
  
  .dashboard-description {
    font-size: 0.875rem;
  }
  
  .dashboard-grid, .dashboard-panels {
    grid-template-columns: repeat(2, 1fr);
    grid-auto-rows: minmax(160px, auto);
    gap: 1rem;
  }
  
  .panel-title {
    font-size: 1rem;
  }
  
  .panel-content {
    padding: 1rem;
  }
  
  .chart-container {
    min-height: 140px;
  }
  
  .metric-card {
    padding: 1.25rem;
    min-height: 150px;
  }
  
  .metric-card__header {
    gap: 0.625rem;
    margin-bottom: 1rem;
  }
  
  .metric-card__icon {
    width: 2.25rem;
    height: 2.25rem;
    font-size: 1.25rem;
  }
  
  .metric-card__label {
    font-size: 0.8125rem;
  }
  
  .metric-card__value {
    font-size: 1.75rem;
  }
  
  .cache-indicator {
    font-size: 0.75rem;
  }
}

/* Tablets - 768px and up */
@media (min-width: 768px) {
  .dashboard-grid, .dashboard-panels {
    grid-template-columns: repeat({{.Columns}}, 1fr);
    grid-auto-rows: {{.RowHeight}}px;
  }
  
  .dashboard-panel {
    min-height: {{.RowHeight}}px;
    /* Use CSS custom property for grid positioning on desktop */
    grid-area: var(--panel-grid-area, auto);
  }
  
  .chart-container {
    min-height: 200px;
  }
  
  .dashboard-table {
    font-size: 0.875rem;
  }
  
  .dashboard-table th, .dashboard-table td {
    padding: 0.5rem;
  }
  
  .dashboard-table th {
    font-size: 0.8125rem;
  }
}

/* Desktop - 992px and up */
@media (min-width: 992px) {
  .dashboard-wrapper {
    padding: 1.5rem;
  }
  
  .dashboard-header {
    padding: 1rem 0;
  }
  
  .dashboard-panel {
    /* Ensure grid positioning works on desktop */
    grid-area: var(--panel-grid-area, auto);
  }
  
  .panel-header {
    padding: 0.75rem 1rem;
  }
  
  .metric-card {
    padding: 1.5rem;
    min-height: 160px;
  }
  
  .metric-card__header {
    gap: 0.75rem;
    margin-bottom: 1.25rem;
  }
  
  .metric-card__icon {
    width: 2.5rem;
    height: 2.5rem;
    font-size: 1.5rem;
  }
  
  .metric-card__label {
    font-size: 0.875rem;
  }
  
  .metric-card__value {
    font-size: 2rem;
    margin-bottom: 0.75rem;
  }
  
  .metric-card__trend {
    font-size: 0.75rem;
    padding: 0.375rem 0.75rem;
    gap: 0.375rem;
  }
}

/* Large desktop - 1200px and up */
@media (min-width: 1200px) {
  .dashboard-wrapper {
    padding: 2rem;
  }
  
  .dashboard-panel {
    /* Ensure grid positioning works on large desktop */
    grid-area: var(--panel-grid-area, auto);
  }
  
  .metric-card {
    padding: 2rem;
  }
  
  .metric-card__value {
    font-size: 2.25rem;
  }
  
  .metric-card__trend {
    font-size: 0.8125rem;
  }
}

/* Landscape orientation adjustments for mobile */
@media (max-height: 500px) and (orientation: landscape) {
  .dashboard-grid, .dashboard-panels {
    grid-auto-rows: minmax(120px, auto);
  }
  
  .dashboard-panel {
    min-height: 120px;
  }
  
  .metric-card {
    min-height: 100px;
    padding: 0.75rem;
  }
  
  .metric-card__header {
    margin-bottom: 0.5rem;
  }
  
  .metric-card__value {
    font-size: 1.25rem;
    margin-bottom: 0.25rem;
  }
}

/* High DPI displays */
@media (-webkit-min-device-pixel-ratio: 2), (min-resolution: 2dppx) {
  .metric-card {
    border-width: 0.5px;
  }
}

/* Reduced motion preferences */
@media (prefers-reduced-motion: reduce) {
  .metric-card {
    transition: none;
  }
  
  .metric-card:hover {
    transform: none;
  }
  
  .panel-actions button {
    transition: none;
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
