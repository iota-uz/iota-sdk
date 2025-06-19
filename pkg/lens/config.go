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
}

// DataSourceConfig represents data source configuration
type DataSourceConfig struct {
	Type string `json:"type"`
	Ref  string `json:"ref"`
}
