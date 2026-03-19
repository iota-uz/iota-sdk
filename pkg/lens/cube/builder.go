package cube

import (
	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/format"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/transform"
)

type Builder struct {
	spec CubeSpec
}

type DimensionBuilder struct {
	parent *Builder
	spec   DimensionSpec
}

type MeasureBuilder struct {
	parent *Builder
	spec   MeasureSpec
}

func New(id, title string) *Builder {
	return &Builder{
		spec: CubeSpec{
			ID:     id,
			Title:  title,
			Params: map[string]lens.ParamValue{},
		},
	}
}

func (b *Builder) Description(description string) *Builder {
	b.spec.Description = description
	return b
}

func (b *Builder) SQL(dataSource, from string) *Builder {
	b.spec.DataMode = DataModeSQL
	b.spec.DataSource = dataSource
	b.spec.FromSQL = from
	return b
}

func (b *Builder) Dataset(data *frame.FrameSet) *Builder {
	b.spec.DataMode = DataModeDataset
	b.spec.Data = data
	return b
}

func (b *Builder) Join(name, sql string) *Builder {
	b.spec.Joins = append(b.spec.Joins, JoinSpec{Name: name, SQL: sql})
	return b
}

func (b *Builder) Where(condition string) *Builder {
	b.spec.Where = append(b.spec.Where, condition)
	return b
}

func (b *Builder) ParamLiteral(name string, value any) *Builder {
	b.spec.Params[name] = lens.ParamValue{Literal: value}
	return b
}

func (b *Builder) ParamVariable(name, variable string) *Builder {
	b.spec.Params[name] = lens.ParamValue{Variable: variable}
	return b
}

func (b *Builder) Variable(spec lens.VariableSpec) *Builder {
	b.spec.Variables = append(b.spec.Variables, spec)
	return b
}

func (b *Builder) Dimension(name, label string) *DimensionBuilder {
	return &DimensionBuilder{
		parent: b,
		spec: DimensionSpec{
			Name:      name,
			Label:     label,
			Type:      DimensionTypeCategory,
			PanelKind: panel.KindBar,
		},
	}
}

func (b *Builder) Measure(name, label string) *MeasureBuilder {
	return &MeasureBuilder{
		parent: b,
		spec: MeasureSpec{
			Name:        name,
			Label:       label,
			Aggregation: AggregationSum,
		},
	}
}

func (b *Builder) DefaultDimension(name string) *Builder {
	b.spec.DefaultDimension = name
	return b
}

func (b *Builder) Leaf(url string) *Builder {
	b.spec.Leaf = LeafSpec{URL: url}
	return b
}

// Build returns the assembled CubeSpec without validation.
// Validation is deferred to [Resolve], which calls [CubeSpec.Validate]
// before generating the dashboard. This matches the panel.Builder.Build
// pattern where Build is a pure constructor and validation is the
// caller's responsibility.
func (b *Builder) Build() CubeSpec {
	return b.spec
}

func (b *DimensionBuilder) Column(column string) *DimensionBuilder {
	b.spec.Column = column
	return b
}

func (b *DimensionBuilder) LabelColumn(column string) *DimensionBuilder {
	b.spec.LabelColumn = column
	return b
}

func (b *DimensionBuilder) ColorColumn(column string) *DimensionBuilder {
	b.spec.ColorColumn = column
	return b
}

func (b *DimensionBuilder) Field(field string) *DimensionBuilder {
	b.spec.Field = field
	return b
}

func (b *DimensionBuilder) LabelField(field string) *DimensionBuilder {
	b.spec.LabelField = field
	return b
}

func (b *DimensionBuilder) ColorField(field string) *DimensionBuilder {
	b.spec.ColorField = field
	return b
}

func (b *DimensionBuilder) PanelKind(kind panel.Kind) *DimensionBuilder {
	b.spec.PanelKind = kind
	return b
}

func (b *DimensionBuilder) Height(height string) *DimensionBuilder {
	b.spec.Height = height
	return b
}

func (b *DimensionBuilder) Description(description string) *DimensionBuilder {
	b.spec.Description = description
	return b
}

func (b *DimensionBuilder) RequiresJoin(name string) *DimensionBuilder {
	b.spec.RequiresJoin = append(b.spec.RequiresJoin, name)
	return b
}

func (b *DimensionBuilder) Override(dataset lens.DatasetSpec) *DimensionBuilder {
	b.spec.Override = &dataset
	return b
}

func (b *DimensionBuilder) Transforms(specs ...transform.Spec) *DimensionBuilder {
	b.spec.Transforms = append(b.spec.Transforms, specs...)
	return b
}

func (b *DimensionBuilder) Colors(colors ...string) *DimensionBuilder {
	b.spec.Colors = append([]string(nil), colors...)
	return b
}

func (b *DimensionBuilder) ColorScale(scale string) *DimensionBuilder {
	b.spec.ColorScale = scale
	return b
}

func (b *DimensionBuilder) ValueAxisScale(scale panel.AxisScale, base int) *DimensionBuilder {
	b.spec.ValueAxis.Scale = scale
	if base > 1 {
		b.spec.ValueAxis.LogBase = base
	}
	return b
}

func (b *DimensionBuilder) LogarithmicValueAxis(base int) *DimensionBuilder {
	return b.ValueAxisScale(panel.AxisScaleLogarithmic, base)
}

func (b *DimensionBuilder) commit() *Builder {
	b.parent.spec.Dimensions = append(b.parent.spec.Dimensions, b.spec)
	return b.parent
}

func (b *DimensionBuilder) Dimension(name, label string) *DimensionBuilder {
	return b.commit().Dimension(name, label)
}

func (b *DimensionBuilder) Measure(name, label string) *MeasureBuilder {
	return b.commit().Measure(name, label)
}

func (b *DimensionBuilder) DefaultDimension(name string) *Builder {
	return b.commit().DefaultDimension(name)
}

func (b *DimensionBuilder) Leaf(url string) *Builder {
	return b.commit().Leaf(url)
}

func (b *DimensionBuilder) Build() CubeSpec {
	return b.commit().Build()
}

func (b *MeasureBuilder) Column(column string) *MeasureBuilder {
	b.spec.Column = column
	return b
}

func (b *MeasureBuilder) Field(field string) *MeasureBuilder {
	b.spec.Field = field
	return b
}

func (b *MeasureBuilder) Count() *MeasureBuilder {
	b.spec.Aggregation = AggregationCount
	return b
}

func (b *MeasureBuilder) Sum() *MeasureBuilder {
	b.spec.Aggregation = AggregationSum
	return b
}

func (b *MeasureBuilder) Avg() *MeasureBuilder {
	b.spec.Aggregation = AggregationAvg
	return b
}

func (b *MeasureBuilder) Formatter(spec format.Spec) *MeasureBuilder {
	b.spec.Formatter = &spec
	return b
}

func (b *MeasureBuilder) AccentColor(color string) *MeasureBuilder {
	b.spec.AccentColor = color
	return b
}

func (b *MeasureBuilder) Description(description string) *MeasureBuilder {
	b.spec.Description = description
	return b
}

func (b *MeasureBuilder) Action(spec action.Spec) *MeasureBuilder {
	b.spec.Action = &spec
	return b
}

func (b *MeasureBuilder) RequiresJoin(name string) *MeasureBuilder {
	b.spec.RequiresJoin = append(b.spec.RequiresJoin, name)
	return b
}

func (b *MeasureBuilder) commit() *Builder {
	b.parent.spec.Measures = append(b.parent.spec.Measures, b.spec)
	return b.parent
}

func (b *MeasureBuilder) Dimension(name, label string) *DimensionBuilder {
	return b.commit().Dimension(name, label)
}

func (b *MeasureBuilder) Measure(name, label string) *MeasureBuilder {
	return b.commit().Measure(name, label)
}

func (b *MeasureBuilder) DefaultDimension(name string) *Builder {
	return b.commit().DefaultDimension(name)
}

func (b *MeasureBuilder) Leaf(url string) *Builder {
	return b.commit().Leaf(url)
}

func (b *MeasureBuilder) Build() CubeSpec {
	return b.commit().Build()
}
