package document

// PanelKindMetadata is the renderer-neutral catalogue of kind-specific wire
// fields. Fields absent from ConfigFields belong to every panel kind.
type PanelKindMetadata struct {
	Semantics    Semantics
	ConfigFields []string
}

var panelKindMetadata = map[PanelKind]PanelKindMetadata{
	PanelKindStat:               {Semantics: SemanticsSeries},
	PanelKindPie:                {Semantics: SemanticsPartition},
	PanelKindDonut:              {Semantics: SemanticsPartition},
	PanelKindRadial:             {Semantics: SemanticsSeries, ConfigFields: []string{"radial"}},
	PanelKindBar:                {Semantics: SemanticsSeries},
	PanelKindHBar:               {Semantics: SemanticsSeries},
	PanelKindLine:               {Semantics: SemanticsSeries},
	PanelKindArea:               {Semantics: SemanticsSeries},
	PanelKindGauge:              {Semantics: SemanticsSeries, ConfigFields: []string{"radial"}},
	PanelKindHistogram:          {Semantics: SemanticsSeries},
	PanelKindBoxPlot:            {Semantics: SemanticsSeries},
	PanelKindHeatmap:            {Semantics: SemanticsSeries},
	PanelKindMap:                {Semantics: SemanticsSeries, ConfigFields: []string{"map"}},
	PanelKindCascade:            {Semantics: SemanticsReconciliation},
	PanelKindTable:              {Semantics: SemanticsEvidence, ConfigFields: []string{"columns", "table"}},
	PanelKindCoverage:           {Semantics: SemanticsPartition},
	PanelKindMetricFlow:         {Semantics: SemanticsReconciliation, ConfigFields: []string{"metricFlow"}},
	PanelKindMetricHierarchy:    {Semantics: SemanticsSeries, ConfigFields: []string{"metricHierarchy"}},
	PanelKindMetricRelationship: {Semantics: SemanticsSeries, ConfigFields: []string{"metricRelationship"}},
}

// PanelKinds returns a copy of the canonical runtime kind catalogue.
func PanelKinds() map[PanelKind]PanelKindMetadata {
	result := make(map[PanelKind]PanelKindMetadata, len(panelKindMetadata))
	for kind, metadata := range panelKindMetadata {
		metadata.ConfigFields = append([]string(nil), metadata.ConfigFields...)
		result[kind] = metadata
	}
	return result
}

func inferPanelSemantics(kind PanelKind) Semantics {
	if metadata, ok := panelKindMetadata[kind]; ok {
		return metadata.Semantics
	}
	return SemanticsSeries
}
