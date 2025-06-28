package layout

import (
	"fmt"
	"github.com/iota-uz/iota-sdk/pkg/lens"
)

// Breakpoint represents responsive breakpoints
type Breakpoint string

const (
	BreakpointXS Breakpoint = "xs" // < 576px
	BreakpointSM Breakpoint = "sm" // >= 576px
	BreakpointMD Breakpoint = "md" // >= 768px
	BreakpointLG Breakpoint = "lg" // >= 992px
	BreakpointXL Breakpoint = "xl" // >= 1200px
)

// BreakpointConfig represents breakpoint configuration
type BreakpointConfig struct {
	Name      Breakpoint
	MinWidth  int
	MaxWidth  int
	Columns   int
	RowHeight int
}

// DefaultBreakpoints returns the default responsive breakpoint configuration
func DefaultBreakpoints() []BreakpointConfig {
	return []BreakpointConfig{
		{
			Name:      BreakpointXS,
			MinWidth:  0,
			MaxWidth:  575,
			Columns:   1,
			RowHeight: 200,
		},
		{
			Name:      BreakpointSM,
			MinWidth:  576,
			MaxWidth:  767,
			Columns:   2,
			RowHeight: 180,
		},
		{
			Name:      BreakpointMD,
			MinWidth:  768,
			MaxWidth:  991,
			Columns:   4,
			RowHeight: 160,
		},
		{
			Name:      BreakpointLG,
			MinWidth:  992,
			MaxWidth:  1199,
			Columns:   6,
			RowHeight: 140,
		},
		{
			Name:      BreakpointXL,
			MinWidth:  1200,
			MaxWidth:  -1, // No upper limit
			Columns:   12,
			RowHeight: 120,
		},
	}
}

// ResponsiveEngine handles responsive layout adjustments
type ResponsiveEngine interface {
	AdjustLayout(layout *Layout, breakpoint Breakpoint) *Layout
	GetBreakpointConfig(breakpoint Breakpoint) BreakpointConfig
	CalculateResponsiveDimensions(panel lens.PanelConfig, breakpoint Breakpoint) lens.GridDimensions
	GetBreakpointFromWidth(width int) Breakpoint
}

// responsiveEngine is the default implementation
type responsiveEngine struct {
	breakpoints []BreakpointConfig
}

// NewResponsiveEngine creates a new responsive engine
func NewResponsiveEngine() ResponsiveEngine {
	return &responsiveEngine{
		breakpoints: DefaultBreakpoints(),
	}
}

// NewResponsiveEngineWithBreakpoints creates a responsive engine with custom breakpoints
func NewResponsiveEngineWithBreakpoints(breakpoints []BreakpointConfig) ResponsiveEngine {
	return &responsiveEngine{
		breakpoints: breakpoints,
	}
}

// AdjustLayout adjusts the layout for a specific breakpoint
func (re *responsiveEngine) AdjustLayout(layout *Layout, breakpoint Breakpoint) *Layout {
	// Create a copy of the layout
	adjusted := &Layout{
		Grid:       layout.Grid,
		Panels:     make([]PanelLayout, len(layout.Panels)),
		Breakpoint: breakpoint,
		CSS:        layout.CSS,
		Bounds:     layout.Bounds,
	}

	// Get breakpoint configuration
	config := re.GetBreakpointConfig(breakpoint)

	// Adjust grid configuration
	adjusted.Grid.Columns = config.Columns
	adjusted.Grid.RowHeight = config.RowHeight

	// Adjust each panel for the breakpoint
	for i, panel := range layout.Panels {
		adjustedPanel := re.adjustPanelForBreakpoint(panel, breakpoint, config)
		adjusted.Panels[i] = adjustedPanel
	}

	// Update container CSS for the breakpoint
	re.updateContainerCSS(&adjusted.CSS, breakpoint, config)

	return adjusted
}

// adjustPanelForBreakpoint adjusts a single panel for a specific breakpoint
func (re *responsiveEngine) adjustPanelForBreakpoint(panel PanelLayout, breakpoint Breakpoint, config BreakpointConfig) PanelLayout {
	adjusted := panel

	switch breakpoint {
	case BreakpointXS:
		// Stack all panels vertically on mobile
		adjusted.Position = lens.GridPosition{X: 0, Y: panel.Position.Y}
		adjusted.Dimensions = lens.GridDimensions{Width: 1, Height: panel.Dimensions.Height}

	case BreakpointSM:
		// Reduce to 2 columns on small tablets
		adjusted.Dimensions.Width = min(panel.Dimensions.Width, 2)
		if panel.Position.X+adjusted.Dimensions.Width > config.Columns {
			adjusted.Position.X = 0
		}

	case BreakpointMD:
		// Reduce to 4 columns on medium screens
		adjusted.Dimensions.Width = min(panel.Dimensions.Width, 4)
		if panel.Position.X+adjusted.Dimensions.Width > config.Columns {
			adjusted.Position.X = max(0, config.Columns-adjusted.Dimensions.Width)
		}

	case BreakpointLG, BreakpointXL:
		// Use original dimensions for large screens, but ensure they fit
		if panel.Position.X+panel.Dimensions.Width > config.Columns {
			adjusted.Dimensions.Width = max(1, config.Columns-panel.Position.X)
		}
	}

	// Update CSS with responsive values
	if responsiveCSS, exists := panel.CSS.ResponsiveCSS[breakpoint]; exists {
		// Merge responsive styles
		for k, v := range responsiveCSS.Styles {
			adjusted.CSS.Styles[k] = v
		}
		// Add responsive classes
		adjusted.CSS.Classes = append(adjusted.CSS.Classes, responsiveCSS.Classes...)
	}

	// Recalculate grid area
	adjusted.CSS.GridArea = calculateGridArea(adjusted.Position, adjusted.Dimensions)
	adjusted.CSS.Styles["grid-area"] = adjusted.CSS.GridArea

	return adjusted
}

// updateContainerCSS updates container CSS for the breakpoint
func (re *responsiveEngine) updateContainerCSS(css *LayoutCSS, breakpoint Breakpoint, config BreakpointConfig) {
	// Update grid template columns
	css.GridTemplate.Columns = generateGridColumns(config.Columns)

	// Add breakpoint-specific classes
	css.ContainerClasses = append(css.ContainerClasses, string(breakpoint))

	// Update container styles
	if css.ContainerStyles == nil {
		css.ContainerStyles = make(map[string]string)
	}

	css.ContainerStyles["grid-template-columns"] = css.GridTemplate.Columns
	css.ContainerStyles["grid-auto-rows"] = generateAutoRows(config.RowHeight)

	// Add responsive-specific styles
	switch breakpoint {
	case BreakpointXS:
		css.ContainerStyles["gap"] = "0.5rem"
		css.ContainerStyles["padding"] = "0.5rem"
	case BreakpointSM:
		css.ContainerStyles["gap"] = "0.75rem"
		css.ContainerStyles["padding"] = "0.75rem"
	case BreakpointMD:
		css.ContainerStyles["gap"] = "1rem"
		css.ContainerStyles["padding"] = "1rem"
	case BreakpointLG:
		css.ContainerStyles["gap"] = "1.25rem"
		css.ContainerStyles["padding"] = "1.25rem"
	case BreakpointXL:
		css.ContainerStyles["gap"] = "1.5rem"
		css.ContainerStyles["padding"] = "1.5rem"
	default:
		css.ContainerStyles["gap"] = "1rem"
		css.ContainerStyles["padding"] = "1rem"
	}
}

// GetBreakpointConfig returns the configuration for a specific breakpoint
func (re *responsiveEngine) GetBreakpointConfig(breakpoint Breakpoint) BreakpointConfig {
	for _, config := range re.breakpoints {
		if config.Name == breakpoint {
			return config
		}
	}

	// Return default if not found
	return BreakpointConfig{
		Name:      breakpoint,
		MinWidth:  0,
		MaxWidth:  -1,
		Columns:   12,
		RowHeight: 120,
	}
}

// CalculateResponsiveDimensions calculates responsive dimensions for a panel
func (re *responsiveEngine) CalculateResponsiveDimensions(panel lens.PanelConfig, breakpoint Breakpoint) lens.GridDimensions {
	config := re.GetBreakpointConfig(breakpoint)

	switch breakpoint {
	case BreakpointXS:
		return lens.GridDimensions{Width: 1, Height: panel.Dimensions.Height}
	case BreakpointSM:
		return lens.GridDimensions{
			Width:  min(panel.Dimensions.Width, 2),
			Height: panel.Dimensions.Height,
		}
	case BreakpointMD:
		return lens.GridDimensions{
			Width:  min(panel.Dimensions.Width, 4),
			Height: panel.Dimensions.Height,
		}
	case BreakpointLG:
		return lens.GridDimensions{
			Width:  min(panel.Dimensions.Width, 8),
			Height: panel.Dimensions.Height,
		}
	case BreakpointXL:
		return lens.GridDimensions{
			Width:  min(panel.Dimensions.Width, config.Columns),
			Height: panel.Dimensions.Height,
		}
	default:
		// For large screens, use original dimensions but ensure they fit
		return lens.GridDimensions{
			Width:  min(panel.Dimensions.Width, config.Columns),
			Height: panel.Dimensions.Height,
		}
	}
}

// GetBreakpointFromWidth determines the breakpoint based on screen width
func (re *responsiveEngine) GetBreakpointFromWidth(width int) Breakpoint {
	for _, config := range re.breakpoints {
		if width >= config.MinWidth && (config.MaxWidth == -1 || width <= config.MaxWidth) {
			return config.Name
		}
	}

	// Default to large if no match found
	return BreakpointLG
}

// Helper functions
func calculateGridArea(pos lens.GridPosition, dim lens.GridDimensions) string {
	return fmt.Sprintf("%d / %d / %d / %d",
		pos.Y+1,
		pos.X+1,
		pos.Y+dim.Height+1,
		pos.X+dim.Width+1)
}

func generateGridColumns(columns int) string {
	return fmt.Sprintf("repeat(%d, 1fr)", columns)
}

func generateAutoRows(rowHeight int) string {
	return fmt.Sprintf("minmax(%dpx, auto)", rowHeight)
}
