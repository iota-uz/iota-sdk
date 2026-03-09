package transform

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type Kind string

const (
	KindFilterRows       Kind = "filter_rows"
	KindProject          Kind = "project"
	KindRename           Kind = "rename"
	KindCast             Kind = "cast"
	KindSort             Kind = "sort"
	KindLimit            Kind = "limit"
	KindGroupBy          Kind = "group_by"
	KindAggregate        Kind = "aggregate"
	KindJoin             Kind = "join"
	KindUnion            Kind = "union"
	KindPivot            Kind = "pivot"
	KindUnpivot          Kind = "unpivot"
	KindFillMissing      Kind = "fill_missing"
	KindFormula          Kind = "formula"
	KindBucketTime       Kind = "bucket_time"
	KindTopN             Kind = "top_n"
	KindBucketBounds     Kind = "bucket_bounds"
	KindLookup           Kind = "lookup"
	KindAgeRange         Kind = "age_range"
	KindMoneyScale       Kind = "money_scale"
	KindFilterZeroSeries Kind = "filter_zero_series"
)

type SortDirection string

const (
	SortAsc  SortDirection = "asc"
	SortDesc SortDirection = "desc"
)

type Predicate struct {
	Field string
	Op    string
	Value any
}

type SortField struct {
	Field     string
	Direction SortDirection
}

type Aggregate struct {
	Field string
	As    string
	Func  string
}

type Formula struct {
	As         string
	Op         string
	Left       string
	Right      string
	RightValue float64
}

type JoinConfig struct {
	Other string
	On    []string
	How   string
}

type FillMissingConfig struct {
	CategoryField string
	SeriesField   string
	ValueField    string
	FillValue     any
}

type BucketTimeConfig struct {
	Field    string
	As       string
	Interval string
}

type BucketBoundsConfig struct {
	Field            string
	GranularityField string
	Granularity      string
	StartAs          string
	EndAs            string
}

type LookupConfig struct {
	Other      string
	LocalField string
	OtherField string
	Fields     map[string]string
}

type AgeRangeConfig struct {
	Field string
	MinAs string
	MaxAs string
}

type MoneyScaleConfig struct {
	Field  string
	As     string
	Factor float64
}

type FilterZeroSeriesConfig struct {
	SeriesField string
	ValueField  string
}

type PivotConfig struct {
	CategoryField string
	SeriesField   string
	ValueField    string
}

type UnpivotConfig struct {
	LabelField string
	ValueField string
	Fields     []string
}

type Spec struct {
	Kind             Kind
	Predicates       []Predicate
	Fields           []string
	Aliases          map[string]string
	Types            map[string]frame.FieldType
	Sort             []SortField
	Limit            int
	GroupBy          []string
	Aggregates       []Aggregate
	Join             *JoinConfig
	FillMissing      *FillMissingConfig
	Formula          *Formula
	BucketTime       *BucketTimeConfig
	TopN             *TopNConfig
	Pivot            *PivotConfig
	Unpivot          *UnpivotConfig
	BucketBounds     *BucketBoundsConfig
	Lookup           *LookupConfig
	AgeRange         *AgeRangeConfig
	MoneyScale       *MoneyScaleConfig
	FilterZeroSeries *FilterZeroSeriesConfig
}

type TopNConfig struct {
	Field string
	N     int
}

type Plugin interface {
	Name() string
	Apply(primary *frame.FrameSet, deps map[string]*frame.FrameSet, spec Spec) (*frame.FrameSet, error)
}

func Apply(primary *frame.FrameSet, deps map[string]*frame.FrameSet, specs []Spec) (*frame.FrameSet, error) {
	if primary == nil {
		return nil, nil
	}
	current := primary.Clone()
	var err error
	for _, spec := range specs {
		current, err = applyOne(current, deps, spec)
		if err != nil {
			return nil, err
		}
	}
	return current, nil
}

func applyOne(primary *frame.FrameSet, deps map[string]*frame.FrameSet, spec Spec) (*frame.FrameSet, error) {
	op := serrors.Op("lens/transform.applyOne")
	switch spec.Kind {
	case KindFilterRows:
		return filterRows(primary, spec.Predicates)
	case KindProject:
		return project(primary, spec.Fields)
	case KindRename:
		return rename(primary, spec.Aliases)
	case KindCast:
		return cast(primary, spec.Types)
	case KindSort:
		return sortRows(primary, spec.Sort)
	case KindLimit:
		return limit(primary, spec.Limit)
	case KindGroupBy, KindAggregate:
		return groupBy(primary, spec.GroupBy, spec.Aggregates)
	case KindUnion:
		return union(primary, deps)
	case KindJoin:
		return join(primary, deps, spec.Join)
	case KindFormula:
		return formula(primary, spec.Formula)
	case KindFillMissing:
		return fillMissing(primary, spec.FillMissing)
	case KindBucketTime:
		return bucketTime(primary, spec.BucketTime)
	case KindTopN:
		return topN(primary, spec.TopN)
	case KindPivot:
		return pivot(primary, spec.Pivot)
	case KindUnpivot:
		return unpivot(primary, spec.Unpivot)
	case KindBucketBounds:
		return bucketBounds(primary, spec.BucketBounds)
	case KindLookup:
		return lookup(primary, deps, spec.Lookup)
	case KindAgeRange:
		return ageRange(primary, spec.AgeRange)
	case KindMoneyScale:
		return moneyScale(primary, spec.MoneyScale)
	case KindFilterZeroSeries:
		return filterZeroSeries(primary, spec.FilterZeroSeries)
	default:
		return nil, serrors.E(op, fmt.Errorf("unsupported transform kind %q", spec.Kind))
	}
}

func BucketBounds(field, startAs, endAs string) Spec {
	return Spec{
		Kind: KindBucketBounds,
		BucketBounds: &BucketBoundsConfig{
			Field:   field,
			StartAs: startAs,
			EndAs:   endAs,
		},
	}
}

func Lookup(other, localField, otherField string, fields map[string]string) Spec {
	return Spec{
		Kind: KindLookup,
		Lookup: &LookupConfig{
			Other:      other,
			LocalField: localField,
			OtherField: otherField,
			Fields:     fields,
		},
	}
}

func ParseAgeRange(field, minAs, maxAs string) Spec {
	return Spec{
		Kind: KindAgeRange,
		AgeRange: &AgeRangeConfig{
			Field: field,
			MinAs: minAs,
			MaxAs: maxAs,
		},
	}
}

func MoneyScale(field, as string, factor float64) Spec {
	return Spec{
		Kind: KindMoneyScale,
		MoneyScale: &MoneyScaleConfig{
			Field:  field,
			As:     as,
			Factor: factor,
		},
	}
}

func FilterZeroSeries(seriesField, valueField string) Spec {
	return Spec{
		Kind: KindFilterZeroSeries,
		FilterZeroSeries: &FilterZeroSeriesConfig{
			SeriesField: seriesField,
			ValueField:  valueField,
		},
	}
}

func filterRows(primary *frame.FrameSet, predicates []Predicate) (*frame.FrameSet, error) {
	fr := primary.Primary()
	if fr == nil {
		return primary.Clone(), nil
	}
	rows := fr.Rows()
	out, err := frame.New(fr.Name)
	if err != nil {
		return nil, err
	}
	out.Fields = make([]frame.Field, len(fr.Fields))
	for i, field := range fr.Fields {
		out.Fields[i] = field.Clone()
		out.Fields[i].Values = nil
	}
	for _, row := range rows {
		if matches(row, predicates) {
			if err := out.AppendRow(row); err != nil {
				return nil, err
			}
		}
	}
	if err := out.Normalize(); err != nil {
		return nil, err
	}
	return frame.NewFrameSet(out)
}

func matches(row map[string]any, predicates []Predicate) bool {
	for _, predicate := range predicates {
		left := row[predicate.Field]
		switch predicate.Op {
		case "=", "==":
			if fmt.Sprint(left) != fmt.Sprint(predicate.Value) {
				return false
			}
		case "!=", "<>":
			if fmt.Sprint(left) == fmt.Sprint(predicate.Value) {
				return false
			}
		case "contains":
			if !strings.Contains(strings.ToLower(fmt.Sprint(left)), strings.ToLower(fmt.Sprint(predicate.Value))) {
				return false
			}
		case ">":
			if toFloat(left) <= toFloat(predicate.Value) {
				return false
			}
		case ">=":
			if toFloat(left) < toFloat(predicate.Value) {
				return false
			}
		case "<":
			if toFloat(left) >= toFloat(predicate.Value) {
				return false
			}
		case "<=":
			if toFloat(left) > toFloat(predicate.Value) {
				return false
			}
		}
	}
	return true
}

func project(primary *frame.FrameSet, fields []string) (*frame.FrameSet, error) {
	fr := primary.Primary()
	if fr == nil {
		return primary.Clone(), nil
	}
	selected := make([]frame.Field, 0, len(fields))
	for _, name := range fields {
		field, ok := fr.Field(name)
		if !ok {
			return nil, fmt.Errorf("project field %q not found", name)
		}
		selected = append(selected, field.Clone())
	}
	next := &frame.Frame{Name: fr.Name, Fields: selected}
	if err := next.Normalize(); err != nil {
		return nil, err
	}
	return frame.NewFrameSet(next)
}

func rename(primary *frame.FrameSet, aliases map[string]string) (*frame.FrameSet, error) {
	next := primary.Clone()
	fr := next.Primary()
	if fr == nil {
		return next, nil
	}
	for i := range fr.Fields {
		if alias, ok := aliases[fr.Fields[i].Name]; ok {
			fr.Fields[i].Name = alias
		}
	}
	return next, nil
}

func cast(primary *frame.FrameSet, types map[string]frame.FieldType) (*frame.FrameSet, error) {
	next := primary.Clone()
	fr := next.Primary()
	if fr == nil {
		return next, nil
	}
	for i := range fr.Fields {
		target, ok := types[fr.Fields[i].Name]
		if !ok {
			continue
		}
		fr.Fields[i].Type = target
		switch target {
		case frame.FieldTypeNumber:
			for idx, value := range fr.Fields[i].Values {
				fr.Fields[i].Values[idx] = toFloat(value)
			}
		case frame.FieldTypeString:
			for idx, value := range fr.Fields[i].Values {
				fr.Fields[i].Values[idx] = fmt.Sprint(value)
			}
		}
	}
	return next, nil
}

func sortRows(primary *frame.FrameSet, fields []SortField) (*frame.FrameSet, error) {
	fr := primary.Primary()
	if fr == nil {
		return primary.Clone(), nil
	}
	rows := fr.Rows()
	sort.SliceStable(rows, func(i, j int) bool {
		for _, field := range fields {
			left := rows[i][field.Field]
			right := rows[j][field.Field]
			cmp := compare(left, right)
			if cmp == 0 {
				continue
			}
			if field.Direction == SortDesc {
				return cmp > 0
			}
			return cmp < 0
		}
		return false
	})
	next, err := frame.New(fr.Name)
	if err != nil {
		return nil, err
	}
	next.Fields = cloneSchema(fr.Fields)
	for _, row := range rows {
		if err := next.AppendRow(row); err != nil {
			return nil, err
		}
	}
	if err := next.Normalize(); err != nil {
		return nil, err
	}
	return frame.NewFrameSet(next)
}

func limit(primary *frame.FrameSet, max int) (*frame.FrameSet, error) {
	if max <= 0 {
		return primary.Clone(), nil
	}
	fr := primary.Primary()
	if fr == nil || fr.RowCount <= max {
		return primary.Clone(), nil
	}
	next := fr.Clone()
	for i := range next.Fields {
		next.Fields[i].Values = next.Fields[i].Values[:max]
	}
	next.RowCount = max
	return frame.NewFrameSet(next)
}

func groupBy(primary *frame.FrameSet, fields []string, aggregates []Aggregate) (*frame.FrameSet, error) {
	fr := primary.Primary()
	if fr == nil {
		return primary.Clone(), nil
	}
	type bucket struct {
		keys map[string]any
		rows []map[string]any
	}
	groups := make(map[string]*bucket)
	for _, row := range fr.Rows() {
		keyParts := make([]string, 0, len(fields))
		keyValues := make(map[string]any, len(fields))
		for _, field := range fields {
			keyValues[field] = row[field]
			keyParts = append(keyParts, fmt.Sprint(row[field]))
		}
		key := strings.Join(keyParts, "|")
		if _, ok := groups[key]; !ok {
			groups[key] = &bucket{keys: keyValues}
		}
		groups[key].rows = append(groups[key].rows, row)
	}
	out, err := frame.New(fr.Name)
	if err != nil {
		return nil, err
	}
	for _, grp := range groups {
		row := make(map[string]any, len(fields)+len(aggregates))
		for _, field := range fields {
			row[field] = grp.keys[field]
		}
		for _, agg := range aggregates {
			row[agg.As] = aggregateRows(grp.rows, agg)
		}
		if err := out.AppendRow(row); err != nil {
			return nil, err
		}
	}
	if err := out.Normalize(); err != nil {
		return nil, err
	}
	return frame.NewFrameSet(out)
}

func aggregateRows(rows []map[string]any, agg Aggregate) any {
	switch agg.Func {
	case "count":
		return float64(len(rows))
	case "sum":
		total := 0.0
		for _, row := range rows {
			total += toFloat(row[agg.Field])
		}
		return total
	case "avg":
		if len(rows) == 0 {
			return 0.0
		}
		total := 0.0
		for _, row := range rows {
			total += toFloat(row[agg.Field])
		}
		return total / float64(len(rows))
	case "min":
		min := 0.0
		for i, row := range rows {
			value := toFloat(row[agg.Field])
			if i == 0 || value < min {
				min = value
			}
		}
		return min
	case "max":
		max := 0.0
		for i, row := range rows {
			value := toFloat(row[agg.Field])
			if i == 0 || value > max {
				max = value
			}
		}
		return max
	default:
		return 0.0
	}
}

func union(primary *frame.FrameSet, deps map[string]*frame.FrameSet) (*frame.FrameSet, error) {
	out := primary.Clone()
	for _, dep := range deps {
		if dep == nil || dep.Primary() == nil {
			continue
		}
		if out == nil || out.Primary() == nil {
			out = dep.Clone()
			continue
		}
		for _, row := range dep.Primary().Rows() {
			if err := out.Primary().AppendRow(row); err != nil {
				return nil, err
			}
		}
	}
	if out != nil && out.Primary() != nil {
		if err := out.Primary().Normalize(); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func join(primary *frame.FrameSet, deps map[string]*frame.FrameSet, cfg *JoinConfig) (*frame.FrameSet, error) {
	if cfg == nil {
		return primary.Clone(), nil
	}
	left := primary.Primary()
	rightSet := deps[cfg.Other]
	if left == nil || rightSet == nil || rightSet.Primary() == nil {
		return primary.Clone(), nil
	}
	right := rightSet.Primary()
	rows := left.Rows()
	rightRows := right.Rows()
	index := make(map[string][]map[string]any, len(rightRows))
	for _, row := range rightRows {
		key := joinKey(row, cfg.On)
		index[key] = append(index[key], row)
	}
	out, err := frame.New(left.Name)
	if err != nil {
		return nil, err
	}
	how := strings.ToLower(strings.TrimSpace(cfg.How))
	if how == "" {
		how = "left"
	}
	for _, row := range rows {
		matches := index[joinKey(row, cfg.On)]
		if len(matches) == 0 {
			if how == "inner" {
				continue
			}
			if err := out.AppendRow(cloneRow(row)); err != nil {
				return nil, err
			}
			continue
		}
		for _, match := range matches {
			merged := cloneRow(row)
			for k, v := range match {
				if _, exists := merged[k]; exists {
					merged[cfg.Other+"_"+k] = v
					continue
				}
				merged[k] = v
			}
			if err := out.AppendRow(merged); err != nil {
				return nil, err
			}
		}
	}
	if err := out.Normalize(); err != nil {
		return nil, err
	}
	return frame.NewFrameSet(out)
}

func formula(primary *frame.FrameSet, cfg *Formula) (*frame.FrameSet, error) {
	if cfg == nil {
		return primary.Clone(), nil
	}
	next := primary.Clone()
	fr := next.Primary()
	if fr == nil {
		return next, nil
	}
	values := make([]any, fr.RowCount)
	for row := 0; row < fr.RowCount; row++ {
		left := toFloat(valueAt(fr, cfg.Left, row))
		right := cfg.RightValue
		if cfg.Right != "" {
			right = toFloat(valueAt(fr, cfg.Right, row))
		}
		switch cfg.Op {
		case "+":
			values[row] = left + right
		case "-":
			values[row] = left - right
		case "*":
			values[row] = left * right
		case "/":
			if right == 0 {
				values[row] = 0.0
			} else {
				values[row] = left / right
			}
		default:
			values[row] = left
		}
	}
	fr.Fields = append(fr.Fields, frame.Field{Name: cfg.As, Type: frame.FieldTypeNumber, Role: frame.RoleMetric, Values: values})
	return next, fr.Normalize()
}

func fillMissing(primary *frame.FrameSet, cfg *FillMissingConfig) (*frame.FrameSet, error) {
	if cfg == nil {
		return primary.Clone(), nil
	}
	fr := primary.Primary()
	if fr == nil {
		return primary.Clone(), nil
	}
	rows := fr.Rows()
	categories := make([]string, 0)
	series := make([]string, 0)
	categorySeen := map[string]bool{}
	seriesSeen := map[string]bool{}
	existing := map[string]map[string]any{}
	for _, row := range rows {
		cat := fmt.Sprint(row[cfg.CategoryField])
		ser := fmt.Sprint(row[cfg.SeriesField])
		if !categorySeen[cat] {
			categorySeen[cat] = true
			categories = append(categories, cat)
		}
		if !seriesSeen[ser] {
			seriesSeen[ser] = true
			series = append(series, ser)
		}
		existing[cat+"|"+ser] = row
	}
	out, err := frame.New(fr.Name)
	if err != nil {
		return nil, err
	}
	for _, cat := range categories {
		for _, ser := range series {
			key := cat + "|" + ser
			if row, ok := existing[key]; ok {
				if err := out.AppendRow(row); err != nil {
					return nil, err
				}
				continue
			}
			row := map[string]any{
				cfg.CategoryField: cat,
				cfg.SeriesField:   ser,
				cfg.ValueField:    cfg.FillValue,
			}
			if err := out.AppendRow(row); err != nil {
				return nil, err
			}
		}
	}
	if err := out.Normalize(); err != nil {
		return nil, err
	}
	return frame.NewFrameSet(out)
}

func bucketTime(primary *frame.FrameSet, cfg *BucketTimeConfig) (*frame.FrameSet, error) {
	if cfg == nil {
		return primary.Clone(), nil
	}
	next := primary.Clone()
	fr := next.Primary()
	if fr == nil {
		return next, nil
	}
	values := make([]any, fr.RowCount)
	for row := 0; row < fr.RowCount; row++ {
		raw, _ := valueAt(fr, cfg.Field, row).(time.Time)
		switch cfg.Interval {
		case "year":
			values[row] = time.Date(raw.Year(), time.January, 1, 0, 0, 0, 0, raw.Location())
		case "month":
			values[row] = time.Date(raw.Year(), raw.Month(), 1, 0, 0, 0, 0, raw.Location())
		default:
			values[row] = time.Date(raw.Year(), raw.Month(), raw.Day(), 0, 0, 0, 0, raw.Location())
		}
	}
	fr.Fields = append(fr.Fields, frame.Field{Name: cfg.As, Type: frame.FieldTypeTime, Role: frame.RoleTime, Values: values})
	return next, fr.Normalize()
}

func topN(primary *frame.FrameSet, cfg *TopNConfig) (*frame.FrameSet, error) {
	if cfg == nil {
		return primary.Clone(), nil
	}
	sorted, err := sortRows(primary, []SortField{{Field: cfg.Field, Direction: SortDesc}})
	if err != nil {
		return nil, err
	}
	return limit(sorted, cfg.N)
}

func pivot(primary *frame.FrameSet, cfg *PivotConfig) (*frame.FrameSet, error) {
	if cfg == nil {
		return primary.Clone(), nil
	}
	fr := primary.Primary()
	if fr == nil {
		return primary.Clone(), nil
	}
	rows := fr.Rows()
	seriesNames := make([]string, 0)
	seriesSeen := map[string]bool{}
	grouped := map[string]map[string]any{}
	for _, row := range rows {
		category := fmt.Sprint(row[cfg.CategoryField])
		series := fmt.Sprint(row[cfg.SeriesField])
		if !seriesSeen[series] {
			seriesSeen[series] = true
			seriesNames = append(seriesNames, series)
		}
		if _, ok := grouped[category]; !ok {
			grouped[category] = map[string]any{cfg.CategoryField: category}
		}
		grouped[category][series] = row[cfg.ValueField]
	}
	out, err := frame.New(fr.Name)
	if err != nil {
		return nil, err
	}
	categories := make([]string, 0, len(grouped))
	for category := range grouped {
		categories = append(categories, category)
	}
	sort.Strings(categories)
	for _, category := range categories {
		if err := out.AppendRow(grouped[category]); err != nil {
			return nil, err
		}
	}
	if err := out.Normalize(); err != nil {
		return nil, err
	}
	return frame.NewFrameSet(out)
}

func unpivot(primary *frame.FrameSet, cfg *UnpivotConfig) (*frame.FrameSet, error) {
	if cfg == nil {
		return primary.Clone(), nil
	}
	fr := primary.Primary()
	if fr == nil {
		return primary.Clone(), nil
	}
	out, err := frame.New(fr.Name)
	if err != nil {
		return nil, err
	}
	for _, row := range fr.Rows() {
		for _, field := range cfg.Fields {
			item := map[string]any{}
			for key, value := range row {
				item[key] = value
			}
			item[cfg.LabelField] = field
			item[cfg.ValueField] = row[field]
			if err := out.AppendRow(item); err != nil {
				return nil, err
			}
		}
	}
	if err := out.Normalize(); err != nil {
		return nil, err
	}
	return frame.NewFrameSet(out)
}

func bucketBounds(primary *frame.FrameSet, cfg *BucketBoundsConfig) (*frame.FrameSet, error) {
	if cfg == nil {
		return primary.Clone(), nil
	}
	fr := primary.Primary()
	if fr == nil {
		return primary.Clone(), nil
	}
	out := &frame.Frame{Name: fr.Name, Fields: cloneSchema(fr.Fields)}
	if startField, ok := fr.Field(cfg.StartAs); !ok || startField == nil {
		out.Fields = append(out.Fields, frame.Field{Name: cfg.StartAs, Type: frame.FieldTypeString, Role: frame.RoleLinkParam})
	}
	if endField, ok := fr.Field(cfg.EndAs); !ok || endField == nil {
		out.Fields = append(out.Fields, frame.Field{Name: cfg.EndAs, Type: frame.FieldTypeString, Role: frame.RoleLinkParam})
	}
	for _, row := range fr.Rows() {
		start, end := computeBucketBounds(row[cfg.Field], bucketGranularity(row, cfg))
		row[cfg.StartAs] = start
		row[cfg.EndAs] = end
		if err := out.AppendRow(row); err != nil {
			return nil, err
		}
	}
	if err := out.Normalize(); err != nil {
		return nil, err
	}
	return frame.NewFrameSet(out)
}

func lookup(primary *frame.FrameSet, deps map[string]*frame.FrameSet, cfg *LookupConfig) (*frame.FrameSet, error) {
	if cfg == nil {
		return primary.Clone(), nil
	}
	dep := deps[cfg.Other]
	if dep == nil || dep.Primary() == nil {
		return primary.Clone(), nil
	}
	source := dep.Primary()
	index := make(map[string]map[string]any, source.RowCount)
	for _, row := range source.Rows() {
		key := fmt.Sprint(row[cfg.OtherField])
		index[key] = row
	}
	fr := primary.Primary()
	if fr == nil {
		return primary.Clone(), nil
	}
	out := &frame.Frame{Name: fr.Name, Fields: cloneSchema(fr.Fields)}
	for _, as := range cfg.Fields {
		if _, ok := out.Field(as); !ok {
			out.Fields = append(out.Fields, frame.Field{Name: as, Type: frame.FieldTypeString, Role: frame.RoleDimension})
		}
	}
	for _, row := range fr.Rows() {
		if match, ok := index[fmt.Sprint(row[cfg.LocalField])]; ok {
			for sourceField, as := range cfg.Fields {
				row[as] = match[sourceField]
			}
		}
		if err := out.AppendRow(row); err != nil {
			return nil, err
		}
	}
	if err := out.Normalize(); err != nil {
		return nil, err
	}
	return frame.NewFrameSet(out)
}

func ageRange(primary *frame.FrameSet, cfg *AgeRangeConfig) (*frame.FrameSet, error) {
	if cfg == nil {
		return primary.Clone(), nil
	}
	fr := primary.Primary()
	if fr == nil {
		return primary.Clone(), nil
	}
	out := &frame.Frame{Name: fr.Name, Fields: cloneSchema(fr.Fields)}
	if _, ok := out.Field(cfg.MinAs); !ok {
		out.Fields = append(out.Fields, frame.Field{Name: cfg.MinAs, Type: frame.FieldTypeNumber, Role: frame.RoleLinkParam})
	}
	if _, ok := out.Field(cfg.MaxAs); !ok {
		out.Fields = append(out.Fields, frame.Field{Name: cfg.MaxAs, Type: frame.FieldTypeNumber, Role: frame.RoleLinkParam})
	}
	for _, row := range fr.Rows() {
		min, max := parseAgeRange(fmt.Sprint(row[cfg.Field]))
		row[cfg.MinAs] = min
		row[cfg.MaxAs] = max
		if err := out.AppendRow(row); err != nil {
			return nil, err
		}
	}
	if err := out.Normalize(); err != nil {
		return nil, err
	}
	return frame.NewFrameSet(out)
}

func moneyScale(primary *frame.FrameSet, cfg *MoneyScaleConfig) (*frame.FrameSet, error) {
	if cfg == nil || cfg.Factor == 0 {
		return primary.Clone(), nil
	}
	fr := primary.Primary()
	if fr == nil {
		return primary.Clone(), nil
	}
	out := &frame.Frame{Name: fr.Name, Fields: cloneSchema(fr.Fields)}
	target := cfg.Field
	if cfg.As != "" && cfg.As != cfg.Field {
		out.Fields = append(out.Fields, frame.Field{Name: cfg.As, Type: frame.FieldTypeNumber, Role: frame.RoleMetric})
		target = cfg.As
	}
	for _, row := range fr.Rows() {
		row[target] = toFloat(row[cfg.Field]) / cfg.Factor
		if err := out.AppendRow(row); err != nil {
			return nil, err
		}
	}
	if err := out.Normalize(); err != nil {
		return nil, err
	}
	return frame.NewFrameSet(out)
}

func filterZeroSeries(primary *frame.FrameSet, cfg *FilterZeroSeriesConfig) (*frame.FrameSet, error) {
	if cfg == nil {
		return primary.Clone(), nil
	}
	fr := primary.Primary()
	if fr == nil {
		return primary.Clone(), nil
	}
	active := make(map[string]bool)
	for _, row := range fr.Rows() {
		if toFloat(row[cfg.ValueField]) != 0 {
			active[fmt.Sprint(row[cfg.SeriesField])] = true
		}
	}
	out := &frame.Frame{Name: fr.Name, Fields: cloneSchema(fr.Fields)}
	for _, row := range fr.Rows() {
		if !active[fmt.Sprint(row[cfg.SeriesField])] {
			continue
		}
		if err := out.AppendRow(row); err != nil {
			return nil, err
		}
	}
	if err := out.Normalize(); err != nil {
		return nil, err
	}
	return frame.NewFrameSet(out)
}

func cloneSchema(fields []frame.Field) []frame.Field {
	out := make([]frame.Field, len(fields))
	for i, field := range fields {
		out[i] = field.Clone()
		out[i].Values = nil
	}
	return out
}

func valueAt(fr *frame.Frame, fieldName string, index int) any {
	field, ok := fr.Field(fieldName)
	if !ok || index >= len(field.Values) {
		return nil
	}
	return field.Values[index]
}

func joinKey(row map[string]any, keys []string) string {
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprint(row[key]))
	}
	return strings.Join(parts, "|")
}

func compare(left, right any) int {
	if isNumeric(left) && isNumeric(right) {
		leftFloat := toFloat(left)
		rightFloat := toFloat(right)
		switch {
		case leftFloat < rightFloat:
			return -1
		case leftFloat > rightFloat:
			return 1
		default:
			return 0
		}
	}
	leftStr := fmt.Sprint(left)
	rightStr := fmt.Sprint(right)
	switch {
	case leftStr < rightStr:
		return -1
	case leftStr > rightStr:
		return 1
	default:
		return 0
	}
}

func isNumeric(value any) bool {
	switch value.(type) {
	case float64, float32,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64:
		return true
	case string, []byte:
		_, err := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(value)), 64)
		return err == nil
	default:
		return false
	}
}

func toFloat(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case int32:
		return float64(v)
	case uint:
		return float64(v)
	case uint64:
		return float64(v)
	case uint32:
		return float64(v)
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err == nil {
			return parsed
		}
		return 0
	case []byte:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(string(v)), 64)
		if err == nil {
			return parsed
		}
		return 0
	default:
		return 0
	}
}

func cloneRow(row map[string]any) map[string]any {
	cloned := make(map[string]any, len(row))
	for k, v := range row {
		cloned[k] = v
	}
	return cloned
}

func computeBucketBounds(raw any, granularity string) (string, string) {
	value, ok := raw.(time.Time)
	if !ok {
		return fmt.Sprint(raw), fmt.Sprint(raw)
	}
	switch granularity {
	case "year":
		start := time.Date(value.Year(), time.January, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(value.Year(), time.December, 31, 0, 0, 0, 0, time.UTC)
		return start.Format("2006-01-02"), end.Format("2006-01-02")
	case "month":
		start := time.Date(value.Year(), value.Month(), 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(value.Year(), value.Month()+1, 0, 0, 0, 0, 0, time.UTC)
		return start.Format("2006-01-02"), end.Format("2006-01-02")
	default:
		iso := value.Format("2006-01-02")
		return iso, iso
	}
}

func bucketGranularity(row map[string]any, cfg *BucketBoundsConfig) string {
	if cfg.Granularity != "" {
		return cfg.Granularity
	}
	if cfg.GranularityField != "" {
		return fmt.Sprint(row[cfg.GranularityField])
	}
	return "day"
}

func parseAgeRange(raw string) (int, int) {
	if strings.HasSuffix(raw, "+") {
		min, _ := strconv.Atoi(strings.TrimSuffix(raw, "+"))
		return min, 999
	}
	parts := strings.Split(raw, "-")
	if len(parts) != 2 {
		return 0, 0
	}
	min, _ := strconv.Atoi(parts[0])
	max, _ := strconv.Atoi(parts[1])
	return min, max
}
