package cube

import (
	"fmt"
	"slices"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/comparison"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/transform"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/sirupsen/logrus"
)

const (
	statsDatasetNamePrefix = "cube_stats"
	statDatasetNamePrefix  = "cube_stat"
	dimDatasetNamePrefix   = "cube_dim"
)

type dimensionDatasetResolution struct {
	Name          string
	Datasets      []lens.DatasetSpec
	HasColorValue bool
	Compared      bool
}

type statDatasetResolution struct {
	Datasets         []lens.DatasetSpec
	DatasetByMeasure map[string]string
}

func datasetDimensionHasColorValue(dim DimensionSpec) bool {
	colorField := strings.TrimSpace(dim.ColorField)
	if colorField == "" {
		return false
	}
	lookupSource := strings.TrimSpace(dim.Field)
	if lookupSource == "" {
		lookupSource = strings.TrimSpace(dim.LabelField)
	}
	return colorField != lookupSource
}

func resolvedDimensionTransforms(spec CubeSpec, transformsIn []transform.Spec) []transform.Spec {
	if len(transformsIn) == 0 {
		return nil
	}
	additiveByField := make(map[string]bool, len(spec.Measures))
	for _, measure := range spec.Measures {
		switch measure.Aggregation {
		case AggregationCount, AggregationSum:
			additiveByField[measure.Name] = true
		case AggregationAvg:
			additiveByField[measure.Name] = false
		}
	}
	out := make([]transform.Spec, len(transformsIn))
	for i, specTransform := range transformsIn {
		out[i] = specTransform
		if specTransform.TopN == nil {
			continue
		}
		cfg := *specTransform.TopN
		if len(additiveByField) > 0 {
			cfg.AdditiveFields = make(map[string]bool, len(additiveByField))
			for field, additive := range additiveByField {
				cfg.AdditiveFields[field] = additive
			}
		}
		out[i].TopN = &cfg
	}
	return out
}

func Resolve(spec CubeSpec, ctx DrillContext, baseURL string) (lens.DashboardSpec, error) {
	const op serrors.Op = "cube.Resolve"
	if err := spec.Validate(); err != nil {
		return lens.DashboardSpec{}, serrors.E(op, err)
	}
	for _, filter := range ctx.Filters {
		if _, ok := spec.Dimension(filter.Dimension); !ok {
			logrus.WithFields(logrus.Fields{
				"cube":      spec.ID,
				"dimension": filter.Dimension,
				"value":     filter.Value,
			}).Warn("cube: ignoring filter for unknown dimension")
		}
	}
	groupBy := groupByDimension(spec, ctx)
	comparison := resolveComparison(spec, ctx)
	ctx.GroupBy = groupBy.Name
	ctx.ActiveDimension = groupBy.Name
	remaining := ctx.RemainingDimensions(spec)
	dashboard := lens.DashboardSpec{
		ID:          spec.ID,
		Title:       spec.Title,
		Description: spec.Description,
		Variables:   append([]lens.VariableSpec(nil), spec.Variables...),
		Drill:       drillMeta(spec, ctx, baseURL, remaining),
	}
	if spec.DataMode == DataModeDataset {
		dashboard.Datasets = append(dashboard.Datasets, baseDataset(spec))
	}

	statsResolution, err := resolveStatDatasets(spec, ctx)
	if err != nil {
		return lens.DashboardSpec{}, serrors.E(op, err)
	}
	if comparison.Enabled {
		statsResolution = compareStatDatasets(spec, statsResolution, comparison)
	}
	dashboard.Datasets = append(dashboard.Datasets, statsResolution.Datasets...)
	if statPanels := buildStatPanelsCompared(spec, statsResolution.DatasetByMeasure, comparison.Enabled); len(statPanels) > 0 {
		dashboard.Rows = append(dashboard.Rows, lens.RowSpec{Panels: []panel.Spec{buildStatStrip(spec, statPanels)}})
	}

	// Render one panel per dimension (the full overview grid). The group-by
	// dimension is sorted to the front so the selector still "focuses" a
	// dimension without collapsing the dashboard to a single chart.
	ordered := reorderByActiveDimension(remaining, groupBy.Name)
	dimensionPanels := make([]panel.Spec, 0, len(ordered))
	for idx, dim := range ordered {
		resolved, err := resolveDimensionDataset(spec, ctx, dim)
		if err != nil {
			return lens.DashboardSpec{}, serrors.E(op, err)
		}
		if comparison.Enabled {
			resolved = compareDimensionDataset(spec, resolved, comparison)
		}
		dashboard.Datasets = append(dashboard.Datasets, resolved.Datasets...)
		dimensionPanels = append(dimensionPanels, buildDimensionPanel(spec, dim, resolved, baseURL, len(ordered), idx))
	}
	if len(dimensionPanels) > 0 {
		dashboard.Rows = append(dashboard.Rows, buildDimensionRows(dimensionPanels)...)
	}

	return dashboard, nil
}

func resolveStatDatasets(spec CubeSpec, ctx DrillContext) (statDatasetResolution, error) {
	resolved := statDatasetResolution{
		Datasets:         make([]lens.DatasetSpec, 0, 1),
		DatasetByMeasure: make(map[string]string, len(spec.Measures)),
	}
	regularMeasures := make([]MeasureSpec, 0, len(spec.Measures))
	for _, measure := range spec.Measures {
		if measure.Override == nil {
			regularMeasures = append(regularMeasures, measure)
			continue
		}
		name := measureDatasetName(measure.Name)
		resolved.Datasets = append(resolved.Datasets, resolveOverrideDataset(spec, ctx, *measure.Override, name))
		resolved.DatasetByMeasure[measure.Name] = name
	}
	if len(regularMeasures) == 0 {
		return resolved, nil
	}
	statsSpec := spec
	statsSpec.Measures = regularMeasures
	statsDataset, err := resolveStatsDataset(statsSpec, ctx)
	if err != nil {
		return statDatasetResolution{}, err
	}
	resolved.Datasets = append([]lens.DatasetSpec{statsDataset}, resolved.Datasets...)
	for _, measure := range regularMeasures {
		resolved.DatasetByMeasure[measure.Name] = statsDataset.Name
	}
	return resolved, nil
}

func resolveStatsDataset(spec CubeSpec, ctx DrillContext) (lens.DatasetSpec, error) {
	name := statsDatasetNamePrefix
	switch spec.DataMode {
	case DataModeSQL:
		return resolveSQLStatsDataset(spec, ctx, name), nil
	case DataModeDataset:
		return resolveDatasetStatsDataset(spec, ctx, name), nil
	default:
		return lens.DatasetSpec{}, fmt.Errorf("unsupported cube mode %q", spec.DataMode)
	}
}

func resolveDimensionDataset(spec CubeSpec, ctx DrillContext, dim DimensionSpec) (dimensionDatasetResolution, error) {
	name := datasetName(dim.Name)
	if dim.Override != nil {
		overrideDataset := resolveOverrideDataset(spec, ctx, *dim.Override, name)
		hasColorValue := strings.TrimSpace(dim.ColorField) != ""
		if len(dim.Transforms) == 0 {
			return dimensionDatasetResolution{
				Name:          name,
				Datasets:      []lens.DatasetSpec{overrideDataset},
				HasColorValue: hasColorValue,
			}, nil
		}
		sourceName := name + "_source"
		overrideDataset.Name = sourceName
		return dimensionDatasetResolution{
			Name:          name,
			HasColorValue: hasColorValue,
			Datasets: []lens.DatasetSpec{
				overrideDataset,
				{
					Name:       name,
					Kind:       lens.DatasetKindTransform,
					DependsOn:  []string{sourceName},
					Transforms: resolvedDimensionTransforms(spec, dim.Transforms),
				},
			},
		}, nil
	}
	switch spec.DataMode {
	case DataModeSQL:
		if len(dim.Transforms) == 0 {
			return dimensionDatasetResolution{
				Name:          name,
				Datasets:      []lens.DatasetSpec{resolveSQLDimensionDataset(spec, ctx, dim, name)},
				HasColorValue: strings.TrimSpace(dim.ColorColumn) != "",
			}, nil
		}
		sourceName := name + "_source"
		return dimensionDatasetResolution{
			Name:          name,
			HasColorValue: strings.TrimSpace(dim.ColorColumn) != "",
			Datasets: []lens.DatasetSpec{
				resolveSQLDimensionDataset(spec, ctx, dim, sourceName),
				{
					Name:       name,
					Kind:       lens.DatasetKindTransform,
					DependsOn:  []string{sourceName},
					Transforms: resolvedDimensionTransforms(spec, dim.Transforms),
				},
			},
		}, nil
	case DataModeDataset:
		return dimensionDatasetResolution{
			Name:          name,
			Datasets:      []lens.DatasetSpec{resolveDatasetDimensionDataset(spec, ctx, dim, name)},
			HasColorValue: datasetDimensionHasColorValue(dim),
		}, nil
	default:
		return dimensionDatasetResolution{}, fmt.Errorf("unsupported cube mode %q", spec.DataMode)
	}
}

func buildStatPanelsCompared(spec CubeSpec, datasetByMeasure map[string]string, compared bool) []panel.Spec {
	panels := make([]panel.Spec, 0, len(spec.Measures))
	span := statSpan(len(spec.Measures))
	for _, measure := range spec.Measures {
		dataset := datasetByMeasure[measure.Name]
		if strings.TrimSpace(dataset) == "" {
			dataset = statsDatasetNamePrefix
		}
		builder := panel.Stat("stat_"+measure.Name, measure.Label, dataset).
			Span(span).
			ValueField(panel.Ref(measure.Name))
		if measure.Formatter != nil {
			builder.Format(*measure.Formatter)
		}
		if strings.TrimSpace(measure.Description) != "" {
			builder.Description(measure.Description)
		}
		if strings.TrimSpace(measure.Info) != "" {
			builder.Info(measure.Info)
		}
		if strings.TrimSpace(measure.AccentColor) != "" {
			builder.AccentColor(measure.AccentColor)
		}
		if measure.Action != nil {
			builder.Action(*measure.Action)
		} else {
			builder.Terminal()
		}
		if compared {
			builder.AutoTrend(
				panel.Ref(comparison.DeltaField(measure.Name)), panel.Ref(comparison.DeltaPercentField(measure.Name)), "", measure.InvertTrend,
			)
		}
		panels = append(panels, builder.Build())
	}
	return panels
}

// buildStatStrip gathers a cube's measure cards into one KPI strip. Left as
// loose panels each measure claims a full card — header chrome, a tall empty
// body, and the figure adrift in the middle of it — so three numbers occupy a
// screenful. A StatGroup renders the same measures as one hairline-separated
// row of compact metrics, which is the shape every hand-built Lens dashboard
// already uses for its KPI band. The strip stays headerless: a cube's stats are
// the dashboard's headline, and the page title above them already says so.
func buildStatStrip(spec CubeSpec, stats []panel.Spec) panel.Spec {
	return panel.StatGroup(spec.ID+"-kpi", "", stats...).
		Layout(panel.GroupColumns).
		Span(12).
		Build()
}

func buildDimensionPanel(spec CubeSpec, dim DimensionSpec, resolved dimensionDatasetResolution, baseURL string, remainingCount, index int) panel.Spec {
	// Dimension charts use the first measure as their value axis.
	// Additional measures appear only in stat panels.
	measure := spec.Measures[0]
	actionURL := baseURL
	builder := panelBuilder(dim.PanelKind, "panel_"+dim.Name, dim.Label, resolved.Name, dim.Map).
		Span(dimensionSpan(remainingCount, index)).
		Height("360px").
		Description(dim.Description).
		Fields(panel.FieldMapping{
			Label:    panel.Ref("label"),
			Category: panel.Ref("label"),
			Value:    panel.Ref(measure.Name),
			Previous: func() panel.FieldRef {
				if resolved.Compared {
					return panel.Ref(comparison.PreviousField(measure.Name))
				}
				return ""
			}(),
			ID: panel.Ref("filter_value"),
		}).
		Action(action.CrossFilter(actionURL, dim.Name))
	if strings.TrimSpace(dim.Height) != "" {
		builder.Height(dim.Height)
	}
	if measure.Formatter != nil {
		builder.Format(*measure.Formatter)
	}
	if dim.ValueAxis.Scale != "" {
		builder.ValueAxisScale(dim.ValueAxis.Scale, dim.ValueAxis.LogBase)
	}
	if !dim.Presentation.IsZero() {
		builder.Presentation(dim.Presentation)
	}
	if strings.TrimSpace(dim.ColorScale) != "" {
		colorField := panel.Ref("filter_value")
		if resolved.HasColorValue {
			colorField = panel.Ref("color_value")
		}
		builder.SemanticColors(dim.ColorScale, colorField)
	}
	if len(dim.Colors) > 0 {
		builder.Colors(dim.Colors...)
	}
	if dim.PanelKind == panel.KindBar || dim.PanelKind == panel.KindHorizontalBar {
		builder.DistributedColors()
	}
	if dim.PanelKind == panel.KindTable && resolved.Compared {
		builder = panel.Table("panel_"+dim.Name, dim.Label, resolved.Name).
			Span(dimensionSpan(remainingCount, index)).
			Height("360px").
			Action(action.CrossFilter(actionURL, dim.Name)).
			Columns(
				panel.TableColumn{Field: panel.Ref("label"), Label: dim.Label},
				panel.TableColumn{Field: panel.Ref(comparison.PreviousField(measure.Name)), Label: "Before", Formatter: measure.Formatter, Align: "right"},
				panel.TableColumn{Field: panel.Ref(measure.Name), Label: "After", Formatter: measure.Formatter, Align: "right"},
				panel.TableColumn{
					Field: panel.Ref(comparison.DeltaField(measure.Name)), Label: "Δ", Formatter: measure.Formatter, Align: "right",
					Cell: &panel.TableCellSpec{Kind: panel.TableCellDelta, PercentField: panel.Ref(comparison.DeltaPercentField(measure.Name))},
				},
			)
		if strings.TrimSpace(dim.Height) != "" {
			builder.Height(dim.Height)
		}
	}
	return builder.Build()
}

func compareStatDatasets(spec CubeSpec, resolved statDatasetResolution, comparison comparisonConfig) statDatasetResolution {
	fieldsByTerminal := make(map[string][]string)
	for _, measure := range spec.Measures {
		terminal := resolved.DatasetByMeasure[measure.Name]
		fieldsByTerminal[terminal] = append(fieldsByTerminal[terminal], measure.Name)
	}
	resolved.Datasets = append(resolved.Datasets, cloneComparisonGraph(resolved.Datasets, comparison, fieldsByTerminal, nil)...)
	for terminal, fields := range fieldsByTerminal {
		resolved.Datasets = append(resolved.Datasets, comparisonJoin(terminal, fields, nil))
		for _, field := range fields {
			resolved.DatasetByMeasure[field] = comparedDatasetName(terminal)
		}
	}
	return resolved
}

func compareDimensionDataset(spec CubeSpec, resolved dimensionDatasetResolution, comparison comparisonConfig) dimensionDatasetResolution {
	fields := make([]string, 0, len(spec.Measures))
	for _, measure := range spec.Measures {
		fields = append(fields, measure.Name)
	}
	resolved.Datasets = append(resolved.Datasets, cloneComparisonGraph(
		resolved.Datasets, comparison, map[string][]string{resolved.Name: fields},
		map[string][]string{resolved.Name: {"filter_value", "label"}},
	)...)
	resolved.Datasets = append(resolved.Datasets, comparisonJoin(resolved.Name, fields, []string{"filter_value", "label"}))
	resolved.Name = comparedDatasetName(resolved.Name)
	resolved.Compared = true
	return resolved
}

func groupByDimension(spec CubeSpec, ctx DrillContext) DimensionSpec {
	groupBy := ctx.normalizedGroupBy()
	if groupBy != "" {
		if dim, ok := spec.Dimension(groupBy); ok {
			return dim
		}
	}
	if defaultDimension := strings.TrimSpace(spec.DefaultDimension); defaultDimension != "" {
		if dim, ok := spec.Dimension(defaultDimension); ok {
			return dim
		}
	}
	if len(spec.Dimensions) == 0 {
		return DimensionSpec{}
	}
	return orderedDimensions(spec)[0]
}

func buildDimensionRows(panels []panel.Spec) []lens.RowSpec {
	if len(panels) == 0 {
		return nil
	}
	rows := make([]lens.RowSpec, 0, 1+(len(panels)-1)/3)
	firstRow := []panel.Spec{panels[0]}
	if len(panels) > 1 {
		firstRow = append(firstRow, panels[1])
	}
	rows = append(rows, lens.RowSpec{Panels: firstRow})
	for start := 2; start < len(panels); start += 3 {
		end := min(start+3, len(panels))
		rows = append(rows, lens.RowSpec{Panels: append([]panel.Spec(nil), panels[start:end]...)})
	}
	return rows
}

func panelBuilder(kind panel.Kind, id, title, dataset string, mapSpec *panel.MapSpec) *panel.Builder {
	switch kind {
	case panel.KindTable:
		return panel.Table(id, title, dataset)
	case panel.KindStat,
		panel.KindTimeSeries,
		panel.KindBar,
		panel.KindSegmentBar, panel.KindCascade,
		panel.KindGauge,
		panel.KindTabs,
		panel.KindGrid,
		panel.KindSplit,
		panel.KindRepeat,
		panel.KindStatGroup:
		return panel.Bar(id, title, dataset)
	case panel.KindHistogram:
		return panel.Histogram(id, title, dataset)
	case panel.KindBoxPlot:
		return panel.BoxPlot(id, title, dataset)
	case panel.KindHeatmap:
		return panel.Heatmap(id, title, dataset)
	case panel.KindHorizontalBar:
		return panel.HorizontalBar(id, title, dataset)
	case panel.KindStackedBar:
		return panel.StackedBar(id, title, dataset)
	case panel.KindDonut:
		return panel.Donut(id, title, dataset)
	case panel.KindPie:
		return panel.Pie(id, title, dataset)
	case panel.KindRadial:
		// A radial panel needs geometry a cube dimension cannot state: a
		// declared maximum for progress, or the ring totals a partition
		// reconciles against. A dimension asking for one is asking for a
		// chart it has not described, so it gets the cube's default bar.
		// Radial panels belong in a hand-written DashboardSpec.
		return panel.Bar(id, title, dataset)
	case panel.KindMap:
		if mapSpec == nil {
			return panel.Bar(id, title, dataset)
		}
		return panel.Choropleth(id, title, dataset, mapSpec.Source, mapSpec.FeatureProperty).
			MapLabelProperty(mapSpec.LabelProperty).
			MapLabelProperties(mapSpec.LabelProperties).
			MapAttribution(mapSpec.Attribution)
	case panel.KindMetricFlow, panel.KindMetricHierarchy, panel.KindMetricRelationship:
		return panel.Bar(id, title, dataset)
	}
	return panel.Bar(id, title, dataset)
}

func orderedDimensions(spec CubeSpec) []DimensionSpec {
	dimensions := append([]DimensionSpec(nil), spec.Dimensions...)
	defaultDimension := strings.TrimSpace(spec.DefaultDimension)
	if defaultDimension == "" {
		return dimensions
	}
	slices.SortStableFunc(dimensions, func(left, right DimensionSpec) int {
		switch {
		case left.Name == defaultDimension && right.Name != defaultDimension:
			return -1
		case left.Name != defaultDimension && right.Name == defaultDimension:
			return 1
		default:
			return 0
		}
	})
	return dimensions
}

func reorderByActiveDimension(dimensions []DimensionSpec, active string) []DimensionSpec {
	active = strings.TrimSpace(active)
	if active == "" || len(dimensions) <= 1 {
		return dimensions
	}
	idx := -1
	for i, dim := range dimensions {
		if dim.Name == active {
			idx = i
			break
		}
	}
	if idx <= 0 {
		return dimensions
	}
	reordered := make([]DimensionSpec, 0, len(dimensions))
	reordered = append(reordered, dimensions[idx])
	reordered = append(reordered, dimensions[:idx]...)
	reordered = append(reordered, dimensions[idx+1:]...)
	return reordered
}

func statSpan(count int) int {
	if count <= 0 {
		return 12
	}
	if count >= 4 {
		return 3
	}
	if count == 3 {
		return 4
	}
	return 6
}

func dimensionSpan(remaining, index int) int {
	if remaining <= 1 {
		return 12
	}
	if index == 0 {
		return 8
	}
	if remaining == 2 {
		return 4
	}
	return 4
}

func datasetName(dimension string) string {
	return dimDatasetNamePrefix + "_" + strings.ReplaceAll(strings.TrimSpace(dimension), " ", "_")
}

func measureDatasetName(measure string) string {
	return statDatasetNamePrefix + "_" + strings.ReplaceAll(strings.TrimSpace(measure), " ", "_")
}

func resolveOverrideDataset(spec CubeSpec, ctx DrillContext, dataset lens.DatasetSpec, name string) lens.DatasetSpec {
	dataset.Name = name
	if dataset.Query == nil {
		return dataset
	}
	query := *dataset.Query
	if strings.TrimSpace(dataset.Source) == "" && spec.DataMode == DataModeSQL {
		dataset.Source = spec.DataSource
	}
	query.Params = mergeParamValues(
		overrideBaseParams(spec),
		query.Params,
		overrideFilterParams(spec, ctx),
	)
	dataset.Query = &query
	return dataset
}

func overrideBaseParams(spec CubeSpec) map[string]lens.ParamValue {
	params := cloneParamValues(spec.Params)
	for _, dim := range spec.Dimensions {
		key := sqlFilterParam(dim.Name)
		if _, ok := params[key]; ok {
			continue
		}
		params[key] = lens.ParamValue{Literal: nil}
	}
	return params
}

func overrideFilterParams(spec CubeSpec, ctx DrillContext) map[string]lens.ParamValue {
	params := map[string]lens.ParamValue{}
	for _, filter := range ctx.Filters {
		if _, ok := spec.Dimension(filter.Dimension); !ok {
			continue
		}
		values := filter.values()
		if len(values) == 0 {
			continue
		}
		params[sqlFilterParam(filter.Dimension)] = lens.ParamValue{Literal: values}
	}
	return params
}

func mergeParamValues(maps ...map[string]lens.ParamValue) map[string]lens.ParamValue {
	merged := map[string]lens.ParamValue{}
	for _, current := range maps {
		for key, value := range current {
			merged[key] = value
		}
	}
	return merged
}

func cloneParamValues(values map[string]lens.ParamValue) map[string]lens.ParamValue {
	cloned := make(map[string]lens.ParamValue, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}
