# Lens Dashboard Framework

Lens supports two patterns:

- Manual `lens.DashboardSpec`: use when the layout and datasets are bespoke and there is no multi-level drill flow.
- `cube.New(...)`: use when you want a dashboard that starts with KPIs plus dimension panels and drills through ordered `_f=dimension:value` state.

## Manual DashboardSpec

Use plain `lens.DashboardSpec` when:

- Panels are unrelated to one another
- You already know the exact dataset graph you want
- Drill-through is row/panel specific instead of hierarchical cube drilling
- A dashboard is mostly one-off presentation work

## Cube Pattern

Use `cube.New(...)` when:

- The dashboard is a KPI summary plus a set of dimensions
- Each click should narrow the same analytical slice
- Raw leaf views should receive accumulated drill context
- You want Lens to build breadcrumbs, remaining-dimension pills, and chart drill actions for you

Basic flow:

```go
spec := cube.New("sales", "Sales Overview").
	SQL("primary", "insurance.contracts c").
	ParamLiteral("tenant_id", tenantID).
	Where("(c.tenant_id = @tenant_id OR c.tenant_id IS NULL)").
	Dimension("product", "Product").
	Column("COALESCE(pr.id::text, '')").
	LabelColumn("COALESCE(pr.name, 'Unknown')").
	RequiresJoin("product").
	Measure("total_policies", "Policies").
	Column("DISTINCT pol.id").
	Count().
	DefaultDimension("product").
	Leaf("/insurance/sales-report/drill/contracts").
	Build()

dashboard, err := cube.Resolve(spec, cube.ParseDrillContext(r.URL.Query()), "/insurance/sales-report")
```

## Data Modes

### SQL mode

Use `.SQL(dataSource, fromSQL)` when the source of truth is the database and drill levels should be translated into SQL filters.

Best for:

- Production analytics over large tables
- Dimensions that rely on joins
- Leaf routes backed by repositories or raw queries

Rules:

- Add cube-wide params with `.ParamLiteral(...)` or `.ParamVariable(...)`
- Add shared filters with `.Where(...)`
- Add reusable joins with `.Join(name, sql)`
- Use `.RequiresJoin(...)` on dimensions/measures so Lens only brings in the joins it needs

### Dataset mode

Use `.Dataset(frameSet)` when the data already exists in memory and drill levels should be resolved with frame transforms.

Best for:

- Dashboards built from already-fetched rows
- Small or medium datasets
- Places where SQL would duplicate upstream query logic

## Adding a Dimension

For a normal dimension, one `.Dimension(...)` block is usually enough:

```go
.Dimension("region", "Region").
	Column("COALESCE(r.id::text, '')").
	LabelColumn("COALESCE(r.name, 'Unknown')").
	PanelKind(panel.KindHorizontalBar).
	RequiresJoin("region")
```

Checklist:

1. Choose a stable filter value expression for `.Column(...)`
2. Choose a display label for `.LabelColumn(...)` or `.LabelField(...)`
3. Add any required join with `.RequiresJoin(...)`
4. Pick a reasonable panel kind and colors
5. Make sure the raw leaf route knows how to consume the drilled value

## Override Escape Hatch

Use `.Override(...)` when a dimension cannot be expressed as a simple `GROUP BY column`, for example:

- Age bands
- Complex CTE-backed buckets
- Multi-stage ranking queries
- Pre-aggregated custom SQL

Example:

```go
.Dimension("age_group", "Age Group").
	Column(ageGroupFilterExpr()).
	RequiresJoin("insurant_person").
	Override(lens.DatasetSpec{
		Kind: lens.DatasetKindQuery,
		Query: &lens.QuerySpec{
			Text: ageDistributionOverrideQuery(),
			Kind: datasource.QueryKindRaw,
		},
	})
```

Important behavior:

- Override datasets automatically inherit cube params
- Override datasets automatically receive active drill filters as `@f_<dimension>`
- In SQL cubes, override query datasets inherit the cube datasource if they do not set one explicitly

That means an override query can safely reference params such as:

- `@tenant_id`
- `@issue_at_from`
- `@issue_at_to`
- `@f_product`
- `@f_region`

If a dimension can be drilled further, still give it a valid `.Column(...)` filter expression so later drill levels and KPI stats can apply that filter outside the override dataset.

## Drill URL Format

Cube drill state is encoded in ordered repeated query params:

```text
_f=product:osago&_f=region:tashkent
```

Rules:

- Order matters
- Each `_f` entry is one drill level
- Lens preserves `_f` values during HTMX/filter interactions as long as the filter form re-emits hidden query inputs
- Leaf routes should parse `cube.ParseDrillContext(r.URL.Query())`

## Date Range + Drill

When a drilled page also has external date filters:

- Preserve drill params with hidden inputs
- Exclude only the date keys when rebuilding hidden query state
- Treat empty results after a date-range change as a valid empty state, not an error

Both report controllers in this repo follow that pattern through `persistentDashboardQuery(...)`.

## Async Shell + Panel Fragments

Lens supports shell-first dashboard rendering when a page wants to lazy load individual panels.

Use this pattern when:

- Panels are backed by Lens runtime execution and can be scoped cheaply with `runtime.PanelScope(panelID)`
- The page already has a Lens spec before execution
- Fetching one panel is meaningfully cheaper than fetching the entire dashboard

Do not use this pattern blindly for dashboards that are built from already-materialized report data or manual in-memory datasets. In those cases, turning one request into N panel requests often makes the page slower, not faster.

Controller contract:

1. Prepare the dashboard spec first with `engine.Prepare(...)` or `engine.Prepare` via existing page helpers
2. On the initial non-HTMX request, render the shell with:
   - `Result: nil`
   - `Spec: prepared.Spec`
   - `AsyncProps.PanelBasePath`
   - `AsyncProps.FilterFormID`
3. Expose a panel fragment route such as `GET /reports/panels/{panel_id}`
4. In that fragment route, execute only the requested panel with `runtime.PanelScope(panelID)`
5. Return the fragment with `render/templ.RenderPanelFragment(...)`

Template contract:

- Pass `DashboardProps{Spec: spec, Async: &templ.AsyncProps{...}}` to `render/templ.Dashboard(...)` for the main area
- Use `render/templ.PanelShell(...)` for sidebar cards or any panel rendered outside the main dashboard grid
- Preserve drill/query state with hidden inputs so `_f=...` and non-date state survive HTMX updates

Reference implementation:

- `modules/insurance/presentation/controllers/sales_report_controller.go`
- `modules/insurance/presentation/templates/pages/sales_report/lens.templ`

Current helper:

- `render/templ.RenderPanelFragment(...)` renders a single panel body from a fully prepared `runtime.Result`

## Performance Profiling

Cube SQL is generated dynamically, so profiling is part of the implementation work for production-facing dashboards.

Before calling a cube rollout done:

1. Capture the generated SQL for stats and each drill level
2. Run `EXPLAIN ANALYZE` against production-scale data
3. Check filter/join indexes for high-use dimensions
4. Measure end-to-end response time at each drill level
5. Watch high-cardinality dimensions and cap or reshape them if charts become noisy

This repo can verify correctness locally, but true performance sign-off still requires a real dataset and environment.

## Critical Rules

- Keep generated `*_templ.go` files out of manual edits
- Run `templ generate` after Templ changes
- Prefer cube mode for hierarchical drill dashboards, not for every dashboard
- Keep leaf routes responsible for translating drill filters into repository/query params
