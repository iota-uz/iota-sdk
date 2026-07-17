// Package exportmeta defines the dependency-free Lens export contract shared
// by dashboards, datasets, panels and JSON specs.
package exportmeta

type Spec struct {
	Enabled          bool     `json:"enabled,omitempty"`
	EvidenceDataset  string   `json:"evidenceDataset,omitempty"`
	EvidenceDatasets []string `json:"evidenceDatasets,omitempty"`
	IncludeUpstream  bool     `json:"includeUpstream,omitempty"`
	Filename         string   `json:"filename,omitempty"`
	URL              string   `json:"url,omitempty"`
}
