package cube

import (
	"sort"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/datasource"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/transform"
)

const (
	baseDatasetName      = "cube_base"
	labelMapDatasetPrefx = "cube_labelmap_"
)

func resolveDatasetStatsDataset(spec CubeSpec, ctx DrillContext, name string) lens.DatasetSpec {
	return lens.DatasetSpec{
		Name:       name,
		Kind:       lens.DatasetKindTransform,
		DependsOn:  []string{baseDatasetName},
		Transforms: append(filteredTransforms(spec, ctx), aggregateTransforms(spec, nil)...),
	}
}

func resolveDatasetDimensionDataset(spec CubeSpec, ctx DrillContext, dim DimensionSpec, name string) lens.DatasetSpec {
	if dim.Type == DimensionTypeTimeBucket || len(dim.LabelMap) > 0 {
		var orderMeasure MeasureSpec
		if len(spec.Measures) > 0 {
			orderMeasure = spec.Measures[0]
		}
		return resolveDatasetChartDataset(spec, ctx, dim, nil, orderMeasure, dim.Transforms, name)
	}
	groupBy := dim.Field
	if dim.LabelField != "" {
		groupBy = dim.LabelField
	}
	transforms := append(filteredTransforms(spec, ctx), aggregateTransforms(spec, []string{groupBy})...)
	transforms = append(transforms, transform.Spec{
		Kind: transform.KindRename,
		Aliases: map[string]string{
			groupBy: "label",
		},
	})
	lookupSource := dim.Field
	if lookupSource == "" {
		lookupSource = groupBy
	}
	lookupFields := map[string]string{
		lookupSource: "filter_value",
	}
	if colorSource := dim.ColorField; colorSource != "" && colorSource != lookupSource {
		lookupFields[colorSource] = "color_value"
	}
	transforms = append(transforms, transform.Spec{
		Kind: transform.KindLookup,
		Lookup: &transform.LookupConfig{
			Other:      baseDatasetName,
			LocalField: "label",
			OtherField: groupBy,
			Fields:     lookupFields,
		},
	})
	// Sort dimension values by the first measure (descending).
	transforms = append(transforms, transform.Spec{
		Kind: transform.KindSort,
		Sort: []transform.SortField{{
			Field:     spec.Measures[0].Name,
			Direction: transform.SortDesc,
		}},
	})
	transforms = append(transforms, resolvedDimensionTransforms(spec, dim.Transforms)...)
	return lens.DatasetSpec{
		Name:       name,
		Kind:       lens.DatasetKindTransform,
		DependsOn:  []string{baseDatasetName},
		Transforms: transforms,
	}
}

func resolveDatasetChartDataset(
	spec CubeSpec,
	ctx DrillContext,
	dim DimensionSpec,
	seriesDim *DimensionSpec,
	orderMeasure MeasureSpec,
	extraTransforms []transform.Spec,
	name string,
) lens.DatasetSpec {
	dimField := dim.datasetValueField()
	dependsOn := []string{baseDatasetName}
	groupByFields := []string{dimField}
	renameAliases := map[string]string{
		dimField: "filter_value",
	}
	if seriesDim != nil {
		seriesField := seriesDim.datasetValueField()
		groupByFields = append(groupByFields, seriesField)
		renameAliases[seriesField] = "series_value"
	}

	transforms := append(filteredTransforms(spec, ctx), aggregateTransforms(spec, groupByFields)...)
	transforms = append(transforms, transform.Spec{
		Kind:    transform.KindRename,
		Aliases: renameAliases,
	})

	// Attach display label for primary dimension from cube_base (LabelField or value field).
	dimLabelSource := strings.TrimSpace(dim.LabelField)
	if dimLabelSource == "" {
		dimLabelSource = dimField
	}
	dimLookupFields := map[string]string{
		dimLabelSource: "label",
	}
	if colorSource := strings.TrimSpace(dim.ColorField); colorSource != "" && colorSource != dimField {
		dimLookupFields[colorSource] = "color_value"
	}
	transforms = append(transforms, transform.Spec{
		Kind: transform.KindLookup,
		Lookup: &transform.LookupConfig{
			Other:      baseDatasetName,
			LocalField: "filter_value",
			OtherField: dimField,
			Fields:     dimLookupFields,
		},
	})
	if len(dim.LabelMap) > 0 {
		labelDS := labelMapDatasetName(dim.Name)
		dependsOn = append(dependsOn, labelDS)
		transforms = append(transforms, transform.Spec{
			Kind: transform.KindLookup,
			Lookup: &transform.LookupConfig{
				Other:      labelDS,
				LocalField: "filter_value",
				OtherField: "filter_value",
				Fields:     map[string]string{"label": "label"},
			},
		})
	}

	// Attach display label for optional series dimension.
	if seriesDim != nil {
		seriesField := seriesDim.datasetValueField()
		seriesLabelSource := strings.TrimSpace(seriesDim.LabelField)
		if seriesLabelSource == "" {
			seriesLabelSource = seriesField
		}
		transforms = append(transforms, transform.Spec{
			Kind: transform.KindLookup,
			Lookup: &transform.LookupConfig{
				Other:      baseDatasetName,
				LocalField: "series_value",
				OtherField: seriesField,
				Fields:     map[string]string{seriesLabelSource: "series"},
			},
		})
		if len(seriesDim.LabelMap) > 0 {
			seriesLabelDS := labelMapDatasetName(seriesDim.Name)
			if !containsString(dependsOn, seriesLabelDS) {
				dependsOn = append(dependsOn, seriesLabelDS)
			}
			transforms = append(transforms, transform.Spec{
				Kind: transform.KindLookup,
				Lookup: &transform.LookupConfig{
					Other:      seriesLabelDS,
					LocalField: "series_value",
					OtherField: "filter_value",
					Fields:     map[string]string{"label": "series"},
				},
			})
		}
	}

	// Sort chronologically for time-bucketed dimensions; descending by measure otherwise.
	if dim.Type == DimensionTypeTimeBucket {
		sortFields := []transform.SortField{{Field: "filter_value", Direction: transform.SortAsc}}
		if seriesDim != nil {
			sortFields = append(sortFields, transform.SortField{Field: "series", Direction: transform.SortAsc})
		}
		transforms = append(transforms, transform.Spec{Kind: transform.KindSort, Sort: sortFields})
	} else if strings.TrimSpace(orderMeasure.Name) != "" {
		sortFields := []transform.SortField{
			{Field: orderMeasure.Name, Direction: transform.SortDesc},
			{Field: "label", Direction: transform.SortAsc},
		}
		if seriesDim != nil {
			sortFields = append(sortFields, transform.SortField{Field: "series", Direction: transform.SortAsc})
		}
		transforms = append(transforms, transform.Spec{Kind: transform.KindSort, Sort: sortFields})
	}

	transforms = append(transforms, resolvedDimensionTransforms(spec, extraTransforms)...)
	return lens.DatasetSpec{
		Name:       name,
		Kind:       lens.DatasetKindTransform,
		DependsOn:  dependsOn,
		Transforms: transforms,
	}
}

func resolveDatasetLeafDataset(spec CubeSpec, ctx DrillContext, table *LeafTableSpec, name string) lens.DatasetSpec {
	title := ""
	if table != nil {
		title = table.Title
	}
	return lens.DatasetSpec{
		Name:       name,
		Title:      title,
		Kind:       lens.DatasetKindTransform,
		DependsOn:  []string{baseDatasetName},
		Transforms: filteredTransforms(spec, ctx),
	}
}

func filteredTransforms(spec CubeSpec, ctx DrillContext) []transform.Spec {
	predicates := make([]transform.Predicate, 0, len(ctx.Filters))
	for _, filter := range ctx.Filters {
		dim, ok := spec.Dimension(filter.Dimension)
		if !ok {
			continue
		}
		field := dim.datasetValueField()
		if field == "" {
			continue
		}
		values := filter.values()
		if len(values) == 0 {
			continue
		}
		var predicateVal any = values
		if containsString(values, UnknownDrillValue) {
			expanded := make([]any, 0, len(values)+2)
			expanded = append(expanded, nil, "")
			for _, value := range values {
				if value == "" {
					continue
				}
				expanded = append(expanded, value)
			}
			predicateVal = expanded
		}
		predicates = append(predicates, transform.Predicate{
			Field: field,
			Op:    "in",
			Value: predicateVal,
		})
	}
	if len(predicates) == 0 {
		return nil
	}
	return []transform.Spec{{
		Kind:       transform.KindFilterRows,
		Predicates: predicates,
	}}
}

func aggregateTransforms(spec CubeSpec, groupBy []string) []transform.Spec {
	aggregates := make([]transform.Aggregate, 0, len(spec.Measures))
	for _, measure := range spec.Measures {
		funcName := string(measure.Aggregation)
		if funcName == "" {
			funcName = string(AggregationSum)
		}
		aggregates = append(aggregates, transform.Aggregate{
			Field: measure.Field,
			As:    measure.Name,
			Func:  funcName,
		})
	}
	return []transform.Spec{{
		Kind:       transform.KindGroupBy,
		GroupBy:    groupBy,
		Aggregates: aggregates,
	}}
}

func baseDataset(spec CubeSpec, ctx DrillContext) lens.DatasetSpec {
	transforms := timeBucketTransforms(spec)
	if spec.DataMode == DataModeNamed {
		return lens.DatasetSpec{
			Name:       baseDatasetName,
			Kind:       lens.DatasetKindQuery,
			Source:     spec.DataSource,
			Transforms: transforms,
			Query: &lens.QuerySpec{
				Text:   spec.NamedQuery,
				Params: mergeParamValues(overrideBaseParams(spec), sqlParams(spec, ctx)),
				Kind:   datasource.QueryKindNamed,
			},
		}
	}
	return lens.DatasetSpec{
		Name:       baseDatasetName,
		Kind:       lens.DatasetKindStatic,
		Static:     spec.Data.Clone(),
		Transforms: transforms,
	}
}

func timeBucketTransforms(spec CubeSpec) []transform.Spec {
	count := 0
	for _, dim := range spec.Dimensions {
		if dim.Type == DimensionTypeTimeBucket && dim.datasetSourceField() != "" {
			count++
		}
	}
	if count == 0 {
		return nil
	}
	specs := make([]transform.Spec, 0, count)
	for _, dim := range spec.Dimensions {
		if dim.Type != DimensionTypeTimeBucket {
			continue
		}
		source := dim.datasetSourceField()
		if source == "" {
			continue
		}
		specs = append(specs, transform.Spec{
			Kind: transform.KindBucketTime,
			BucketTime: &transform.BucketTimeConfig{
				Field:    source,
				As:       dim.datasetValueField(),
				Interval: normalizeTimeGranularity(dim.TimeGranularity),
				AsKey:    true,
			},
		})
	}
	return specs
}

func labelMapDatasetName(dimension string) string {
	return labelMapDatasetPrefx + strings.ReplaceAll(strings.TrimSpace(dimension), " ", "_")
}

func labelMapDatasets(spec CubeSpec) []lens.DatasetSpec {
	count := 0
	for _, dim := range spec.Dimensions {
		if len(dim.LabelMap) > 0 {
			count++
		}
	}
	if count == 0 {
		return nil
	}
	datasets := make([]lens.DatasetSpec, 0, count)
	for _, dim := range spec.Dimensions {
		if len(dim.LabelMap) == 0 {
			continue
		}
		keys := make([]string, 0, len(dim.LabelMap))
		for k := range dim.LabelMap {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		keyVals := make([]any, len(keys))
		lblVals := make([]any, len(keys))
		for i, k := range keys {
			keyVals[i] = k
			lblVals[i] = dim.LabelMap[k]
		}
		fr, err := frame.New(
			labelMapDatasetName(dim.Name),
			frame.Field{Name: "filter_value", Type: frame.FieldTypeString, Role: frame.RoleDimension, Values: keyVals},
			frame.Field{Name: "label", Type: frame.FieldTypeString, Role: frame.RoleDimension, Values: lblVals},
		)
		if err != nil {
			continue
		}
		fs, err := frame.NewFrameSet(fr)
		if err != nil {
			continue
		}
		datasets = append(datasets, lens.DatasetSpec{
			Name:   labelMapDatasetName(dim.Name),
			Kind:   lens.DatasetKindStatic,
			Static: fs,
		})
	}
	return datasets
}
