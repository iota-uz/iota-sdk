// Package jsonspec loads Lens cube specifications from repo-owned JSON documents.
package jsonspec

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"reflect"
	"regexp"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/cube"
	"github.com/iota-uz/iota-sdk/pkg/lens/format"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/transform"
)

const CubeDocumentVersion = 1

var placeholderPattern = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_.-]+)\s*\}\}`)

type ResolveOptions struct {
	Locale string
	Values map[string]any
}

type CubeDocument struct {
	Version int `json:"version"`
	CubeSpec
}

type CubeSpec struct {
	ID               string                     `json:"id"`
	Title            Text                       `json:"title"`
	Description      Text                       `json:"description"`
	DataMode         cube.DataMode              `json:"dataMode"`
	DataSource       string                     `json:"dataSource"`
	FromSQL          string                     `json:"fromSQL"`
	DataRef          string                     `json:"dataRef"`
	Variables        []VariableSpec             `json:"variables"`
	Params           map[string]lens.ParamValue `json:"params"`
	Where            []string                   `json:"where"`
	Joins            []cube.JoinSpec            `json:"joins"`
	Dimensions       []DimensionSpec            `json:"dimensions"`
	Measures         []MeasureSpec              `json:"measures"`
	DefaultDimension string                     `json:"defaultDimension"`
	Leaf             *cube.LeafSpec             `json:"leaf"`
}

type VariableSpec struct {
	Name            string            `json:"name"`
	Label           Text              `json:"label"`
	Kind            lens.VariableKind `json:"kind"`
	RequestKeys     []string          `json:"requestKeys"`
	Default         any               `json:"default"`
	Required        bool              `json:"required"`
	Description     Text              `json:"description"`
	Options         []VariableOption  `json:"options"`
	AllowAllTime    bool              `json:"allowAllTime"`
	DefaultDuration Duration          `json:"defaultDuration"`
}

type VariableOption struct {
	Label Text   `json:"label"`
	Value string `json:"value"`
}

type DimensionSpec struct {
	Name         string             `json:"name"`
	Label        Text               `json:"label"`
	Type         cube.DimensionType `json:"type"`
	Column       string             `json:"column"`
	LabelColumn  string             `json:"labelColumn"`
	ColorColumn  string             `json:"colorColumn"`
	Field        string             `json:"field"`
	LabelField   string             `json:"labelField"`
	ColorField   string             `json:"colorField"`
	PanelKind    panel.Kind         `json:"panelKind"`
	Height       string             `json:"height"`
	Description  Text               `json:"description"`
	RequiresJoin []string           `json:"requiresJoin"`
	Override     *DatasetSpec       `json:"override"`
	Transforms   []transform.Spec   `json:"transforms"`
	Colors       []string           `json:"colors"`
	ValueAxis    panel.ValueAxis    `json:"valueAxis"`
	ColorScale   string             `json:"colorScale"`
}

type MeasureSpec struct {
	Name         string           `json:"name"`
	Label        Text             `json:"label"`
	Column       string           `json:"column"`
	Field        string           `json:"field"`
	Aggregation  cube.Aggregation `json:"aggregation"`
	Formatter    *format.Spec     `json:"formatter"`
	AccentColor  string           `json:"accentColor"`
	Description  Text             `json:"description"`
	Info         Text             `json:"info"`
	RequiresJoin []string         `json:"requiresJoin"`
	Action       *action.Spec     `json:"action"`
}

type DatasetSpec struct {
	Name        string           `json:"name"`
	Title       Text             `json:"title"`
	Kind        lens.DatasetKind `json:"kind"`
	Source      string           `json:"source"`
	DependsOn   []string         `json:"dependsOn"`
	Query       *lens.QuerySpec  `json:"query"`
	Transforms  []transform.Spec `json:"transforms"`
	StaticRef   string           `json:"staticRef"`
	Description Text             `json:"description"`
}

func LoadCube(data []byte, opts ResolveOptions) (cube.CubeSpec, error) {
	var doc CubeDocument
	//nolint:musttag // CubeDocument is a loader type for trusted repo-owned JSON specs.
	if err := json.Unmarshal(data, &doc); err != nil {
		return cube.CubeSpec{}, err
	}
	return doc.Resolve(opts)
}

func LoadCubeFS(fsys fs.FS, name string, opts ResolveOptions) (cube.CubeSpec, error) {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return cube.CubeSpec{}, err
	}
	return LoadCube(data, opts)
}

func (d CubeDocument) Resolve(opts ResolveOptions) (cube.CubeSpec, error) {
	if d.Version != CubeDocumentVersion {
		return cube.CubeSpec{}, fmt.Errorf("unsupported cube document version %d", d.Version)
	}
	return d.CubeSpec.Resolve(opts)
}

func (s CubeSpec) Resolve(opts ResolveOptions) (cube.CubeSpec, error) {
	spec := cube.CubeSpec{
		ID:               resolveString(s.ID, opts.Values),
		Title:            resolveText(s.Title, opts),
		Description:      resolveText(s.Description, opts),
		DataMode:         s.DataMode,
		DataSource:       resolveString(s.DataSource, opts.Values),
		FromSQL:          resolveString(s.FromSQL, opts.Values),
		Variables:        make([]lens.VariableSpec, 0, len(s.Variables)),
		Params:           make(map[string]lens.ParamValue, len(s.Params)),
		Where:            make([]string, 0, len(s.Where)),
		Joins:            make([]cube.JoinSpec, 0, len(s.Joins)),
		Dimensions:       make([]cube.DimensionSpec, 0, len(s.Dimensions)),
		Measures:         make([]cube.MeasureSpec, 0, len(s.Measures)),
		DefaultDimension: resolveString(s.DefaultDimension, opts.Values),
	}

	if s.Leaf != nil {
		spec.Leaf = cube.LeafSpec{URL: resolveString(s.Leaf.URL, opts.Values)}
	}

	if strings.TrimSpace(s.DataRef) != "" {
		resolved, err := requireValue(s.DataRef, opts.Values)
		if err != nil {
			return cube.CubeSpec{}, err
		}
		frameSet, ok := resolved.(*frame.FrameSet)
		if !ok {
			return cube.CubeSpec{}, fmt.Errorf("value %q must be *frame.FrameSet, got %T", s.DataRef, resolved)
		}
		spec.Data = frameSet
	}

	for _, variableSpec := range s.Variables {
		resolved, err := variableSpec.resolve(opts)
		if err != nil {
			return cube.CubeSpec{}, err
		}
		spec.Variables = append(spec.Variables, resolved)
	}

	for name, value := range s.Params {
		resolved, err := resolveParamValue(value, opts.Values)
		if err != nil {
			return cube.CubeSpec{}, fmt.Errorf("resolve param %q: %w", name, err)
		}
		spec.Params[resolveString(name, opts.Values)] = resolved
	}

	for _, where := range s.Where {
		spec.Where = append(spec.Where, resolveString(where, opts.Values))
	}

	for _, join := range s.Joins {
		spec.Joins = append(spec.Joins, cube.JoinSpec{
			Name: resolveString(join.Name, opts.Values),
			SQL:  resolveString(join.SQL, opts.Values),
		})
	}

	for _, dimension := range s.Dimensions {
		resolved, err := dimension.resolve(opts)
		if err != nil {
			return cube.CubeSpec{}, err
		}
		spec.Dimensions = append(spec.Dimensions, resolved)
	}

	for _, measure := range s.Measures {
		resolved, err := measure.resolve(opts)
		if err != nil {
			return cube.CubeSpec{}, err
		}
		spec.Measures = append(spec.Measures, resolved)
	}

	return spec, nil
}

func (s VariableSpec) resolve(opts ResolveOptions) (lens.VariableSpec, error) {
	defaultValue, err := resolveValueRefs(s.Default, opts.Values)
	if err != nil {
		return lens.VariableSpec{}, err
	}

	spec := lens.VariableSpec{
		Name:            resolveString(s.Name, opts.Values),
		Label:           resolveText(s.Label, opts),
		Kind:            s.Kind,
		RequestKeys:     resolveStringSlice(s.RequestKeys, opts.Values),
		Default:         defaultValue,
		Required:        s.Required,
		Description:     resolveText(s.Description, opts),
		AllowAllTime:    s.AllowAllTime,
		DefaultDuration: s.DefaultDuration.Std(),
		Options:         make([]lens.VariableOption, 0, len(s.Options)),
	}
	for _, option := range s.Options {
		spec.Options = append(spec.Options, lens.VariableOption{
			Label: resolveText(option.Label, opts),
			Value: resolveString(option.Value, opts.Values),
		})
	}
	return spec, nil
}

func (s DimensionSpec) resolve(opts ResolveOptions) (cube.DimensionSpec, error) {
	resolved := cube.DimensionSpec{
		Name:         resolveString(s.Name, opts.Values),
		Label:        resolveText(s.Label, opts),
		Type:         s.Type,
		Column:       resolveString(s.Column, opts.Values),
		LabelColumn:  resolveString(s.LabelColumn, opts.Values),
		ColorColumn:  resolveString(s.ColorColumn, opts.Values),
		Field:        resolveString(s.Field, opts.Values),
		LabelField:   resolveString(s.LabelField, opts.Values),
		ColorField:   resolveString(s.ColorField, opts.Values),
		PanelKind:    s.PanelKind,
		Height:       resolveString(s.Height, opts.Values),
		Description:  resolveText(s.Description, opts),
		RequiresJoin: resolveStringSlice(s.RequiresJoin, opts.Values),
		Colors:       resolveStringSlice(s.Colors, opts.Values),
		ValueAxis:    s.ValueAxis,
		ColorScale:   resolveString(s.ColorScale, opts.Values),
	}
	transforms, err := resolveTransformSpecs(s.Transforms, opts.Values)
	if err != nil {
		return cube.DimensionSpec{}, fmt.Errorf("resolve dimension %q transforms: %w", s.Name, err)
	}
	resolved.Transforms = transforms
	if resolved.Type == "" {
		resolved.Type = cube.DimensionTypeCategory
	}
	if s.Override != nil {
		dataset, err := s.Override.resolve(opts)
		if err != nil {
			return cube.DimensionSpec{}, err
		}
		resolved.Override = &dataset
	}
	return resolved, nil
}

func (s MeasureSpec) resolve(opts ResolveOptions) (cube.MeasureSpec, error) {
	resolved := cube.MeasureSpec{
		Name:         resolveString(s.Name, opts.Values),
		Label:        resolveText(s.Label, opts),
		Column:       resolveString(s.Column, opts.Values),
		Field:        resolveString(s.Field, opts.Values),
		Aggregation:  s.Aggregation,
		Formatter:    s.Formatter,
		AccentColor:  resolveString(s.AccentColor, opts.Values),
		Description:  resolveText(s.Description, opts),
		Info:         resolveText(s.Info, opts),
		RequiresJoin: resolveStringSlice(s.RequiresJoin, opts.Values),
	}
	if s.Action != nil {
		actionSpec, err := resolveActionSpec(*s.Action, opts.Values)
		if err != nil {
			return cube.MeasureSpec{}, err
		}
		resolved.Action = &actionSpec
	}
	return resolved, nil
}

func (s DatasetSpec) resolve(opts ResolveOptions) (lens.DatasetSpec, error) {
	dataset := lens.DatasetSpec{
		Name:        resolveString(s.Name, opts.Values),
		Title:       resolveText(s.Title, opts),
		Kind:        s.Kind,
		Source:      resolveString(s.Source, opts.Values),
		DependsOn:   resolveStringSlice(s.DependsOn, opts.Values),
		Description: resolveText(s.Description, opts),
	}
	transforms, err := resolveTransformSpecs(s.Transforms, opts.Values)
	if err != nil {
		return lens.DatasetSpec{}, fmt.Errorf("resolve dataset %q transforms: %w", s.Name, err)
	}
	dataset.Transforms = transforms
	if s.Query != nil {
		query, err := resolveQuerySpec(*s.Query, opts.Values)
		if err != nil {
			return lens.DatasetSpec{}, err
		}
		dataset.Query = &query
	}
	if strings.TrimSpace(s.StaticRef) != "" {
		resolved, err := requireValue(s.StaticRef, opts.Values)
		if err != nil {
			return lens.DatasetSpec{}, err
		}
		frameSet, ok := resolved.(*frame.FrameSet)
		if !ok {
			return lens.DatasetSpec{}, fmt.Errorf("value %q must be *frame.FrameSet, got %T", s.StaticRef, resolved)
		}
		dataset.Static = frameSet
	}
	return dataset, nil
}

func resolveQuerySpec(spec lens.QuerySpec, values map[string]any) (lens.QuerySpec, error) {
	resolved := lens.QuerySpec{
		Text:    resolveString(spec.Text, values),
		Kind:    spec.Kind,
		MaxRows: spec.MaxRows,
	}
	if len(spec.Params) > 0 {
		resolved.Params = make(map[string]lens.ParamValue, len(spec.Params))
		for name, param := range spec.Params {
			value, err := resolveParamValue(param, values)
			if err != nil {
				return lens.QuerySpec{}, err
			}
			resolved.Params[resolveString(name, values)] = value
		}
	}
	return resolved, nil
}

func resolveParamValue(spec lens.ParamValue, values map[string]any) (lens.ParamValue, error) {
	resolved := lens.ParamValue{
		Variable: resolveString(spec.Variable, values),
	}
	if !isNilValue(spec.Literal) {
		value, err := resolveValueRefs(spec.Literal, values)
		if err != nil {
			return lens.ParamValue{}, err
		}
		resolved.Literal = value
	}
	return resolved, nil
}

func resolveActionSpec(spec action.Spec, values map[string]any) (action.Spec, error) {
	resolved := spec
	resolved.Method = resolveString(spec.Method, values)
	resolved.URL = resolveString(spec.URL, values)
	resolved.Target = resolveString(spec.Target, values)
	resolved.Event = resolveString(spec.Event, values)
	if spec.Drill != nil {
		drill := *spec.Drill
		value, err := resolveValueSource(spec.Drill.Value, values)
		if err != nil {
			return action.Spec{}, err
		}
		drill.Dimension = resolveString(drill.Dimension, values)
		drill.Value = value
		resolved.Drill = &drill
	}
	if len(spec.Params) > 0 {
		resolved.Params = make([]action.Param, 0, len(spec.Params))
		for _, param := range spec.Params {
			source, err := resolveValueSource(param.Source, values)
			if err != nil {
				return action.Spec{}, err
			}
			resolved.Params = append(resolved.Params, action.Param{
				Name:   resolveString(param.Name, values),
				Source: source,
			})
		}
	}
	if len(spec.Payload) > 0 {
		resolved.Payload = make(map[string]action.ValueSource, len(spec.Payload))
		for key, source := range spec.Payload {
			resolvedSource, err := resolveValueSource(source, values)
			if err != nil {
				return action.Spec{}, err
			}
			resolved.Payload[resolveString(key, values)] = resolvedSource
		}
	}
	return resolved, nil
}

func resolveValueSource(source action.ValueSource, values map[string]any) (action.ValueSource, error) {
	resolved := source
	resolved.Name = resolveString(source.Name, values)
	if !isNilValue(source.Value) {
		value, err := resolveValueRefs(source.Value, values)
		if err != nil {
			return action.ValueSource{}, err
		}
		resolved.Value = value
	}
	if !isNilValue(source.Fallback) {
		value, err := resolveValueRefs(source.Fallback, values)
		if err != nil {
			return action.ValueSource{}, err
		}
		resolved.Fallback = value
	}
	return resolved, nil
}

func resolveTransformSpecs(specs []transform.Spec, values map[string]any) ([]transform.Spec, error) {
	if len(specs) == 0 {
		return nil, nil
	}
	out := make([]transform.Spec, len(specs))
	for i, spec := range specs {
		out[i] = spec
		out[i].Fields = resolveStringSlice(spec.Fields, values)
		out[i].Aliases = resolveStringMap(spec.Aliases, values)
		out[i].GroupBy = resolveStringSlice(spec.GroupBy, values)
		out[i].Sort = resolveSortFields(spec.Sort, values)
		out[i].Aggregates = resolveAggregates(spec.Aggregates, values)
		predicates, err := resolvePredicates(spec.Predicates, values)
		if err != nil {
			return nil, fmt.Errorf("resolve transform spec %d predicates: %w", i, err)
		}
		out[i].Predicates = predicates
		if spec.Join != nil {
			join := *spec.Join
			join.Other = resolveString(join.Other, values)
			join.On = resolveStringSlice(join.On, values)
			join.How = resolveString(join.How, values)
			out[i].Join = &join
		}
		if spec.FillMissing != nil {
			fill := *spec.FillMissing
			fill.CategoryField = resolveString(fill.CategoryField, values)
			fill.SeriesField = resolveString(fill.SeriesField, values)
			fill.ValueField = resolveString(fill.ValueField, values)
			if !isNilValue(fill.FillValue) {
				fillValue, err := resolveValueRefs(fill.FillValue, values)
				if err != nil {
					return nil, fmt.Errorf("resolve fill missing fill value for transform spec %d: %w", i, err)
				}
				fill.FillValue = fillValue
			}
			out[i].FillMissing = &fill
		}
		if spec.Formula != nil {
			formula := *spec.Formula
			formula.As = resolveString(formula.As, values)
			formula.Op = resolveString(formula.Op, values)
			formula.Left = resolveString(formula.Left, values)
			formula.Right = resolveString(formula.Right, values)
			out[i].Formula = &formula
		}
		if spec.BucketTime != nil {
			bucket := *spec.BucketTime
			bucket.Field = resolveString(bucket.Field, values)
			bucket.As = resolveString(bucket.As, values)
			bucket.Interval = resolveString(bucket.Interval, values)
			out[i].BucketTime = &bucket
		}
		if spec.TopN != nil {
			topN := *spec.TopN
			topN.Field = resolveString(topN.Field, values)
			topN.Other = resolveString(topN.Other, values)
			out[i].TopN = &topN
		}
		if spec.Pivot != nil {
			pivot := *spec.Pivot
			pivot.CategoryField = resolveString(pivot.CategoryField, values)
			pivot.SeriesField = resolveString(pivot.SeriesField, values)
			pivot.ValueField = resolveString(pivot.ValueField, values)
			out[i].Pivot = &pivot
		}
		if spec.Unpivot != nil {
			unpivot := *spec.Unpivot
			unpivot.LabelField = resolveString(unpivot.LabelField, values)
			unpivot.ValueField = resolveString(unpivot.ValueField, values)
			unpivot.Fields = resolveStringSlice(unpivot.Fields, values)
			out[i].Unpivot = &unpivot
		}
		if spec.BucketBounds != nil {
			bounds := *spec.BucketBounds
			bounds.Field = resolveString(bounds.Field, values)
			bounds.GranularityField = resolveString(bounds.GranularityField, values)
			bounds.Granularity = resolveString(bounds.Granularity, values)
			bounds.StartAs = resolveString(bounds.StartAs, values)
			bounds.EndAs = resolveString(bounds.EndAs, values)
			out[i].BucketBounds = &bounds
		}
		if spec.Lookup != nil {
			lookup := *spec.Lookup
			lookup.Other = resolveString(lookup.Other, values)
			lookup.LocalField = resolveString(lookup.LocalField, values)
			lookup.OtherField = resolveString(lookup.OtherField, values)
			lookup.Fields = resolveStringMap(lookup.Fields, values)
			out[i].Lookup = &lookup
		}
		if spec.AgeRange != nil {
			ageRange := *spec.AgeRange
			ageRange.Field = resolveString(ageRange.Field, values)
			ageRange.MinAs = resolveString(ageRange.MinAs, values)
			ageRange.MaxAs = resolveString(ageRange.MaxAs, values)
			out[i].AgeRange = &ageRange
		}
		if spec.MoneyScale != nil {
			moneyScale := *spec.MoneyScale
			moneyScale.Field = resolveString(moneyScale.Field, values)
			moneyScale.As = resolveString(moneyScale.As, values)
			out[i].MoneyScale = &moneyScale
		}
		if spec.FilterZeroSeries != nil {
			filterZero := *spec.FilterZeroSeries
			filterZero.SeriesField = resolveString(filterZero.SeriesField, values)
			filterZero.ValueField = resolveString(filterZero.ValueField, values)
			out[i].FilterZeroSeries = &filterZero
		}
	}
	return out, nil
}

func resolvePredicates(predicates []transform.Predicate, values map[string]any) ([]transform.Predicate, error) {
	if len(predicates) == 0 {
		return nil, nil
	}
	out := make([]transform.Predicate, len(predicates))
	for i, predicate := range predicates {
		out[i] = predicate
		out[i].Field = resolveString(predicate.Field, values)
		out[i].Op = resolveString(predicate.Op, values)
		if !isNilValue(predicate.Value) {
			resolved, err := resolveValueRefs(predicate.Value, values)
			if err != nil {
				return nil, fmt.Errorf("resolve predicate %d value: %w", i, err)
			}
			out[i].Value = resolved
		}
	}
	return out, nil
}

func resolveAggregates(aggregates []transform.Aggregate, values map[string]any) []transform.Aggregate {
	if len(aggregates) == 0 {
		return nil
	}
	out := make([]transform.Aggregate, len(aggregates))
	for i, aggregate := range aggregates {
		out[i] = aggregate
		out[i].Field = resolveString(aggregate.Field, values)
		out[i].As = resolveString(aggregate.As, values)
		out[i].Func = resolveString(aggregate.Func, values)
	}
	return out
}

func resolveSortFields(fields []transform.SortField, values map[string]any) []transform.SortField {
	if len(fields) == 0 {
		return nil
	}
	out := make([]transform.SortField, len(fields))
	for i, field := range fields {
		out[i] = field
		out[i].Field = resolveString(field.Field, values)
	}
	return out
}

func resolveStringSlice(items []string, values map[string]any) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = resolveString(item, values)
	}
	return out
}

func resolveStringMap(items map[string]string, values map[string]any) map[string]string {
	if len(items) == 0 {
		return nil
	}
	out := make(map[string]string, len(items))
	for key, value := range items {
		out[resolveString(key, values)] = resolveString(value, values)
	}
	return out
}

func resolveText(text Text, opts ResolveOptions) string {
	return resolveString(text.Resolve(opts.Locale), opts.Values)
}

func resolveString(text string, values map[string]any) string {
	if text == "" || len(values) == 0 {
		return text
	}
	return placeholderPattern.ReplaceAllStringFunc(text, func(match string) string {
		parts := placeholderPattern.FindStringSubmatch(match)
		if len(parts) != 2 {
			return match
		}
		value, ok := values[parts[1]]
		if !ok || value == nil {
			return match
		}
		return fmt.Sprint(value)
	})
}

func resolveValueRefs(value any, values map[string]any) (any, error) {
	switch typed := value.(type) {
	case map[string]any:
		if ref, ok := typed["$ref"]; ok && len(typed) == 1 {
			return requireValue(fmt.Sprint(ref), values)
		}
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			resolved, err := resolveValueRefs(item, values)
			if err != nil {
				return nil, err
			}
			out[resolveString(key, values)] = resolved
		}
		return out, nil
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			resolved, err := resolveValueRefs(item, values)
			if err != nil {
				return nil, err
			}
			out[i] = resolved
		}
		return out, nil
	case string:
		return resolveString(typed, values), nil
	default:
		return value, nil
	}
}

func requireValue(name string, values map[string]any) (any, error) {
	trimmed := strings.TrimSpace(name)
	value, ok := values[trimmed]
	if !ok {
		return nil, fmt.Errorf("missing resolve value %q", trimmed)
	}
	return value, nil
}

func isNilValue(value any) bool {
	if value == nil {
		return true
	}
	rv := reflect.ValueOf(value)
	//nolint:exhaustive // Only nil-capable kinds need explicit handling here.
	switch rv.Kind() {
	case reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}
