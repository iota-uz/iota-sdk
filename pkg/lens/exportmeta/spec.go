// Package exportmeta defines the dependency-free Lens export contract shared
// by dashboards, datasets, panels and JSON specs.
package exportmeta

type Spec struct {
	Enabled          bool     `json:"enabled,omitempty"`
	EvidenceDatasets []string `json:"evidenceDatasets,omitempty"`
	IncludeUpstream  bool     `json:"includeUpstream,omitempty"`
	Filename         string   `json:"filename,omitempty"`
	URL              string   `json:"url,omitempty"`
	// SheetName gives an export-only dataset a stable, human-readable sheet
	// name. TableName turns its primary frame into an Excel table so audit
	// formulas can use structured references instead of fragile cell ranges.
	SheetName    string `json:"sheetName,omitempty"`
	TableName    string `json:"tableName,omitempty"`
	FreezeHeader bool   `json:"freezeHeader,omitempty"`
}
