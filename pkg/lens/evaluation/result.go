package evaluation

import (
	"fmt"
	"github.com/iota-uz/iota-sdk/pkg/lens"
	"time"
)

// EvaluatedDashboard represents a dashboard ready for rendering
type EvaluatedDashboard struct {
	Config      lens.DashboardConfig
	Layout      Layout
	Panels      []EvaluatedPanel
	Errors      []EvaluationError
	EvaluatedAt time.Time
	Context     *EvaluationContext
}

// EvaluatedPanel represents an evaluated panel ready for rendering
type EvaluatedPanel struct {
	Config        lens.PanelConfig
	ResolvedQuery string
	DataSourceRef string
	RenderConfig  RenderConfig
	Variables     map[string]any
	Errors        []EvaluationError
}

// RenderConfig contains everything needed for rendering a panel
type RenderConfig struct {
	ChartType    lens.ChartType
	ChartOptions map[string]any
	GridCSS      GridCSS
	RefreshRate  time.Duration
	DataEndpoint string
	HTMXConfig   HTMXConfig
}

// Layout represents the calculated dashboard layout
type Layout struct {
	Grid       lens.GridConfig
	Panels     []PanelLayout
	Breakpoint Breakpoint
	CSS        LayoutCSS
}

// PanelLayout represents a panel's calculated layout
type PanelLayout struct {
	PanelID    string
	Position   lens.GridPosition
	Dimensions lens.GridDimensions
	CSS        PanelCSS
	ZIndex     int
}

// GridCSS represents grid-specific CSS configuration
type GridCSS struct {
	Classes  []string
	Styles   map[string]string
	GridArea string
	Position Position
}

// PanelCSS represents panel-specific CSS configuration
type PanelCSS struct {
	Classes       []string
	Styles        map[string]string
	GridArea      string
	ResponsiveCSS map[Breakpoint]ResponsiveCSS
}

// LayoutCSS represents layout-level CSS
type LayoutCSS struct {
	ContainerClasses []string
	ContainerStyles  map[string]string
	GridTemplate     GridTemplate
}

// ResponsiveCSS represents responsive CSS for different breakpoints
type ResponsiveCSS struct {
	Classes []string
	Styles  map[string]string
}

// GridTemplate represents CSS Grid template configuration
type GridTemplate struct {
	Columns string
	Rows    string
	Areas   []string
}

// Position represents CSS position information
type Position struct {
	Top    string
	Left   string
	Right  string
	Bottom string
}

// Breakpoint represents responsive breakpoints
type Breakpoint string

const (
	BreakpointXS Breakpoint = "xs"
	BreakpointSM Breakpoint = "sm"
	BreakpointMD Breakpoint = "md"
	BreakpointLG Breakpoint = "lg"
	BreakpointXL Breakpoint = "xl"
)

// HTMXConfig contains HTMX-specific configuration for panels
type HTMXConfig struct {
	Trigger    string
	Target     string
	Swap       string
	PushURL    bool
	Headers    map[string]string
	Indicators []string
}

// EvaluationError represents an error that occurred during evaluation
type EvaluationError struct {
	PanelID   string
	Phase     EvaluationPhase
	Message   string
	Cause     error
	Timestamp time.Time
}

// EvaluationPhase represents the phase where an error occurred
type EvaluationPhase string

const (
	PhaseValidation        EvaluationPhase = "validation"
	PhaseInterpolation     EvaluationPhase = "interpolation"
	PhaseLayoutCalculation EvaluationPhase = "layout_calculation"
	PhaseRenderConfig      EvaluationPhase = "render_config"
)

func (e EvaluationError) Error() string {
	if e.Cause != nil {
		if e.PanelID != "" {
			return fmt.Sprintf("panel %s (%s): %s: %v", e.PanelID, e.Phase, e.Message, e.Cause)
		}
		return fmt.Sprintf("%s: %s: %v", e.Phase, e.Message, e.Cause)
	}

	if e.PanelID != "" {
		return fmt.Sprintf("panel %s (%s): %s", e.PanelID, e.Phase, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Phase, e.Message)
}

// NewEvaluationError creates a new evaluation error
func NewEvaluationError(panelID string, phase EvaluationPhase, message string, cause error) EvaluationError {
	return EvaluationError{
		PanelID:   panelID,
		Phase:     phase,
		Message:   message,
		Cause:     cause,
		Timestamp: time.Now(),
	}
}

// HasErrors returns true if the evaluated dashboard has any errors
func (ed *EvaluatedDashboard) HasErrors() bool {
	if len(ed.Errors) > 0 {
		return true
	}

	for _, panel := range ed.Panels {
		if len(panel.Errors) > 0 {
			return true
		}
	}

	return false
}

// GetAllErrors returns all errors from the dashboard and panels
func (ed *EvaluatedDashboard) GetAllErrors() []EvaluationError {
	var allErrors []EvaluationError
	allErrors = append(allErrors, ed.Errors...)

	for _, panel := range ed.Panels {
		allErrors = append(allErrors, panel.Errors...)
	}

	return allErrors
}

// GetPanelByID returns a panel by its ID
func (ed *EvaluatedDashboard) GetPanelByID(id string) (*EvaluatedPanel, bool) {
	for i, panel := range ed.Panels {
		if panel.Config.ID == id {
			return &ed.Panels[i], true
		}
	}
	return nil, false
}

// IsValid returns true if the evaluation completed without errors
func (ed *EvaluatedDashboard) IsValid() bool {
	return !ed.HasErrors()
}
