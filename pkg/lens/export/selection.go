package export

import (
	"fmt"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	lensruntime "github.com/iota-uz/iota-sdk/pkg/lens/runtime"
)

// RequiredEvidence returns exactly the evidence sheets the exporter selects,
// including requested upstream datasets and respecting OmitLineage. It never
// opens a provider. Dataset declarations must be present before selection.
func RequiredEvidence(spec lens.DashboardSpec, panelID string) ([]string, error) {
	if panelID != "" {
		if _, ok := lens.FindPanel(spec, panelID); !ok {
			return nil, fmt.Errorf("panel %q not found", panelID)
		}
	}
	result := &lensruntime.DashboardResult{Spec: spec, Panels: map[string]*lensruntime.PanelResult{}}
	for _, p := range lens.FlattenPanels(spec) {
		result.Panels[p.ID] = &lensruntime.PanelResult{Panel: p, Frames: &frame.FrameSet{}}
	}
	panels := selectedPanels(result, panelID)
	var names []string
	for _, name := range exportDatasetNames(spec, panels, panelID, !spec.Export.OmitLineage) {
		if containsPanelDataset(panels, name) {
			continue
		}
		found := false
		for _, dataset := range spec.Datasets {
			if dataset.Name == name {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("declared export evidence dataset %q is unknown", name)
		}
		names = append(names, name)
	}
	return names, nil
}

func exportDatasetNames(spec lens.DashboardSpec, panels []*lensruntime.PanelResult, panelID string, lineage bool) []string {
	seen := map[string]bool{}
	var order []string
	add := func(name string) bool {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] || (!lineage && findDataset(spec, name).Export.Lineage) {
			return false
		}
		seen[name] = true
		order = append(order, name)
		return true
	}
	if panelID == "" {
		for _, name := range spec.Export.EvidenceDatasets {
			add(name)
			if spec.Export.IncludeUpstream {
				collectUpstream(spec, name, add)
			}
		}
	}
	for _, p := range panels {
		if p == nil || p.Frames == nil {
			continue
		}
		add(p.Panel.Dataset)
		options := datasetExport(spec, p.Panel.Dataset)
		evidence := p.Panel.Export.EvidenceDatasets
		if len(evidence) == 0 {
			evidence = options.EvidenceDatasets
		}
		for _, name := range evidence {
			add(name)
		}
		if p.Panel.Export.IncludeUpstream || options.IncludeUpstream {
			collectUpstream(spec, p.Panel.Dataset, add)
		}
	}
	return order
}
