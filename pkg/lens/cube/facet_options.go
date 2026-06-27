package cube

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const (
	QueryFacet       = "_facet"
	QueryFacetSearch = "_facet_search"
)

type SQLFacetOptionsLookup func(context.Context, string, map[string]lens.ParamValue, int) ([]lens.DrillFacetOptionMeta, error)

func ResolveFacetOptions(
	ctx context.Context,
	spec CubeSpec,
	drillCtx DrillContext,
	dimension string,
	search string,
	limit int,
	lookup SQLFacetOptionsLookup,
) ([]lens.DrillFacetOptionMeta, error) {
	const op serrors.Op = "cube.ResolveFacetOptions"

	if limit <= 0 {
		limit = 50
	}
	dim, ok := spec.Dimension(strings.TrimSpace(dimension))
	if !ok {
		return nil, serrors.E(op, fmt.Errorf("unknown cube dimension %q", dimension))
	}
	switch spec.DataMode {
	case DataModeDataset:
		return resolveDatasetFacetOptions(spec, drillCtx, dim, search, limit), nil
	case DataModeSQL:
		if lookup == nil {
			return nil, serrors.E(op, fmt.Errorf("sql facet lookup is required"))
		}
		text, params := sqlFacetOptionsQuery(spec, drillCtx, dim, search)
		options, err := lookup(ctx, text, params, limit)
		if err != nil {
			return nil, serrors.E(op, err)
		}
		markSelected(options, drillCtx, dim.Name)
		return options, nil
	default:
		return nil, serrors.E(op, fmt.Errorf("unsupported cube mode %q", spec.DataMode))
	}
}

func resolveDatasetFacetOptions(spec CubeSpec, drillCtx DrillContext, dim DimensionSpec, search string, limit int) []lens.DrillFacetOptionMeta {
	if spec.Data == nil || spec.Data.Primary() == nil {
		return nil
	}
	valueField := strings.TrimSpace(dim.Field)
	labelField := strings.TrimSpace(dim.LabelField)
	if labelField == "" {
		labelField = valueField
	}
	if valueField == "" {
		valueField = labelField
	}
	otherFilters := drillCtx.WithoutDimension(dim.Name)
	search = strings.ToLower(strings.TrimSpace(search))
	type bucket struct {
		value string
		label string
		count int
	}
	buckets := map[string]*bucket{}
	for _, row := range spec.Data.Primary().Rows() {
		if !rowMatchesDrill(row, spec, otherFilters) {
			continue
		}
		value := strings.TrimSpace(fmt.Sprint(row[valueField]))
		if value == "" {
			continue
		}
		label := strings.TrimSpace(fmt.Sprint(row[labelField]))
		if label == "" {
			label = value
		}
		if search != "" && !strings.Contains(strings.ToLower(label), search) && !strings.Contains(strings.ToLower(value), search) {
			continue
		}
		item := buckets[value]
		if item == nil {
			item = &bucket{value: value, label: label}
			buckets[value] = item
		}
		item.count++
	}
	options := make([]lens.DrillFacetOptionMeta, 0, len(buckets))
	for _, item := range buckets {
		options = append(options, lens.DrillFacetOptionMeta{
			Dimension: dim.Name,
			Value:     item.value,
			Label:     item.label,
			Count:     item.count,
		})
	}
	sort.SliceStable(options, func(i, j int) bool {
		if options[i].Count != options[j].Count {
			return options[i].Count > options[j].Count
		}
		return options[i].Label < options[j].Label
	})
	if len(options) > limit {
		options = options[:limit]
	}
	markSelected(options, drillCtx, dim.Name)
	return options
}

func sqlFacetOptionsQuery(spec CubeSpec, drillCtx DrillContext, dim DimensionSpec, search string) (string, map[string]lens.ParamValue) {
	facetCtx := drillCtx.WithoutDimension(dim.Name)
	if dim.Override != nil && dim.Override.Query != nil {
		return sqlOverrideFacetOptionsQuery(spec, facetCtx, dim, search)
	}
	joins := requiredSQLJoins(spec, dim, facetCtx)
	labelColumn := strings.TrimSpace(dim.LabelColumn)
	if labelColumn == "" {
		labelColumn = dim.Column
	}
	text := "SELECT\n  " + dim.Column + " AS value,\n  " + labelColumn + " AS label,\n  COUNT(*)::int AS count\nFROM " + spec.FromSQL + sqlJoinSQL(spec, joins)
	where := sqlWhere(spec, facetCtx)
	search = strings.TrimSpace(search)
	if search != "" {
		searchClause := "(" + labelColumn + "::text ILIKE @facet_search OR " + dim.Column + "::text ILIKE @facet_search)"
		if where == "" {
			where = searchClause
		} else {
			where += "\n  AND " + searchClause
		}
	}
	if where != "" {
		text += "\nWHERE " + where
	}
	text += "\nGROUP BY value, label\nORDER BY count DESC, label ASC"

	params := sqlParams(spec, facetCtx)
	if search != "" {
		params["facet_search"] = lens.ParamValue{Literal: "%" + search + "%"}
	}
	return text, params
}

func sqlOverrideFacetOptionsQuery(spec CubeSpec, drillCtx DrillContext, dim DimensionSpec, search string) (string, map[string]lens.ParamValue) {
	source := resolveOverrideDataset(spec, drillCtx, *dim.Override, datasetName(dim.Name)+"_facet_source")
	if source.Query == nil {
		return "", nil
	}
	query := *source.Query
	countColumn := firstOverrideCountColumn(spec)
	text := "SELECT\n  filter_value AS value,\n  label,\n  " + countColumn + "::int AS count\nFROM (\n" + query.Text + "\n) facet_options"
	search = strings.TrimSpace(search)
	if search != "" {
		text += "\nWHERE label::text ILIKE @facet_search OR filter_value::text ILIKE @facet_search"
	}
	text += "\nORDER BY count DESC, label ASC"
	params := cloneParamValues(query.Params)
	if search != "" {
		params["facet_search"] = lens.ParamValue{Literal: "%" + search + "%"}
	}
	return text, params
}

func firstOverrideCountColumn(spec CubeSpec) string {
	if len(spec.Measures) > 0 && strings.TrimSpace(spec.Measures[0].Name) != "" {
		return spec.Measures[0].Name
	}
	return "count"
}

func markSelected(options []lens.DrillFacetOptionMeta, drillCtx DrillContext, dimension string) {
	selected := map[string]struct{}{}
	for _, filter := range drillCtx.Filters {
		if filter.Dimension != dimension {
			continue
		}
		for _, value := range filter.values() {
			selected[value] = struct{}{}
		}
	}
	for idx := range options {
		if options[idx].Dimension == "" {
			options[idx].Dimension = dimension
		}
		_, options[idx].Selected = selected[options[idx].Value]
	}
}
