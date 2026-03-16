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
	joins := requiredSQLJoins(spec, dim, ctx)
	labelColumn := dim.LabelColumn
	if strings.TrimSpace(labelColumn) == "" {
		labelColumn = dim.Column
	}
	measureSelects := make([]string, 0, len(spec.Measures))
	for _, measure := range spec.Measures {
		measureSelects = append(measureSelects, sqlMeasureSelect(measure))
	}
	text := "SELECT\n  " + dim.Column + " AS filter_value,\n  " + labelColumn + " AS label"
	if len(measureSelects) > 0 {
		text += ",\n  " + strings.Join(measureSelects, ",\n  ")
	}
	text += "\nFROM " + spec.FromSQL + sqlJoinSQL(spec, joins)
	where := sqlWhere(spec, ctx)
	if where != "" {
		text += "\nWHERE " + where
	}
	text += "\nGROUP BY filter_value, label"
	if len(spec.Measures) > 0 {
		text += "\nORDER BY " + spec.Measures[0].Name + " DESC, label ASC"
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
		clauses = append(clauses, fmt.Sprintf("%s = @%s", dim.Column, sqlFilterParam(filter.Dimension)))
	}
	return strings.Join(clauses, "\n  AND ")
}

func sqlParams(spec CubeSpec, ctx DrillContext) map[string]lens.ParamValue {
	params := make(map[string]lens.ParamValue, len(spec.Params)+len(ctx.Filters))
	for key, value := range spec.Params {
		params[key] = value
	}
	for _, filter := range ctx.Filters {
		params[sqlFilterParam(filter.Dimension)] = lens.ParamValue{Literal: filter.Value}
	}
	return params
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
