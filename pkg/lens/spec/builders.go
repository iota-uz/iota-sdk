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
func SegmentBar(id, title, dataset string) *PanelBuilder {
	return newPanelBuilder(panel.KindSegmentBar, id, title, dataset)
}
func Cascade(id, title, dataset string) *PanelBuilder {
	return newPanelBuilder(panel.KindCascade, id, title, dataset)
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

func Tabs(id, title string, children ...PanelSpec) PanelSpec {
	return PanelSpec{
		ID:       id,
		Title:    LiteralText(title),
		Kind:     panel.KindTabs,
		Span:     6,
		Children: children,
	}
}

func Grid(id, title string, children ...PanelSpec) PanelSpec {
	return PanelSpec{
		ID:       id,
		Title:    LiteralText(title),
		Kind:     panel.KindGrid,
		Span:     12,
		Children: children,
	}
}

// StatGroup builds a container that renders its Stat children inside one
// shared card, separated by hairlines (columns by default; see Layout).
func StatGroup(id, title string, children ...PanelSpec) *PanelBuilder {
	return &PanelBuilder{
		panel: PanelSpec{
			ID:          id,
			Title:       LiteralText(title),
			Kind:        panel.KindStatGroup,
			Span:        12,
			GroupLayout: panel.GroupColumns,
			Children:    children,
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
				Cut:      string(panel.DefaultCutField),
				CutLabel: string(panel.DefaultCutLabelField),
				Final:    string(panel.DefaultFinalField),
			},
		},
	}
}

func (b *PanelBuilder) Span(span int) *PanelBuilder           { b.panel.Span = span; return b }
func (b *PanelBuilder) Height(height string) *PanelBuilder    { b.panel.Height = height; return b }
func (b *PanelBuilder) Colors(colors ...string) *PanelBuilder { b.panel.Colors = colors; return b }
func (b *PanelBuilder) Legend() *PanelBuilder                 { b.panel.ShowLegend = true; return b }
func (b *PanelBuilder) LegendAt(position panel.LegendPosition) *PanelBuilder {
	b.panel.ShowLegend = true
	b.panel.LegendPosition = position
	return b
}
func (b *PanelBuilder) LegendWidth(px int) *PanelBuilder {
	b.panel.ShowLegend = true
	b.panel.LegendWidthPx = px
	return b
}
func (b *PanelBuilder) LegendOffsetY(px int) *PanelBuilder {
	b.panel.ShowLegend = true
	b.panel.LegendOffsetY = px
	return b
}
func (b *PanelBuilder) FloatingLegend() *PanelBuilder {
	b.panel.ShowLegend = true
	b.panel.LegendFloating = true
	return b
}
func (b *PanelBuilder) CircularScale(scale float64) *PanelBuilder {
	b.panel.CircularScale = scale
	return b
}
func (b *PanelBuilder) CircularOffsetX(px int) *PanelBuilder {
	b.panel.CircularOffsetX = px
	return b
}
func (b *PanelBuilder) TotalBadge() *PanelBuilder { b.panel.ShowTotalBadge = true; return b }

// TotalBadgeValue shows the total badge with a server-computed value instead
// of the client-side sum of plotted points. Use when the plotted series are
// not the raw amounts (e.g. log-scaled panels).
func (b *PanelBuilder) TotalBadgeValue(v float64) *PanelBuilder {
	b.panel.ShowTotalBadge = true
	b.panel.TotalBadgeValue = &v
	return b
}

// HeadlineValue overrides the computed headline of native summary panels
// while leaving their plotted denominator and segment shares unchanged.
func (b *PanelBuilder) HeadlineValue(v float64) *PanelBuilder {
	b.panel.HeadlineValue = &v
	return b
}
func (b *PanelBuilder) DrillHierarchy(h panel.DrillHierarchy) *PanelBuilder {
	b.panel.DrillHierarchy = &h
	return b
}

// DrillTree enables stable, key-based in-place navigation. Configure IDField
// with the initial dataset field whose values match branch trigger keys.
func (b *PanelBuilder) DrillTree(tree panel.DrillTree) *PanelBuilder {
	b.panel.DrillTree = &tree
	return b
}

func (b *PanelBuilder) Trend(percent float64, label string) *PanelBuilder {
	b.panel.Trend = &panel.TrendSpec{Percent: percent, Label: label}
	return b
}

// TrendWithInvert is Trend for down-is-good metrics: invert flips the
// good/bad color mapping while the arrow still follows the sign.
func (b *PanelBuilder) TrendWithInvert(percent float64, label string, invert bool) *PanelBuilder {
	b.panel.Trend = &panel.TrendSpec{Percent: percent, Label: label, Invert: invert}
	return b
}

// Status renders a small tone-colored chip in the stat card's label row.
func (b *PanelBuilder) Status(label string, tone panel.StatusTone) *PanelBuilder {
	b.panel.Status = &panel.StatusSpec{Label: label, Tone: tone}
	return b
}

// Sparkline renders an inline trend polyline in the stat card's footer row
// using the default accent stroke.
func (b *PanelBuilder) Sparkline(values []float64) *PanelBuilder {
	b.panel.Sparkline = &panel.SparklineSpec{Values: values}
	return b
}

// SparklineColored is Sparkline with an explicit stroke color.
func (b *PanelBuilder) SparklineColored(values []float64, color string) *PanelBuilder {
	b.panel.Sparkline = &panel.SparklineSpec{Values: values, Color: color}
	return b
}

// Layout selects a StatGroup's child arrangement (columns or rows).
func (b *PanelBuilder) Layout(l panel.GroupLayout) *PanelBuilder {
	b.panel.GroupLayout = l
	return b
}
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
func (b *PanelBuilder) IDField(name string) *PanelBuilder {
	b.panel.Fields.ID = name
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
func (b *PanelBuilder) CutField(name string) *PanelBuilder {
	b.panel.Fields.Cut = name
	return b
}
func (b *PanelBuilder) CutLabelField(name string) *PanelBuilder {
	b.panel.Fields.CutLabel = name
	return b
}
func (b *PanelBuilder) FinalField(name string) *PanelBuilder {
	b.panel.Fields.Final = name
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

// Presentation sets the panel's opt-in renderer density hints.
func (b *PanelBuilder) Presentation(hints panel.PresentationHints) *PanelBuilder {
	b.panel.Presentation = hints
	return b
}

// NonSortable removes a table panel's sort affordances — for a fixed
// decomposition whose rows have an inherent order, not a record list.
func (b *PanelBuilder) NonSortable() *PanelBuilder {
	b.panel.Presentation.NonSortable = true
	return b
}

// NonExpandable removes the panel's expand-to-overlay control. Every panel
// rendered inside a drawer sets it — an overlay over a modal is meaningless.
func (b *PanelBuilder) NonExpandable() *PanelBuilder {
	b.panel.Presentation.NonExpandable = true
	return b
}

// NonExportable removes the panel's export control, e.g. a small derived table
// that is already the drawer's whole point.
func (b *PanelBuilder) NonExportable() *PanelBuilder {
	b.panel.Presentation.NonExportable = true
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

// HeadingRow returns a panel-less row that renders as a section header band,
// used to group the following panel rows under a labeled section.
func HeadingRow(heading string) RowSpec {
	return RowSpec{Heading: LiteralText(heading)}
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

// AlignRight right-aligns the column's header and cell text.
func (c TableColumnSpec) AlignRight() TableColumnSpec {
	c.Align = "right"
	return c
}

// Width sets a min-width (px) on the column's cells.
func (c TableColumnSpec) Width(px int) TableColumnSpec {
	c.WidthPx = px
	return c
}

// Bar renders the column as a numeric value with a proportional mini-bar,
// scaled against the column's max absolute value across rows.
func (c TableColumnSpec) Bar() TableColumnSpec {
	c.Cell = &panel.TableCellSpec{Kind: panel.TableCellBar}
	return c
}

// Delta renders the column as a signed delta plus a percent change read from
// percentField, colored by the delta's sign.
func (c TableColumnSpec) Delta(percentField string) TableColumnSpec {
	c.Cell = &panel.TableCellSpec{Kind: panel.TableCellDelta, PercentField: panel.FieldRef(percentField)}
	return c
}

// Underline renders the column as a value over a thin proportional rule
// colored by sign — a low-ink alternative to Bar.
func (c TableColumnSpec) Underline() TableColumnSpec {
	c.Cell = &panel.TableCellSpec{Kind: panel.TableCellUnderline}
	return c
}

// Stacked puts a rich cell's secondary value on its own line under the primary
// one. It is a no-op on columns without a rich cell.
func (c TableColumnSpec) Stacked() TableColumnSpec {
	if c.Cell == nil {
		return c
	}
	cell := *c.Cell
	cell.Stacked = true
	c.Cell = &cell
	return c
}

// Pill marks an actionable column's cells as compact drill pills.
func (c TableColumnSpec) Pill() TableColumnSpec {
	c.Affordance = "pill"
	return c
}

// Quiet makes an actionable column's whole cell the drill target with no
// standing chrome; the drill arrow and value underline appear only on hover.
func (c TableColumnSpec) Quiet() TableColumnSpec {
	c.Affordance = "quiet"
	return c
}

// Tone colors the cell value by a per-row status read from toneField ("pos",
// "warn", "neg"; empty keeps the default text color).
func (c TableColumnSpec) Tone(toneField string) TableColumnSpec {
	c.ToneField = toneField
	return c
}

// Badge renders a muted "?" badge after the cell value for rows whose
// badgeField value is non-empty, using that value as the badge's title.
func (c TableColumnSpec) Badge(badgeField string) TableColumnSpec {
	c.BadgeField = badgeField
	return c
}

// Clamp limits the column's cell text to lines rendered lines.
func (c TableColumnSpec) Clamp(lines int) TableColumnSpec {
	c.ClampLines = lines
	return c
}

func Ref(name string) string {
	return name
}
