// Package compile resolves Lens spec documents into executable dashboard specs.
package compile

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/chrome"
	"github.com/iota-uz/iota-sdk/pkg/lens/cube"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	lensspec "github.com/iota-uz/iota-sdk/pkg/lens/spec"
	"github.com/iota-uz/iota-sdk/pkg/lens/transform"
)

var placeholderPattern = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_.-]+)\s*\}\}`)

type IconResolver func(name string) (chrome.Icon, bool)

type Options struct {
	Locale       string
	BasePath     string
	Values       map[string]any
	Drill        cube.DrillContext
	IconResolver IconResolver
}

type CompiledDocument struct {
	Spec     lens.DashboardSpec
	Semantic *cube.CubeSpec
}

func Document(doc lensspec.Document, opts Options) (CompiledDocument, error) {
	if err := doc.Validate(); err != nil {
		return CompiledDocument{}, err
	}

	var compiled CompiledDocument
	var err error

	if doc.HasSemantic() {
		semantic, semanticErr := compileSemantic(doc, opts)
		if semanticErr != nil {
			return CompiledDocument{}, semanticErr
		}
		compiled.Semantic = &semantic
		compiled.Spec, err = cube.Resolve(semantic, opts.Drill, opts.BasePath)
		if err != nil {
			return CompiledDocument{}, err
		}
	}

	datasets, rows, variables, err := compileManualBody(doc, opts)
	if err != nil {
		return CompiledDocument{}, err
	}

	if compiled.Semantic == nil {
		compiled.Spec = lens.DashboardSpec{
			ID:          resolveString(doc.ID, opts.Values),
			Title:       resolveText(doc.Title, opts),
			Description: resolveText(doc.Description, opts),
			Variables:   variables,
			Datasets:    datasets,
			Rows:        rows,
			Drill:       doc.Drill,
		}
		return compiled, nil
	}

	if len(datasets) > 0 || len(rows) > 0 {
		if doc.BodyPosition == lensspec.BodyPositionPrepend {
			compiled.Spec.Datasets = append(datasets, compiled.Spec.Datasets...)
			compiled.Spec.Rows = append(rows, compiled.Spec.Rows...)
		} else {
			compiled.Spec.Datasets = append(compiled.Spec.Datasets, datasets...)
			compiled.Spec.Rows = append(compiled.Spec.Rows, rows...)
		}
	}

	if len(variables) > 0 {
		compiled.Spec.Variables = variables
	}

	return compiled, nil
}

func LeafURL(doc lensspec.Document, opts Options) (string, bool, error) {
	if !doc.HasSemantic() {
		return "", false, nil
	}
	semantic, err := compileSemantic(doc, opts)
	if err != nil {
		return "", false, err
	}
	if !opts.Drill.IsLeaf(semantic) || strings.TrimSpace(semantic.Leaf.URL) == "" {
		return "", false, nil
	}
	return semantic.Leaf.URL, true, nil
}

func compileSemantic(doc lensspec.Document, opts Options) (cube.CubeSpec, error) {
	spec := cube.CubeSpec{
		ID:               resolveString(doc.ID, opts.Values),
		Title:            resolveText(doc.Title, opts),
		Description:      resolveText(doc.Description, opts),
		DataMode:         doc.DataMode,
		DataSource:       resolveString(doc.DataSource, opts.Values),
		FromSQL:          resolveString(doc.FromSQL, opts.Values),
		Variables:        make([]lens.VariableSpec, 0, len(doc.Variables)),
		Params:           make(map[string]lens.ParamValue, len(doc.Params)),
		Where:            resolveStringSlice(doc.Where, opts.Values),
		Joins:            make([]cube.JoinSpec, 0, len(doc.Joins)),
		Dimensions:       make([]cube.DimensionSpec, 0, len(doc.Dimensions)),
		Measures:         make([]cube.MeasureSpec, 0, len(doc.Measures)),
		DefaultDimension: resolveString(doc.DefaultDimension, opts.Values),
	}

	if strings.TrimSpace(doc.DataRef) != "" {
		value, err := requireValue(doc.DataRef, opts.Values)
		if err != nil {
			return cube.CubeSpec{}, err
		}
		frames, ok := value.(*frame.FrameSet)
		if !ok {
			return cube.CubeSpec{}, fmt.Errorf("semantic dataRef %q must be *frame.FrameSet, got %T", doc.DataRef, value)
		}
		spec.Data = frames
	}

	if doc.Leaf != nil {
		spec.Leaf = cube.LeafSpec{URL: resolveString(doc.Leaf.URL, opts.Values)}
	}

	for _, item := range doc.Variables {
		variable, err := compileVariable(item, opts)
		if err != nil {
			return cube.CubeSpec{}, err
		}
		spec.Variables = append(spec.Variables, variable)
	}
	for name, value := range doc.Params {
		param, err := resolveParamValue(value, opts.Values)
		if err != nil {
			return cube.CubeSpec{}, fmt.Errorf("resolve param %q: %w", name, err)
		}
		spec.Params[resolveString(name, opts.Values)] = param
	}
	for _, join := range doc.Joins {
		spec.Joins = append(spec.Joins, cube.JoinSpec{
			Name: resolveString(join.Name, opts.Values),
			SQL:  resolveString(join.SQL, opts.Values),
		})
	}
	for _, item := range doc.Dimensions {
		dimension, err := compileDimension(item, opts)
		if err != nil {
			return cube.CubeSpec{}, err
		}
		spec.Dimensions = append(spec.Dimensions, dimension)
	}
	for _, item := range doc.Measures {
		measure, err := compileMeasure(item, opts)
		if err != nil {
			return cube.CubeSpec{}, err
		}
		spec.Measures = append(spec.Measures, measure)
	}
	return spec, nil
}

func compileManualBody(doc lensspec.Document, opts Options) ([]lens.DatasetSpec, []lens.RowSpec, []lens.VariableSpec, error) {
	datasets := make([]lens.DatasetSpec, 0, len(doc.Datasets))
	rows := make([]lens.RowSpec, 0, len(doc.Rows))
	variables := make([]lens.VariableSpec, 0, len(doc.Variables))

	for _, item := range doc.Variables {
		variable, err := compileVariable(item, opts)
		if err != nil {
			return nil, nil, nil, err
		}
		variables = append(variables, variable)
	}
	for _, item := range doc.Datasets {
		dataset, err := compileDataset(item, opts)
		if err != nil {
			return nil, nil, nil, err
		}
		datasets = append(datasets, dataset)
	}
	for _, item := range doc.Rows {
		row, err := compileRow(item, opts)
		if err != nil {
			return nil, nil, nil, err
		}
		rows = append(rows, row)
	}
	return datasets, rows, variables, nil
}

func compileVariable(item lensspec.VariableSpec, opts Options) (lens.VariableSpec, error) {
	defaultValue, err := resolveValueRefs(item.Default, opts.Values)
	if err != nil {
		return lens.VariableSpec{}, err
	}
	out := lens.VariableSpec{
		Name:            resolveString(item.Name, opts.Values),
		Label:           resolveText(item.Label, opts),
		Kind:            item.Kind,
		Component:       lens.VariableComponent(resolveString(item.Component, opts.Values)),
		RequestKeys:     resolveStringSlice(item.RequestKeys, opts.Values),
		Default:         defaultValue,
		Required:        item.Required,
		Description:     resolveText(item.Description, opts),
		AllowAllTime:    item.AllowAllTime,
		DefaultDuration: item.DefaultDuration.Std(),
		Options:         make([]lens.VariableOption, 0, len(item.Options)),
	}
	for _, option := range item.Options {
		out.Options = append(out.Options, lens.VariableOption{
			Label: resolveText(option.Label, opts),
			Value: resolveString(option.Value, opts.Values),
		})
	}
	return out, nil
}

func compileDimension(item lensspec.DimensionSpec, opts Options) (cube.DimensionSpec, error) {
	out := cube.DimensionSpec{
		Name:         resolveString(item.Name, opts.Values),
		Label:        resolveText(item.Label, opts),
		Type:         item.Type,
		Column:       resolveString(item.Column, opts.Values),
		LabelColumn:  resolveString(item.LabelColumn, opts.Values),
		ColorColumn:  resolveString(item.ColorColumn, opts.Values),
		Field:        resolveString(item.Field, opts.Values),
		LabelField:   resolveString(item.LabelField, opts.Values),
		ColorField:   resolveString(item.ColorField, opts.Values),
		PanelKind:    item.PanelKind,
		Height:       resolveString(item.Height, opts.Values),
		Description:  resolveText(item.Description, opts),
		RequiresJoin: resolveStringSlice(item.RequiresJoin, opts.Values),
		Colors:       resolveStringSlice(item.Colors, opts.Values),
		ValueAxis:    item.ValueAxis,
		ColorScale:   resolveString(item.ColorScale, opts.Values),
	}
	transforms, err := resolveTransformSpecs(item.Transforms, opts.Values)
	if err != nil {
		return cube.DimensionSpec{}, fmt.Errorf("compile dimension %q transforms: %w", item.Name, err)
	}
	out.Transforms = transforms
	if out.Type == "" {
		out.Type = cube.DimensionTypeCategory
	}
	if item.Override != nil {
		override, err := compileDataset(*item.Override, opts)
		if err != nil {
			return cube.DimensionSpec{}, err
		}
		out.Override = &override
	}
	return out, nil
}

func compileMeasure(item lensspec.MeasureSpec, opts Options) (cube.MeasureSpec, error) {
	out := cube.MeasureSpec{
		Name:         resolveString(item.Name, opts.Values),
		Label:        resolveText(item.Label, opts),
		Column:       resolveString(item.Column, opts.Values),
		Field:        resolveString(item.Field, opts.Values),
		Aggregation:  item.Aggregation,
		Formatter:    item.Formatter,
		AccentColor:  resolveString(item.AccentColor, opts.Values),
		Description:  resolveText(item.Description, opts),
		Info:         resolveText(item.Info, opts),
		RequiresJoin: resolveStringSlice(item.RequiresJoin, opts.Values),
	}
	if item.Action != nil {
		actionSpec, err := resolveActionSpec(*item.Action, opts.Values)
		if err != nil {
			return cube.MeasureSpec{}, err
		}
		out.Action = &actionSpec
	}
	return out, nil
}

func compileDataset(item lensspec.DatasetSpec, opts Options) (lens.DatasetSpec, error) {
	out := lens.DatasetSpec{
		Name:        resolveString(item.Name, opts.Values),
		Title:       resolveText(item.Title, opts),
		Kind:        item.Kind,
		Source:      resolveString(item.Source, opts.Values),
		DependsOn:   resolveStringSlice(item.DependsOn, opts.Values),
		Description: resolveText(item.Description, opts),
		Static:      item.Static,
	}
	transforms, err := resolveTransformSpecs(item.Transforms, opts.Values)
	if err != nil {
		return lens.DatasetSpec{}, fmt.Errorf("compile dataset %q transforms: %w", item.Name, err)
	}
	out.Transforms = transforms
	if item.Query != nil {
		query, err := resolveQuerySpec(*item.Query, opts.Values)
		if err != nil {
			return lens.DatasetSpec{}, err
		}
		out.Query = &query
	}
	if strings.TrimSpace(item.StaticRef) != "" {
		value, err := requireValue(item.StaticRef, opts.Values)
		if err != nil {
			return lens.DatasetSpec{}, err
		}
		frames, ok := value.(*frame.FrameSet)
		if !ok {
			return lens.DatasetSpec{}, fmt.Errorf("dataset staticRef %q must be *frame.FrameSet, got %T", item.StaticRef, value)
		}
		out.Static = frames
	}
	return out, nil
}

func compileRow(item lensspec.RowSpec, opts Options) (lens.RowSpec, error) {
	out := lens.RowSpec{
		Panels: make([]panel.Spec, 0, len(item.Panels)),
		Class:  resolveString(item.Class, opts.Values),
	}
	for _, panelSpec := range item.Panels {
		resolved, err := compilePanel(panelSpec, opts)
		if err != nil {
			return lens.RowSpec{}, err
		}
		out.Panels = append(out.Panels, resolved)
	}
	return out, nil
}

func compilePanel(item lensspec.PanelSpec, opts Options) (panel.Spec, error) {
	out := panel.Spec{
		ID:          resolveString(item.ID, opts.Values),
		Title:       resolveText(item.Title, opts),
		Description: resolveText(item.Description, opts),
		Info:        resolveText(item.Info, opts),
		Kind:        item.Kind,
		Dataset:     resolveString(item.Dataset, opts.Values),
		Span:        item.Span,
		Height:      resolveString(item.Height, opts.Values),
		Colors:      resolveStringSlice(item.Colors, opts.Values),
		ShowLegend:  item.ShowLegend,
		Fields: panel.FieldMapping{
			Label:     panel.Ref(resolveString(item.Fields.Label, opts.Values)),
			Value:     panel.Ref(resolveString(item.Fields.Value, opts.Values)),
			Series:    panel.Ref(resolveString(item.Fields.Series, opts.Values)),
			Category:  panel.Ref(resolveString(item.Fields.Category, opts.Values)),
			ID:        panel.Ref(resolveString(item.Fields.ID, opts.Values)),
			StartTime: panel.Ref(resolveString(item.Fields.StartTime, opts.Values)),
			EndTime:   panel.Ref(resolveString(item.Fields.EndTime, opts.Values)),
		},
		Formatter:   item.Formatter,
		ClassName:   resolveString(item.ClassName, opts.Values),
		ValueAxis:   item.ValueAxis,
		Distributed: item.Distributed,
		ColorField:  panel.Ref(resolveString(item.ColorField, opts.Values)),
		ColorScale:  resolveString(item.ColorScale, opts.Values),
	}
	transforms, err := resolveTransformSpecs(item.Transforms, opts.Values)
	if err != nil {
		return panel.Spec{}, fmt.Errorf("compile panel %q transforms: %w", item.ID, err)
	}
	out.Transforms = transforms

	for _, column := range item.Columns {
		var actionSpec *action.Spec
		if column.Action != nil {
			resolvedAction, err := resolveActionSpec(*column.Action, opts.Values)
			if err != nil {
				return panel.Spec{}, err
			}
			actionSpec = &resolvedAction
		}
		out.Columns = append(out.Columns, panel.TableColumn{
			Field:     panel.Ref(resolveString(column.Field, opts.Values)),
			Label:     resolveText(column.Label, opts),
			Formatter: column.Formatter,
			Action:    actionSpec,
			Text:      resolveText(column.Text, opts),
		})
	}

	if item.Action != nil {
		resolvedAction, err := resolveActionSpec(*item.Action, opts.Values)
		if err != nil {
			return panel.Spec{}, err
		}
		out.Action = &resolvedAction
	}

	for _, child := range item.Children {
		resolvedChild, err := compilePanel(child, opts)
		if err != nil {
			return panel.Spec{}, err
		}
		out.Children = append(out.Children, resolvedChild)
	}

	out.Chrome = item.Chrome
	if strings.TrimSpace(item.AccentColor) != "" {
		out.Chrome.AccentColor = resolveString(item.AccentColor, opts.Values)
	}
	if out.Chrome.Icon.Empty() && strings.TrimSpace(item.ChromeIcon) != "" {
		if opts.IconResolver == nil {
			return panel.Spec{}, fmt.Errorf("panel %q requires icon resolver for %q", out.ID, item.ChromeIcon)
		}
		icon, ok := opts.IconResolver(resolveString(item.ChromeIcon, opts.Values))
		if !ok {
			return panel.Spec{}, fmt.Errorf("panel %q references unknown icon %q", out.ID, item.ChromeIcon)
		}
		out.Chrome.Icon = icon
	}

	return out, nil
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
					return nil, fmt.Errorf("resolve transform spec %d FillMissing.FillValue: %w", i, err)
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

func resolveText(text lensspec.Text, opts Options) string {
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
