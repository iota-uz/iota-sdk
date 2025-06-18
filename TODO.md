# Dashboard System Implementation Plan

## Overview
Build a Grafana-like dashboard system as a pure Go package that supports configurable panels, multiple data sources, and JSON-based configuration management. The package provides core types, validation, and an evaluation engine that transforms configurations into renderable structures.

## Phase 1: Core Dashboard Types

### 1.1 Configuration Types
**Location**: `pkg/lens/`

**Files to create**:
- `config.go` - Configuration structures
- `types.go` - Core types and enums
- `errors.go` - Domain-specific errors

**Key types**:
```go
// Input configuration (from JSON)
type DashboardConfig struct {
    ID          string       `json:"id"`
    Name        string       `json:"name"`
    Description string       `json:"description"`
    Version     string       `json:"version"`
    Grid        GridConfig   `json:"grid"`
    Panels      []PanelConfig `json:"panels"`
    Variables   []Variable    `json:"variables"`
}

type PanelConfig struct {
    ID         string                 `json:"id"`
    Title      string                 `json:"title"`
    Type       ChartType             `json:"type"`
    Position   GridPosition          `json:"position"`
    Dimensions GridDimensions        `json:"dimensions"`
    DataSource DataSourceConfig      `json:"dataSource"`
    Query      string                `json:"query"`
    Options    map[string]interface{} `json:"options"`
}

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
```

### 1.2 Value Objects
**Location**: `pkg/lens/`

**Files**:
- `grid.go` - Grid system types (GridConfig, GridPosition, GridDimensions)
- `datasource.go` - DataSource interface and config
- `variable.go` - Dashboard variables (for templating)
- `timerange.go` - Time range handling

### 1.3 Validation
**Location**: `pkg/lens/validation/`

**Files**:
- `validator.go` - Main validation interface
- `rules.go` - Validation rules
- `errors.go` - Validation error types

**Key functions**:
```go
func ValidateConfig(config *DashboardConfig) ValidationResult
func ValidatePanel(panel *PanelConfig, grid GridConfig) ValidationResult
func ValidateGridLayout(panels []PanelConfig, grid GridConfig) ValidationResult
```

## Phase 2: Evaluation Engine

### 2.1 Core Evaluation
**Location**: `pkg/lens/evaluation/`

**Files**:
- `evaluator.go` - Main evaluation engine
- `context.go` - Evaluation context (variables, time range, etc.)
- `result.go` - Evaluation result types

**Key types**:
```go
// Evaluation context
type EvaluationContext struct {
    TimeRange TimeRange
    Variables map[string]interface{}
    User      UserContext // For permission-based filtering
}

// Evaluated dashboard ready for rendering
type EvaluatedDashboard struct {
    Config     DashboardConfig
    Layout     Layout
    Panels     []EvaluatedPanel
    Errors     []EvaluationError
}

type EvaluatedPanel struct {
    Config         PanelConfig
    ResolvedQuery  string // Variables interpolated
    DataSourceRef  string // Resolved data source reference
    RenderConfig   RenderConfig
}

// What the renderer needs
type RenderConfig struct {
    ChartType    ChartType
    ChartOptions map[string]interface{} // ApexCharts-ready config
    GridCSS      GridCSS
    RefreshRate  time.Duration
}
```

### 2.2 Query Processing
**Location**: `pkg/lens/evaluation/`

**Files**:
- `query_processor.go` - Query variable interpolation
- `query_validator.go` - Query validation (without execution)

**Key functions**:
```go
func InterpolateQuery(query string, ctx EvaluationContext) (string, error)
func ValidateQuery(query string, dataSourceType string) error
```

### 2.3 Layout Engine
**Location**: `pkg/lens/layout/`

**Files**:
- `layout.go` - Layout calculation
- `responsive.go` - Responsive breakpoint handling
- `overlap.go` - Overlap detection and resolution

**Key types**:
```go
type Layout struct {
    Grid       GridConfig
    Panels     []PanelLayout
    Breakpoint Breakpoint
}

type PanelLayout struct {
    PanelID    string
    Position   GridPosition
    Dimensions GridDimensions
    CSS        PanelCSS // Computed CSS classes/styles
}
```

## Phase 3: Rendering Interface

### 3.1 Render Mappers
**Location**: `pkg/lens/render/`

**Files**:
- `mapper.go` - Main mapper interface
- `apex_mapper.go` - ApexCharts configuration mapper
- `css_mapper.go` - CSS class mapper

**Key interfaces**:
```go
type ChartMapper interface {
    MapToChartConfig(panel EvaluatedPanel) ChartRenderConfig
}

type ChartRenderConfig struct {
    Type         string                 // ApexCharts type
    Options      map[string]interface{} // ApexCharts options
    DataEndpoint string                 // Where to fetch data
    RefreshRate  string                 // For HTMX polling
}
```

### 3.2 Data Source Interface
**Location**: `pkg/lens/datasource/`

**Files**:
- `interface.go` - DataSource interface
- `registry.go` - DataSource registry

**Interface only** (implementations live elsewhere):
```go
type DataSource interface {
    Type() string
    ValidateQuery(query string) error
    // Note: Actual execution happens outside this package
}

type DataResult struct {
    Columns []Column
    Rows    [][]interface{}
    Error   error
}
```

## Phase 4: Builder Pattern

### 4.1 Dashboard Builder
**Location**: `pkg/lens/builder/`

**Files**:
- `builder.go` - Fluent API for building dashboards
- `panel_builder.go` - Panel builder

**Example usage**:
```go
dashboard := builder.NewDashboard("sales-dashboard").
    WithName("Sales Dashboard").
    WithGrid(12, 60). // 12 columns, 60px row height
    AddPanel(
        builder.NewPanel("revenue-chart").
            WithTitle("Revenue Over Time").
            WithType(ChartTypeLine).
            AtPosition(0, 0).
            WithSize(6, 4).
            WithQuery("SELECT date, revenue FROM sales WHERE date >= :start").
            WithDataSource("postgres", "main")
    ).
    Build()
```

## Phase 5: JSON Schema & Examples

### 5.1 Schema Definition
**Location**: `pkg/lens/schema/`

**Files**:
- `dashboard.schema.json` - JSON Schema for validation
- `examples/` - Example dashboard configurations

### 5.2 Example Dashboard
```json
{
  "version": "1.0",
  "id": "sales-dashboard",
  "name": "Sales Dashboard",
  "description": "Company sales metrics",
  "grid": {
    "columns": 12,
    "rowHeight": 60,
    "breakpoints": {
      "lg": 1200,
      "md": 996,
      "sm": 768,
      "xs": 480
    }
  },
  "panels": [
    {
      "id": "revenue-line",
      "title": "Revenue Trend",
      "type": "line",
      "position": {"x": 0, "y": 0},
      "dimensions": {"width": 6, "height": 4},
      "dataSource": {
        "type": "postgres",
        "ref": "main"
      },
      "query": "SELECT date, revenue FROM sales WHERE date >= :timeRange.start",
      "options": {
        "xaxis": {"type": "datetime"},
        "yaxis": {"title": "Revenue ($)"}
      }
    }
  ],
  "variables": [
    {
      "name": "timeRange",
      "type": "timeRange",
      "default": "last7days"
    }
  ]
}
```

## Phase 6: UI Rendering Package

### 6.1 Dashboard UI Package
**Location**: `pkg/lens/ui/`

**Dependencies**:
- Imports parent `pkg/lens` for types
- Uses existing `components/charts/` for chart rendering

**Files**:
- `renderer.go` - Main rendering interface
- `components.go` - Component definitions
- `htmx.go` - HTMX integration helpers

**Key interfaces**:
```go
type DashboardRenderer interface {
    RenderDashboard(dashboard *lens.EvaluatedDashboard) templ.Component
    RenderPanel(panel *lens.EvaluatedPanel, data DataResult) templ.Component
}

type UIConfig struct {
    ChartComponents map[lens.ChartType]ChartComponent
    GridClasses     GridClassConfig
    RefreshStrategy RefreshStrategy // polling, websocket, sse
}
```

### 6.2 Templ Components
**Location**: `pkg/lens/ui/templates/`

**Components**:
- `dashboard.templ` - Main dashboard container
- `panel.templ` - Panel wrapper with loading states
- `grid.templ` - Grid layout component
- `error.templ` - Error display component

**Example panel component**:
```go
templ Panel(panel EvaluatedPanel, chartConfig ApexChartConfig) {
    <div 
        class={ panel.RenderConfig.GridCSS.Classes() }
        id={ fmt.Sprintf("panel-%s", panel.Config.ID) }
        hx-get={ fmt.Sprintf("/api/panels/%s/refresh", panel.Config.ID) }
        hx-trigger={ fmt.Sprintf("every %s", panel.RenderConfig.RefreshRate) }
        hx-swap="innerHTML"
    >
        <div class="panel-header">
            <h3>{ panel.Config.Title }</h3>
        </div>
        <div class="panel-body">
            @components.Chart(chartConfig)
        </div>
    </div>
}
```

### 6.3 ApexCharts Integration
**Location**: `pkg/lens/ui/charts/`

**Files**:
- `apex_mapper.go` - Maps EvaluatedPanel to ApexCharts config
- `chart_factory.go` - Creates appropriate chart components
- `options.go` - Default chart options by type

**Mapping example**:
```go
func MapToApexConfig(panel *lens.EvaluatedPanel, data DataResult) ApexChartConfig {
    return ApexChartConfig{
        Type:    string(panel.Config.Type),
        Series:  transformDataToSeries(data),
        Options: mergeOptions(defaultOptions[panel.Config.Type], panel.Config.Options),
        Height:  calculateHeight(panel.Config.Dimensions),
    }
}
```

### 6.4 Grid System
**Location**: `pkg/lens/ui/grid/`

**Files**:
- `grid.go` - Grid CSS generation
- `responsive.go` - Breakpoint handling

**CSS Strategy**:
- Use CSS Grid for layout
- Generate inline styles for dynamic positioning
- Responsive classes for breakpoints

### 6.5 Data Fetching Integration
**Location**: `pkg/lens/ui/data/`

**Files**:
- `fetcher.go` - Data fetching interface
- `cache.go` - Client-side caching

**Note**: Actual data fetching is handled by the implementing application, not the package.

## Phase 7: Testing Strategy

### 7.1 Core Package Tests
**Location**: `pkg/lens/tests/`

- Configuration validation tests
- Evaluation engine tests
- Layout calculation tests
- Query interpolation tests
- Grid overlap detection tests

### 7.2 UI Package Tests
**Location**: `pkg/lens/ui/tests/`

- Component rendering tests
- ApexCharts config generation tests
- Grid CSS generation tests
- HTMX attribute generation tests

### 7.3 Test Fixtures
**Location**: `pkg/lens/testdata/` and `pkg/lens/ui/testdata/`

- Example dashboard configurations (valid and invalid)
- Expected evaluation results
- Sample data results for chart rendering
- Expected ApexCharts configurations

## Phase 8: Implementation Example

### 8.1 Usage Example
**How the packages work together**:

```go
// 1. Load configuration
config := &lens.DashboardConfig{}
json.Unmarshal(jsonData, config)

// 2. Validate
result := validation.ValidateConfig(config)
if !result.IsValid() {
    // Handle validation errors
}

// 3. Create evaluation context
ctx := evaluation.EvaluationContext{
    TimeRange: timeRange,
    Variables: map[string]interface{}{
        "department": "sales",
    },
}

// 4. Evaluate dashboard
evaluator := evaluation.NewEvaluator()
evaluated, err := evaluator.Evaluate(config, ctx)

// 5. Render UI (in the application layer)
renderer := ui.NewRenderer(uiConfig)
component := renderer.RenderDashboard(evaluated)

// 6. Fetch data (application implements this)
for _, panel := range evaluated.Panels {
    data := dataFetcher.Fetch(panel.ResolvedQuery)
    panelComponent := renderer.RenderPanel(panel, data)
}
```

### 8.2 Storage Implementation Example
**Application-specific storage using the package**:

```go
// Redis storage (in application, not package)
type RedisDashboardStore struct {
    client *redis.Client
}

func (s *RedisDashboardStore) Save(config *lens.DashboardConfig) error {
    json, _ := json.Marshal(config)
    return s.client.Set(ctx, fmt.Sprintf("dashboard:%s", config.ID), json, 0).Err()
}
```

## Phase 9: Package Boundaries

### 9.1 What's IN the packages
**pkg/lens**:
- Configuration types and validation
- Evaluation engine
- Layout calculations
- Query interpolation
- Builder API

**pkg/lens/ui**:
- Templ components
- ApexCharts mapping
- Grid CSS generation
- HTMX helpers

### 9.2 What's OUT (application responsibility)
- Data persistence (Redis, PostgreSQL, etc.)
- Actual query execution
- Authentication/authorization
- HTTP routing
- WebSocket handling
- Caching implementation

## Implementation Order

1. **Week 1**: Core types and validation in `pkg/lens`
2. **Week 2**: Evaluation engine and layout system
3. **Week 3**: UI package with templ components
4. **Week 4**: ApexCharts integration and mappers
5. **Week 5**: Builder API and examples
6. **Week 6**: Testing and documentation

## Success Criteria

- [ ] Configuration validation catches all invalid states
- [ ] Evaluation engine correctly interpolates variables
- [ ] Layout engine prevents panel overlaps
- [ ] UI components render with correct ApexCharts config
- [ ] Packages have zero external dependencies (except stdlib)
- [ ] Clear separation between package and application concerns
- [ ] Comprehensive test coverage (>90%)
- [ ] Performance: Evaluation completes in <50ms for 20 panels

## Technical Decisions

1. **Two packages instead of one**: Separation of concerns between logic and UI
2. **No persistence in packages**: Maximum flexibility for applications
3. **Evaluation engine**: Enables pre-computation and validation before rendering
4. **Builder pattern**: Programmatic dashboard creation without JSON
5. **Templ for UI**: Consistency with existing project patterns

## Definition of Done

- [ ] All types have godoc documentation
- [ ] Validation provides field-level error paths
- [ ] Evaluation engine handles all variable types
- [ ] UI package generates valid HTMX attributes
- [ ] Zero coupling between packages and application
- [ ] Example applications demonstrate usage
- [ ] Integration guide with code examples
- [ ] Benchmark suite for performance validation


