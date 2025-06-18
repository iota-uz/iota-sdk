package lens

import "time"

// ChartType represents the type of visualization
type ChartType string

const (
	ChartTypeLine   ChartType = "line"
	ChartTypeBar    ChartType = "bar"
	ChartTypePie    ChartType = "pie"
	ChartTypeArea   ChartType = "area"
	ChartTypeColumn ChartType = "column"
	ChartTypeGauge  ChartType = "gauge"
	ChartTypeTable  ChartType = "table"
)

// TimeRange represents a time period for data queries
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// GridPosition represents a panel's position in the grid
type GridPosition struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// GridDimensions represents a panel's size in the grid
type GridDimensions struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Variable represents a dashboard variable for templating
type Variable struct {
	Name    string      `json:"name"`
	Type    string      `json:"type"`
	Default any `json:"default"`
	Value   any `json:"value,omitempty"`
}

// DataResult represents query execution results
type DataResult struct {
	Columns []Column
	Rows    [][]any
	Error   error
}

// Column represents a data column
type Column struct {
	Name string
	Type string
}