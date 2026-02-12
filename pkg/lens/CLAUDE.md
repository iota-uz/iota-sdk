# Lens Dashboard Framework

Dashboard framework with layered architecture: Configuration → Evaluation → Execution → Rendering.

## Architecture

```
pkg/lens/
├── types.go              # Core types - START HERE
├── builder/              # Fluent API for dashboard creation
├── evaluation/           # Layout & config processing
├── executor/             # Query execution (line ~184 critical!)
├── datasource/           # Data abstraction - NEVER import ui/
└── ui/                   # Rendering - consumes evaluated configs
```

**RULE**: Never import `datasource` from `ui/` or `ui` from `datasource/`.

## Adding New Chart Types

### 1. Define Type (`types.go`)
```go
const ChartTypeNewChart ChartType = "new_chart"
```

### 2. Add Builder (`builder/dashboard.go`)
```go
func NewChart() PanelBuilder {
    return NewPanel().Type(lens.ChartTypeNewChart)
}
```

### 3. Set Query Format (`executor/executor.go:184`)
```go
case lens.ChartTypeBar, lens.ChartTypeNewChart:
    query.Format = datasource.FormatTable  // or FormatTimeSeries
```

**CRITICAL**: Use `FormatTable` for categorical data, `FormatTimeSeries` for time-based.

### 4. Handle Conversion (`ui/helpers.go`)
```go
case lens.ChartTypeNewChart:
    return charts.NewChartType
```

### 5. Add Data Processing (`ui/helpers.go`)
```go
} else if config.Type == lens.ChartTypeNewChart {
    options.Series = buildNewChartSeriesFromResult(result)
}
```

### 6. Add Chart Options (`ui/helpers.go`)
```go
case lens.ChartTypeNewChart:
    options.PlotOptions = &charts.PlotOptions{...}
```

### 7. Add Colors (`ui/helpers.go`)
```go
case lens.ChartTypeNewChart:
    return []string{"#3b82f6", "#10b981"}
```

## Query Format Rules

| Format | Chart Types | Query Pattern |
|--------|-------------|---------------|
| **FormatTable** | bar, column, stacked_bar, pie, table, metric, gauge | `SELECT category, series, value FROM ...` |
| **FormatTimeSeries** | line, area | `SELECT timestamp, value FROM ...` |

## Data Processing Patterns

### Standard Charts (single series)
```go
func buildSeriesFromResult(result *executor.ExecutionResult) []charts.Series {
    // Expects: label/timestamp, value columns
}
```

### Multi-Series (stacked_bar)
```go
func buildStackedSeriesFromResult(result *executor.ExecutionResult) []charts.Series {
    // Expects: category, series, value columns
    // Groups by series name
}
```

### Single Value (pie, gauge)
```go
func buildPieSeriesFromResult(result *executor.ExecutionResult) []interface{} {
    // Returns flat array of values
}
```

## PostgreSQL Column Mapping

**FormatTable**: All columns → `point.Fields`
```go
fields[columnName] = value
```

**FormatTimeSeries**: Structured mapping
```go
// col 0: timestamp, col 1: value
col 2+: if string → labels[columnName], else → fields[columnName]
```

## Critical Gotchas

1. **Empty Chart**: Most common cause is wrong query format in `executor.go:184`. Verify chart type is in correct case (Table vs TimeSeries).

2. **No Data Grouping**: Query must return `category, series, value` columns for multi-series charts.

3. **Import Cycles**: Never import `datasource` from `ui/` or vice versa. `executor` orchestrates between layers.

4. **Template Changes**: Always run `templ generate && just css` after modifying .templ files.

5. **Query Timeout**: Default 30s. Long queries need optimization or pagination.

## Testing & Debugging

```bash
# Common fixes
templ generate && just css              # After template changes
go vet ./...                            # Check types

# Debug empty charts
1. Check executor.go:184 - is chart type in correct format case?
2. Verify query returns expected columns
3. Test query directly in database
```

## Code Style

- Function naming: `buildXxxFromResult`, `addXxxOptions`, `convertXxxType`
- Constants: `lens.ChartTypeXxx`
- Type safety: Use `value, ok := column.(type)` assertions
