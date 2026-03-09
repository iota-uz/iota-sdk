// Package panel defines Lens dashboard panel specs and builders.
package panel

import (
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/lens/action"
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
	KindPie           Kind = "pie"
	KindDonut         Kind = "donut"
	KindTable         Kind = "table"
	KindGauge         Kind = "gauge"
	KindTabs          Kind = "tabs"
	KindGrid          Kind = "grid"
	KindSplit         Kind = "split"
	KindRepeat        Kind = "repeat"
)

type TableColumn struct {
	Field     FieldRef
	Label     string
	Formatter *format.Spec
}

type FieldRef string

const (
	DefaultLabelField    FieldRef = "label"
	DefaultValueField    FieldRef = "value"
	DefaultSeriesField   FieldRef = "series"
	DefaultCategoryField FieldRef = "category"
	DefaultIDField       FieldRef = "id"
)

func (f FieldRef) Name() string {
	return string(f)
}

func (f FieldRef) Empty() bool {
	return strings.TrimSpace(f.Name()) == ""
}

type Spec struct {
	ID           string
	Title        string
	Description  string
	Kind         Kind
	Dataset      string
	Span         int
	Height       string
	Colors       []string
	ShowLegend   bool
	Fields       FieldMapping
	Formatter    *format.Spec
	Columns      []TableColumn
	Transforms   []transform.Spec
	Action       *action.Spec
	Children     []Spec
	DefaultChild string
	ClassName    string
}

type FieldMapping struct {
	Label     FieldRef
	Value     FieldRef
	Series    FieldRef
	Category  FieldRef
	ID        FieldRef
	StartTime FieldRef
	EndTime   FieldRef
}

type Plugin interface {
	Name() string
	Kind() Kind
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
func Pie(id, title, dataset string) *Builder   { return newBuilder(KindPie, id, title, dataset) }
func Donut(id, title, dataset string) *Builder { return newBuilder(KindDonut, id, title, dataset) }
func Table(id, title, dataset string) *Builder { return newBuilder(KindTable, id, title, dataset) }
func Gauge(id, title, dataset string) *Builder { return newBuilder(KindGauge, id, title, dataset) }

func Tabs(id, title string, children ...Spec) Spec {
	return Spec{
		ID:       id,
		Title:    title,
		Kind:     KindTabs,
		Span:     6,
		Children: children,
	}
}

func Grid(id, title string, children ...Spec) Spec {
	return Spec{
		ID:       id,
		Title:    title,
		Kind:     KindGrid,
		Span:     12,
		Children: children,
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
			},
		},
	}
}

func (b *Builder) Span(span int) *Builder           { b.spec.Span = span; return b }
func (b *Builder) Height(height string) *Builder    { b.spec.Height = height; return b }
func (b *Builder) Colors(colors ...string) *Builder { b.spec.Colors = colors; return b }
func (b *Builder) Legend() *Builder                 { b.spec.ShowLegend = true; return b }
func (b *Builder) Format(spec format.Spec) *Builder { b.spec.Formatter = &spec; return b }
func (b *Builder) Action(spec action.Spec) *Builder { b.spec.Action = &spec; return b }
func (b *Builder) Description(text string) *Builder { b.spec.Description = text; return b }
func (b *Builder) ClassName(name string) *Builder   { b.spec.ClassName = name; return b }
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
func (b *Builder) Columns(columns ...TableColumn) *Builder {
	b.spec.Columns = columns
	return b
}
func (b *Builder) Transforms(specs ...transform.Spec) *Builder {
	b.spec.Transforms = append(b.spec.Transforms, specs...)
	return b
}
func (b *Builder) Build() Spec { return b.spec }

func Ref(name string) FieldRef {
	return FieldRef(name)
}

func Label(name string) FieldRef {
	return Ref(name)
}

func Value(name string) FieldRef {
	return Ref(name)
}

func Series(name string) FieldRef {
	return Ref(name)
}

func Category(name string) FieldRef {
	return Ref(name)
}

func ID(name string) FieldRef {
	return Ref(name)
}

func StartTime(name string) FieldRef {
	return Ref(name)
}

func EndTime(name string) FieldRef {
	return Ref(name)
}
