package builder

import (
	"fmt"
	"github.com/iota-uz/iota-sdk/pkg/lens"
	"time"
)

// DashboardBuilder provides a fluent API for building dashboards
type DashboardBuilder interface {
	// ID sets the dashboard ID
	ID(id string) DashboardBuilder

	// Title sets the dashboard title
	Title(title string) DashboardBuilder

	// Description sets the dashboard description
	Description(desc string) DashboardBuilder

	// Grid configures the grid layout
	Grid(columns int, rowHeight int) DashboardBuilder

	// RefreshRate sets the dashboard refresh rate
	RefreshRate(rate time.Duration) DashboardBuilder

	// Variable adds a dashboard variable
	Variable(name string, value interface{}) DashboardBuilder

	// Panel adds a panel to the dashboard
	Panel(panel lens.PanelConfig) DashboardBuilder

	// Build creates the dashboard configuration
	Build() lens.DashboardConfig
}

// PanelBuilder provides a fluent API for building panels
type PanelBuilder interface {
	// ID sets the panel ID
	ID(id string) PanelBuilder

	// Title sets the panel title
	Title(title string) PanelBuilder

	// Type sets the chart type
	Type(chartType lens.ChartType) PanelBuilder

	// Position sets the panel position in the grid
	Position(x, y int) PanelBuilder

	// Size sets the panel dimensions
	Size(width, height int) PanelBuilder

	// DataSource sets the data source for the panel
	DataSource(dsID string) PanelBuilder

	// Query sets the query for the panel
	Query(query string) PanelBuilder

	// RefreshRate sets the panel refresh rate
	RefreshRate(rate time.Duration) PanelBuilder

	// Option sets a custom option for the panel
	Option(key string, value interface{}) PanelBuilder

	// OnClick sets a click event handler for the panel
	OnClick(action lens.ActionConfig) PanelBuilder

	// OnDataPointClick sets a data point click event handler
	OnDataPointClick(action lens.ActionConfig) PanelBuilder

	// OnLegendClick sets a legend click event handler
	OnLegendClick(action lens.ActionConfig) PanelBuilder

	// OnMarkerClick sets a marker click event handler
	OnMarkerClick(action lens.ActionConfig) PanelBuilder

	// OnXAxisLabelClick sets an X-axis label click event handler
	OnXAxisLabelClick(action lens.ActionConfig) PanelBuilder

	// OnNavigate creates a navigation action for click events
	OnNavigate(url string, target ...string) PanelBuilder

	// OnDrillDown creates a drill-down action for click events
	OnDrillDown(filters map[string]string, dashboard ...string) PanelBuilder

	// OnModal creates a modal action for click events
	OnModal(title, content string, url ...string) PanelBuilder

	// OnCustom creates a custom JavaScript action for click events
	OnCustom(function string, variables ...map[string]string) PanelBuilder

	// Build creates the panel configuration
	Build() lens.PanelConfig
}

// dashboardBuilder is the default implementation
type dashboardBuilder struct {
	config lens.DashboardConfig
}

// NewDashboard creates a new dashboard builder
func NewDashboard() DashboardBuilder {
	return &dashboardBuilder{
		config: lens.DashboardConfig{
			Grid: lens.GridConfig{
				Columns:   12,
				RowHeight: 120,
			},
			Variables: []lens.Variable{},
			Panels:    []lens.PanelConfig{},
		},
	}
}

// ID sets the dashboard ID
func (db *dashboardBuilder) ID(id string) DashboardBuilder {
	db.config.ID = id
	return db
}

// Title sets the dashboard title (using Name field)
func (db *dashboardBuilder) Title(title string) DashboardBuilder {
	db.config.Name = title
	return db
}

// Description sets the dashboard description
func (db *dashboardBuilder) Description(desc string) DashboardBuilder {
	db.config.Description = desc
	return db
}

// Grid configures the grid layout
func (db *dashboardBuilder) Grid(columns int, rowHeight int) DashboardBuilder {
	db.config.Grid.Columns = columns
	db.config.Grid.RowHeight = rowHeight
	return db
}

// RefreshRate sets the dashboard refresh rate (stored in a variable)
func (db *dashboardBuilder) RefreshRate(rate time.Duration) DashboardBuilder {
	// Store refresh rate as a variable since DashboardConfig doesn't have this field
	db.config.Variables = append(db.config.Variables, lens.Variable{
		Name:    "refreshRate",
		Type:    "duration",
		Default: rate.String(),
		Value:   rate.String(),
	})
	return db
}

// Variable adds a dashboard variable
func (db *dashboardBuilder) Variable(name string, value interface{}) DashboardBuilder {
	db.config.Variables = append(db.config.Variables, lens.Variable{
		Name:    name,
		Type:    "string", // Default type
		Default: value,
		Value:   value,
	})
	return db
}

// Panel adds a panel to the dashboard
func (db *dashboardBuilder) Panel(panel lens.PanelConfig) DashboardBuilder {
	db.config.Panels = append(db.config.Panels, panel)
	return db
}

// Build creates the dashboard configuration
func (db *dashboardBuilder) Build() lens.DashboardConfig {
	return db.config
}

// panelBuilder is the default implementation
type panelBuilder struct {
	config lens.PanelConfig
}

// NewPanel creates a new panel builder
func NewPanel() PanelBuilder {
	return &panelBuilder{
		config: lens.PanelConfig{
			Type: lens.ChartTypeLine, // Default to line chart
			Position: lens.GridPosition{
				X: 0,
				Y: 0,
			},
			Dimensions: lens.GridDimensions{
				Width:  6,
				Height: 4,
			},
			Options: make(map[string]interface{}),
		},
	}
}

// ID sets the panel ID
func (pb *panelBuilder) ID(id string) PanelBuilder {
	pb.config.ID = id
	return pb
}

// Title sets the panel title
func (pb *panelBuilder) Title(title string) PanelBuilder {
	pb.config.Title = title
	return pb
}

// Type sets the chart type
func (pb *panelBuilder) Type(chartType lens.ChartType) PanelBuilder {
	pb.config.Type = chartType
	return pb
}

// Position sets the panel position in the grid
func (pb *panelBuilder) Position(x, y int) PanelBuilder {
	pb.config.Position.X = x
	pb.config.Position.Y = y
	return pb
}

// Size sets the panel dimensions
func (pb *panelBuilder) Size(width, height int) PanelBuilder {
	pb.config.Dimensions.Width = width
	pb.config.Dimensions.Height = height
	return pb
}

// DataSource sets the data source for the panel
func (pb *panelBuilder) DataSource(dsID string) PanelBuilder {
	pb.config.DataSource = lens.DataSourceConfig{
		Type: "default",
		Ref:  dsID,
	}
	return pb
}

// Query sets the query for the panel
func (pb *panelBuilder) Query(query string) PanelBuilder {
	pb.config.Query = query
	return pb
}

// RefreshRate sets the panel refresh rate (stored in options)
func (pb *panelBuilder) RefreshRate(rate time.Duration) PanelBuilder {
	// Store refresh rate in options since PanelConfig doesn't have this field
	pb.config.Options["refreshRate"] = rate.String()
	return pb
}

// Option sets a custom option for the panel
func (pb *panelBuilder) Option(key string, value interface{}) PanelBuilder {
	pb.config.Options[key] = value
	return pb
}

// OnClick sets a click event handler for the panel
func (pb *panelBuilder) OnClick(action lens.ActionConfig) PanelBuilder {
	if pb.config.Events == nil {
		pb.config.Events = &lens.PanelEvents{}
	}
	pb.config.Events.Click = &lens.ClickEvent{Action: action}
	return pb
}

// OnDataPointClick sets a data point click event handler
func (pb *panelBuilder) OnDataPointClick(action lens.ActionConfig) PanelBuilder {
	if pb.config.Events == nil {
		pb.config.Events = &lens.PanelEvents{}
	}
	pb.config.Events.DataPoint = &lens.DataPointEvent{Action: action}
	return pb
}

// OnLegendClick sets a legend click event handler
func (pb *panelBuilder) OnLegendClick(action lens.ActionConfig) PanelBuilder {
	if pb.config.Events == nil {
		pb.config.Events = &lens.PanelEvents{}
	}
	pb.config.Events.Legend = &lens.LegendEvent{Action: action}
	return pb
}

// OnMarkerClick sets a marker click event handler
func (pb *panelBuilder) OnMarkerClick(action lens.ActionConfig) PanelBuilder {
	if pb.config.Events == nil {
		pb.config.Events = &lens.PanelEvents{}
	}
	pb.config.Events.Marker = &lens.MarkerEvent{Action: action}
	return pb
}

// OnXAxisLabelClick sets an X-axis label click event handler
func (pb *panelBuilder) OnXAxisLabelClick(action lens.ActionConfig) PanelBuilder {
	if pb.config.Events == nil {
		pb.config.Events = &lens.PanelEvents{}
	}
	pb.config.Events.XAxisLabel = &lens.XAxisLabelEvent{Action: action}
	return pb
}

// OnNavigate creates a navigation action for click events
func (pb *panelBuilder) OnNavigate(url string, target ...string) PanelBuilder {
	actionTarget := "_self"
	if len(target) > 0 {
		actionTarget = target[0]
	}

	action := lens.ActionConfig{
		Type: lens.ActionTypeNavigation,
		Navigation: &lens.NavigationAction{
			URL:       url,
			Target:    actionTarget,
			Variables: make(map[string]string),
		},
	}

	return pb.OnClick(action)
}

// OnDrillDown creates a drill-down action for click events
func (pb *panelBuilder) OnDrillDown(filters map[string]string, dashboard ...string) PanelBuilder {
	dashboardName := ""
	if len(dashboard) > 0 {
		dashboardName = dashboard[0]
	}

	action := lens.ActionConfig{
		Type: lens.ActionTypeDrillDown,
		DrillDown: &lens.DrillDownAction{
			Dashboard: dashboardName,
			Filters:   filters,
			Variables: make(map[string]string),
		},
	}

	return pb.OnClick(action)
}

// OnModal creates a modal action for click events
func (pb *panelBuilder) OnModal(title, content string, url ...string) PanelBuilder {
	modalURL := ""
	if len(url) > 0 {
		modalURL = url[0]
	}

	action := lens.ActionConfig{
		Type: lens.ActionTypeModal,
		Modal: &lens.ModalAction{
			Title:     title,
			Content:   content,
			URL:       modalURL,
			Variables: make(map[string]string),
		},
	}

	return pb.OnClick(action)
}

// OnCustom creates a custom JavaScript action for click events
func (pb *panelBuilder) OnCustom(function string, variables ...map[string]string) PanelBuilder {
	customVars := make(map[string]string)
	if len(variables) > 0 {
		customVars = variables[0]
	}

	action := lens.ActionConfig{
		Type: lens.ActionTypeCustom,
		Custom: &lens.CustomAction{
			Function:  function,
			Variables: customVars,
		},
	}

	return pb.OnClick(action)
}

// Build creates the panel configuration
func (pb *panelBuilder) Build() lens.PanelConfig {
	return pb.config
}

// Convenience functions for common chart types

// LineChart creates a line chart panel builder
func LineChart() PanelBuilder {
	return NewPanel().Type(lens.ChartTypeLine)
}

// BarChart creates a bar chart panel builder
func BarChart() PanelBuilder {
	return NewPanel().Type(lens.ChartTypeBar)
}

// StackedBarChart creates a stacked bar chart panel builder
func StackedBarChart() PanelBuilder {
	return NewPanel().Type(lens.ChartTypeStackedBar)
}

// PieChart creates a pie chart panel builder
func PieChart() PanelBuilder {
	return NewPanel().Type(lens.ChartTypePie)
}

// AreaChart creates an area chart panel builder
func AreaChart() PanelBuilder {
	return NewPanel().Type(lens.ChartTypeArea)
}

// ColumnChart creates a column chart panel builder
func ColumnChart() PanelBuilder {
	return NewPanel().Type(lens.ChartTypeColumn)
}

// GaugeChart creates a gauge chart panel builder
func GaugeChart() PanelBuilder {
	return NewPanel().Type(lens.ChartTypeGauge)
}

// TableChart creates a table chart panel builder
func TableChart() PanelBuilder {
	return NewPanel().Type(lens.ChartTypeTable)
}

// MetricCard creates a metric card panel builder
func MetricCard() PanelBuilder {
	return NewPanel().Type(lens.ChartTypeMetric)
}

// Helper functions for common panel configurations

// QuickPanel creates a panel with basic configuration
func QuickPanel(id, title string, chartType lens.ChartType, x, y, width, height int) lens.PanelConfig {
	return NewPanel().
		ID(id).
		Title(title).
		Type(chartType).
		Position(x, y).
		Size(width, height).
		Build()
}

// FullWidthPanel creates a panel that spans the full width
func FullWidthPanel(id, title string, chartType lens.ChartType, y, height int) lens.PanelConfig {
	return NewPanel().
		ID(id).
		Title(title).
		Type(chartType).
		Position(0, y).
		Size(12, height).
		Build()
}

// HalfWidthPanel creates a panel that spans half the width
func HalfWidthPanel(id, title string, chartType lens.ChartType, x, y, height int) lens.PanelConfig {
	return NewPanel().
		ID(id).
		Title(title).
		Type(chartType).
		Position(x, y).
		Size(6, height).
		Build()
}

// QuarterWidthPanel creates a panel that spans a quarter of the width
func QuarterWidthPanel(id, title string, chartType lens.ChartType, x, y, height int) lens.PanelConfig {
	return NewPanel().
		ID(id).
		Title(title).
		Type(chartType).
		Position(x, y).
		Size(3, height).
		Build()
}

// Example usage helper that demonstrates the builder pattern
func ExampleDashboard() lens.DashboardConfig {
	return NewDashboard().
		ID("example-dashboard").
		Title("Example Dashboard").
		Description("A sample dashboard created with the builder pattern").
		Grid(12, 120).
		RefreshRate(30*time.Second).
		Variable("timeRange", "1h").
		Variable("environment", "production").
		Panel(
			LineChart().
				ID("cpu-usage").
				Title("CPU Usage").
				Position(0, 0).
				Size(6, 4).
				DataSource("prometheus").
				Query("cpu_usage_percent").
				RefreshRate(10*time.Second).
				Option("yAxis", map[string]interface{}{
					"min": 0,
					"max": 100,
				}).
				Build(),
		).
		Panel(
			BarChart().
				ID("memory-usage").
				Title("Memory Usage").
				Position(6, 0).
				Size(6, 4).
				DataSource("prometheus").
				Query("memory_usage_bytes").
				Build(),
		).
		Panel(
			TableChart().
				ID("server-list").
				Title("Server Status").
				Position(0, 4).
				Size(12, 6).
				DataSource("database").
				Query("SELECT * FROM servers WHERE status = 'active'").
				Build(),
		).
		Build()
}

// ValidatePanel validates a panel configuration
func ValidatePanel(panel lens.PanelConfig) error {
	if panel.ID == "" {
		return fmt.Errorf("panel ID is required")
	}

	if panel.Title == "" {
		return fmt.Errorf("panel title is required")
	}

	if panel.Dimensions.Width <= 0 || panel.Dimensions.Height <= 0 {
		return fmt.Errorf("panel dimensions must be positive")
	}

	if panel.Position.X < 0 || panel.Position.Y < 0 {
		return fmt.Errorf("panel position cannot be negative")
	}

	return nil
}

// ValidateDashboard validates a dashboard configuration
func ValidateDashboard(dashboard lens.DashboardConfig) error {
	if dashboard.ID == "" {
		return fmt.Errorf("dashboard ID is required")
	}

	if dashboard.Name == "" {
		return fmt.Errorf("dashboard name is required")
	}

	if dashboard.Grid.Columns <= 0 {
		return fmt.Errorf("grid columns must be positive")
	}

	if dashboard.Grid.RowHeight <= 0 {
		return fmt.Errorf("grid row height must be positive")
	}

	// Validate each panel
	for i, panel := range dashboard.Panels {
		if err := ValidatePanel(panel); err != nil {
			return fmt.Errorf("panel %d validation failed: %w", i, err)
		}
	}

	return nil
}
