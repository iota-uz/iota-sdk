# Lens Excel export

`export.Exporter` produces a multi-sheet workbook directly from an executed
`runtime.DashboardResult`; it never queries a datasource. Dashboard-level
`ExportSpec` enables automatic export actions for every supported leaf panel.

Manual aggregate datasets declare `EvidenceDatasets`. A whole-dashboard
export can declare `EvidenceDatasets`; upstream frames can be included with
`IncludeUpstream`. This is explicit because arbitrary aggregate SQL cannot be
safely reversed into contributing records.

The workbook includes summary, current chart data, evidence breakdown,
parameters and safe dataset lineage. The summary lists every single-value
(stat/gauge) panel in one metric/value table, followed by the remaining panels.
Numeric cells keep their values and get an Excel number format from the panel
`Formatter`, a table column `Formatter` or the field `FormatterHint`. Percent
values are already scaled (`45.3` is 45.3%). Parameters show the dashboard's
declared variables by label with humanized values. Set
`ExportSpec.OmitLineage` to drop the technical lineage sheet and datasets marked `Export.Lineage` (such as report filter context). Use `export.Handler` when a standard HTTP
endpoint is desired, or call `Exporter.Write` from an existing authorized
route.

## Audit workbooks

Evidence datasets can use `ExportSpec.SheetName`, `TableName` and
`FreezeHeader` to become stable Excel tables. Cells containing `frame.Formula`
remain real, recalculable Excel formulas; `frame.Hyperlink` produces a clickable
source-record link. A panel may declare several ordered evidence datasets, for
example a formula/reconciliation sheet followed by policy, transaction and
accounting tables. Lens forces a full recalculation when the workbook opens.

These primitives intentionally keep business formulas and source selection in
the consumer's declarative evidence model while keeping authorization,
workbook generation, sheet safety and the download endpoint canonical.

## Streaming evidence and atomic downloads

For large evidence tables, keep metadata-only dataset declarations in
`Result.Spec.Datasets` and provide `Request.EvidenceSources` keyed by dataset
name. Each `EvidenceSource` has `Fields []frame.Field` (without `Values`),
optional `Meta frame.FrameMeta`, and
`Rows func(context.Context, func([]any) error) error`. Sources are opened only
when selected by the existing export evidence/upstream/lineage rules, once and
sequentially. `RequiredEvidence(spec, panelID)` exposes that same selection.
Callbacks must honor cancellation, defer cursor cleanup, and return yield
errors. They may reuse a row after yield returns. Values keep their native
types; explicit `frame.Formula` and `frame.Hyperlink` remain Excel formulas.
Exact decimal/Numeric values reuse `excel.NormalizeNumeric` precision checks;
existing Lens float ratios retain their approximate-number contract.

```go
request.EvidenceSources = map[string]export.EvidenceSource{
    "policy_rows": {
        Fields: []frame.Field{{Name: "policy"}, {Name: "premium", Type: frame.FieldTypeNumber}},
        Rows: func(ctx context.Context, yield func([]any) error) error {
            // Open an authorized cursor against the frozen report snapshot.
            // Defer Close, check rows.Err, and propagate yield errors here.
            return walkPolicyRows(ctx, yield)
        },
    },
}
request.Spool = &export.SpoolOptions{Directory: exportTempRoot, Budget: sharedLensBudget}
err := export.New().WriteHTTP(ctx, response, "report.xlsx", request)
```

`Prepare` returns a complete `PreparedWorkbook` for callers needing to set a
download signal before `WriteHTTP`. Always close the prepared artifact; it is
request-owned and must never be retained in a report cache. Workbook generation
errors occur before download headers or bytes. A network failure while copying
a completed artifact can still interrupt a download.

The private workspace is removed on success, cancellation and failure. Its
default per-request combined XML/archive bound is 256 MiB; `MaxBytes`,
`MaxRows` and `MaxRowBytes` are configurable. Excel limits are enforced without
truncation. The row bound defaults to Excel's capacity, the row-size bound to
1 MiB. Disk accounting charges a conservative escaped-XML bound before rows
are written and the actual ZIP output before each write. This is a per-request
disk bound, not a process-wide disk quota: hosts should also bound concurrent
exports. No interactive payload/cache limit is raised.

Pass the host's shared `resource.Budget` for working-memory admission. Small
sheets reserve growth in MiB steps; large sheets reserve up to twice Excelize's
16 MiB stream chunk plus row/schema allowances. Reservations stay until the
workbook closes because reset buffers can retain capacity. These are
conservative accounting units, not a hard Go heap guarantee. ZIP output uses
Excelize's public ZIP-writer hook to avoid its default whole-archive buffer.
