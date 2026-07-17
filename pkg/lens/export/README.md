# Lens Excel export

`export.Exporter` produces a multi-sheet workbook directly from an executed
`runtime.DashboardResult`; it never queries a datasource. Dashboard-level
`ExportSpec` enables automatic export actions for every supported leaf panel.

Manual aggregate datasets declare an `EvidenceDataset`. A whole-dashboard
export can declare `EvidenceDatasets`; upstream frames can be included with
`IncludeUpstream`. This is explicit because arbitrary aggregate SQL cannot be
safely reversed into contributing records.

The workbook includes summary, current chart data, evidence breakdown,
parameters and safe dataset lineage. Use `export.Handler` when a standard HTTP
endpoint is desired, or call `Exporter.Write` from an existing authorized
route.
