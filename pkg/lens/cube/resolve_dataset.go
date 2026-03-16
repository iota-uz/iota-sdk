package cube

import (
	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/transform"
)

const baseDatasetName = "cube_base"

func resolveDatasetStatsDataset(spec CubeSpec, ctx DrillContext, name string) lens.DatasetSpec {
	return lens.DatasetSpec{
		Name:       name,
		Kind:       lens.DatasetKindTransform,
		DependsOn:  []string{baseDatasetName},
		Transforms: append(filteredTransforms(spec, ctx), aggregateTransforms(spec, nil)...),
	}
}

func resolveDatasetDimensionDataset(spec CubeSpec, ctx DrillContext, dim DimensionSpec, name string) lens.DatasetSpec {
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
	transforms = append(transforms, transform.Spec{
		Kind: transform.KindLookup,
		Lookup: &transform.LookupConfig{
			Other:      baseDatasetName,
			LocalField: "label",
			OtherField: groupBy,
			Fields: map[string]string{
				lookupSource: "filter_value",
			},
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
	return lens.DatasetSpec{
		Name:       name,
		Kind:       lens.DatasetKindTransform,
		DependsOn:  []string{baseDatasetName},
		Transforms: transforms,
	}
}

func resolveDatasetLeafDataset(spec CubeSpec, ctx DrillContext, name string) lens.DatasetSpec {
	return lens.DatasetSpec{
		Name:       name,
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
		field := dim.Field
		if field == "" {
			field = dim.LabelField
		}
		if field == "" {
			continue
		}
		predicates = append(predicates, transform.Predicate{
			Field: field,
			Op:    "=",
			Value: filter.Value,
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

func baseDataset(spec CubeSpec) lens.DatasetSpec {
	return lens.DatasetSpec{
		Name:   baseDatasetName,
		Kind:   lens.DatasetKindStatic,
		Static: spec.Data.Clone(),
	}
}

