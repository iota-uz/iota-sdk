package spec

import (
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/chrome"
	"github.com/iota-uz/iota-sdk/pkg/lens/format"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/transform"
)

type PanelBuilder struct {
	panel PanelSpec
}

func Stat(id, title, dataset string) *PanelBuilder {
	return newPanelBuilder(panel.KindStat, id, title, dataset)
}
func TimeSeries(id, title, dataset string) *PanelBuilder {
	return newPanelBuilder(panel.KindTimeSeries, id, title, dataset)
}
func Bar(id, title, dataset string) *PanelBuilder {
	return newPanelBuilder(panel.KindBar, id, title, dataset)
}
func HorizontalBar(id, title, dataset string) *PanelBuilder {
	return newPanelBuilder(panel.KindHorizontalBar, id, title, dataset)
}
func StackedBar(id, title, dataset string) *PanelBuilder {
	return newPanelBuilder(panel.KindStackedBar, id, title, dataset)
}
func Pie(id, title, dataset string) *PanelBuilder {
	return newPanelBuilder(panel.KindPie, id, title, dataset)
}
func Donut(id, title, dataset string) *PanelBuilder {
	return newPanelBuilder(panel.KindDonut, id, title, dataset)
}
func Table(id, title, dataset string) *PanelBuilder {
	return newPanelBuilder(panel.KindTable, id, title, dataset)
}
func Gauge(id, title, dataset string) *PanelBuilder {
	return newPanelBuilder(panel.KindGauge, id, title, dataset)
}

func Tabs(id, title string, children ...PanelSpec) *PanelBuilder {
	return &PanelBuilder{
		panel: PanelSpec{
			ID:       id,
			Title:    LiteralText(title),
			Kind:     panel.KindTabs,
			Span:     6,
			Children: children,
		},
	}
}

func Grid(id, title string, children ...PanelSpec) *PanelBuilder {
	return &PanelBuilder{
		panel: PanelSpec{
			ID:       id,
			Title:    LiteralText(title),
			Kind:     panel.KindGrid,
			Span:     12,
			Children: children,
		},
	}
}

func newPanelBuilder(kind panel.Kind, id, title, dataset string) *PanelBuilder {
	return &PanelBuilder{
		panel: PanelSpec{
			ID:      id,
			Title:   LiteralText(title),
			Kind:    kind,
			Dataset: dataset,
			Span:    6,
			Fields: FieldMappingSpec{
				Label:    string(panel.DefaultLabelField),
				Value:    string(panel.DefaultValueField),
				Series:   string(panel.DefaultSeriesField),
				Category: string(panel.DefaultCategoryField),
				ID:       string(panel.DefaultIDField),
			},
		},
	}
}

func (b *PanelBuilder) Span(span int) *PanelBuilder           { b.panel.Span = span; return b }
func (b *PanelBuilder) Height(height string) *PanelBuilder    { b.panel.Height = height; return b }
func (b *PanelBuilder) Colors(colors ...string) *PanelBuilder { b.panel.Colors = colors; return b }
func (b *PanelBuilder) Legend() *PanelBuilder                 { b.panel.ShowLegend = true; return b }
func (b *PanelBuilder) Format(spec format.Spec) *PanelBuilder { b.panel.Formatter = &spec; return b }
func (b *PanelBuilder) Action(spec action.Spec) *PanelBuilder { b.panel.Action = &spec; return b }
func (b *PanelBuilder) Description(text string) *PanelBuilder {
	b.panel.Description = LiteralText(text)
	return b
}
func (b *PanelBuilder) Info(text string) *PanelBuilder {
	b.panel.Info = LiteralText(text)
	return b
}
func (b *PanelBuilder) ClassName(name string) *PanelBuilder { b.panel.ClassName = name; return b }
func (b *PanelBuilder) ValueAxisScale(scale panel.AxisScale, base int) *PanelBuilder {
	b.panel.ValueAxis.Scale = scale
	if base > 1 {
		b.panel.ValueAxis.LogBase = base
	}
	return b
}
func (b *PanelBuilder) LogarithmicValueAxis(base int) *PanelBuilder {
	return b.ValueAxisScale(panel.AxisScaleLogarithmic, base)
}
func (b *PanelBuilder) Icon(icon chrome.Icon) *PanelBuilder {
	b.panel.Chrome.Icon = icon
	return b
}
func (b *PanelBuilder) AccentColor(color string) *PanelBuilder {
	b.panel.Chrome.AccentColor = color
	return b
}
func (b *PanelBuilder) DistributedColors() *PanelBuilder {
	b.panel.Distributed = true
	return b
}
func (b *PanelBuilder) SemanticColors(scale, field string) *PanelBuilder {
	b.panel.ColorScale = strings.TrimSpace(scale)
	b.panel.ColorField = field
	return b
}
func (b *PanelBuilder) Fields(mapping FieldMappingSpec) *PanelBuilder {
	b.panel.Fields = mapping
	return b
}
func (b *PanelBuilder) LabelField(name string) *PanelBuilder {
	b.panel.Fields.Label = name
	return b
}
func (b *PanelBuilder) ValueField(name string) *PanelBuilder {
	b.panel.Fields.Value = name
	return b
}
func (b *PanelBuilder) SeriesField(name string) *PanelBuilder {
	b.panel.Fields.Series = name
	return b
}
func (b *PanelBuilder) CategoryField(name string) *PanelBuilder {
	b.panel.Fields.Category = name
	return b
}
func (b *PanelBuilder) StartField(name string) *PanelBuilder {
	b.panel.Fields.StartTime = name
	return b
}
func (b *PanelBuilder) EndField(name string) *PanelBuilder {
	b.panel.Fields.EndTime = name
	return b
}
func (b *PanelBuilder) Columns(columns ...TableColumnSpec) *PanelBuilder {
	b.panel.Columns = columns
	return b
}
func (b *PanelBuilder) Transforms(specs ...transform.Spec) *PanelBuilder {
	b.panel.Transforms = append(b.panel.Transforms, specs...)
	return b
}
func (b *PanelBuilder) Children(children ...PanelSpec) *PanelBuilder {
	b.panel.Children = append(b.panel.Children, children...)
	return b
}
func (b *PanelBuilder) Build() PanelSpec { return b.panel }

func StaticDataset(name string, static *frame.FrameSet, transforms ...transform.Spec) DatasetSpec {
	return DatasetSpec{
		Name:       name,
		Kind:       "static",
		Static:     static,
		Transforms: append([]transform.Spec(nil), transforms...),
	}
}

func Row(panels ...PanelSpec) RowSpec {
	return RowSpec{Panels: panels}
}

func Column(field, label string) TableColumnSpec {
	return TableColumnSpec{
		Field: field,
		Label: LiteralText(label),
	}
}

func (c TableColumnSpec) WithFormatter(spec *format.Spec) TableColumnSpec {
	c.Formatter = spec
	return c
}

func (c TableColumnSpec) WithAction(spec *action.Spec) TableColumnSpec {
	c.Action = spec
	return c
}

func (c TableColumnSpec) WithText(text string) TableColumnSpec {
	c.Text = LiteralText(text)
	return c
}

func Ref(name string) string {
	return name
}
