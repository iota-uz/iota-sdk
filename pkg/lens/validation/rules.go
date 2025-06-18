package validation

import (
	"fmt"
	"github.com/iota-uz/iota-sdk/pkg/lens"
)

// ValidationRule represents a validation rule
type ValidationRule interface {
	Name() string
}

// DashboardValidationRule validates dashboard-level configuration
type DashboardValidationRule struct{}

func (r *DashboardValidationRule) Name() string {
	return "dashboard"
}

func (r *DashboardValidationRule) ValidateDashboard(config *lens.DashboardConfig) ValidationResult {
	result := ValidationResult{Valid: true}
	
	if config.ID == "" {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "id",
			Message: "dashboard ID is required",
		})
	}
	
	if config.Name == "" {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "name",
			Message: "dashboard name is required",
		})
	}
	
	if config.Version == "" {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "version",
			Message: "dashboard version is required",
		})
	}
	
	// Check for duplicate panel IDs
	panelIDs := make(map[string]bool)
	for _, panel := range config.Panels {
		if panelIDs[panel.ID] {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("panels[%s].id", panel.ID),
				Message: "duplicate panel ID",
			})
		}
		panelIDs[panel.ID] = true
	}
	
	return result
}

// PanelValidationRule validates panel configuration
type PanelValidationRule struct{}

func (r *PanelValidationRule) Name() string {
	return "panel"
}

func (r *PanelValidationRule) ValidatePanel(panel *lens.PanelConfig, grid lens.GridConfig) ValidationResult {
	result := ValidationResult{Valid: true}
	
	if panel.ID == "" {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "id",
			Message: "panel ID is required",
		})
	}
	
	if panel.Title == "" {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   fmt.Sprintf("panels[%s].title", panel.ID),
			Message: "panel title is required",
		})
	}
	
	// Validate chart type
	if !isValidChartType(panel.Type) {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   fmt.Sprintf("panels[%s].type", panel.ID),
			Message: "invalid chart type",
		})
	}
	
	// Validate position
	if panel.Position.X < 0 || panel.Position.Y < 0 {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   fmt.Sprintf("panels[%s].position", panel.ID),
			Message: "position coordinates must be non-negative",
		})
	}
	
	// Validate dimensions
	if panel.Dimensions.Width <= 0 || panel.Dimensions.Height <= 0 {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   fmt.Sprintf("panels[%s].dimensions", panel.ID),
			Message: "dimensions must be positive",
		})
	}
	
	// Validate panel fits within grid
	if panel.Position.X+panel.Dimensions.Width > grid.Columns {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   fmt.Sprintf("panels[%s].position", panel.ID),
			Message: "panel extends beyond grid columns",
		})
	}
	
	// Validate data source
	if panel.DataSource.Type == "" {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   fmt.Sprintf("panels[%s].dataSource.type", panel.ID),
			Message: "data source type is required",
		})
	}
	
	if panel.Query == "" {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   fmt.Sprintf("panels[%s].query", panel.ID),
			Message: "panel query is required",
		})
	}
	
	return result
}

// GridValidationRule validates grid layout configuration
type GridValidationRule struct{}

func (r *GridValidationRule) Name() string {
	return "grid"
}

func (r *GridValidationRule) ValidateGrid(panels []lens.PanelConfig, grid lens.GridConfig) ValidationResult {
	result := ValidationResult{Valid: true}
	
	if grid.Columns <= 0 {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "grid.columns",
			Message: "grid columns must be positive",
		})
	}
	
	if grid.RowHeight <= 0 {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "grid.rowHeight",
			Message: "grid row height must be positive",
		})
	}
	
	// Check for panel overlaps
	overlaps := detectPanelOverlaps(panels)
	for _, overlap := range overlaps {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "grid.layout",
			Message: fmt.Sprintf("panels %s and %s overlap", overlap.Panel1, overlap.Panel2),
		})
	}
	
	return result
}

// PanelOverlap represents overlapping panels
type PanelOverlap struct {
	Panel1 string
	Panel2 string
}

// detectPanelOverlaps detects overlapping panels in the grid
func detectPanelOverlaps(panels []lens.PanelConfig) []PanelOverlap {
	var overlaps []PanelOverlap
	
	for i, panel1 := range panels {
		for j, panel2 := range panels {
			if i >= j {
				continue
			}
			
			if panelsOverlap(panel1, panel2) {
				overlaps = append(overlaps, PanelOverlap{
					Panel1: panel1.ID,
					Panel2: panel2.ID,
				})
			}
		}
	}
	
	return overlaps
}

// panelsOverlap checks if two panels overlap
func panelsOverlap(panel1, panel2 lens.PanelConfig) bool {
	// Check if panels overlap using rectangle intersection logic
	x1_left := panel1.Position.X
	x1_right := panel1.Position.X + panel1.Dimensions.Width
	y1_top := panel1.Position.Y
	y1_bottom := panel1.Position.Y + panel1.Dimensions.Height
	
	x2_left := panel2.Position.X
	x2_right := panel2.Position.X + panel2.Dimensions.Width
	y2_top := panel2.Position.Y
	y2_bottom := panel2.Position.Y + panel2.Dimensions.Height
	
	// No overlap if one panel is completely to the left, right, above, or below the other
	if x1_right <= x2_left || x2_right <= x1_left || y1_bottom <= y2_top || y2_bottom <= y1_top {
		return false
	}
	
	return true
}

// isValidChartType checks if the chart type is valid
func isValidChartType(chartType lens.ChartType) bool {
	validTypes := []lens.ChartType{
		lens.ChartTypeLine,
		lens.ChartTypeBar,
		lens.ChartTypePie,
		lens.ChartTypeArea,
		lens.ChartTypeColumn,
		lens.ChartTypeGauge,
		lens.ChartTypeTable,
	}
	
	for _, validType := range validTypes {
		if chartType == validType {
			return true
		}
	}
	
	return false
}