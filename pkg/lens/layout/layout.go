package layout

import (
	"fmt"
	"github.com/iota-uz/iota-sdk/pkg/lens"
)

// Engine calculates panel layouts and handles grid positioning
type Engine interface {
	CalculateLayout(panels []lens.PanelConfig, grid lens.GridConfig) (*Layout, error)
	DetectOverlaps(panels []lens.PanelConfig) []OverlapError
	GetResponsiveLayout(layout *Layout, breakpoint Breakpoint) *Layout
	ValidateLayout(panels []lens.PanelConfig, grid lens.GridConfig) []ValidationError
}

// Layout represents the calculated dashboard layout
type Layout struct {
	Grid       lens.GridConfig
	Panels     []PanelLayout
	Breakpoint Breakpoint
	CSS        LayoutCSS
	Bounds     LayoutBounds
}

// PanelLayout represents a panel's calculated layout
type PanelLayout struct {
	PanelID    string
	Position   lens.GridPosition
	Dimensions lens.GridDimensions
	CSS        PanelCSS
	ZIndex     int
	Bounds     PanelBounds
}

// LayoutCSS represents layout-level CSS
type LayoutCSS struct {
	ContainerClasses []string
	ContainerStyles  map[string]string
	GridTemplate     GridTemplate
}

// PanelCSS represents panel-specific CSS
type PanelCSS struct {
	Classes       []string
	Styles        map[string]string
	GridArea      string
	ResponsiveCSS map[Breakpoint]ResponsiveCSS
}

// GridTemplate represents CSS Grid template configuration
type GridTemplate struct {
	Columns string
	Rows    string
	Areas   []string
}

// ResponsiveCSS represents responsive CSS for different breakpoints
type ResponsiveCSS struct {
	Classes []string
	Styles  map[string]string
}

// LayoutBounds represents the overall layout boundaries
type LayoutBounds struct {
	MaxX      int
	MaxY      int
	MinWidth  int
	MinHeight int
}

// PanelBounds represents a panel's boundaries
type PanelBounds struct {
	Left   int
	Top    int
	Right  int
	Bottom int
}

// engine is the default implementation
type engine struct {
	responsiveEngine ResponsiveEngine
	overlapDetector  OverlapDetector
}

// NewEngine creates a new layout engine
func NewEngine() Engine {
	return &engine{
		responsiveEngine: NewResponsiveEngine(),
		overlapDetector:  NewOverlapDetector(),
	}
}

// CalculateLayout calculates the layout for all panels
func (e *engine) CalculateLayout(panels []lens.PanelConfig, grid lens.GridConfig) (*Layout, error) {
	layout := &Layout{
		Grid:       grid,
		Panels:     make([]PanelLayout, 0, len(panels)),
		Breakpoint: BreakpointLG, // Default breakpoint
	}

	// Calculate layout bounds
	layout.Bounds = e.calculateLayoutBounds(panels, grid)

	// Generate CSS grid template
	layout.CSS = LayoutCSS{
		ContainerClasses: []string{"dashboard-grid", "grid-container"},
		ContainerStyles: map[string]string{
			"display":        "grid",
			"gap":            "1rem",
			"padding":        "1rem",
			"grid-auto-rows": fmt.Sprintf("%dpx", grid.RowHeight),
		},
		GridTemplate: GridTemplate{
			Columns: fmt.Sprintf("repeat(%d, 1fr)", grid.Columns),
			Rows:    "repeat(auto-fit, minmax(60px, auto))",
		},
	}

	// Calculate layout for each panel
	for i, panel := range panels {
		panelLayout, err := e.calculatePanelLayout(panel, i, grid)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate layout for panel %s: %w", panel.ID, err)
		}

		layout.Panels = append(layout.Panels, *panelLayout)
	}

	return layout, nil
}

// calculatePanelLayout calculates layout for a single panel
func (e *engine) calculatePanelLayout(panel lens.PanelConfig, index int, grid lens.GridConfig) (*PanelLayout, error) {
	// Calculate panel bounds
	bounds := PanelBounds{
		Left:   panel.Position.X,
		Top:    panel.Position.Y,
		Right:  panel.Position.X + panel.Dimensions.Width,
		Bottom: panel.Position.Y + panel.Dimensions.Height,
	}

	// Generate CSS grid area
	gridArea := fmt.Sprintf("%d / %d / %d / %d",
		panel.Position.Y+1,
		panel.Position.X+1,
		panel.Position.Y+panel.Dimensions.Height+1,
		panel.Position.X+panel.Dimensions.Width+1)

	panelLayout := &PanelLayout{
		PanelID:    panel.ID,
		Position:   panel.Position,
		Dimensions: panel.Dimensions,
		ZIndex:     index + 1,
		Bounds:     bounds,
		CSS: PanelCSS{
			Classes:  []string{"dashboard-panel", fmt.Sprintf("panel-%s", panel.Type)},
			GridArea: gridArea,
			Styles: map[string]string{
				"grid-area":      gridArea,
				"display":        "flex",
				"flex-direction": "column",
				"border":         "1px solid #e5e7eb",
				"border-radius":  "0.5rem",
				"background":     "white",
				"overflow":       "hidden",
			},
			ResponsiveCSS: make(map[Breakpoint]ResponsiveCSS),
		},
	}

	// Add responsive CSS for different breakpoints
	e.addResponsiveCSS(panelLayout, panel)

	return panelLayout, nil
}

// addResponsiveCSS adds responsive CSS rules for different breakpoints
func (e *engine) addResponsiveCSS(layout *PanelLayout, panel lens.PanelConfig) {
	// Mobile (xs) - stack panels vertically
	layout.CSS.ResponsiveCSS[BreakpointXS] = ResponsiveCSS{
		Classes: []string{"panel-mobile"},
		Styles: map[string]string{
			"grid-column": "1 / -1",
			"min-height":  "200px",
		},
	}

	// Tablet (sm) - reduce to 2 columns
	layout.CSS.ResponsiveCSS[BreakpointSM] = ResponsiveCSS{
		Classes: []string{"panel-tablet"},
		Styles: map[string]string{
			"grid-column": fmt.Sprintf("span %d", min(panel.Dimensions.Width, 2)),
		},
	}

	// Desktop (md+) - use original layout
	for _, bp := range []Breakpoint{BreakpointMD, BreakpointLG, BreakpointXL} {
		layout.CSS.ResponsiveCSS[bp] = ResponsiveCSS{
			Classes: []string{"panel-desktop"},
			Styles: map[string]string{
				"grid-area": layout.CSS.GridArea,
			},
		}
	}
}

// calculateLayoutBounds calculates the overall layout boundaries
func (e *engine) calculateLayoutBounds(panels []lens.PanelConfig, grid lens.GridConfig) LayoutBounds {
	bounds := LayoutBounds{
		MaxX:      0,
		MaxY:      0,
		MinWidth:  grid.Columns,
		MinHeight: 1,
	}

	for _, panel := range panels {
		rightBound := panel.Position.X + panel.Dimensions.Width
		bottomBound := panel.Position.Y + panel.Dimensions.Height

		if rightBound > bounds.MaxX {
			bounds.MaxX = rightBound
		}

		if bottomBound > bounds.MaxY {
			bounds.MaxY = bottomBound
		}
	}

	return bounds
}

// DetectOverlaps detects overlapping panels
func (e *engine) DetectOverlaps(panels []lens.PanelConfig) []OverlapError {
	return e.overlapDetector.DetectOverlaps(panels)
}

// GetResponsiveLayout adjusts layout for different breakpoints
func (e *engine) GetResponsiveLayout(layout *Layout, breakpoint Breakpoint) *Layout {
	return e.responsiveEngine.AdjustLayout(layout, breakpoint)
}

// ValidateLayout validates the layout configuration
func (e *engine) ValidateLayout(panels []lens.PanelConfig, grid lens.GridConfig) []ValidationError {
	var errors []ValidationError

	// Check if panels fit within grid
	for _, panel := range panels {
		if panel.Position.X+panel.Dimensions.Width > grid.Columns {
			errors = append(errors, ValidationError{
				PanelID: panel.ID,
				Message: fmt.Sprintf("panel extends beyond grid columns (%d > %d)",
					panel.Position.X+panel.Dimensions.Width, grid.Columns),
			})
		}

		if panel.Position.X < 0 || panel.Position.Y < 0 {
			errors = append(errors, ValidationError{
				PanelID: panel.ID,
				Message: "panel position cannot be negative",
			})
		}

		if panel.Dimensions.Width <= 0 || panel.Dimensions.Height <= 0 {
			errors = append(errors, ValidationError{
				PanelID: panel.ID,
				Message: "panel dimensions must be positive",
			})
		}
	}

	// Check for overlaps
	overlaps := e.DetectOverlaps(panels)
	for _, overlap := range overlaps {
		errors = append(errors, ValidationError{
			PanelID: overlap.Panel1,
			Message: fmt.Sprintf("panel overlaps with %s", overlap.Panel2),
		})
	}

	return errors
}

// ValidationError represents a layout validation error
type ValidationError struct {
	PanelID string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("panel %s: %s", e.PanelID, e.Message)
}

// Helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
