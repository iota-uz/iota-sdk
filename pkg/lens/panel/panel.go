// Package panel defines Lens dashboard panel specs and builders.
package panel

import (
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/chrome"
	"github.com/iota-uz/iota-sdk/pkg/lens/format"
	"github.com/iota-uz/iota-sdk/pkg/lens/transform"
)

type Kind string

const (
	KindStat          Kind = "stat"
	KindTimeSeries    Kind = "time_series"
	KindBar           Kind = "bar"
	KindHorizontalBar Kind = "horizontal_bar"
	KindStackedBar    Kind = "stacked_bar"
	// KindSegmentBar renders a part-to-whole value as a single horizontal
	// segmented bar (a headline total, the segmented track, and a legend)
	// using native HTML/CSS rather than a chart engine. Each dataset row is
	// one segment: LabelField → name, ValueField → amount; Colors are applied
	// positionally. Built for two- or few-part splits (e.g. claims paid =
	// within reserve + over reserve) where a chart's axes and plot area are
	// pure noise.
	KindSegmentBar Kind = "segment_bar"
	// KindCascade renders a bridge/cascade as narrowing running-total rows
	// with deduction connectors between them. It is native HTML/CSS rather
	// than an ApexCharts chart.
	KindCascade Kind = "cascade"
	KindPie     Kind = "pie"
	KindDonut   Kind = "donut"
	KindTable   Kind = "table"
	KindGauge   Kind = "gauge"
	KindTabs    Kind = "tabs"
	KindGrid    Kind = "grid"
	KindSplit   Kind = "split"
	KindRepeat  Kind = "repeat"
)

// IsContainer reports whether the kind is a layout container that renders its
// Children rather than its own dataset. Membership: KindTabs, KindGrid,
// KindSplit, KindRepeat. These are the kinds the runtime/render code recurses
// into instead of validating a dataset or drawing a chart body.
//
// Keeping this membership in one predicate lets the recursion sites branch on a
// category instead of re-enumerating the container kinds in every switch, so a
// new container kind only has to be added here.
func (k Kind) IsContainer() bool {
	switch k {
	case KindTabs, KindGrid, KindSplit, KindRepeat:
		return true
	case KindStat, KindTimeSeries, KindBar, KindHorizontalBar, KindStackedBar,
		KindSegmentBar, KindCascade, KindPie, KindDonut, KindTable, KindGauge:
		return false
	}
	return false
}

// IsChart reports whether the kind is a leaf panel rendered through the Apex
// charts engine. Membership: KindTimeSeries, KindBar, KindHorizontalBar,
// KindStackedBar, KindPie, KindDonut, KindGauge.
//
// This is the complement, among leaf panels, of RendersNatively: every leaf is
// either an apex chart or a native (non-apex) render. KindStat, KindSegmentBar,
// KindCascade and KindTable draw their own HTML/CSS and are therefore NOT charts.
func (k Kind) IsChart() bool {
	switch k {
	case KindTimeSeries, KindBar, KindHorizontalBar, KindStackedBar,
		KindPie, KindDonut, KindGauge:
		return true
	case KindStat, KindSegmentBar, KindCascade, KindTable,
		KindTabs, KindGrid, KindSplit, KindRepeat:
		return false
	}
	return false
}

// RendersNatively reports whether the kind is a leaf panel drawn with native
// HTML/CSS rather than the ApexCharts engine. Membership: KindStat,
// KindSegmentBar, KindCascade, KindTable.
//
// Together, IsChart() and RendersNatively() partition the leaf (non-container)
// panel kinds, so "this kind is a renderable leaf" is exactly
// `k.IsChart() || k.RendersNatively()`.
func (k Kind) RendersNatively() bool {
	switch k {
	case KindStat, KindSegmentBar, KindCascade, KindTable:
		return true
	case KindTimeSeries, KindBar, KindHorizontalBar, KindStackedBar,
		KindPie, KindDonut, KindGauge,
		KindTabs, KindGrid, KindSplit, KindRepeat:
		return false
	}
	return false
}

type AxisScale string

const (
	AxisScaleLinear      AxisScale = "linear"
	AxisScaleLogarithmic AxisScale = "logarithmic"
)

type ValueAxis struct {
	Scale   AxisScale
	LogBase int
}

type TableColumn struct {
	Field     FieldRef
	Label     string
	Formatter *format.Spec
	Action    *action.Spec
	Text      string
}

type FieldRef string

const (
	DefaultLabelField    FieldRef = "label"
	DefaultValueField    FieldRef = "value"
	DefaultSeriesField   FieldRef = "series"
	DefaultCategoryField FieldRef = "category"
	DefaultIDField       FieldRef = "id"
	DefaultCutField      FieldRef = "cut"
	DefaultCutLabelField FieldRef = "cutLabel"
	DefaultFinalField    FieldRef = "final"
)

func (f FieldRef) Name() string {
	return string(f)
}

func (f FieldRef) Empty() bool {
	return strings.TrimSpace(f.Name()) == ""
}

type Spec struct {
	ID             string
	Title          string
	Description    string
	Info           string
	Kind           Kind
	Dataset        string
	Span           int
	Height         string
	Colors         []string
	ShowLegend     bool
	ShowTotalBadge bool
	Fields         FieldMapping
	Formatter      *format.Spec
	Columns        []TableColumn
	Transforms     []transform.Spec
	Action         *action.Spec
	Children       []Spec
	ClassName      string
	Chrome         chrome.Spec
	ValueAxis      ValueAxis
	Distributed    bool
	ColorField     FieldRef
	ColorScale     string
}

type FieldMapping struct {
	Label     FieldRef
	Value     FieldRef
	Series    FieldRef
	Category  FieldRef
	ID        FieldRef
	StartTime FieldRef
	EndTime   FieldRef
	Cut       FieldRef
	CutLabel  FieldRef
	Final     FieldRef
}

type Builder struct {
	spec Spec
}

func Stat(id, title, dataset string) *Builder { return newBuilder(KindStat, id, title, dataset) }
func TimeSeries(id, title, dataset string) *Builder {
	return newBuilder(KindTimeSeries, id, title, dataset)
}
func Bar(id, title, dataset string) *Builder { return newBuilder(KindBar, id, title, dataset) }
func HorizontalBar(id, title, dataset string) *Builder {
	return newBuilder(KindHorizontalBar, id, title, dataset)
}
func StackedBar(id, title, dataset string) *Builder {
	return newBuilder(KindStackedBar, id, title, dataset)
}
func Cascade(id, title, dataset string) *Builder { return newBuilder(KindCascade, id, title, dataset) }
func Pie(id, title, dataset string) *Builder     { return newBuilder(KindPie, id, title, dataset) }
func Donut(id, title, dataset string) *Builder   { return newBuilder(KindDonut, id, title, dataset) }
func Table(id, title, dataset string) *Builder   { return newBuilder(KindTable, id, title, dataset) }
func Gauge(id, title, dataset string) *Builder   { return newBuilder(KindGauge, id, title, dataset) }

func Tabs(id, title string, children ...Spec) *Builder {
	return &Builder{
		spec: Spec{
			ID:       id,
			Title:    title,
			Kind:     KindTabs,
			Span:     6,
			Children: children,
		},
	}
}

func Grid(id, title string, children ...Spec) *Builder {
	return &Builder{
		spec: Spec{
			ID:       id,
			Title:    title,
			Kind:     KindGrid,
			Span:     12,
			Children: children,
		},
	}
}

func newBuilder(kind Kind, id, title, dataset string) *Builder {
	return &Builder{
		spec: Spec{
			ID:      id,
			Title:   title,
			Kind:    kind,
			Dataset: dataset,
			Span:    6,
			Fields: FieldMapping{
				Label:    DefaultLabelField,
				Value:    DefaultValueField,
				Series:   DefaultSeriesField,
				Category: DefaultCategoryField,
				ID:       DefaultIDField,
				Cut:      DefaultCutField,
				CutLabel: DefaultCutLabelField,
				Final:    DefaultFinalField,
			},
		},
	}
}

func (b *Builder) Span(span int) *Builder           { b.spec.Span = span; return b }
func (b *Builder) Height(height string) *Builder    { b.spec.Height = height; return b }
func (b *Builder) Colors(colors ...string) *Builder { b.spec.Colors = colors; return b }
func (b *Builder) Legend() *Builder                 { b.spec.ShowLegend = true; return b }
func (b *Builder) TotalBadge() *Builder             { b.spec.ShowTotalBadge = true; return b }
func (b *Builder) Format(spec format.Spec) *Builder { b.spec.Formatter = &spec; return b }
func (b *Builder) Action(spec action.Spec) *Builder { b.spec.Action = &spec; return b }
func (b *Builder) Description(text string) *Builder { b.spec.Description = text; return b }
func (b *Builder) Info(text string) *Builder        { b.spec.Info = text; return b }
func (b *Builder) ClassName(name string) *Builder   { b.spec.ClassName = name; return b }
func (b *Builder) ValueAxisScale(scale AxisScale, base int) *Builder {
	b.spec.ValueAxis.Scale = scale
	if base > 1 {
		b.spec.ValueAxis.LogBase = base
	}
	return b
}
func (b *Builder) LogarithmicValueAxis(base int) *Builder {
	return b.ValueAxisScale(AxisScaleLogarithmic, base)
}
func (b *Builder) Icon(icon chrome.Icon) *Builder {
	b.spec.Chrome.Icon = icon
	return b
}
func (b *Builder) AccentColor(color string) *Builder {
	b.spec.Chrome.AccentColor = color
	return b
}
func (b *Builder) DistributedColors() *Builder {
	b.spec.Distributed = true
	return b
}
func (b *Builder) SemanticColors(scale string, field FieldRef) *Builder {
	b.spec.ColorScale = strings.TrimSpace(scale)
	b.spec.ColorField = field
	return b
}
func (b *Builder) Fields(mapping FieldMapping) *Builder {
	b.spec.Fields = mapping
	return b
}
func (b *Builder) LabelField(name FieldRef) *Builder    { b.spec.Fields.Label = name; return b }
func (b *Builder) ValueField(name FieldRef) *Builder    { b.spec.Fields.Value = name; return b }
func (b *Builder) SeriesField(name FieldRef) *Builder   { b.spec.Fields.Series = name; return b }
func (b *Builder) CategoryField(name FieldRef) *Builder { b.spec.Fields.Category = name; return b }
func (b *Builder) StartField(name FieldRef) *Builder    { b.spec.Fields.StartTime = name; return b }
func (b *Builder) EndField(name FieldRef) *Builder      { b.spec.Fields.EndTime = name; return b }
func (b *Builder) CutField(name FieldRef) *Builder      { b.spec.Fields.Cut = name; return b }
func (b *Builder) CutLabelField(name FieldRef) *Builder { b.spec.Fields.CutLabel = name; return b }
func (b *Builder) FinalField(name FieldRef) *Builder    { b.spec.Fields.Final = name; return b }
func (b *Builder) Columns(columns ...TableColumn) *Builder {
	b.spec.Columns = columns
	return b
}
func (b *Builder) Transforms(specs ...transform.Spec) *Builder {
	b.spec.Transforms = append(b.spec.Transforms, specs...)
	return b
}
func (b *Builder) Children(children ...Spec) *Builder {
	b.spec.Children = append(b.spec.Children, children...)
	return b
}
func (b *Builder) Build() Spec { return b.spec }

func Ref(name string) FieldRef {
	return FieldRef(name)
}
