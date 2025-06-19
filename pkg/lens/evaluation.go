package lens

// Evaluator evaluates dashboard configurations
type Evaluator interface {
	Evaluate(config *DashboardConfig, ctx EvaluationContext) (*EvaluatedDashboard, error)
}

// EvaluationContext provides context for evaluation
type EvaluationContext struct {
	TimeRange TimeRange
	Variables map[string]any
	User      UserContext
}

// UserContext represents user information for permissions
type UserContext struct {
	ID    string
	Roles []string
}

// EvaluatedDashboard represents an evaluated dashboard ready for rendering
type EvaluatedDashboard struct {
	Config DashboardConfig
	Layout Layout
	Panels []EvaluatedPanel
	Errors []EvaluationError
}

// EvaluatedPanel represents an evaluated panel
type EvaluatedPanel struct {
	Config        PanelConfig
	ResolvedQuery string
	DataSourceRef string
	RenderConfig  RenderConfig
}

// RenderConfig contains rendering configuration
type RenderConfig struct {
	ChartType    ChartType
	ChartOptions map[string]any
	GridCSS      GridCSS
	RefreshRate  int // seconds
}

// GridCSS represents grid-specific CSS (UI concern)
type GridCSS struct {
	Classes []string
	Styles  map[string]string
}

// evaluator is the default implementation
type evaluator struct{}

// NewEvaluator creates a new evaluator
func NewEvaluator() Evaluator {
	return &evaluator{}
}

// Evaluate evaluates the dashboard configuration
func (e *evaluator) Evaluate(config *DashboardConfig, ctx EvaluationContext) (*EvaluatedDashboard, error) {
	// Minimal implementation
	result := &EvaluatedDashboard{
		Config: *config,
		Layout: Layout{
			Grid:       config.Grid,
			Breakpoint: "lg",
		},
		Panels: make([]EvaluatedPanel, 0, len(config.Panels)),
		Errors: []EvaluationError{},
	}

	// Process panels
	for _, panel := range config.Panels {
		evaluated := EvaluatedPanel{
			Config:        panel,
			ResolvedQuery: panel.Query, // No interpolation in minimal implementation
			DataSourceRef: panel.DataSource.Ref,
			RenderConfig: RenderConfig{
				ChartType:    panel.Type,
				ChartOptions: panel.Options,
				RefreshRate:  30, // Default 30 seconds
			},
		}
		result.Panels = append(result.Panels, evaluated)
	}

	return result, nil
}
