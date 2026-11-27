package cube

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
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
		return nil, serrors.Wrap(op, fmt.Errorf("unknown cube dimension %q", dimension))
	}
	switch spec.DataMode {
	case DataModeDataset:
		return ResolveFacetOptionsFromFrames(spec, spec.Data, drillCtx, dim, search, limit), nil
	case DataModeSQL:
		if lookup == nil {
			return nil, serrors.Wrap(op, fmt.Errorf("sql facet lookup is required"))
		}
		text, params := sqlFacetOptionsQuery(spec, drillCtx, dim, search)
		options, err := lookup(ctx, text, params, limit)
		if err != nil {
			return nil, serrors.Wrap(op, err)
		}
		for idx := range options {
			if strings.TrimSpace(options[idx].Label) == "" || options[idx].Label == options[idx].Value {
				options[idx].Label = dim.DisplayLabel(options[idx].Value)
			}
		}
		markSelected(options, drillCtx, dim.Name)
		return options, nil
	case DataModeNamed:
		if lookup == nil {
			return nil, serrors.Wrap(op, fmt.Errorf("named facet lookup is required"))
		}
		options, err := lookup(ctx, spec.NamedQuery, sqlParams(spec, drillCtx.WithoutDimension(dim.Name)), limit)
		if err != nil {
			return nil, serrors.Wrap(op, err)
		}
		markSelected(options, drillCtx, dim.Name)
		return options, nil
	default:
		return nil, serrors.Wrap(op, fmt.Errorf("unsupported cube mode %q", spec.DataMode))
	}
}

// ResolveFacetOptionsFromFrames computes facet options from an in-memory FrameSet
// for Dataset or Named cube modes.
func ResolveFacetOptionsFromFrames(
	spec CubeSpec,
	data *frame.FrameSet,
	drillCtx DrillContext,
	dim DimensionSpec,
	search string,
	limit int,
) []lens.DrillFacetOptionMeta {
	if limit <= 0 {
		limit = 50
	}
	if data == nil || data.Primary() == nil {
		return nil
	}
	valueField := strings.TrimSpace(dim.datasetSourceField())
	labelField := strings.TrimSpace(dim.LabelField)
	if labelField == "" {
		labelField = valueField
	}
	otherFilters := drillCtx.WithoutDimension(dim.Name)
	search = strings.ToLower(strings.TrimSpace(search))
	type bucket struct {
		value string
		label string
		count int
	}
	buckets := map[string]*bucket{}
	for _, row := range data.Primary().Rows() {
		if !rowMatchesDrill(row, spec, otherFilters) {
			continue
		}
		var value string
		if dim.Type == DimensionTypeTimeBucket {
			value = coerceRowTimeBucket(row[valueField], dim.TimeGranularity)
		} else {
			value = strings.TrimSpace(fmt.Sprint(row[valueField]))
		}
		if value == "" || value == "<nil>" {
			continue
		}
		var label string
		if len(dim.LabelMap) > 0 {
			label = dim.DisplayLabel(value)
		} else if dim.Type == DimensionTypeTimeBucket {
			label = value
		} else {
			label = strings.TrimSpace(fmt.Sprint(row[labelField]))
			if label == "" || label == "<nil>" {
				label = value
			}
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
		if dim.Type == DimensionTypeTimeBucket {
			return options[i].Value < options[j].Value
		}
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
	dimCol := dim.sqlColumn()
	labelColumn := dim.sqlLabelColumn()
	text := "SELECT\n  " + dimCol + " AS value,\n  " + labelColumn + " AS label,\n  COUNT(*)::int AS count\nFROM " + spec.FromSQL + sqlJoinSQL(spec, joins)
	where := sqlWhere(spec, facetCtx)
	search = strings.TrimSpace(search)
	if search != "" {
		searchClause := "(" + labelColumn + "::text ILIKE @facet_search OR " + dimCol + "::text ILIKE @facet_search)"
		if where == "" {
			where = searchClause
		} else {
			where += "\n  AND " + searchClause
		}
	}
	if where != "" {
		text += "\nWHERE " + where
	}
	if dim.Type == DimensionTypeTimeBucket {
		text += "\nGROUP BY 1, 2\nORDER BY value ASC"
	} else {
		text += "\nGROUP BY 1, 2\nORDER BY count DESC, label ASC"
	}

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
