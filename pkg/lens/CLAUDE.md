# Lens Dashboard Framework - Development Guide

## Code Structure and Architecture

The lens framework follows a strict layered architecture with **evaluation-first design**:

```
pkg/lens/
├── types.go                    # Core types - ALWAYS start here
├── builder/                    # Fluent API - programmatic dashboard creation
│   └── dashboard.go           # Dashboard and panel builders
├── evaluation/                 # CORE: Layout & config processing
│   └── layout.go              # Grid calculations, responsive breakpoints
├── executor/                   # Query execution - data format assignment
│   └── executor.go            # Query orchestration (line ~184 critical!)
├── datasource/                 # Data abstraction - NEVER import ui/
│   └── postgres/              # Implementation - handles FormatTable/TimeSeries
└── ui/                        # Rendering layer - consumes evaluated configs
    ├── helpers.go             # Chart data transformation only
    └── dashboard.templ        # Templates - pure presentation
```

**RULE**: Configuration → Evaluation → Execution → Rendering

## Adding New Chart Types

**CRITICAL**: Follow this exact sequence when adding chart types:

### 1. Define the Type (types.go)
```go
const ChartTypeNewChart ChartType = "new_chart"
```

### 2. Add Builder Function (builder/dashboard.go)
```go
// NewChart creates a new chart panel builder
func NewChart() PanelBuilder {
    return NewPanel().Type(lens.ChartTypeNewChart)
}
```

### 3. Set Query Format (executor/executor.go)
**CRITICAL**: Add to the switch statement at line ~184:
```go
case lens.ChartTypeBar, lens.ChartTypeColumn, lens.ChartTypeStackedBar, lens.ChartTypeNewChart:
    query.Format = datasource.FormatTable
```
**RULE**: Use `FormatTable` for categorical data, `FormatTimeSeries` for time-based data.

### 4. Handle Chart Conversion (ui/helpers.go)
In `convertLensToChartsType` function:
```go
case lens.ChartTypeNewChart:
    return charts.NewChartType
```

### 5. Add Data Processing (ui/helpers.go)
In `buildChartOptionsFromResult` function:
```go
} else if config.Type == lens.ChartTypeNewChart {
    options.Series = buildNewChartSeriesFromResult(result)
    // Add any specific options
```

### 6. Add Chart-Specific Options (ui/helpers.go)
In `addChartSpecificOptions` function:
```go
case lens.ChartTypeNewChart:
    // Configure chart-specific options
    options.PlotOptions = &charts.PlotOptions{
        // Chart configuration
    }
```

### 7. Add Colors (ui/helpers.go)
In `getChartColors` function:
```go
case lens.ChartTypeNewChart:
    return []string{"#color1", "#color2"}
```

## Query Format Rules

**CRITICAL**: Chart types must use the correct query format:

### FormatTable (Categorical Data)
- `bar`, `column`, `stacked_bar`, `pie`, `table`, `metric`, `gauge`
- All columns mapped to `point.Fields`
- Use for: `SELECT category, series, value FROM ...`

### FormatTimeSeries (Time Data)  
- `line`, `area`
- First column: timestamp, Second: value, Rest: fields/labels
- Use for: `SELECT timestamp, value FROM ...`

## Data Processing Patterns

### Standard Charts (bar, line, area)
```go
func buildSeriesFromResult(result *executor.ExecutionResult) []charts.Series {
    // Extract single value series
    // Expects: label/timestamp, value columns
}
```

### Multi-Series Charts (stacked_bar)
```go
func buildStackedSeriesFromResult(result *executor.ExecutionResult) []charts.Series {
    // Extract multiple series grouped by series name
    // Expects: category, series, value columns
}
```

### Single Value Charts (pie, gauge)
```go
func buildPieSeriesFromResult(result *executor.ExecutionResult) []interface{} {
    // Extract values for pie/gauge charts
    // Returns flat array of values
}
```

## Data Source Development

### PostgreSQL Query Execution Flow

1. **executeTableQuery**: Maps all columns to `Fields`
2. **executeTimeSeriesQuery**: Maps timestamp+value, extras to Fields/Labels

**RULE**: Never modify PostgreSQL data source without understanding both execution paths.

### Column Mapping Rules
```go
// Table format - all columns to Fields
fields[columnName] = value

// TimeSeries format - structured mapping
if i >= 2 { // Additional columns beyond timestamp, value
    if strValue, ok := columnValue.(string); ok {
        labels[columnName] = strValue  // Strings to Labels
    } else {
        fields[columnName] = columnValue  // Numbers to Fields
    }
}
```

## Common Development Patterns

### Multi-Series Chart Pattern
1. Use `FormatTable` format in `executor.go`
2. Query: `SELECT category, series, value FROM ...`
3. Create `buildXxxSeriesFromResult(result)` function that groups by series name

### Chart Option Configuration
```go
// In addChartSpecificOptions
case lens.ChartTypeNewChart:
    options.PlotOptions = &charts.PlotOptions{
        // Chart-specific configuration
    }
    // Position legend if needed
    position := charts.LegendPositionBottom
    options.Legend = &charts.LegendConfig{
        Position: &position,
    }
```

### Color Schemes
Follow the existing color pattern:
- Single series: One color
- Multi-series: Array of complementary colors
- Use hex colors: `#3b82f6`, `#10b981`, etc.

## Critical Dependencies

### Import Rules
- `ui/helpers.go` imports: `charts`, `lens`, `evaluation`, `executor`
- **NEVER** import `datasource` from `ui/`
- **NEVER** import `ui/` from `datasource/`
- `executor` orchestrates between layers

### Chart Component Integration
The lens framework integrates with `components/charts`:
- Lens types convert to Charts types via `convertLensToChartsType`
- Chart options built in `buildChartOptionsFromResult`  
- Final rendering handled by `components/charts/chars.templ`

## Testing and Debugging

### Common Issues & Quick Fixes
1. **Empty Chart**: Check query format in `executor.go` line ~184
2. **No Data Grouping**: Verify query returns `category, series, value` columns
3. **JS Errors**: Run `templ generate && make css` after template changes

### Empty Chart Debugging
**MOST COMMON**: Wrong query format assignment in `executor.go` line ~184
1. Check if chart type is in correct format case (Table vs TimeSeries)
2. Verify query returns expected columns: `category, series, value` (Table) or `timestamp, value` (TimeSeries)
3. Test query directly in database first

## Performance Guidelines

### Query Optimization
- Always include appropriate `ORDER BY` clauses
- Use `LIMIT` for large datasets (handled automatically)
- Avoid N+1 queries in multi-panel dashboards

### Data Processing
- Minimize allocations in series building functions
- Use pre-allocated slices where possible
- Cache color arrays and options objects

## Error Handling Requirements

### Data Source Errors
```go
return &datasource.QueryError{
    Code:    datasource.ErrorCodeSyntax,
    Message: "Description",
    Query:   query,
}
```

### Graceful Degradation
- Charts must render empty when data unavailable
- Log errors but don't crash dashboard
- Provide meaningful error messages in UI

## Code Style Guidelines

1. **Function Naming**: `buildXxxFromResult`, `addXxxOptions`, `convertXxxType`
2. **Constants**: Use lens prefix: `lens.ChartTypeXxx`
3. **Error Messages**: Include context and query information
4. **Comments**: Document expected query formats and column requirements
5. **Type Safety**: Prefer type assertions with ok checks over casting

## Integration Points

### Builder Pattern Integration
Use fluent API for programmatic dashboard creation:
```go
dashboard := builder.NewDashboard("sales-dashboard").
    Panel(builder.StackedBarChart().
        ID("expenses").Title("Monthly Expenses").
        Position(0, 0).Size(6, 4).
        DataSource("postgres").
        Query("SELECT month, category, amount FROM expenses").
        Build()).
    Build()
```

### Controller Integration  
Controllers use builder → executor → renderer flow:
```go
dashboardConfig := c.createDashboard()  // Uses builder pattern
result, err := c.executor.ExecuteDashboard(ctx, dashboardConfig)
```

**RULE**: Builder → Evaluation → Execution → Rendering flow
