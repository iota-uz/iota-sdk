package cube

import (
	"context"
	"fmt"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/lens"
)

type SQLLabelLookup func(context.Context, string, map[string]any) (string, bool, error)

func ResolveDrillFilters(
	ctx context.Context,
	spec CubeSpec,
	drillCtx DrillContext,
	lookup SQLLabelLookup,
) ([]lens.DrillFilterMeta, error) {
	filters := make([]lens.DrillFilterMeta, 0, len(drillCtx.Filters))
	for _, item := range drillCtx.Filters {
		display, err := resolveFilterDisplay(ctx, spec, drillCtx, item, lookup)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(display) == "" {
			display = item.Value
		}
		filters = append(filters, lens.DrillFilterMeta{
			Dimension: item.Dimension,
			Value:     item.Value,
			Display:   display,
		})
	}
	return filters, nil
}

func resolveFilterDisplay(
	ctx context.Context,
	spec CubeSpec,
	drillCtx DrillContext,
	filter DimensionFilter,
	lookup SQLLabelLookup,
) (string, error) {
	dim, ok := spec.Dimension(filter.Dimension)
	if !ok {
		return filter.Value, nil
	}
	switch spec.DataMode {
	case DataModeDataset:
		return datasetFilterDisplay(spec, drillCtx, dim), nil
	case DataModeSQL:
		if lookup == nil {
			return filter.Value, nil
		}
		text, params := sqlFilterLabelQuery(spec, drillCtx, dim)
		label, ok, err := lookup(ctx, text, params)
		if err != nil {
			return "", err
		}
		if !ok {
			return filter.Value, nil
		}
		return label, nil
	default:
		return filter.Value, nil
	}
}

func datasetFilterDisplay(spec CubeSpec, drillCtx DrillContext, dim DimensionSpec) string {
	if spec.Data == nil || spec.Data.Primary() == nil {
		return ""
	}
	labelField := strings.TrimSpace(dim.LabelField)
	if labelField == "" {
		labelField = strings.TrimSpace(dim.Field)
	}
	if labelField == "" {
		return ""
	}
	rows := spec.Data.Primary().Rows()
	for _, row := range rows {
		if !rowMatchesDrill(row, spec, drillCtx) {
			continue
		}
		label := strings.TrimSpace(fmt.Sprint(row[labelField]))
		if label != "" {
			return label
		}
	}
	return ""
}

func rowMatchesDrill(row map[string]any, spec CubeSpec, drillCtx DrillContext) bool {
	for _, filter := range drillCtx.Filters {
		dim, ok := spec.Dimension(filter.Dimension)
		if !ok {
			continue
		}
		field := strings.TrimSpace(dim.Field)
		if field == "" {
			field = strings.TrimSpace(dim.LabelField)
		}
		if field == "" {
			continue
		}
		if strings.TrimSpace(fmt.Sprint(row[field])) != strings.TrimSpace(filter.Value) {
			return false
		}
	}
	return true
}

func sqlFilterLabelQuery(spec CubeSpec, drillCtx DrillContext, dim DimensionSpec) (string, map[string]any) {
	joins := requiredSQLJoins(spec, dim, drillCtx)
	labelColumn := strings.TrimSpace(dim.LabelColumn)
	if labelColumn == "" {
		labelColumn = dim.Column
	}
	text := "SELECT " + labelColumn + " AS label\nFROM " + spec.FromSQL + sqlJoinSQL(spec, joins)
	where := sqlWhere(spec, drillCtx)
	if where != "" {
		text += "\nWHERE " + where
	}
	text += "\nLIMIT 1"

	params := make(map[string]any, len(spec.Params)+len(drillCtx.Filters))
	for key, value := range sqlParams(spec, drillCtx) {
		params[key] = value.Literal
	}
	return text, params
}
