package cube

import (
	"fmt"
	"slices"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/sirupsen/logrus"
)

const (
	statsDatasetNamePrefix = "cube_stats"
	dimDatasetNamePrefix   = "cube_dim"
	leafDatasetNamePrefix  = "cube_leaf"
)

func Resolve(spec CubeSpec, ctx DrillContext, baseURL string) (lens.DashboardSpec, error) {
	if err := spec.Validate(); err != nil {
		return lens.DashboardSpec{}, err
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
	remaining := ctx.RemainingDimensions(spec)
	remaining = reorderByActiveDimension(remaining, ctx.ActiveDimension)
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

	statsDataset, err := resolveStatsDataset(spec, ctx)
	if err != nil {
		return lens.DashboardSpec{}, err
	}
	dashboard.Datasets = append(dashboard.Datasets, statsDataset)
	dashboard.Rows = append(dashboard.Rows, lens.RowSpec{Panels: buildStatPanels(spec, statsDataset.Name)})

	if ctx.IsLeaf(spec) {
		leafSpec, leafDataset, leafErr := resolveLeaf(spec, ctx)
		if leafErr != nil {
			return lens.DashboardSpec{}, leafErr
		}
		if leafDataset != nil {
			dashboard.Datasets = append(dashboard.Datasets, *leafDataset)
		}
		if len(leafSpec.Panels) > 0 {
			dashboard.Rows = append(dashboard.Rows, leafSpec)
		}
		return dashboard, nil
	}

	dimensionPanels := make([]panel.Spec, 0, len(remaining))
	for idx, dim := range remaining {
		dataset, err := resolveDimensionDataset(spec, ctx, dim)
		if err != nil {
			return lens.DashboardSpec{}, err
		}
		dashboard.Datasets = append(dashboard.Datasets, dataset)
		dimensionPanels = append(dimensionPanels, buildDimensionPanel(spec, dim, dataset.Name, baseURL, len(remaining), idx))
	}
	dashboard.Rows = append(dashboard.Rows, buildDimensionRows(dimensionPanels)...)

	return dashboard, nil
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

func resolveDimensionDataset(spec CubeSpec, ctx DrillContext, dim DimensionSpec) (lens.DatasetSpec, error) {
	name := datasetName(dim.Name)
	if dim.Override != nil {
		return resolveOverrideDataset(spec, ctx, *dim.Override, name), nil
	}
	switch spec.DataMode {
	case DataModeSQL:
		return resolveSQLDimensionDataset(spec, ctx, dim, name), nil
	case DataModeDataset:
		return resolveDatasetDimensionDataset(spec, ctx, dim, name), nil
	default:
		return lens.DatasetSpec{}, fmt.Errorf("unsupported cube mode %q", spec.DataMode)
	}
}

func resolveLeaf(spec CubeSpec, ctx DrillContext) (lens.RowSpec, *lens.DatasetSpec, error) {
	switch spec.DataMode {
	case DataModeDataset:
		if strings.TrimSpace(spec.Leaf.URL) != "" {
			return lens.RowSpec{}, nil, nil
		}
		dataset := resolveDatasetLeafDataset(spec, ctx, leafDatasetNamePrefix)
		return lens.RowSpec{
			Panels: []panel.Spec{
				panel.Table("leaf_records", "Records", dataset.Name).
					Span(12).
					Build(),
			},
		}, &dataset, nil
	case DataModeSQL:
		return lens.RowSpec{}, nil, nil
	default:
		return lens.RowSpec{}, nil, fmt.Errorf("unsupported cube mode %q", spec.DataMode)
	}
}

func buildStatPanels(spec CubeSpec, dataset string) []panel.Spec {
	panels := make([]panel.Spec, 0, len(spec.Measures))
	span := statSpan(len(spec.Measures))
	for _, measure := range spec.Measures {
		builder := panel.Stat("stat_"+measure.Name, measure.Label, dataset).
			Span(span).
			ValueField(panel.Ref(measure.Name))
		if measure.Formatter != nil {
			builder.Format(*measure.Formatter)
		}
		if strings.TrimSpace(measure.Description) != "" {
			builder.Description(measure.Description)
		}
		if strings.TrimSpace(measure.AccentColor) != "" {
			builder.AccentColor(measure.AccentColor)
		}
		panels = append(panels, builder.Build())
	}
	return panels
}

func buildDimensionPanel(spec CubeSpec, dim DimensionSpec, dataset, baseURL string, remainingCount, index int) panel.Spec {
	// Dimension charts use the first measure as their value axis.
	// Additional measures appear only in stat panels.
	measure := spec.Measures[0]
	actionURL := baseURL
	if remainingCount == 1 && strings.TrimSpace(spec.Leaf.URL) != "" {
		actionURL = spec.Leaf.URL
	}
	builder := panelBuilder(dim.PanelKind, "panel_"+dim.Name, dim.Label, dataset).
		Span(dimensionSpan(remainingCount, index)).
		Height("360px").
		Description(dim.Description).
		Fields(panel.FieldMapping{
			Label:    panel.Ref("label"),
			Category: panel.Ref("label"),
			Value:    panel.Ref(measure.Name),
			ID:       panel.Ref("filter_value"),
		}).
		Action(action.CubeDrill(actionURL, dim.Name))
	if strings.TrimSpace(dim.Height) != "" {
		builder.Height(dim.Height)
	}
	if measure.Formatter != nil {
		builder.Format(*measure.Formatter)
	}
	if dim.ValueAxis.Scale != "" {
		builder.ValueAxisScale(dim.ValueAxis.Scale, dim.ValueAxis.LogBase)
	}
	if strings.TrimSpace(dim.ColorScale) != "" {
		colorField := panel.Ref("filter_value")
		if strings.TrimSpace(dim.ColorColumn) != "" || strings.TrimSpace(dim.ColorField) != "" {
			colorField = panel.Ref("color_value")
		}
		builder.SemanticColors(dim.ColorScale, colorField)
		if dim.PanelKind == panel.KindBar || dim.PanelKind == panel.KindHorizontalBar {
			builder.DistributedColors()
		}
	}
	if len(dim.Colors) > 0 {
		builder.Colors(dim.Colors...)
		if dim.PanelKind == panel.KindBar || dim.PanelKind == panel.KindHorizontalBar {
			builder.DistributedColors()
		}
	}
	return builder.Build()
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

func panelBuilder(kind panel.Kind, id, title, dataset string) *panel.Builder {
	switch kind {
	case panel.KindStat,
		panel.KindTimeSeries,
		panel.KindBar,
		panel.KindTable,
		panel.KindGauge,
		panel.KindTabs,
		panel.KindGrid,
		panel.KindSplit,
		panel.KindRepeat:
		return panel.Bar(id, title, dataset)
	case panel.KindHorizontalBar:
		return panel.HorizontalBar(id, title, dataset)
	case panel.KindStackedBar:
		return panel.StackedBar(id, title, dataset)
	case panel.KindDonut:
		return panel.Donut(id, title, dataset)
	case panel.KindPie:
		return panel.Pie(id, title, dataset)
	}
	return panel.Bar(id, title, dataset)
}

func orderedDimensions(spec CubeSpec) []DimensionSpec {
	dimensions := append([]DimensionSpec(nil), spec.Dimensions...)
	if strings.TrimSpace(spec.DefaultDimension) == "" {
		return dimensions
	}
	slices.SortStableFunc(dimensions, func(left, right DimensionSpec) int {
		switch {
		case left.Name == spec.DefaultDimension && right.Name != spec.DefaultDimension:
			return -1
		case left.Name != spec.DefaultDimension && right.Name == spec.DefaultDimension:
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
		params[sqlFilterParam(filter.Dimension)] = lens.ParamValue{Literal: filter.Value}
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
