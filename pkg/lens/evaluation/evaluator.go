package evaluation

import (
	"fmt"
	"github.com/iota-uz/iota-sdk/pkg/lens"
	"regexp"
	"strings"
	"time"
)

// Evaluator evaluates dashboard configurations into renderable structures
type Evaluator interface {
	Evaluate(config *lens.DashboardConfig, ctx *EvaluationContext) (*EvaluatedDashboard, error)
	EvaluatePanel(panel *lens.PanelConfig, ctx *EvaluationContext) (*EvaluatedPanel, error)
}

// evaluator is the default implementation
type evaluator struct {
	queryProcessor QueryProcessor
	layoutEngine   LayoutEngine
	renderMapper   RenderMapper
}

// NewEvaluator creates a new evaluator with default processors
func NewEvaluator() Evaluator {
	return &evaluator{
		queryProcessor: NewQueryProcessor(),
		layoutEngine:   NewLayoutEngine(),
		renderMapper:   NewRenderMapper(),
	}
}

// NewEvaluatorWithProcessors creates an evaluator with custom processors
func NewEvaluatorWithProcessors(queryProcessor QueryProcessor, layoutEngine LayoutEngine, renderMapper RenderMapper) Evaluator {
	return &evaluator{
		queryProcessor: queryProcessor,
		layoutEngine:   layoutEngine,
		renderMapper:   renderMapper,
	}
}

// Evaluate evaluates the entire dashboard configuration
func (e *evaluator) Evaluate(config *lens.DashboardConfig, ctx *EvaluationContext) (*EvaluatedDashboard, error) {
	result := &EvaluatedDashboard{
		Config:      *config,
		Panels:      make([]EvaluatedPanel, 0, len(config.Panels)),
		Errors:      []EvaluationError{},
		EvaluatedAt: time.Now(),
		Context:     ctx,
	}

	// Calculate layout
	if ctx.Options.CalculateLayout {
		layout, err := e.layoutEngine.CalculateLayout(config.Panels, config.Grid)
		if err != nil {
			result.Errors = append(result.Errors, NewEvaluationError("", PhaseLayoutCalculation, "failed to calculate layout", err))
		} else {
			result.Layout = *layout
		}
	}

	// Evaluate each panel
	for _, panelConfig := range config.Panels {
		panel, err := e.EvaluatePanel(&panelConfig, ctx)
		if err != nil {
			result.Errors = append(result.Errors, NewEvaluationError(panelConfig.ID, PhaseRenderConfig, "failed to evaluate panel", err))
			continue
		}

		result.Panels = append(result.Panels, *panel)
	}

	return result, nil
}

// EvaluatePanel evaluates a single panel configuration
func (e *evaluator) EvaluatePanel(panel *lens.PanelConfig, ctx *EvaluationContext) (*EvaluatedPanel, error) {
	result := &EvaluatedPanel{
		Config:    *panel,
		Variables: make(map[string]any),
		Errors:    []EvaluationError{},
	}

	// Interpolate query variables
	if ctx.Options.InterpolateVariables {
		resolvedQuery, err := e.queryProcessor.InterpolateQuery(panel.Query, ctx)
		if err != nil {
			result.Errors = append(result.Errors, NewEvaluationError(panel.ID, PhaseInterpolation, "failed to interpolate query", err))
			result.ResolvedQuery = panel.Query // Fallback to original
		} else {
			result.ResolvedQuery = resolvedQuery
		}
	} else {
		result.ResolvedQuery = panel.Query
	}

	// Set data source reference
	result.DataSourceRef = panel.DataSource.Ref

	// Generate render configuration
	renderConfig, err := e.renderMapper.MapToRenderConfig(panel, ctx)
	if err != nil {
		result.Errors = append(result.Errors, NewEvaluationError(panel.ID, PhaseRenderConfig, "failed to generate render config", err))
	} else {
		result.RenderConfig = *renderConfig
	}

	// Copy relevant variables for this panel
	for name, value := range ctx.Variables {
		result.Variables[name] = value
	}

	return result, nil
}

// QueryProcessor handles query variable interpolation
type QueryProcessor interface {
	InterpolateQuery(query string, ctx *EvaluationContext) (string, error)
	ValidateQuery(query string, dataSourceType string) error
}

// queryProcessor is the default implementation
type queryProcessor struct {
	variablePattern *regexp.Regexp
}

// NewQueryProcessor creates a new query processor
func NewQueryProcessor() QueryProcessor {
	return &queryProcessor{
		variablePattern: regexp.MustCompile(`:\w+(?:\.\w+)*`),
	}
}

// InterpolateQuery replaces variables in the query with actual values
func (qp *queryProcessor) InterpolateQuery(query string, ctx *EvaluationContext) (string, error) {
	result := query

	// Find all variable references in the query
	matches := qp.variablePattern.FindAllString(query, -1)

	for _, match := range matches {
		// Remove the leading colon
		varName := strings.TrimPrefix(match, ":")

		// Handle nested variable access (e.g., :timeRange.start)
		varValue, err := qp.resolveVariable(varName, ctx)
		if err != nil {
			return "", fmt.Errorf("failed to resolve variable %s: %w", varName, err)
		}

		// Replace the variable reference with the actual value
		result = strings.ReplaceAll(result, match, fmt.Sprintf("%v", varValue))
	}

	return result, nil
}

// ValidateQuery validates a query without executing it
func (qp *queryProcessor) ValidateQuery(query string, dataSourceType string) error {
	// Basic validation - check for potentially dangerous SQL
	originalQuery := query
	query = strings.ToLower(strings.TrimSpace(query))

	// Check for basic SQL injection patterns - order matters for accurate detection
	dangerousPatterns := []string{
		"--",
		";",
		"drop table",
		"delete from",
		"truncate",
		"alter table",
		"create table",
		"insert into",
		"update ",
	}

	for _, pattern := range dangerousPatterns {
		if strings.Contains(query, pattern) {
			return fmt.Errorf("potentially dangerous query pattern detected: %s", pattern)
		}
	}

	return nil
}

// resolveVariable resolves a variable from the context, supporting nested access
func (qp *queryProcessor) resolveVariable(varName string, ctx *EvaluationContext) (any, error) {
	parts := strings.Split(varName, ".")

	// Get the root variable
	value, exists := ctx.GetVariable(parts[0])
	if !exists {
		return nil, fmt.Errorf("variable %s not found", parts[0])
	}

	// Handle nested access
	current := value
	for i := 1; i < len(parts); i++ {
		switch v := current.(type) {
		case map[string]any:
			if val, ok := v[parts[i]]; ok {
				current = val
			} else {
				return nil, fmt.Errorf("property %s not found in variable %s", parts[i], parts[0])
			}
		case lens.TimeRange:
			switch parts[i] {
			case "start":
				current = v.Start.Format(time.RFC3339)
			case "end":
				current = v.End.Format(time.RFC3339)
			default:
				return nil, fmt.Errorf("unknown property %s for TimeRange", parts[i])
			}
		default:
			return nil, fmt.Errorf("cannot access property %s on non-object variable", parts[i])
		}
	}

	return current, nil
}

// LayoutEngine calculates panel layouts
type LayoutEngine interface {
	CalculateLayout(panels []lens.PanelConfig, grid lens.GridConfig) (*Layout, error)
}

// layoutEngine is the default implementation
type layoutEngine struct{}

// NewLayoutEngine creates a new layout engine
func NewLayoutEngine() LayoutEngine {
	return &layoutEngine{}
}

// CalculateLayout calculates the layout for all panels
func (le *layoutEngine) CalculateLayout(panels []lens.PanelConfig, grid lens.GridConfig) (*Layout, error) {
	layout := &Layout{
		Grid:       grid,
		Panels:     make([]PanelLayout, 0, len(panels)),
		Breakpoint: BreakpointLG, // Default breakpoint
	}

	// Generate CSS grid template
	layout.CSS = LayoutCSS{
		ContainerClasses: []string{"dashboard-grid"},
		GridTemplate: GridTemplate{
			Columns: fmt.Sprintf("repeat(%d, 1fr)", grid.Columns),
			Rows:    fmt.Sprintf("repeat(auto-fit, %dpx)", grid.RowHeight),
		},
	}

	// Calculate layout for each panel
	for i, panel := range panels {
		panelLayout := PanelLayout{
			PanelID:    panel.ID,
			Position:   panel.Position,
			Dimensions: panel.Dimensions,
			ZIndex:     i + 1,
			CSS: PanelCSS{
				Classes: []string{"dashboard-panel"},
				GridArea: fmt.Sprintf("%d / %d / %d / %d",
					panel.Position.Y+1,
					panel.Position.X+1,
					panel.Position.Y+panel.Dimensions.Height+1,
					panel.Position.X+panel.Dimensions.Width+1),
			},
		}

		layout.Panels = append(layout.Panels, panelLayout)
	}

	return layout, nil
}

// RenderMapper maps panels to render configurations
type RenderMapper interface {
	MapToRenderConfig(panel *lens.PanelConfig, ctx *EvaluationContext) (*RenderConfig, error)
}

// renderMapper is the default implementation
type renderMapper struct{}

// NewRenderMapper creates a new render mapper
func NewRenderMapper() RenderMapper {
	return &renderMapper{}
}

// MapToRenderConfig creates a render configuration for a panel
func (rm *renderMapper) MapToRenderConfig(panel *lens.PanelConfig, ctx *EvaluationContext) (*RenderConfig, error) {
	config := &RenderConfig{
		ChartType:    panel.Type,
		ChartOptions: panel.Options,
		RefreshRate:  30 * time.Second, // Default refresh rate
		DataEndpoint: fmt.Sprintf("/api/panels/%s/data", panel.ID),
		GridCSS: GridCSS{
			Classes: []string{"panel-container"},
			GridArea: fmt.Sprintf("%d / %d / %d / %d",
				panel.Position.Y+1,
				panel.Position.X+1,
				panel.Position.Y+panel.Dimensions.Height+1,
				panel.Position.X+panel.Dimensions.Width+1),
		},
		HTMXConfig: HTMXConfig{
			Trigger: "every 30s",
			Target:  fmt.Sprintf("#panel-%s", panel.ID),
			Swap:    "innerHTML",
		},
	}

	// Customize options based on chart type
	if config.ChartOptions == nil {
		config.ChartOptions = make(map[string]any)
	}

	// Set default options based on chart type
	switch panel.Type {
	case lens.ChartTypeLine:
		config.ChartOptions["stroke"] = map[string]any{"curve": "smooth"}
	case lens.ChartTypeBar:
		config.ChartOptions["plotOptions"] = map[string]any{
			"bar": map[string]any{"horizontal": false},
		}
	case lens.ChartTypePie:
		config.ChartOptions["legend"] = map[string]any{"position": "bottom"}
	case lens.ChartTypeStackedBar:
		config.ChartOptions["chart"] = map[string]any{"stacked": true}
	case lens.ChartTypeArea:
		config.ChartOptions["fill"] = map[string]any{"type": "gradient"}
	case lens.ChartTypeColumn:
		config.ChartOptions["plotOptions"] = map[string]any{
			"bar": map[string]any{"horizontal": false, "columnWidth": "55%"},
		}
	case lens.ChartTypeGauge:
		config.ChartOptions["chart"] = map[string]any{"type": "radialBar"}
	case lens.ChartTypeTable:
		// Table doesn't need chart options
	case lens.ChartTypeMetric:
		// Metric doesn't need chart options
	}

	return config, nil
}
