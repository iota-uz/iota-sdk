# Semantic report plans

The compiler is independent of any assistant and individual reports. Owners register
`Dataset` definitions; BiChat discovers them on demand and executes a strict
`Plan`. New combinations need no report-specific tools or database views.

```json
{
  "dataset": "invoice_sales",
  "version": "sales-v1",
  "mode": "aggregate",
  "period": {"start": "2026-01-01", "end_exclusive": "2026-07-01"},
  "measures": ["net_amount", "record_count"],
  "dimensions": [{"name": "issued_at", "bucket": "month"}],
  "filters": [{"field": "customer", "operator": "not_null"}],
  "order": [{"field": "issued_at"}]
}
```

The compiler adds `currency` and records the normalized plan. Each
currency stays separate. Detail projects measures at source grain; aggregates
support additive sums and population counts. Filters act before grouping.
Output row count means rows/groups in the workbook, not population count.

## Register a source

1. Find the canonical service/query owner and inspect its source-backed metric,
   relevant ADRs and established semantics. Do not rebuild its logic from model
   SQL or an unverified financial target total.
2. Supply parameterized population SQL with `$1` inclusive and `$2` exclusive
   period bounds. Enforce tenant scope, population, join cardinality and
   deduplication. Return one row per declared grain.
3. Declare bounds as `timestamp` (zoned time.Time) or `date` (calendar strings).
   Project date dimensions as DATE or timezone-free local calendar timestamps
   in the declared timezone; UTC instants are not local date dimensions.
4. Declare dimensions and additive measures with types, units and currency
   dimensions. Count needs no source column and is aggregate-only. Missing
   money/currency is a blocker, not an inferred zero or a default currency. No implicit FX
   conversion occurs. Ratios, distinct counts and balances need a verified
   aggregation primitive before registration; sum is not a fallback.
5. Increment the version when its contract changes; register via NewCatalog
   at composition. Verify real PostgreSQL boundaries, nulls, repeated child
   rows and mixed currencies where applicable.

Catalog reads return copies; JSON excludes SourceSQL. Search is bounded and
The discovery tool returns summaries before fetching fields of an exact definition. Catalog
entries grow with business concepts; saved reports are versioned plan JSON.

Execution uses existing readonly authorization and tenant scope. The complete
statement result is fetched once; totals and Excel derive from those rows.
Aggregate statements also return population and missing-measure counts: SUM
cannot silently skip NULL. Diagnostics are checked and stripped before export.
The manifest distinguishes output groups from source records.
The file preserves plan, definition, exact totals and timestamp. Fingerprints
identify plan/SQL and file bytes; rerunning against changed live data does not
promise the same historical snapshot. A registered definition is not financial
statement approval. More than 50,000 output rows or unsafe Excel precision
causes an error without a partial file. Arbitrary joins/formulas are not plan
fields; internal source joins remain controlled by the canonical owner.

## Tests

`GOWORK=off go test ./pkg/reporting` checks validation and discovery over 500
sources. Run `go test ./pkg/bichat/tools/reporting` for saved XLSX, completeness,
precision, same-result totals and refusal-before-query/file checks. Consumers
must also test their source adapters on PostgreSQL and their mounted browser
flow, including artifact persistence and the downloaded workbook.
