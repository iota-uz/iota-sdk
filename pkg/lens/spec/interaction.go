package spec

// FinalizeInteractionContract marks every non-actionable dashboard leaf as an
// intentional terminal panel. Call it after attaching all actions and drill
// paths. Explorer hosts remain non-terminal because compilation supplies
// their interaction path from the explorer graph.
func FinalizeInteractionContract(document Document) Document {
	explorerHosts := make(map[string]struct{}, len(document.Explorers))
	for _, explorer := range document.Explorers {
		explorerHosts[explorer.HostPanelID] = struct{}{}
	}
	var finalize func(*PanelSpec)
	finalize = func(candidate *PanelSpec) {
		for index := range candidate.Children {
			finalize(&candidate.Children[index])
		}
		if len(candidate.Children) > 0 || candidate.Terminal || panelIsActionable(candidate) {
			return
		}
		if _, explorerHost := explorerHosts[candidate.ID]; explorerHost {
			return
		}
		candidate.Terminal = true
	}
	for rowIndex := range document.Rows {
		for panelIndex := range document.Rows[rowIndex].Panels {
			finalize(&document.Rows[rowIndex].Panels[panelIndex])
		}
	}
	return document
}

func panelIsActionable(candidate *PanelSpec) bool {
	if candidate.Action != nil || candidate.DrillTree != nil {
		return true
	}
	for _, column := range candidate.Columns {
		if column.Action != nil {
			return true
		}
	}
	for _, stage := range candidate.FlowStages {
		if stage.Action != nil {
			return true
		}
	}
	for _, row := range candidate.HierarchyRows {
		if row.Action != nil {
			return true
		}
	}
	return candidate.Relationship != nil &&
		(candidate.Relationship.Source.Action != nil || candidate.Relationship.Target.Action != nil)
}
