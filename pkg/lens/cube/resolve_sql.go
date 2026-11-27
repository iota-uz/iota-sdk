package cube

import (
	"fmt"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/datasource"
)

func resolveSQLStatsDataset(spec CubeSpec, ctx DrillContext, name string) lens.DatasetSpec {
	joins := requiredSQLJoins(spec, DimensionSpec{}, ctx)
	text := "SELECT\n  " + strings.Join(sqlMeasureSelects(spec.Measures), ",\n  ") + "\nFROM " + spec.FromSQL + sqlJoinSQL(spec, joins)
	where := sqlWhere(spec, ctx)
	if where != "" {
		text += "\nWHERE " + where
	}
	return lens.DatasetSpec{
		Name:   name,
		Kind:   lens.DatasetKindQuery,
		Source: spec.DataSource,
		Query: &lens.QuerySpec{
			Text:   text,
			Params: sqlParams(spec, ctx),
			Kind:   datasource.QueryKindRaw,
		},
	}
}

func resolveSQLDimensionDataset(spec CubeSpec, ctx DrillContext, dim DimensionSpec, name string) lens.DatasetSpec {
	var primaryMeasure MeasureSpec
	if len(spec.Measures) > 0 {
		primaryMeasure = spec.Measures[0]
	}
	return resolveSQLChartDataset(spec, ctx, dim, nil, primaryMeasure, name)
}

func resolveSQLChartDataset(
	spec CubeSpec,
	ctx DrillContext,
	dim DimensionSpec,
	seriesDim *DimensionSpec,
	orderMeasure MeasureSpec,
	name string,
) lens.DatasetSpec {
	joins := requiredSQLJoins(spec, dim, ctx)
	if seriesDim != nil {
		addJoinNames(joins, seriesDim.RequiresJoin)
	}
	dimCol := dim.sqlColumn()
	labelCol := dim.sqlLabelColumn()
	measureSelects := make([]string, 0, len(spec.Measures))
	for _, measure := range spec.Measures {
		measureSelects = append(measureSelects, sqlMeasureSelect(measure))
	}
	text := "SELECT\n  " + dimCol + " AS filter_value,\n  " + labelCol + " AS label"
	if seriesDim != nil {
		text += ",\n  " + seriesDim.sqlColumn() + " AS series_value,\n  " + seriesDim.sqlLabelColumn() + " AS series"
	}
	if strings.TrimSpace(dim.ColorColumn) != "" {
		text += ",\n  " + dim.ColorColumn + " AS color_value"
	}
	if len(measureSelects) > 0 {
		text += ",\n  " + strings.Join(measureSelects, ",\n  ")
	}
	text += "\nFROM " + spec.FromSQL + sqlJoinSQL(spec, joins)
	where := sqlWhere(spec, ctx)
	if where != "" {
		text += "\nWHERE " + where
	}
	groupBy := []string{"filter_value", "label"}
	if seriesDim != nil {
		groupBy = append(groupBy, "series_value", "series")
	}
	if strings.TrimSpace(dim.ColorColumn) != "" {
		groupBy = append(groupBy, "color_value")
	}
	text += "\nGROUP BY " + strings.Join(groupBy, ", ")
	if dim.Type == DimensionTypeTimeBucket {
		if seriesDim != nil {
			text += "\nORDER BY filter_value ASC, series ASC"
		} else {
			text += "\nORDER BY filter_value ASC"
		}
	} else if strings.TrimSpace(orderMeasure.Name) != "" {
		if seriesDim != nil {
			text += "\nORDER BY " + orderMeasure.Name + " DESC, label ASC, series ASC"
		} else {
			text += "\nORDER BY " + orderMeasure.Name + " DESC, label ASC"
		}
	}
	return lens.DatasetSpec{
		Name:   name,
		Kind:   lens.DatasetKindQuery,
		Source: spec.DataSource,
		Query: &lens.QuerySpec{
			Text:   text,
			Params: sqlParams(spec, ctx),
			Kind:   datasource.QueryKindRaw,
		},
	}
}

func resolveSQLLeafDataset(spec CubeSpec, ctx DrillContext, table *LeafTableSpec, name string) lens.DatasetSpec {
	if table != nil && strings.TrimSpace(table.SQL) != "" {
		return lens.DatasetSpec{
			Name:   name,
			Title:  table.Title,
			Kind:   lens.DatasetKindQuery,
			Source: spec.DataSource,
			Query: &lens.QuerySpec{
				Text:   table.SQL,
				Params: mergeParamValues(overrideBaseParams(spec), overrideFilterParams(spec, ctx)),
				Kind:   datasource.QueryKindRaw,
			},
		}
	}
	joins := requiredSQLJoins(spec, DimensionSpec{}, ctx)
	selects := make([]string, 0, len(spec.Dimensions)+len(spec.Measures))
	for _, dim := range spec.Dimensions {
		addJoinNames(joins, dim.RequiresJoin)
		col := dim.sqlLabelColumn()
		if col == "" {
			continue
		}
		selects = append(selects, col+" AS "+dim.Name)
	}
	for _, measure := range spec.Measures {
		addJoinNames(joins, measure.RequiresJoin)
		col := strings.TrimSpace(measure.Column)
		if col == "" || col == "*" || strings.Contains(strings.ToUpper(col), "DISTINCT") {
			continue
		}
		selects = append(selects, col+" AS "+measure.Name)
	}
	if len(selects) == 0 {
		selects = []string{"*"}
	}
	text := "SELECT\n  " + strings.Join(selects, ",\n  ") + "\nFROM " + spec.FromSQL + sqlJoinSQL(spec, joins)
	if where := sqlWhere(spec, ctx); where != "" {
		text += "\nWHERE " + where
	}
	if table != nil && strings.TrimSpace(table.OrderBy) != "" {
		text += "\nORDER BY " + strings.TrimSpace(table.OrderBy)
	}
	title := ""
	if table != nil {
		title = table.Title
	}
	return lens.DatasetSpec{
		Name:   name,
		Title:  title,
		Kind:   lens.DatasetKindQuery,
		Source: spec.DataSource,
		Query: &lens.QuerySpec{
			Text:   text,
			Params: sqlParams(spec, ctx),
			Kind:   datasource.QueryKindRaw,
		},
	}
}

func sqlMeasureSelects(measures []MeasureSpec) []string {
	out := make([]string, 0, len(measures))
	for _, measure := range measures {
		out = append(out, sqlMeasureSelect(measure))
	}
	return out
}

func sqlMeasureSelect(measure MeasureSpec) string {
	switch measure.Aggregation {
	case AggregationCount:
		column := strings.TrimSpace(measure.Column)
		if column == "" || column == "*" {
			return "COUNT(*)::float8 AS " + measure.Name
		}
		return fmt.Sprintf("COUNT(%s)::float8 AS %s", column, measure.Name)
	case AggregationAvg:
		return fmt.Sprintf("COALESCE(AVG(%s), 0)::float8 AS %s", measure.Column, measure.Name)
	case AggregationSum:
		fallthrough
	default:
		return fmt.Sprintf("COALESCE(SUM(%s), 0)::float8 AS %s", measure.Column, measure.Name)
	}
}

func sqlWhere(spec CubeSpec, ctx DrillContext) string {
	clauses := make([]string, 0, len(spec.Where)+len(ctx.Filters))
	clauses = append(clauses, spec.Where...)
	for _, filter := range ctx.Filters {
		dim, ok := spec.Dimension(filter.Dimension)
		if !ok {
			continue
		}
		values := filter.values()
		if len(values) == 0 {
			continue
		}
		col := dim.sqlColumn()
		if col == "" {
			continue
		}
		paramName := sqlFilterParam(filter.Dimension)
		if containsString(values, UnknownDrillValue) {
			clauses = append(clauses, fmt.Sprintf("((%s) IS NULL OR (%s)::text = ANY(@%s))", col, col, paramName))
			continue
		}
		clauses = append(clauses, fmt.Sprintf("%s = ANY(@%s)", col, paramName))
	}
	return strings.Join(clauses, "\n  AND ")
}

func sqlParams(spec CubeSpec, ctx DrillContext) map[string]lens.ParamValue {
	params := make(map[string]lens.ParamValue, len(spec.Params)+len(ctx.Filters))
	for key, value := range spec.Params {
		params[key] = value
	}
	for _, filter := range ctx.Filters {
		values := filter.values()
		if len(values) == 0 {
			continue
		}
		params[sqlFilterParam(filter.Dimension)] = lens.ParamValue{Literal: sqlFilterValues(values)}
	}
	return params
}

func sqlFilterValues(values []string) []string {
	if !containsString(values, UnknownDrillValue) {
		return values
	}
	expanded := make([]string, 0, len(values)+1)
	expanded = append(expanded, "")
	for _, v := range values {
		if v == "" {
			continue
		}
		expanded = append(expanded, v)
	}
	return expanded
}

func sqlFilterParam(name string) string {
	return "f_" + strings.ReplaceAll(strings.TrimSpace(name), " ", "_")
}

func requiredSQLJoins(spec CubeSpec, active DimensionSpec, ctx DrillContext) map[string]struct{} {
	required := map[string]struct{}{}
	addJoinNames(required, active.RequiresJoin)
	for _, measure := range spec.Measures {
		addJoinNames(required, measure.RequiresJoin)
	}
	for _, filter := range ctx.Filters {
		dim, ok := spec.Dimension(filter.Dimension)
		if !ok {
			continue
		}
		addJoinNames(required, dim.RequiresJoin)
	}
	return required
}

func addJoinNames(target map[string]struct{}, names []string) {
	for _, name := range names {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			target[trimmed] = struct{}{}
		}
	}
}

func sqlJoinSQL(spec CubeSpec, required map[string]struct{}) string {
	if len(spec.Joins) == 0 {
		return ""
	}
	var builder strings.Builder
	for _, join := range spec.Joins {
		if len(required) > 0 {
			if _, ok := required[join.Name]; !ok {
				continue
			}
		}
		builder.WriteByte('\n')
		builder.WriteString(join.SQL)
	}
	return builder.String()
}
