package cube

import (
	"context"
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
	chartDatasetNamePrefix = "cube_chart"
	leafDatasetNamePrefix  = "cube_leaf"
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
	return ResolveWithContext(context.Background(), spec, ctx, baseURL)
}

func ResolveWithContext(goCtx context.Context, spec CubeSpec, ctx DrillContext, baseURL string) (lens.DashboardSpec, error) {
	const op serrors.Op = "cube.Resolve"
	if err := spec.Validate(); err != nil {
		return lens.DashboardSpec{}, serrors.Wrap(op, err)
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
	if spec.DataMode == DataModeDataset || spec.DataMode == DataModeNamed {
		dashboard.Datasets = append(dashboard.Datasets, baseDataset(spec, ctx))
		dashboard.Datasets = append(dashboard.Datasets, labelMapDatasets(spec)...)
	}

	statsResolution, err := resolveStatDatasets(spec, ctx)
	if err != nil {
		return lens.DashboardSpec{}, serrors.Wrap(op, err)
	}
	if comparison.Enabled {
		statsResolution = compareStatDatasets(spec, statsResolution, comparison)
	}
	dashboard.Datasets = append(dashboard.Datasets, titledDatasets(statsResolution.Datasets, spec.Title)...)

	showingLeaf := (ctx.IsLeaf(spec) || ctx.Leaf) && spec.Leaf.Table != nil

	if len(spec.Layout) > 0 {
		leafAppended := false
		for rowIdx, layoutRow := range spec.Layout {
			switch layoutRow.Kind {
			case LayoutRowKPI:
				measures := selectRowMeasures(spec, layoutRow.Measures)
				kpiAction := resolveKPIAction(goCtx, spec, ctx, baseURL, showingLeaf)
				if statPanels := buildStatPanelsForMeasures(spec, measures, statsResolution.DatasetByMeasure, comparison.Enabled, kpiAction); len(statPanels) > 0 {
					groupID := spec.ID + "-kpi"
					if rowIdx > 0 {
						groupID = fmt.Sprintf("%s-kpi-%d", spec.ID, rowIdx)
					}
					dashboard.Rows = append(dashboard.Rows, lens.RowSpec{
						Panels: []panel.Spec{buildStatStripWithID(groupID, statPanels)},
					})
				}
			case LayoutRowCharts:
				if showingLeaf {
					if !leafAppended {
						leafDS, leafPanel := buildLeafTable(goCtx, spec, ctx, baseURL)
						dashboard.Datasets = append(dashboard.Datasets, titledDatasets([]lens.DatasetSpec{leafDS}, spec.Leaf.Table.Title)...)
						dashboard.Rows = append(dashboard.Rows, lens.RowSpec{Panels: []panel.Spec{leafPanel}})
						leafAppended = true
					}
					continue
				}
				if len(layoutRow.Charts) == 0 {
					continue
				}
				spans := chartRowSpans(layoutRow.Charts)
				rowPanels := make([]panel.Spec, 0, len(layoutRow.Charts))
				for idx, chart := range layoutRow.Charts {
					resolved, err := resolveChartDataset(spec, ctx, chart)
					if err != nil {
						return lens.DashboardSpec{}, serrors.Wrap(op, err)
					}
					if comparison.Enabled && chart.Series == "" {
						resolved = compareDimensionDataset(spec, resolved, comparison)
					}
					title := strings.TrimSpace(chart.Title)
					if title == "" {
						if dim, ok := spec.Dimension(chart.Dimension); ok {
							title = dim.Label
						}
					}
					dashboard.Datasets = append(dashboard.Datasets, titledDatasets(resolved.Datasets, title)...)
					rowPanels = append(rowPanels, buildChartPanel(spec, ctx, chart, resolved, baseURL, spans[idx]))
				}
				if len(rowPanels) > 0 {
					dashboard.Rows = append(dashboard.Rows, lens.RowSpec{Panels: rowPanels})
				}
			}
		}
		if showingLeaf && !leafAppended {
			leafDS, leafPanel := buildLeafTable(goCtx, spec, ctx, baseURL)
			dashboard.Datasets = append(dashboard.Datasets, titledDatasets([]lens.DatasetSpec{leafDS}, spec.Leaf.Table.Title)...)
			dashboard.Rows = append(dashboard.Rows, lens.RowSpec{Panels: []panel.Spec{leafPanel}})
		}
		return dashboard, nil
	}

	kpiAction := resolveKPIAction(goCtx, spec, ctx, baseURL, showingLeaf)
	if statPanels := buildStatPanelsForMeasures(spec, spec.Measures, statsResolution.DatasetByMeasure, comparison.Enabled, kpiAction); len(statPanels) > 0 {
		dashboard.Rows = append(dashboard.Rows, lens.RowSpec{Panels: []panel.Spec{buildStatStrip(spec, statPanels)}})
	}

	if showingLeaf {
		leafDS, leafPanel := buildLeafTable(goCtx, spec, ctx, baseURL)
		dashboard.Datasets = append(dashboard.Datasets, titledDatasets([]lens.DatasetSpec{leafDS}, spec.Leaf.Table.Title)...)
		dashboard.Rows = append(dashboard.Rows, lens.RowSpec{Panels: []panel.Spec{leafPanel}})
		return dashboard, nil
	}

	// Render one panel per dimension (the full overview grid). The group-by
	// dimension is sorted to the front so the selector still "focuses" a
	// dimension without collapsing the dashboard to a single chart.
	ordered := reorderByActiveDimension(remaining, groupBy.Name)
	dimensionPanels := make([]panel.Spec, 0, len(ordered))
	for idx, dim := range ordered {
		resolved, err := resolveDimensionDataset(spec, ctx, dim)
		if err != nil {
			return lens.DashboardSpec{}, serrors.Wrap(op, err)
		}
		if comparison.Enabled {
			resolved = compareDimensionDataset(spec, resolved, comparison)
		}
		dashboard.Datasets = append(dashboard.Datasets, titledDatasets(resolved.Datasets, dim.Label)...)
		dimensionPanels = append(dimensionPanels, buildDimensionPanelWithContext(spec, ctx, dim, resolved, baseURL, len(ordered), idx))
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

func titledDatasets(datasets []lens.DatasetSpec, title string) []lens.DatasetSpec {
	title = strings.TrimSpace(title)
	for i := range datasets {
		if strings.TrimSpace(datasets[i].Title) == "" {
			datasets[i].Title = title
		}
	}
	return datasets
}

func resolveStatsDataset(spec CubeSpec, ctx DrillContext) (lens.DatasetSpec, error) {
	name := statsDatasetNamePrefix
	switch spec.DataMode {
	case DataModeSQL:
		return resolveSQLStatsDataset(spec, ctx, name), nil
	case DataModeDataset, DataModeNamed:
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
	case DataModeDataset, DataModeNamed:
		return dimensionDatasetResolution{
			Name:          name,
			Datasets:      []lens.DatasetSpec{resolveDatasetDimensionDataset(spec, ctx, dim, name)},
			HasColorValue: datasetDimensionHasColorValue(dim),
		}, nil
	default:
		return dimensionDatasetResolution{}, fmt.Errorf("unsupported cube mode %q", spec.DataMode)
	}
}

func resolveChartDataset(spec CubeSpec, ctx DrillContext, chart ChartSpec) (dimensionDatasetResolution, error) {
	dim, ok := spec.Dimension(chart.Dimension)
	if !ok {
		return dimensionDatasetResolution{}, fmt.Errorf("unknown dimension %q", chart.Dimension)
	}
	var seriesDim *DimensionSpec
	if chart.Series != "" {
		resolvedSeries, exists := spec.Dimension(chart.Series)
		if !exists {
			return dimensionDatasetResolution{}, fmt.Errorf("unknown series dimension %q", chart.Series)
		}
		seriesDim = &resolvedSeries
	}
	measure := chartMeasure(spec, chart)
	name := chartDatasetName(chart.ID)

	extraTransforms := append([]transform.Spec(nil), dim.Transforms...)
	if chart.TopN > 0 {
		if chart.WithRemainder {
			remainder := strings.TrimSpace(chart.RemainderLabel)
			if remainder == "" {
				remainder = "Other"
			}
			extraTransforms = append(extraTransforms, transform.ExpandableTopN(measure.Name, chart.TopN, remainder))
		} else {
			extraTransforms = append(extraTransforms, transform.TopN(measure.Name, chart.TopN, ""))
		}
	}

	if dim.Override != nil && seriesDim == nil {
		overrideDataset := resolveOverrideDataset(spec, ctx, *dim.Override, name)
		hasColorValue := strings.TrimSpace(dim.ColorField) != ""
		if len(extraTransforms) == 0 {
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
					Transforms: resolvedDimensionTransforms(spec, extraTransforms),
				},
			},
		}, nil
	}

	switch spec.DataMode {
	case DataModeSQL:
		if len(extraTransforms) == 0 {
			return dimensionDatasetResolution{
				Name:          name,
				Datasets:      []lens.DatasetSpec{resolveSQLChartDataset(spec, ctx, dim, seriesDim, measure, name)},
				HasColorValue: strings.TrimSpace(dim.ColorColumn) != "",
			}, nil
		}
		sourceName := name + "_source"
		return dimensionDatasetResolution{
			Name:          name,
			HasColorValue: strings.TrimSpace(dim.ColorColumn) != "",
			Datasets: []lens.DatasetSpec{
				resolveSQLChartDataset(spec, ctx, dim, seriesDim, measure, sourceName),
				{
					Name:       name,
					Kind:       lens.DatasetKindTransform,
					DependsOn:  []string{sourceName},
					Transforms: resolvedDimensionTransforms(spec, extraTransforms),
				},
			},
		}, nil
	case DataModeDataset, DataModeNamed:
		return dimensionDatasetResolution{
			Name:          name,
			Datasets:      []lens.DatasetSpec{resolveDatasetChartDataset(spec, ctx, dim, seriesDim, measure, extraTransforms, name)},
			HasColorValue: datasetDimensionHasColorValue(dim),
		}, nil
	default:
		return dimensionDatasetResolution{}, fmt.Errorf("unsupported cube mode %q", spec.DataMode)
	}
}

func buildStatPanelsCompared(spec CubeSpec, datasetByMeasure map[string]string, compared bool) []panel.Spec {
	return buildStatPanelsForMeasures(spec, spec.Measures, datasetByMeasure, compared, nil)
}

func buildStatPanelsForMeasures(
	spec CubeSpec,
	measures []MeasureSpec,
	datasetByMeasure map[string]string,
	compared bool,
	defaultAction *action.Spec,
) []panel.Spec {
	panels := make([]panel.Spec, 0, len(measures))
	span := statSpan(len(measures))
	for _, measure := range measures {
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
		if measure.Formula != nil {
			builder.Formula(*measure.Formula)
		}
		if strings.TrimSpace(measure.AccentColor) != "" {
			builder.AccentColor(measure.AccentColor)
		}
		switch {
		case measure.Action != nil:
			builder.Action(*measure.Action)
		case defaultAction != nil:
			builder.Action(cloneActionSpec(*defaultAction))
		default:
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

func resolveKPIAction(goCtx context.Context, spec CubeSpec, drillCtx DrillContext, baseURL string, showingLeaf bool) *action.Spec {
	if len(spec.Layout) == 0 && len(spec.DrillPath) == 0 && !spec.HasLeaf() {
		return nil
	}
	if !showingLeaf && spec.Leaf.Table != nil {
		leafCtx := drillCtx.clone()
		leafCtx.Leaf = true
		act := action.Navigate(mergeURL(baseURL, leafCtx.WithValues(drillCtx.Request))).WithPreservedQuery()
		return &act
	}
	if extURL := spec.ResolveExternalLeafURL(goCtx, drillCtx, literalCubeParams(spec)); extURL != "" {
		act := action.Navigate(extURL).WithPreservedQuery()
		return &act
	}
	if nextDim := spec.NextDrillDimension(drillCtx); nextDim != "" {
		nextCtx := drillCtx.clone()
		nextCtx.GroupBy = nextDim
		nextCtx.ActiveDimension = nextDim
		act := action.Navigate(mergeURL(baseURL, nextCtx.WithValues(drillCtx.Request))).WithPreservedQuery()
		return &act
	}
	act := action.Navigate(mergeURL(baseURL, drillCtx.WithValues(drillCtx.Request))).WithPreservedQuery()
	return &act
}

func literalCubeParams(spec CubeSpec) map[string]any {
	if len(spec.Params) == 0 {
		return nil
	}
	out := make(map[string]any, len(spec.Params))
	for key, param := range spec.Params {
		if param.Literal != nil {
			out[key] = param.Literal
		}
	}
	return out
}

func selectRowMeasures(spec CubeSpec, names []string) []MeasureSpec {
	if len(names) == 0 {
		return spec.Measures
	}
	out := make([]MeasureSpec, 0, len(names))
	for _, name := range names {
		if m, ok := spec.Measure(name); ok {
			out = append(out, m)
		}
	}
	return out
}

func chartMeasure(spec CubeSpec, chart ChartSpec) MeasureSpec {
	if chart.Measure != "" {
		if m, ok := spec.Measure(chart.Measure); ok {
			return m
		}
	}
	return spec.Measures[0]
}

func chartRowSpans(charts []ChartSpec) []int {
	spans := make([]int, len(charts))
	if len(charts) == 0 {
		return spans
	}
	explicitTotal := 0
	autoCount := 0
	for i, c := range charts {
		if c.Span > 0 {
			spans[i] = c.Span
			explicitTotal += c.Span
		} else {
			autoCount++
		}
	}
	if autoCount == 0 {
		return spans
	}
	remaining := 12 - explicitTotal
	if remaining < autoCount {
		remaining = 12
	}
	base := remaining / autoCount
	if base < 1 {
		base = 1
	}
	extra := remaining - base*autoCount
	for i := range charts {
		if spans[i] > 0 {
			continue
		}
		spans[i] = base
		if extra > 0 {
			spans[i]++
			extra--
		}
	}
	return spans
}

func buildStatStrip(spec CubeSpec, stats []panel.Spec) panel.Spec {
	return buildStatStripWithID(spec.ID+"-kpi", stats)
}

func buildStatStripWithID(id string, stats []panel.Spec) panel.Spec {
	return panel.StatGroup(id, "", stats...).
		Layout(panel.GroupColumns).
		Span(12).
		Build()
}

func buildChartPanel(
	spec CubeSpec,
	ctx DrillContext,
	chart ChartSpec,
	resolved dimensionDatasetResolution,
	baseURL string,
	span int,
) panel.Spec {
	dim, _ := spec.Dimension(chart.Dimension)
	measure := chartMeasure(spec, chart)
	kind := chart.Kind
	if kind == "" {
		kind = dim.PanelKind
	}
	if kind == "" {
		kind = panel.KindBar
	}
	title := strings.TrimSpace(chart.Title)
	if title == "" {
		title = dim.Label
	}
	description := strings.TrimSpace(chart.Description)
	if description == "" {
		description = dim.Description
	}
	fields := panel.FieldMapping{
		Label:    panel.Ref("label"),
		Category: panel.Ref("label"),
		Value:    panel.Ref(measure.Name),
		ID:       panel.Ref("filter_value"),
	}
	if chart.Series != "" {
		fields.Series = panel.Ref("series")
	}
	if resolved.Compared {
		fields.Previous = panel.Ref(comparison.PreviousField(measure.Name))
	}

	drillAction := action.CubeDrill(baseURL, dim.Name).WithPreservedQuery()
	if nextDim := nextDrillDimensionExcluding(spec, ctx, dim.Name); nextDim != "" {
		drillAction = drillAction.WithDrillGroupBy(nextDim)
	}

	builder := panelBuilder(kind, chart.ID, title, resolved.Name, dim.Map).
		Span(span).
		Height("360px").
		Description(description).
		Fields(fields).
		Action(drillAction)

	if strings.TrimSpace(chart.Height) != "" {
		builder.Height(chart.Height)
	} else if strings.TrimSpace(dim.Height) != "" {
		builder.Height(dim.Height)
	}
	if measure.Formatter != nil {
		builder.Format(*measure.Formatter)
	}
	if dim.ValueAxis.Scale != "" {
		builder.ValueAxisScale(dim.ValueAxis.Scale, dim.ValueAxis.LogBase)
	}
	if !chart.Presentation.IsZero() {
		builder.Presentation(chart.Presentation)
	} else if !dim.Presentation.IsZero() {
		builder.Presentation(dim.Presentation)
	}
	if strings.TrimSpace(dim.ColorScale) != "" {
		colorField := panel.Ref("filter_value")
		if resolved.HasColorValue {
			colorField = panel.Ref("color_value")
		}
		builder.SemanticColors(dim.ColorScale, colorField)
	}
	if len(chart.Colors) > 0 {
		builder.Colors(chart.Colors...)
	} else if len(dim.Colors) > 0 {
		builder.Colors(dim.Colors...)
	}
	if (kind == panel.KindBar || kind == panel.KindHorizontalBar) && chart.Series == "" {
		builder.DistributedColors()
	}
	if chart.Series != "" {
		builder.Legend()
	}
	return builder.Build()
}

func buildLeafTable(goCtx context.Context, spec CubeSpec, ctx DrillContext, baseURL string) (lens.DatasetSpec, panel.Spec) {
	table := spec.Leaf.Table
	dsName := leafDatasetName(table.ID)
	var ds lens.DatasetSpec
	switch spec.DataMode {
	case DataModeSQL:
		ds = resolveSQLLeafDataset(spec, ctx, table, dsName)
	default:
		ds = resolveDatasetLeafDataset(spec, ctx, table, dsName)
	}

	title := strings.TrimSpace(table.Title)
	if title == "" {
		title = spec.Title
	}
	builder := panel.Table(table.ID, title, dsName).
		Span(12).
		Height("420px").
		IDField(panel.Ref(table.IDField))
	if strings.TrimSpace(table.Height) != "" {
		builder.Height(table.Height)
	}
	if strings.TrimSpace(table.Description) != "" {
		builder.Description(table.Description)
	}
	if table.Searchable {
		builder.Searchable()
	}

	columns := table.Columns
	if len(columns) == 0 {
		columns = defaultLeafTableColumns(spec)
	}
	if len(columns) > 0 {
		builder.Columns(columns...)
	}
	switch {
	case table.RowAction != nil:
		builder.Action(cloneActionSpec(*table.RowAction))
	default:
		if extURL := spec.ResolveExternalLeafURL(goCtx, ctx, literalCubeParams(spec)); extURL != "" {
			builder.Action(action.Navigate(extURL).WithPreservedQuery())
		} else {
			builder.Terminal()
		}
	}
	return ds, builder.Build()
}

func defaultLeafTableColumns(spec CubeSpec) []panel.TableColumn {
	cols := make([]panel.TableColumn, 0, len(spec.Dimensions)+len(spec.Measures))
	seen := make(map[string]struct{})
	addCol := func(col panel.TableColumn) {
		name := col.Field.Name()
		if name == "" {
			return
		}
		if _, dup := seen[name]; dup {
			return
		}
		seen[name] = struct{}{}
		cols = append(cols, col)
	}
	switch spec.DataMode {
	case DataModeSQL:
		for _, dim := range spec.Dimensions {
			if dim.sqlColumn() == "" {
				continue
			}
			addCol(panel.TableColumn{Field: panel.Ref(dim.Name), Label: dim.Label})
		}
		for _, measure := range spec.Measures {
			col := strings.TrimSpace(measure.Column)
			if col == "" || col == "*" || strings.Contains(strings.ToUpper(col), "DISTINCT") {
				continue
			}
			addCol(panel.TableColumn{Field: panel.Ref(measure.Name), Label: measure.Label, Formatter: measure.Formatter, Align: "right"})
		}
	default:
		for _, dim := range spec.Dimensions {
			fieldName := dim.datasetValueField()
			if dim.Type != DimensionTypeTimeBucket && strings.TrimSpace(dim.LabelField) != "" {
				fieldName = strings.TrimSpace(dim.LabelField)
			}
			addCol(panel.TableColumn{Field: panel.Ref(fieldName), Label: dim.Label})
		}
		for _, measure := range spec.Measures {
			if strings.TrimSpace(measure.Field) == "" {
				continue
			}
			addCol(panel.TableColumn{Field: panel.Ref(measure.Field), Label: measure.Label, Formatter: measure.Formatter, Align: "right"})
		}
	}
	return cols
}

func buildDimensionPanel(spec CubeSpec, dim DimensionSpec, resolved dimensionDatasetResolution, baseURL string, remainingCount, index int) panel.Spec {
	return buildDimensionPanelWithContext(spec, DrillContext{}, dim, resolved, baseURL, remainingCount, index)
}

func buildDimensionPanelWithContext(spec CubeSpec, ctx DrillContext, dim DimensionSpec, resolved dimensionDatasetResolution, baseURL string, remainingCount, index int) panel.Spec {
	// Dimension charts use the first measure as their value axis.
	// Additional measures appear only in stat panels.
	measure := spec.Measures[0]
	actionURL := baseURL
	panelAction := action.CrossFilter(actionURL, dim.Name)
	if len(spec.DrillPath) > 0 {
		panelAction = action.CubeDrill(actionURL, dim.Name).WithPreservedQuery()
		if nextDim := nextDrillDimensionExcluding(spec, ctx, dim.Name); nextDim != "" {
			panelAction = panelAction.WithDrillGroupBy(nextDim)
		}
	}
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
		Action(panelAction)
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
			Action(panelAction).
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
	case panel.KindTimeSeries:
		return panel.TimeSeries(id, title, dataset)
	case panel.KindStat,
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

func chartDatasetName(chartID string) string {
	return chartDatasetNamePrefix + "_" + strings.ReplaceAll(strings.TrimSpace(chartID), " ", "_")
}

func leafDatasetName(tableID string) string {
	return leafDatasetNamePrefix + "_" + strings.ReplaceAll(strings.TrimSpace(tableID), " ", "_")
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
