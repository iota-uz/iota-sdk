# Lens Dashboard Framework

Row-based dashboard framework: Build → Execute → Render.

## Architecture

```
pkg/lens/
├── lens.go          # Core types: Dashboard, Row, Panel, PanelType
├── builder.go       # Fluent API: Metric(), Line(), Bar(), Table(), etc.
├── datasource.go    # DataSource interface, QueryResult
├── executor.go      # Execute() — runs all panel queries concurrently
├── postgres/
│   └── postgres.go  # PostgreSQL DataSource (pgxpool)
└── ui/
    ├── dashboard.templ  # Dashboard + Row rendering
    ├── row.templ        # 12-column grid row with panel dispatch
    ├── metric.templ     # Metric cards
    ├── chart.templ      # Chart panels (delegates to components/charts)
    ├── table.templ      # Data tables with drill-through
    ├── states.templ     # Empty, Error, Loading states
    └── helpers.go       # QueryResult → chart series/metric transformations
```

## Quick Start

```go
// 1. Build dashboard
dash := lens.NewDashboard("Finance Overview",
    lens.NewRow(
        lens.Metric("revenue", "Revenue").Query("SELECT SUM(amount) as value FROM orders").Unit("USD").Span(3).Build(),
        lens.Metric("orders", "Orders").Query("SELECT COUNT(*) as value FROM orders").Span(3).Build(),
    ),
    lens.NewRow(
        lens.Line("trend", "Revenue Trend").Query("SELECT date as label, SUM(amount) as value FROM orders GROUP BY 1").Span(8).Build(),
        lens.Pie("categories", "By Category").Query("SELECT category as label, SUM(amount) as value FROM orders GROUP BY 1").Span(4).Build(),
    ),
)

// 2. Execute queries
results := lens.Execute(ctx, dataSource, dash)

// 3. Render (in templ)
@lensui.Dashboard(dash, results)
```

## Panel Types & Query Formats

| Type | Constructor | Expected Columns |
|------|-------------|-----------------|
| metric | `Metric()` | `value` (single row) |
| line | `Line()` | `label`, `value` |
| bar | `Bar()` | `label`, `value` |
| stacked_bar | `StackedBar()` | `category`, `series`, `value` |
| pie/donut | `Pie()`/`Donut()` | `label`, `value` |
| area | `Area()` | `label`, `value` |
| gauge | `Gauge()` | `value` (single row, 0–100) |
| table | `Table()` | any columns |

## Drill-Through (HTMX)

```go
// Chart click → full navigation
lens.Bar("sales", "Sales").DrillTo("/analytics?category={label}").Build()

// Chart click → HTMX partial update
lens.Bar("sales", "Sales").DrillToTarget("/analytics?category={label}", "#detail").Build()

// Table row click → navigation
lens.Table("orders", "Orders").DrillTo("/orders/{id}").Build()
```

## Adding New Panel Types

1. Add constant in `lens.go`: `TypeNewType PanelType = "new_type"`
2. Add constructor in `builder.go`: `func NewType(id, title string) *PanelBuilder`
3. Handle in `ui/helpers.go`: add to `panelTypeToChartType()`, `panelColors()`
4. If needed, add templ component in `ui/` and dispatch case in `row.templ`

## Critical Rules

- **Query column conventions**: Use `label`/`category` for x-axis, `value` for y-axis, `series` for stacking
- **Import rule**: `ui/` may import `lens` root, never the reverse
- **Template changes**: Always run `templ generate && just css`
- **Never read `*_templ.go` files** — they're generated
