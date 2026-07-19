# Lens Excel export

`export.Exporter` produces a multi-sheet workbook directly from an executed
`runtime.DashboardResult`; it never queries a datasource. Dashboard-level
`ExportSpec` enables automatic export actions for every supported leaf panel.

Manual aggregate datasets declare `EvidenceDatasets`. A whole-dashboard
export can declare `EvidenceDatasets`; upstream frames can be included with
`IncludeUpstream`. This is explicit because arbitrary aggregate SQL cannot be
safely reversed into contributing records.

The workbook includes summary, current chart data, evidence breakdown,
parameters and safe dataset lineage. Use `export.Handler` when a standard HTTP
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
