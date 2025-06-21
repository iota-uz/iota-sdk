package lens

// LayoutEngine calculates panel layouts and handles grid positioning (business logic only)
type LayoutEngine interface {
	CalculateLayout(panels []PanelConfig, grid GridConfig) (Layout, error)
	DetectOverlaps(panels []PanelConfig) []GridError
	GetResponsiveLayout(layout Layout, breakpoint string) Layout
	ValidateLayout(panels []PanelConfig, grid GridConfig) []GridError
}

// layoutEngine is the default implementation
type layoutEngine struct{}

// NewLayoutEngine creates a new layout engine
func NewLayoutEngine() LayoutEngine {
	return &layoutEngine{}
}

// CalculateLayout calculates the layout for all panels (no CSS generation)
func (l *layoutEngine) CalculateLayout(panels []PanelConfig, grid GridConfig) (Layout, error) {
	layout := Layout{
		Grid:       grid,
		Panels:     make([]PanelLayout, 0, len(panels)),
		Breakpoint: "lg",
	}

	// Business logic only - no CSS generation
	for _, panel := range panels {
		panelLayout := PanelLayout{
			PanelID:    panel.ID,
			Position:   panel.Position,
			Dimensions: panel.Dimensions,
		}
		layout.Panels = append(layout.Panels, panelLayout)
	}

	return layout, nil
}

// DetectOverlaps detects overlapping panels
func (l *layoutEngine) DetectOverlaps(panels []PanelConfig) []GridError {
	var errors []GridError

	// Check for exact position matches (simple implementation)
	for i, panel1 := range panels {
		for j, panel2 := range panels {
			if i >= j {
				continue
			}

			if panel1.Position.X == panel2.Position.X && panel1.Position.Y == panel2.Position.Y {
				errors = append(errors, GridError{
					Message: "panels overlap at same position",
					Panels:  []string{panel1.ID, panel2.ID},
				})
			}

			// Check for actual area overlaps
			if l.panelsOverlap(panel1, panel2) {
				errors = append(errors, GridError{
					Message: "panels overlap in grid area",
					Panels:  []string{panel1.ID, panel2.ID},
				})
			}
		}
	}

	return errors
}

// panelsOverlap checks if two panels overlap in grid space
func (l *layoutEngine) panelsOverlap(panel1, panel2 PanelConfig) bool {
	p1Right := panel1.Position.X + panel1.Dimensions.Width
	p1Bottom := panel1.Position.Y + panel1.Dimensions.Height
	p2Right := panel2.Position.X + panel2.Dimensions.Width
	p2Bottom := panel2.Position.Y + panel2.Dimensions.Height

	return !(p1Right <= panel2.Position.X ||
		p2Right <= panel1.Position.X ||
		p1Bottom <= panel2.Position.Y ||
		p2Bottom <= panel1.Position.Y)
}

// GetResponsiveLayout adjusts layout for different breakpoints
func (l *layoutEngine) GetResponsiveLayout(layout Layout, breakpoint string) Layout {
	responsive := layout
	responsive.Breakpoint = breakpoint
	return responsive
}

// ValidateLayout validates the layout configuration
func (l *layoutEngine) ValidateLayout(panels []PanelConfig, grid GridConfig) []GridError {
	var errors []GridError

	// Check if panels fit within grid bounds
	for _, panel := range panels {
		if panel.Position.X+panel.Dimensions.Width > grid.Columns {
			errors = append(errors, GridError{
				Message: "panel extends beyond grid columns",
				Panels:  []string{panel.ID},
			})
		}

		if panel.Position.X < 0 || panel.Position.Y < 0 {
			errors = append(errors, GridError{
				Message: "panel position cannot be negative",
				Panels:  []string{panel.ID},
			})
		}

		if panel.Dimensions.Width <= 0 || panel.Dimensions.Height <= 0 {
			errors = append(errors, GridError{
				Message: "panel dimensions must be positive",
				Panels:  []string{panel.ID},
			})
		}
	}

	// Check for overlaps
	overlaps := l.DetectOverlaps(panels)
	errors = append(errors, overlaps...)

	return errors
}
