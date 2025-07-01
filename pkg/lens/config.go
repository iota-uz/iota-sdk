package lens

import (
	"encoding/json"
)

// DashboardConfig represents the dashboard configuration
type DashboardConfig struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Version     string        `json:"version"`
	Grid        GridConfig    `json:"grid"`
	Panels      []PanelConfig `json:"panels"`
	Variables   []Variable    `json:"variables"`
}

// MarshalJSON serializes a DashboardConfig to JSON bytes
func (d *DashboardConfig) MarshalJSON() ([]byte, error) {
	type dashboardConfigAlias DashboardConfig
	return json.Marshal((*dashboardConfigAlias)(d))
}

// UnmarshalJSON deserializes JSON bytes into a DashboardConfig
func (d *DashboardConfig) UnmarshalJSON(data []byte) error {
	type dashboardConfigAlias DashboardConfig
	return json.Unmarshal(data, (*dashboardConfigAlias)(d))
}

// ToJSON converts a DashboardConfig to JSON string
func (d *DashboardConfig) ToJSON() (string, error) {
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// GridConfig represents the grid system configuration
type GridConfig struct {
	Columns     int            `json:"columns"`
	RowHeight   int            `json:"rowHeight"`
	Breakpoints map[string]int `json:"breakpoints"`
}

// PanelConfig represents a single panel configuration
type PanelConfig struct {
	ID         string           `json:"id"`
	Title      string           `json:"title"`
	Type       ChartType        `json:"type"`
	Position   GridPosition     `json:"position"`
	Dimensions GridDimensions   `json:"dimensions"`
	DataSource DataSourceConfig `json:"dataSource"`
	Query      string           `json:"query"`
	Options    map[string]any   `json:"options"`
	Events     *PanelEvents     `json:"events,omitempty"`
}

// DataSourceConfig represents data source configuration
type DataSourceConfig struct {
	Type string `json:"type"`
	Ref  string `json:"ref"`
}

// PanelEvents represents event configuration for a panel
type PanelEvents struct {
	Click      *ClickEvent      `json:"click,omitempty"`
	DataPoint  *DataPointEvent  `json:"dataPoint,omitempty"`
	Legend     *LegendEvent     `json:"legend,omitempty"`
	Marker     *MarkerEvent     `json:"marker,omitempty"`
	XAxisLabel *XAxisLabelEvent `json:"xAxisLabel,omitempty"`
}

// ClickEvent represents a general chart click event configuration
type ClickEvent struct {
	Action ActionConfig `json:"action"`
}

// DataPointEvent represents a data point click event configuration
type DataPointEvent struct {
	Action ActionConfig `json:"action"`
}

// LegendEvent represents a legend click event configuration
type LegendEvent struct {
	Action ActionConfig `json:"action"`
}

// MarkerEvent represents a marker click event configuration
type MarkerEvent struct {
	Action ActionConfig `json:"action"`
}

// XAxisLabelEvent represents an X-axis label click event configuration
type XAxisLabelEvent struct {
	Action ActionConfig `json:"action"`
}

// ActionConfig represents the action to take when an event occurs
type ActionConfig struct {
	Type       ActionType        `json:"type"`
	Navigation *NavigationAction `json:"navigation,omitempty"`
	DrillDown  *DrillDownAction  `json:"drillDown,omitempty"`
	Modal      *ModalAction      `json:"modal,omitempty"`
	Custom     *CustomAction     `json:"custom,omitempty"`
}

// ActionType represents the type of action to perform
type ActionType string

const (
	ActionTypeNavigation ActionType = "navigation"
	ActionTypeDrillDown  ActionType = "drillDown"
	ActionTypeModal      ActionType = "modal"
	ActionTypeCustom     ActionType = "custom"
)

// NavigationAction represents a navigation action (redirect to URL)
type NavigationAction struct {
	URL       string            `json:"url"`
	Target    string            `json:"target,omitempty"` // "_blank", "_self", etc.
	Variables map[string]string `json:"variables,omitempty"`
}

// DrillDownAction represents a drill-down action (filter current dashboard)
type DrillDownAction struct {
	Dashboard string            `json:"dashboard,omitempty"`
	Filters   map[string]string `json:"filters"`
	Variables map[string]string `json:"variables,omitempty"`
}

// ModalAction represents a modal popup action
type ModalAction struct {
	Title     string            `json:"title"`
	Content   string            `json:"content,omitempty"`
	URL       string            `json:"url,omitempty"`
	Variables map[string]string `json:"variables,omitempty"`
}

// CustomAction represents a custom JavaScript action
type CustomAction struct {
	Function  string            `json:"function"`
	Variables map[string]string `json:"variables,omitempty"`
}
