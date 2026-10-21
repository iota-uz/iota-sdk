package reporting

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/shopspring/decimal"
)

type Period struct {
	Start string `json:"start"`
	End   string `json:"end_exclusive"`
}
type Dimension struct {
	Name   string `json:"name"`
	Bucket string `json:"bucket,omitempty"` // day, week, month, quarter, year
}
type Filter struct {
	Field    string `json:"field"`
	Operator string `json:"operator"` // eq, ne, in, gt, gte, lt, lte, is_null, not_null
	Values   []any  `json:"values,omitempty"`
}
type Order struct {
	Field      string `json:"field"`
	Descending bool   `json:"descending,omitempty"`
}
type Plan struct {
	Dataset    string      `json:"dataset"`
	Version    string      `json:"version"`
	Mode       string      `json:"mode"` // detail, aggregate
	Period     Period      `json:"period"`
	Measures   []string    `json:"measures"`
	Dimensions []Dimension `json:"dimensions"`
	Filters    []Filter    `json:"filters,omitempty"`
	Order      []Order     `json:"order,omitempty"`
}
type Compiled struct {
	Plan       Plan
	Definition Dataset
	SQL        string
	Args       []any
	Columns    []Field
	Measures   []Measure
	// Aggregate diagnostics travel in the same statement result, then are
	// validated and removed before preview/export. They are never selectable.
	Diagnostics []Field
	Fingerprint string
}

func ParsePlan(input string) (Plan, error) {
	var p Plan
	d := json.NewDecoder(bytes.NewBufferString(input))
	d.DisallowUnknownFields()
	d.UseNumber()
	if err := d.Decode(&p); err != nil {
		return p, fmt.Errorf("%w: %v", ErrPlan, err)
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return p, ErrPlan
	}
	return p, nil
}

func (c *Catalog) Compile(p Plan) (*Compiled, error) {
	d, err := c.Dataset(p.Dataset)
	if err != nil {
		return nil, err
	}
	if p.Version != d.Version {
		return nil, fmt.Errorf("%w: dataset %s; current version %s", ErrVersion, d.ID, d.Version)
	}
	if (p.Mode != "detail" && p.Mode != "aggregate") || len(p.Measures) == 0 || len(p.Measures) > 16 || len(p.Dimensions) > 32 || len(p.Filters) > 32 || len(p.Order) > 16 {
		return nil, ErrPlan
	}
	zone, err := time.LoadLocation(d.Timezone)
	if err != nil {
		return nil, ErrDefinition
	}
	start, err := time.ParseInLocation("2006-01-02", p.Period.Start, zone)
	if err != nil {
		return nil, ErrPlan
	}
	end, err := time.ParseInLocation("2006-01-02", p.Period.End, zone)
	if err != nil || !start.Before(end) {
		return nil, ErrPlan
	}
	args := []any{start, end}
	if d.PeriodParameterType == "date" {
		args = []any{p.Period.Start, p.Period.End}
	}
	fields := map[string]Field{}
	for _, f := range d.Dimensions {
		fields[f.Name] = f
	}
	measures := map[string]Measure{}
	for _, m := range d.Measures {
		measures[m.Name] = m
	}
	out := &Compiled{Plan: p, Definition: d, Args: args}
	// Currency is a mandatory unbucketed dimension, including in detail plans.
	// Never silently add unlike currencies; normalize it into the recorded plan.
	out.Plan.Dimensions = append([]Dimension(nil), p.Dimensions...)
	selected := map[string]bool{}
	for _, name := range p.Measures {
		m, ok := measures[name]
		if !ok || selected[name] || (p.Mode == "detail" && m.Aggregation == "count") {
			return nil, fmt.Errorf("%w: measure %q is unknown, duplicated or incompatible with %s mode; use the discovered definition", ErrPlan, name, p.Mode)
		}
		selected[name] = true
		out.Measures = append(out.Measures, m)
		if m.CurrencyDimension != "" {
			found := false
			for _, dim := range out.Plan.Dimensions {
				if dim.Name == m.CurrencyDimension {
					found = true
					if dim.Bucket != "" {
						return nil, ErrPlan
					}
				}
			}
			if !found {
				out.Plan.Dimensions = append(out.Plan.Dimensions, Dimension{Name: m.CurrencyDimension})
			}
		}
	}
	selects, groups := []string{}, []string{}
	for _, dim := range out.Plan.Dimensions {
		f, ok := fields[dim.Name]
		if !ok || selected[dim.Name] {
			return nil, fmt.Errorf("%w: dimension %q is unknown or duplicated; use the discovered definition", ErrPlan, dim.Name)
		}
		selected[dim.Name] = true
		expression := quote(dim.Name)
		if dim.Bucket != "" {
			if f.Type != "date" || p.Mode != "aggregate" {
				return nil, ErrPlan
			}
			switch dim.Bucket {
			case "day", "week", "month", "quarter", "year":
			default:
				return nil, ErrPlan
			}
			expression = fmt.Sprintf("date_trunc('%s', %s::timestamp)::date", dim.Bucket, expression)
		}
		selects = append(selects, expression+" AS "+quote(dim.Name))
		groups = append(groups, expression)
		out.Columns = append(out.Columns, f)
	}
	for _, m := range out.Measures {
		expression := quote(m.Name)
		if p.Mode == "aggregate" {
			if m.Aggregation == "count" {
				expression = "COUNT(*)::numeric"
			} else {
				expression = "COALESCE(SUM(" + expression + "),0)"
			}
		}
		selects = append(selects, expression+" AS "+quote(m.Name))
		out.Columns = append(out.Columns, m.Field)
	}
	if p.Mode == "aggregate" {
		out.Diagnostics = append(out.Diagnostics, Field{Name: "_report_population_count", Type: "number"})
		selects = append(selects, `COUNT(*)::numeric AS "_report_population_count"`)
		for _, m := range out.Measures {
			if m.Aggregation != "sum" {
				continue
			}
			name := "_report_null_" + m.Name
			out.Diagnostics = append(out.Diagnostics, Field{Name: name, Type: "number"})
			selects = append(selects, "COUNT(*) FILTER (WHERE "+quote(m.Name)+" IS NULL)::numeric AS "+quote(name))
		}
	}
	conditions := []string{}
	for _, filter := range p.Filters {
		f, ok := fields[filter.Field]
		if !ok || len(filter.Values) > 100 {
			return nil, fmt.Errorf("%w: filter %q must name a registered dimension with at most 100 values", ErrPlan, filter.Field)
		}
		expression, values, err := compileFilter(f, filter, len(out.Args)+1)
		if err != nil {
			return nil, err
		}
		conditions = append(conditions, expression)
		out.Args = append(out.Args, values...)
	}
	parts := []string{"WITH report_source AS (", d.SourceSQL, ") SELECT", strings.Join(selects, ", "), "FROM report_source"}
	if len(conditions) > 0 {
		parts = append(parts, repo.JoinWhere(conditions...))
	}
	if p.Mode == "aggregate" && len(groups) > 0 {
		parts = append(parts, "GROUP BY", strings.Join(groups, ", "))
	}
	orders := []string{}
	ordered := map[string]bool{}
	for _, order := range p.Order {
		if !selected[order.Field] || ordered[order.Field] {
			return nil, ErrPlan
		}
		ordered[order.Field] = true
		direction := "ASC"
		if order.Descending {
			direction = "DESC"
		}
		orders = append(orders, quote(order.Field)+" "+direction+" NULLS LAST")
	}
	// Add all output columns as tie breakers; an ordering change cannot change
	// the population and a plan never carries LIMIT or OFFSET.
	for _, f := range out.Columns {
		if !ordered[f.Name] {
			orders = append(orders, quote(f.Name)+" ASC NULLS LAST")
		}
	}
	parts = append(parts, "ORDER BY", strings.Join(orders, ", "))
	out.SQL = repo.Join(parts...)
	encoded, err := json.Marshal(struct {
		Plan       Plan    `json:"plan"`
		Definition Dataset `json:"definition"`
	}{out.Plan, out.Definition})
	if err != nil {
		return nil, ErrPlan
	}
	out.Fingerprint = fmt.Sprintf("%x", sha256.Sum256(append(encoded, []byte(out.SQL)...)))
	return out, nil
}

func quote(name string) string { return `"` + name + `"` }

func compileFilter(f Field, filter Filter, position int) (string, []any, error) {
	column := quote(f.Name)
	if filter.Operator == "is_null" || filter.Operator == "not_null" {
		if len(filter.Values) != 0 {
			return "", nil, ErrPlan
		}
		if filter.Operator == "is_null" {
			return column + " IS NULL", nil, nil
		}
		return column + " IS NOT NULL", nil, nil
	}
	operators := map[string]string{"eq": "=", "ne": "<>", "gt": ">", "gte": ">=", "lt": "<", "lte": "<="}
	op, ok := operators[filter.Operator]
	if filter.Operator != "in" && (!ok || len(filter.Values) != 1) {
		return "", nil, ErrPlan
	}
	if filter.Operator == "in" && len(filter.Values) == 0 {
		return "", nil, ErrPlan
	}
	values, placeholders := []any{}, []string{}
	for i, v := range filter.Values {
		cast := "text"
		switch f.Type {
		case "string":
			if _, ok := v.(string); !ok {
				return "", nil, ErrPlan
			}
		case "number":
			cast = "numeric"
			switch v.(type) {
			case json.Number, string, float64, int, int64:
			default:
				return "", nil, ErrPlan
			}
			if _, err := decimal.NewFromString(fmt.Sprint(v)); err != nil {
				return "", nil, ErrPlan
			}
			v = fmt.Sprint(v)
		case "date":
			cast = "date"
			s, ok := v.(string)
			if !ok {
				return "", nil, ErrPlan
			}
			if _, err := time.Parse("2006-01-02", s); err != nil {
				return "", nil, ErrPlan
			}
		}
		values = append(values, v)
		placeholders = append(placeholders, fmt.Sprintf("$%d::%s", position+i, cast))
	}
	// UUID identifiers are represented as strings and compared through text;
	// date equality/ranges explicitly compare business calendar days.
	if f.Type == "string" {
		column += "::text"
	}
	if f.Type == "date" {
		column += "::date"
	}
	if filter.Operator == "in" {
		return column + " IN (" + strings.Join(placeholders, ", ") + ")", values, nil
	}
	return column + " " + op + " " + placeholders[0], values, nil
}
