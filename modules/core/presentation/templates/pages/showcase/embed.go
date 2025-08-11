package showcase

import _ "embed"

//go:embed components/input.templ
var InputComponentSource string

//go:embed components/textarea.templ
var TextareaComponentSource string

//go:embed components/number.templ
var NumberComponentSource string

//go:embed components/select.templ
var SelectComponentSource string

//go:embed components/combobox.templ
var ComboboxComponentSource string

//go:embed components/radio.templ
var RadioComponentSource string

//go:embed components/avatar.templ
var AvatarComponentSource string

//go:embed components/card.templ
var CardComponentSource string

//go:embed components/datepicker.templ
var DatepickerComponentSource string

//go:embed components/advanced_datepicker.templ
var AdvancedDatepickerComponentSource string

//go:embed components/table.templ
var TableComponentSource string

//go:embed components/editable_table.templ
var EditableTableComponentSource string

//go:embed components/buttons.templ
var ButtonsComponentSource string

//go:embed components/navtabs.templ
var NavTabsComponentSource string

//go:embed components/slider.templ
var SliderComponentSource string

//go:embed components/spinners.templ
var SpinnersComponentSource string

//go:embed components/skeletons.templ
var SkeletonsComponentSource string

// Chart components
//
//go:embed components/charts/bar_chart.templ
var BarChartSource string

//go:embed components/charts/line_chart.templ
var LineChartSource string

//go:embed components/charts/area_chart.templ
var AreaChartSource string

//go:embed components/charts/pie_chart.templ
var PieChartSource string

//go:embed components/charts/donut_chart.templ
var DonutChartSource string

//go:embed components/charts/radial_bar_chart.templ
var RadialBarChartSource string

//go:embed components/charts/scatter_chart.templ
var ScatterChartSource string

//go:embed components/charts/heatmap_chart.templ
var HeatmapChartSource string

//go:embed components/charts/radar_chart.templ
var RadarChartSource string

//go:embed components/charts/polar_area_chart.templ
var PolarAreaChartSource string

//go:embed components/kanban.templ
var KanbanBoardSource string
