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
	Join("product", "LEFT JOIN insurance.products pr ON pr.id = c.product_id").
	ParamLiteral("tenant_id", tenantID).
	Where("(c.tenant_id = @tenant_id OR c.tenant_id IS NULL)").
	Dimension("product", "Product").
		Column("COALESCE(pr.id::text, '')").
		LabelColumn("COALESCE(pr.name, 'Unknown')").
		RequiresJoin("product").
		AsFacet().
	Dimension("period", "Period").
		TimeBucket("c.issue_date", "month").
	Measure("total_policies", "Policies").
		Column("DISTINCT c.id").
		Count().
		Integer().
	Measure("total_premium", "Premium").
		Column("c.premium").
		Sum().
		Money("UZS").
	Layout(
		cube.KPIRow("total_policies", "total_premium"),
		cube.SplitRow(
			cube.Chart("by_product", panel.KindDonut).Dimension("product").Measure("total_premium").TopN(6).WithRemainder(),
			cube.Chart("over_time", panel.KindLine).Dimension("period").Series("product").Measure("total_premium"),
		),
	).
	DrillPath("product", "period").
	LeafSpec(cube.LeafSpec{
		Table: cube.LeafTable("policies_leaf", "Policies").
			WithColumns(
				panel.TableColumn{Field: "product", Label: "Product"},
				panel.TableColumn{Field: "period", Label: "Period"},
				panel.TableColumn{Field: "total_premium", Label: "Premium"},
			).
			WithOrderBy("total_premium DESC").
			WithPageSize(25),
		ExternalLink: func(ctx context.Context, drill cube.DrillContext, params map[string]any) string {
			return "/insurance/contracts?" + drill.Encode().Encode()
		},
	}).
	Build()

dashboard, err := cube.ResolveWithContext(ctx, spec, cube.ParseDrillContextNormalized(r.URL.Query(), ""), "/insurance/sales-report")
```

### Declarative Cube Extensions

- **Dimensions**:
  - `.AsFacet()`: exposes the dimension to `/facet-options` and `/toggle-filter` without duplicating facet logic.
  - `.LabelMap(map[string]string{...})`: maps raw dimension keys to display labels in charts, breadcrumbs, filter chips, and facet options (both SQL `CASE` and Dataset static lookup).
  - `.TimeBucket(column, granularity)`: declares a time-bucketed dimension (`"day"`, `"week"`, `"month"`, `"quarter"`, `"year"`) ordered chronologically; clicking a point narrows the slice via `_f=<dim>:<bucket>`.
  - `.Done()`: returns to the parent `*cube.Builder` (`.Layout`, `.DrillPath`, `.Leaf`, `.LeafSpec` can also be chained directly from dimension/measure builders).
- **Measures**: `.Money(currency)`, `.MoneyExact(currency, precision)`, `.Percent(precision)`, `.Integer()`, `.Done()`.
- **Data modes**: `.SQL(dataSource, fromSQL)`, `.Dataset(frameSet)`, and `.Named(dataSource, queryName)` (`DataModeNamed`, executing a registered named query on a datasource and applying dataset-mode transforms).
- **Layout & DrillPath**:
  - `.Layout(cube.KPIRow(...), cube.SplitRow(cube.Chart(id, kind)...))` declares KPI rows and split chart rows explicitly. `cube.Chart(id, kind)` supports `.Dimension(...)`, `.Series(...)`, `.Measure(...)`, `.TopN(n)`, and `.WithRemainder()`.
  - `.DrillPath(dims...)` defines the ordered drill progression. Every KPI stat card and chart (`Donut`, `Bar`, `HorizontalBar`, `Line`/`TimeSeries`) emits a valid `action.CubeDrill` action preserving period and active `_f` filters, advancing to the next dimension or `_leaf=1`.
- **Typed Leaf**: `.LeafSpec(cube.LeafSpec{URL, Table, ExternalLink})` renders an embedded paginated Lens table at leaf depth and/or resolves a dynamic external link (`LeafLinkResolver`). `.Leaf(url)` preserves the string-URL shorthand.

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

## React document endpoints & `serve.Host`

Mount dashboards with `render/solid.LensDashboard` and serve their data through
`pkg/lens/serve`. Prefer `serve.NewHost(serve.HostConfig{...})` over hand-wiring
`serve.Handlers` + `serve.ReportCache` + `document.SnapshotStore` + custom export
and facet handlers:

```go
host, err := serve.NewHost(serve.HostConfig{
	BasePath:         "/analytics/sales/lens",
	Executor:         engine.New(rt).Executor(),
	ExecutionTimeout: 60 * time.Second,
	Progressive:      true,
	Cache: serve.CacheOptions{
		TTL:               5 * time.Minute,
		MaxEntries:        64,
		Budget:            budget,
		BuildLimiter:      limiter,
		RecomputeCooldown: 5 * time.Second,
	},
	Export: serve.ExportOptions{
		VisibleTableColumnsOnly: true,
		PanelExportFields:       map[string][]string{"summary": {"label", "value"}},
		OmitLineage:             true,
		Authorize:               checkExportPermission,
	},
	ResolveCube: func(ctx context.Context, r *http.Request, drill cube.DrillContext) (serve.ResolvedCube, error) {
		return serve.ResolvedCube{
			Spec:     buildSalesCube(r, drill),
			PagePath: "/analytics/sales",
			Request:  buildRuntimeRequest(r),
		}, nil
	},
})
host.Mount(subRouter) // mounts /document, /query, /panel, /export, /release, /facet-options, /toggle-filter
```

`serve.Host` supports two mutually exclusive modes:

- **`ResolveCube`**: declarative cube dashboards with automatic `_f=dim:val` drill
  parsing (including empty-value normalization to `cube.UnknownDrillValue`),
  parameter-keyed `ReportCache` reuse, `/facet-options`, and `/toggle-filter`.
- **`ResolveDashboard`**: plain/custom `lens.DashboardSpec` dashboards with optional
  `Exploration` + `ResolveLoader` and `DrawerResolver` (`/drawer`).

Key `serve.Host` behaviors:

- **Cache-key-first & cube caching**: configure `HostConfig.ResolveCacheKey`
  (or return a lazy `ResolvedCube.LoadSpec` with optional `CubeCacheKey`) when
  cube/dashboard resolution performs expensive DB work. `Host` then checks
  `ReportCache`, coalesces concurrent requests, and acquires `BuildLimiter`
  admission before invoking the resolver/loader, and reuses the cached cube
  across `/document` drills and `/facet-options` requests (scoped by tenant
  `DataScope`, locale, timezone, and variant).
- **Recompute cooldown**: when `/panel` receives `recompute=true` within
  `RecomputeCooldown` (e.g. a global recompute immediately after initial load),
  `Host.Panel` waits out the remaining cooldown window instead of failing panels,
  and sets `Content-Encoding: identity` so streaming NDJSON flushes immediately.
  Executors can inspect `serve.RecomputeRequested(ctx)` to bypass upstream caches.
- **Export policy**: `ExportOptions` handles progressive panel materialization,
  per-export `Authorize`, `VisibleTableColumnsOnly`, `PanelExportFields`,
  `OmitLineage`, and download-cookie signaling (`lensexport.SetDownloadSignal`).
- **Low-level handlers**: existing `serve.New(serve.Config{...})` and `serve.ReportCache`
  remain fully supported and unchanged for specialized hosts.

The React runtime owns loading, error, retry, drill, filter, and export states;
hosts do not render parallel HTML fragments. Use `runtime.PanelScope(panelID)`
only inside a generic document/query mechanism when one panel can be executed
independently. Do not add renderer-specific panel URLs or reintroduce `PanelBasePath`.

## Document chrome, date ranges and themes

- `serve.Config.DocumentOptions` returns the per-request `document.BuildOptions`
  (drawer header, page header, translations, theme). It receives the request the
  engine runs, so a host reads the same range and tenant the document uses.
  `BuildOptions.Filters`, when non-nil, replaces the filters derived from the
  dashboard's variables.
- A `date_range` variable (`build.DateRangeVariable`) becomes a period filter
  automatically. Bind it into SQL with `lens.RangeParam(variable, part)`:
  `RangeStart`, `RangeEnd` (inclusive) or `RangeUntil` (the first instant after
  the range, for `< @until`). An all-time range resolves to NULL.
- `LensDashboard` mounted with `solid.WithTheme(solid.ThemeAuto)` follows the
  `dark` class on `<html>` (or the system scheme when the page declares
  neither); `ThemeLight` and `ThemeDark` stay fixed.

## Explorer levels

- A level draws a table of source records with
  `explore.PanelNode(key, label, tableSpec).WithView(panel.KindTable)`. It is
  terminal, receives the selected path like a chart level, paginates, and ships
  its columns, formats and row action as `Level.table`.
- A column declared with `Total: true` gets a footer under the level table. The
  `/lens/query` response carries it as `summary`. When the host returned every
  row of the selection, Lens sums them; when the host loads one page at a time
  (`TablePagination` with more rows or a page after the first), the page's sum
  is not the selection's, so the footer shows a total only for columns the host
  supplies with `runtime.ApplyTableTotals(result, panelID, map[string]float64{"amount": total})`
  and the other totalled columns show an em dash.
- `DynamicChildren.Action` URLs substitute only `{name}` placeholders matching
  the action's own params; params are never appended to the address.
- Colour a company's own categories from `color.Categories()` (twelve colours
  without red, grey or green) and use `color.Positive`, `color.Negative` and
  `color.Neutral` for meaning. `SemanticColors("literal", field)` on a bar or
  horizontal bar paints each row in its own colour without `ColorByCategory`.

## Runtime translations

Chrome text (including every `aria-label`, `title` and screen-reader-only string)
is looked up as host `document.i18n`, then the runtime's built-in catalogue for
the document locale (`web/lens/src/runtime/messages.ts`: `ru`, `uz`), then the
English call-site text. A new key needs a Go constant in `document/i18n.go`, its
entry in `RuntimeI18nKeys`, and `ru` and `uz` entries in `messages.ts`.
`serve` seeds `document.i18n` with `RuntimeI18nDefaults()` only for locales the
runtime cannot translate itself (`RuntimeI18nDefaultsFor`); seeding English into
a `ru` document would hide the built-in Russian text. A host that passes its own
`BuildOptions.I18n` replaces the seed and owns the wording.

## Performance Profiling

Runtime payload preflights follow `runtime.Options.MaxEntryBytes`. When omitted,
the runtime inherits `Store.PayloadLimit()` if exposed by the store (including
`MemorySnapshotStore`); otherwise it uses the SDK default. A smaller explicit
store bound wins. Configure the document store and report cache with the same
host policy. This does not change the shared retained budget or small identity
metadata limits, and a larger allowed entry does not grant extra shared capacity.

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
