# Lens Dashboard Framework

React runtime, story, fixture, and visual-regression work follows
[`web/lens/CLAUDE.md`](../../web/lens/CLAUDE.md). A visible runtime surface is
not complete until its stories and VR coverage are included.

Lens supports two patterns:

- Manual `lens.DashboardSpec`: use when the layout and datasets are bespoke and there is no multi-level drill flow.
- `cube.New(...)`: use when you want a dashboard that starts with KPIs plus dimension panels and drills through ordered `_f=dimension:value` state.

## Manual DashboardSpec

Use plain `lens.DashboardSpec` when:

- Panels are unrelated to one another
- You already know the exact dataset graph you want
- Drill-through is row/panel specific instead of hierarchical cube drilling
- A dashboard is mostly one-off presentation work

### Native cascade panels

Use `spec.Cascade(id, title, dataset)` or `panel.Cascade(id, title, dataset)`
for bridge/waterfall-style running totals. The primary dataset rows should contain:

- `label`: stage label
- `value`: running total after the stage
- `cut`: amount deducted to reach this stage (optional for the first row)
- `cutLabel`: connector label for the deduction
- `final`: boolean that renders the stage with success styling

Override fields with `LabelField`, `ValueField`, `CutField`, `CutLabelField`,
and `FinalField` when a dataset uses different column names.

### Radial panels

Use `spec.RadialBars(id, title, dataset, maximum)` for several labeled values
measured against one explicit maximum. The maximum is mandatory and uses the
same unit as `ValueField`; Lens never assumes a percentage scale.

Use `spec.MultiRingDonut(id, title, dataset, rings...)` when several rings are
different decompositions of the same headline:

```go
spec.MultiRingDonut("mix", "Portfolio mix", "mix",
    panel.RadialRing{Key: "actual", Label: "Actual", Order: 1, Total: 100},
    panel.RadialRing{Key: "plan", Label: "Plan", Order: 2, Total: 100},
).
    IDField("category_key").
    LabelField("category_label").
    SeriesField("ring_key").
    ValueField("amount").
    RadialTolerance(0.01)
```

The frame is tidy: one row per `(ring_key, category_key)`. Category keys are
stable across rings when the same category should keep one color. Every ring
declares its authoritative total and its rows must reconcile within the
configured tolerance. Lens does not invent or normalize a residual; producers
must emit an explicit `Other` row when that is the intended business statement.
Mark actions receive the selected row, while `panel.RadialNodeKey(ring, category)`
provides the stable composite identity used by drill graphs.

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

Use measure `.Override(...)` when a stat card needs an authoritative dataset that differs from the cube's regular aggregate source. For example, a dashboard may render charts from a bounded frame while a KPI card uses an exact precomputed count:

```go
.Measure("total_policies", "Total Policies").
	Count().
	Override(lens.DatasetSpec{
		Kind: lens.DatasetKindStatic,
		Static: exactTotalPoliciesFrame,
	})
```

Important behavior:

- Override datasets automatically inherit cube params
- Override datasets automatically receive active drill filters as `@f_<dimension>`
- In SQL cubes, override query datasets inherit the cube datasource if they do not set one explicitly
- Dimension overrides back dimension panels; measure overrides back only the corresponding stat panel

That means an override query can safely reference params such as:

- `@tenant_id`
- `@issue_at_from`
- `@issue_at_to`
- `@f_product`
- `@f_region`

Drill-filter params (`@f_<dimension>`) are bound as Postgres text arrays so multi-select drills work, so override SQL must use array semantics — e.g. `WHERE pr.id::text = ANY(@f_product)` rather than `= @f_product`.

If a dimension can be drilled further, still give it a valid `.Column(...)` filter expression so later drill levels and KPI stats can apply that filter outside the override dataset.

## Drill URL Format

Cube drill state is encoded in ordered repeated query params:

```text
_f=product:osago&_f=region:tashkent
```

Rules:

- Order matters
- Each `_f` entry is one drill level
- Lens preserves `_f` values when document requests and filter controls re-emit the current query inputs
- Leaf routes should parse `cube.ParseDrillContext(r.URL.Query())`

## Date Range + Drill

When a drilled page also has external date filters:

- Preserve drill params with hidden inputs
- Exclude only the date keys when rebuilding hidden query state
- Treat empty results after a date-range change as a valid empty state, not an error

Both report controllers in this repo follow that pattern through `persistentDashboardQuery(...)`.

## React document endpoints

Mount dashboards with `render/react.LensDashboard` and serve their data through
`pkg/lens/serve`. Register the document, query, and export handlers below the
same authenticated middleware chain and supply a shared, bounded
`document.SnapshotStore`. The React runtime owns loading, error, retry, drill,
filter, and export states; hosts do not render parallel HTML fragments.

Use `runtime.PanelScope(panelID)` only inside a generic document/query mechanism
when one panel can be executed independently. Do not add renderer-specific panel
URLs or reintroduce `PanelBasePath`.

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
- Run `templ generate` after changing the React mount template
- Prefer cube mode for hierarchical drill dashboards, not for every dashboard
- Keep leaf routes responsible for translating drill filters into repository/query params
