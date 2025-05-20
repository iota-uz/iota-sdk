package charts

import "github.com/a-h/templ"

type ChartType string

const (
	LineChart        ChartType = "line"
	AreaChart        ChartType = "area"
	BarChart         ChartType = "bar"
	PieChart         ChartType = "pie"
	DonutChart       ChartType = "donut"
	RadialBarChart   ChartType = "radialBar"
	ScatterChart     ChartType = "scatter"
	BubbleChart      ChartType = "bubble"
	HeatmapChart     ChartType = "heatmap"
	CandlestickChart ChartType = "candlestick"
	BoxPlotChart     ChartType = "boxPlot"
	RadarChart       ChartType = "radar"
	PolarAreaChart   ChartType = "polarArea"
	RangeBarChart    ChartType = "rangeBar"
	RangeAreaChart   ChartType = "rangeArea"
	TreemapChart     ChartType = "treemap"
)

type ChartOptions struct {
	Chart       ChartConfig    `json:"chart"`
	Series      []Series       `json:"series"`
	XAxis       XAxisConfig    `json:"xaxis"`
	YAxis       []YAxisConfig  `json:"yaxis"` // YAxis can be an array for multiple y-axes
	Colors      []string       `json:"colors,omitempty"`
	DataLabels  *DataLabels    `json:"dataLabels,omitempty"`
	Grid        *GridConfig    `json:"grid,omitempty"`
	PlotOptions *PlotOptions   `json:"plotOptions,omitempty"`
	Tooltip     *TooltipConfig `json:"tooltip,omitempty"`
	Title       *TitleConfig   `json:"title,omitempty"`
	Theme       *ThemeConfig   `json:"theme,omitempty"`
	Stroke      *StrokeConfig  `json:"stroke,omitempty"`      // Added based on common ApexCharts options
	Markers     *MarkersConfig `json:"markers,omitempty"`     // Added based on common ApexCharts options
	Legend      *LegendConfig  `json:"legend,omitempty"`      // Added based on common ApexCharts options
	NoData      *NoDataConfig  `json:"noData,omitempty"`      // Added based on common ApexCharts options
	States      *StatesConfig  `json:"states,omitempty"`      // Added based on common ApexCharts options
	Fill        *FillConfig    `json:"fill,omitempty"`        // Added based on common ApexCharts options
	Annotations *Annotations   `json:"annotations,omitempty"` // Added based on common ApexCharts options
}

type ChartConfig struct {
	Type    ChartType `json:"type"`
	Height  string    `json:"height,omitempty"`
	Toolbar Toolbar   `json:"toolbar,omitempty"`
}

type Toolbar struct {
	Show bool `json:"show"`
}

type Series struct {
	Name string        `json:"name"`
	Type *ChartType    `json:"type,omitempty"` // For combo charts
	Data []interface{} `json:"data"`           // Can be []float64, or complex objects for candle/box etc.
}

type AxisTitle struct {
	Text  string    `json:"text"`
	Style TextStyle `json:"style"`
}

type TextStyle struct {
	FontWeight string `json:"fontWeight,omitempty"`
	FontSize   string `json:"fontSize,omitempty"`
	Color      string `json:"color,omitempty"`
}

type LabelFormatter struct {
	Style LabelStyle `json:"style"`
}

type LabelStyle struct {
	Colors   string `json:"colors,omitempty"`
	FontSize string `json:"fontSize,omitempty"`
}

type DataLabelStyle struct {
	Colors     []string `json:"colors,omitempty"`
	FontSize   string   `json:"fontSize,omitempty"`
	FontWeight string   `json:"fontWeight,omitempty"`
}

type DataLabels struct {
	Enabled    bool               `json:"enabled"`
	Formatter  templ.JSExpression `json:"formatter,omitempty"`
	Style      *DataLabelStyle    `json:"style,omitempty"`
	OffsetY    int                `json:"offsetY,omitempty"`
	DropShadow *DropShadow        `json:"dropShadow,omitempty"`
}

type DropShadow struct {
	Enabled bool    `json:"enabled"`
	Top     int     `json:"top,omitempty"`
	Left    int     `json:"left,omitempty"`
	Blur    int     `json:"blur,omitempty"`
	Color   string  `json:"color,omitempty"`
	Opacity float64 `json:"opacity,omitempty"`
}

type GridConfig struct {
	BorderColor string         `json:"borderColor,omitempty"`
	Row         *GridRowColumn `json:"row,omitempty"`
	Column      *GridRowColumn `json:"column,omitempty"`
	Padding     *Padding       `json:"padding,omitempty"`
}

type GridRowColumn struct {
	Colors  []string `json:"colors,omitempty"`
	Opacity *float64 `json:"opacity,omitempty"`
}

type Padding struct {
	Top    *int `json:"top,omitempty"`
	Right  *int `json:"right,omitempty"`
	Bottom *int `json:"bottom,omitempty"`
	Left   *int `json:"left,omitempty"`
}

type PlotOptions struct {
	Bar       *BarConfig       `json:"bar,omitempty"`
	Pie       *PieDonutConfig  `json:"pie,omitempty"`
	Donut     *PieDonutConfig  `json:"donut,omitempty"`
	RadialBar *RadialBarConfig `json:"radialBar,omitempty"`
	Heatmap   *HeatmapConfig   `json:"heatmap,omitempty"`
}

type BarConfig struct {
	BorderRadius int       `json:"borderRadius,omitempty"`
	ColumnWidth  string    `json:"columnWidth,omitempty"`
	DataLabels   BarLabels `json:"dataLabels,omitempty"`
}

type BarLabels struct {
	Position string `json:"position,omitempty"`
}

type PieDonutConfig struct {
	Size        *string         `json:"size,omitempty"`
	Donut       *DonutSpecifics `json:"donut,omitempty"` // Only for donut
	CustomScale *float64        `json:"customScale,omitempty"`
	OffsetX     *int            `json:"offsetX,omitempty"`
	OffsetY     *int            `json:"offsetY,omitempty"`
	DataLabels  *PieDataLabels  `json:"dataLabels,omitempty"`
}

type DonutSpecifics struct {
	Size   *string `json:"size,omitempty"`
	Labels *Labels `json:"labels,omitempty"`
}

type Labels struct {
	Show      *bool              `json:"show,omitempty"`
	Name      *LabelNameValue    `json:"name,omitempty"`
	Value     *LabelNameValue    `json:"value,omitempty"`
	Total     *LabelTotal        `json:"total,omitempty"`
	Formatter templ.JSExpression `json:"formatter,omitempty"`
}

type LabelNameValue struct {
	Show       *bool              `json:"show,omitempty"`
	FontSize   *string            `json:"fontSize,omitempty"`
	FontFamily *string            `json:"fontFamily,omitempty"`
	FontWeight *string            `json:"fontWeight,omitempty"`
	Color      *string            `json:"color,omitempty"`
	OffsetY    *int               `json:"offsetY,omitempty"`
	Formatter  templ.JSExpression `json:"formatter,omitempty"`
}

type LabelTotal struct {
	Show       *bool              `json:"show,omitempty"`
	ShowAlways *bool              `json:"showAlways,omitempty"`
	Label      *string            `json:"label,omitempty"`
	FontSize   *string            `json:"fontSize,omitempty"`
	FontFamily *string            `json:"fontFamily,omitempty"`
	FontWeight *string            `json:"fontWeight,omitempty"`
	Color      *string            `json:"color,omitempty"`
	Formatter  templ.JSExpression `json:"formatter,omitempty"`
}

type PieDataLabels struct {
	Offset *int `json:"offset,omitempty"`
}

type RadialBarConfig struct {
	Size         *string              `json:"size,omitempty"`
	InverseOrder *bool                `json:"inverseOrder,omitempty"`
	StartAngle   *int                 `json:"startAngle,omitempty"`
	EndAngle     *int                 `json:"endAngle,omitempty"`
	OffsetX      *int                 `json:"offsetX,omitempty"`
	OffsetY      *int                 `json:"offsetY,omitempty"`
	Hollow       *RadialBarHollow     `json:"hollow,omitempty"`
	Track        *RadialBarTrack      `json:"track,omitempty"`
	DataLabels   *RadialBarDataLabels `json:"dataLabels,omitempty"`
}

type RadialBarHollow struct {
	Margin       *int        `json:"margin,omitempty"`
	Size         *string     `json:"size,omitempty"`
	Background   *string     `json:"background,omitempty"`
	Image        *string     `json:"image,omitempty"`
	ImageWidth   *int        `json:"imageWidth,omitempty"`
	ImageHeight  *int        `json:"imageHeight,omitempty"`
	ImageOffsetX *int        `json:"imageOffsetX,omitempty"`
	ImageOffsetY *int        `json:"imageOffsetY,omitempty"`
	ImageClipped *bool       `json:"imageClipped,omitempty"`
	Position     *string     `json:"position,omitempty"` // 'front', 'back'
	DropShadow   *DropShadow `json:"dropShadow,omitempty"`
}

type RadialBarTrack struct {
	Show        *bool       `json:"show,omitempty"`
	Background  *string     `json:"background,omitempty"`
	StrokeWidth *string     `json:"strokeWidth,omitempty"`
	Opacity     *float64    `json:"opacity,omitempty"`
	Margin      *int        `json:"margin,omitempty"`
	DropShadow  *DropShadow `json:"dropShadow,omitempty"`
}

type RadialBarDataLabels struct {
	Show      *bool              `json:"show,omitempty"`
	Name      *LabelNameValue    `json:"name,omitempty"`
	Value     *LabelNameValue    `json:"value,omitempty"`
	Total     *LabelTotal        `json:"total,omitempty"`
	Formatter templ.JSExpression `json:"formatter,omitempty"`
}

type HeatmapConfig struct {
	Radius          *int               `json:"radius,omitempty"`
	EnableShades    *bool              `json:"enableShades,omitempty"`
	ShadeIntensity  *float64           `json:"shadeIntensity,omitempty"`
	ReverseNegative *bool              `json:"reverseNegative,omitempty"`
	Distributed     *bool              `json:"distributed,omitempty"`
	ColorScale      *HeatmapColorScale `json:"colorScale,omitempty"`
}

type HeatmapColorScale struct {
	Ranges  []HeatmapRange `json:"ranges,omitempty"`
	Inverse *bool          `json:"inverse,omitempty"`
	Min     *float64       `json:"min,omitempty"`
	Max     *float64       `json:"max,omitempty"`
}

type HeatmapRange struct {
	From  *float64 `json:"from,omitempty"`
	To    *float64 `json:"to,omitempty"`
	Color *string  `json:"color,omitempty"`
	Name  *string  `json:"name,omitempty"`
}

type XAxisType string

const (
	XAxisTypeCategory XAxisType = "category"
	XAxisTypeDateTime XAxisType = "datetime"
	XAxisTypeNumeric  XAxisType = "numeric"
)

type XAxisTickPlacement string

const (
	XAxisTickPlacementBetween XAxisTickPlacement = "between"
	XAxisTickPlacementOn      XAxisTickPlacement = "on"
)

type XAxisPosition string

const (
	XAxisPositionTop    XAxisPosition = "top"
	XAxisPositionBottom XAxisPosition = "bottom"
)

type XAxisBorderType string

const (
	XAxisBorderTypeSolid  XAxisBorderType = "solid"
	XAxisBorderTypeDotted XAxisBorderType = "dotted"
)

type XAxisCrosshairPosition string

const (
	XAxisCrosshairPositionBack  XAxisCrosshairPosition = "back"
	XAxisCrosshairPositionFront XAxisCrosshairPosition = "front"
)

type XAxisCrosshairFillType string

const (
	XAxisCrosshairFillTypeSolid    XAxisCrosshairFillType = "solid"
	XAxisCrosshairFillTypeGradient XAxisCrosshairFillType = "gradient"
)

type XAxisLabelStyleConfig struct {
	Colors     interface{} `json:"colors,omitempty"`
	FontSize   *string     `json:"fontSize,omitempty"`
	FontFamily *string     `json:"fontFamily,omitempty"`
	FontWeight interface{} `json:"fontWeight,omitempty"`
	CSSClass   *string     `json:"cssClass,omitempty"`
}

type XAxisDateTimeFormatterConfig struct {
	Year   *string `json:"year,omitempty"`
	Month  *string `json:"month,omitempty"`
	Day    *string `json:"day,omitempty"`
	Hour   *string `json:"hour,omitempty"`
	Minute *string `json:"minute,omitempty"`
	Second *string `json:"second,omitempty"`
}

type XAxisLabelsConfig struct {
	Show                  *bool                         `json:"show,omitempty"`
	Rotate                *int                          `json:"rotate,omitempty"`
	RotateAlways          *bool                         `json:"rotateAlways,omitempty"`
	HideOverlappingLabels *bool                         `json:"hideOverlappingLabels,omitempty"`
	ShowDuplicates        *bool                         `json:"showDuplicates,omitempty"`
	Trim                  *bool                         `json:"trim,omitempty"`
	MinHeight             *int                          `json:"minHeight,omitempty"`
	MaxHeight             *int                          `json:"maxHeight,omitempty"`
	Style                 *XAxisLabelStyleConfig        `json:"style,omitempty"`
	OffsetX               *int                          `json:"offsetX,omitempty"`
	OffsetY               *int                          `json:"offsetY,omitempty"`
	Format                *string                       `json:"format,omitempty"`
	Formatter             templ.JSExpression            `json:"formatter,omitempty"`
	DatetimeUTC           *bool                         `json:"datetimeUTC,omitempty"`
	DatetimeFormatter     *XAxisDateTimeFormatterConfig `json:"datetimeFormatter,omitempty"`
}

type XAxisGroupItemConfig struct {
	Title string `json:"title"`
	Cols  int    `json:"cols"`
}

type XAxisGroupStyleConfig struct {
	Colors     interface{} `json:"colors,omitempty"`
	FontSize   *string     `json:"fontSize,omitempty"`
	FontWeight interface{} `json:"fontWeight,omitempty"`
	FontFamily *string     `json:"fontFamily,omitempty"`
	CSSClass   *string     `json:"cssClass,omitempty"`
}

type XAxisGroupConfig struct {
	Groups []XAxisGroupItemConfig `json:"groups,omitempty"`
	Style  *XAxisGroupStyleConfig `json:"style,omitempty"`
}

type XAxisBorderConfig struct {
	Show    *bool       `json:"show,omitempty"`
	Color   *string     `json:"color,omitempty"`
	Height  *int        `json:"height,omitempty"`
	Width   interface{} `json:"width,omitempty"`
	OffsetX *int        `json:"offsetX,omitempty"`
	OffsetY *int        `json:"offsetY,omitempty"`
}

type XAxisTicksConfig struct {
	Show       *bool            `json:"show,omitempty"`
	BorderType *XAxisBorderType `json:"borderType,omitempty"`
	Color      *string          `json:"color,omitempty"`
	Height     *int             `json:"height,omitempty"`
	OffsetX    *int             `json:"offsetX,omitempty"`
	OffsetY    *int             `json:"offsetY,omitempty"`
}

type XAxisTitleStyleConfig struct {
	Color      *string     `json:"color,omitempty"`
	FontSize   *string     `json:"fontSize,omitempty"`
	FontFamily *string     `json:"fontFamily,omitempty"`
	FontWeight interface{} `json:"fontWeight,omitempty"`
	CSSClass   *string     `json:"cssClass,omitempty"`
}

type XAxisTitleConfig struct {
	Text    *string                `json:"text,omitempty"`
	OffsetX *int                   `json:"offsetX,omitempty"`
	OffsetY *int                   `json:"offsetY,omitempty"`
	Style   *XAxisTitleStyleConfig `json:"style,omitempty"`
}

type XAxisCrosshairsStrokeConfig struct {
	Color     *string `json:"color,omitempty"`
	Width     *int    `json:"width,omitempty"`
	DashArray *int    `json:"dashArray,omitempty"`
}

type XAxisCrosshairsGradientConfig struct {
	ColorFrom   *string   `json:"colorFrom,omitempty"`
	ColorTo     *string   `json:"colorTo,omitempty"`
	Stops       []float64 `json:"stops,omitempty"`
	OpacityFrom *float64  `json:"opacityFrom,omitempty"`
	OpacityTo   *float64  `json:"opacityTo,omitempty"`
}

type XAxisCrosshairsFillConfig struct {
	Type     *XAxisCrosshairFillType        `json:"type,omitempty"`
	Color    *string                        `json:"color,omitempty"`
	Gradient *XAxisCrosshairsGradientConfig `json:"gradient,omitempty"`
}

type XAxisCrosshairsConfig struct {
	Show       *bool                        `json:"show,omitempty"`
	Width      interface{}                  `json:"width,omitempty"`
	Position   *XAxisCrosshairPosition      `json:"position,omitempty"`
	Opacity    *float64                     `json:"opacity,omitempty"`
	Stroke     *XAxisCrosshairsStrokeConfig `json:"stroke,omitempty"`
	Fill       *XAxisCrosshairsFillConfig   `json:"fill,omitempty"`
	DropShadow *DropShadow                  `json:"dropShadow,omitempty"`
}

type XAxisTooltipStyleConfig struct {
	FontSize   *string `json:"fontSize,omitempty"`
	FontFamily *string `json:"fontFamily,omitempty"`
}

type XAxisTooltipConfig struct {
	Enabled   *bool                    `json:"enabled,omitempty"`
	Formatter templ.JSExpression       `json:"formatter,omitempty"`
	OffsetY   *int                     `json:"offsetY,omitempty"`
	Style     *XAxisTooltipStyleConfig `json:"style,omitempty"`
}

type XAxisConfig struct {
	Type                *XAxisType             `json:"type,omitempty"`
	Categories          []string               `json:"categories,omitempty"`
	TickAmount          interface{}            `json:"tickAmount,omitempty"`
	TickPlacement       *XAxisTickPlacement    `json:"tickPlacement,omitempty"`
	Min                 *float64               `json:"min,omitempty"`
	Max                 *float64               `json:"max,omitempty"`
	StepSize            *float64               `json:"stepSize,omitempty"`
	Range               *float64               `json:"range,omitempty"`
	Floating            *bool                  `json:"floating,omitempty"`
	DecimalsInFloat     *int                   `json:"decimalsInFloat,omitempty"`
	OverwriteCategories []string               `json:"overwriteCategories,omitempty"`
	Position            *XAxisPosition         `json:"position,omitempty"`
	Labels              *XAxisLabelsConfig     `json:"labels,omitempty"`
	Group               *XAxisGroupConfig      `json:"group,omitempty"`
	AxisBorder          *XAxisBorderConfig     `json:"axisBorder,omitempty"`
	AxisTicks           *XAxisTicksConfig      `json:"axisTicks,omitempty"`
	Title               *XAxisTitleConfig      `json:"title,omitempty"`
	Crosshairs          *XAxisCrosshairsConfig `json:"crosshairs,omitempty"`
	Tooltip             *XAxisTooltipConfig    `json:"tooltip,omitempty"`
}

type YAxisLabelAlign string

const (
	YAxisLabelAlignLeft   YAxisLabelAlign = "left"
	YAxisLabelAlignCenter YAxisLabelAlign = "center"
	YAxisLabelAlignRight  YAxisLabelAlign = "right"
)

type YAxisLabelStyleConfig struct {
	Colors     interface{} `json:"colors,omitempty"`
	FontSize   *string     `json:"fontSize,omitempty"`
	FontFamily *string     `json:"fontFamily,omitempty"`
	FontWeight interface{} `json:"fontWeight,omitempty"`
	CSSClass   *string     `json:"cssClass,omitempty"`
}

type YAxisLabelsConfig struct {
	Show           *bool                  `json:"show,omitempty"`
	ShowDuplicates *bool                  `json:"showDuplicates,omitempty"`
	Align          *YAxisLabelAlign       `json:"align,omitempty"`
	MinWidth       *int                   `json:"minWidth,omitempty"`
	MaxWidth       *int                   `json:"maxWidth,omitempty"`
	Style          *YAxisLabelStyleConfig `json:"style,omitempty"`
	OffsetX        *int                   `json:"offsetX,omitempty"`
	OffsetY        *int                   `json:"offsetY,omitempty"`
	Rotate         *int                   `json:"rotate,omitempty"`
	Formatter      templ.JSExpression     `json:"formatter,omitempty"`
}

type YAxisBorderConfig struct {
	Show    *bool   `json:"show,omitempty"`
	Color   *string `json:"color,omitempty"`
	OffsetX *int    `json:"offsetX,omitempty"`
	OffsetY *int    `json:"offsetY,omitempty"`
}

type YAxisTicksConfig struct {
	Show       *bool            `json:"show,omitempty"`
	BorderType *XAxisBorderType `json:"borderType,omitempty"`
	Color      *string          `json:"color,omitempty"`
	Width      *int             `json:"width,omitempty"`
	OffsetX    *int             `json:"offsetX,omitempty"`
	OffsetY    *int             `json:"offsetY,omitempty"`
}

type YAxisTitleStyleConfig struct {
	Color      *string     `json:"color,omitempty"`
	FontSize   *string     `json:"fontSize,omitempty"`
	FontFamily *string     `json:"fontFamily,omitempty"`
	FontWeight interface{} `json:"fontWeight,omitempty"`
	CSSClass   *string     `json:"cssClass,omitempty"`
}

type YAxisTitleConfig struct {
	Text    *string                `json:"text,omitempty"`
	Rotate  *int                   `json:"rotate,omitempty"`
	OffsetX *int                   `json:"offsetX,omitempty"`
	OffsetY *int                   `json:"offsetY,omitempty"`
	Style   *YAxisTitleStyleConfig `json:"style,omitempty"`
}

type YAxisCrosshairsStrokeConfig struct {
	Color     *string `json:"color,omitempty"`
	Width     *int    `json:"width,omitempty"`
	DashArray *int    `json:"dashArray,omitempty"`
}

type YAxisCrosshairsConfig struct {
	Show     *bool                        `json:"show,omitempty"`
	Position *XAxisCrosshairPosition      `json:"position,omitempty"`
	Stroke   *YAxisCrosshairsStrokeConfig `json:"stroke,omitempty"`
}

type YAxisTooltipConfig struct {
	Enabled *bool `json:"enabled,omitempty"`
	OffsetX *int  `json:"offsetX,omitempty"`
}

type YAxisConfig struct {
	Show              *bool                  `json:"show,omitempty"`
	ShowAlways        *bool                  `json:"showAlways,omitempty"`
	ShowForNullSeries *bool                  `json:"showForNullSeries,omitempty"`
	SeriesName        interface{}            `json:"seriesName,omitempty"`
	Opposite          *bool                  `json:"opposite,omitempty"`
	Reversed          *bool                  `json:"reversed,omitempty"`
	Logarithmic       *bool                  `json:"logarithmic,omitempty"`
	LogBase           *int                   `json:"logBase,omitempty"`
	TickAmount        *int                   `json:"tickAmount,omitempty"`
	Min               interface{}            `json:"min,omitempty"`
	Max               interface{}            `json:"max,omitempty"`
	StepSize          *float64               `json:"stepSize,omitempty"`
	ForceNiceScale    *bool                  `json:"forceNiceScale,omitempty"`
	Floating          *bool                  `json:"floating,omitempty"`
	DecimalsInFloat   *int                   `json:"decimalsInFloat,omitempty"`
	Labels            *YAxisLabelsConfig     `json:"labels,omitempty"`
	AxisBorder        *YAxisBorderConfig     `json:"axisBorder,omitempty"`
	AxisTicks         *YAxisTicksConfig      `json:"axisTicks,omitempty"`
	Title             *YAxisTitleConfig      `json:"title,omitempty"`
	Crosshairs        *YAxisCrosshairsConfig `json:"crosshairs,omitempty"`
	Tooltip           *YAxisTooltipConfig    `json:"tooltip,omitempty"`
}

type TooltipStyleConfig struct {
	FontSize   *string `json:"fontSize,omitempty"`
	FontFamily *string `json:"fontFamily,omitempty"`
}

type TooltipOnDatasetHoverConfig struct {
	HighlightDataSeries *bool `json:"highlightDataSeries,omitempty"`
}

type TooltipXConfig struct {
	Show      *bool              `json:"show,omitempty"`
	Format    *string            `json:"format,omitempty"`
	Formatter templ.JSExpression `json:"formatter,omitempty"`
}

type TooltipYTitleConfig struct {
	Formatter templ.JSExpression `json:"formatter,omitempty"`
}

type TooltipYConfig struct {
	Formatter templ.JSExpression   `json:"formatter,omitempty"`
	Title     *TooltipYTitleConfig `json:"title,omitempty"`
}

type TooltipZConfig struct {
	Formatter templ.JSExpression `json:"formatter,omitempty"`
	Title     *string            `json:"title,omitempty"`
}

type TooltipMarkerConfig struct {
	Show *bool `json:"show,omitempty"`
}

type TooltipItemsConfig struct {
	Display *string `json:"display,omitempty"`
}

type TooltipFixedPosition string

const (
	TooltipFixedPositionTopLeft     TooltipFixedPosition = "topLeft"
	TooltipFixedPositionTopRight    TooltipFixedPosition = "topRight"
	TooltipFixedPositionBottomLeft  TooltipFixedPosition = "bottomLeft"
	TooltipFixedPositionBottomRight TooltipFixedPosition = "bottomRight"
)

type TooltipFixedConfig struct {
	Enabled  *bool                 `json:"enabled,omitempty"`
	Position *TooltipFixedPosition `json:"position,omitempty"`
	OffsetX  *int                  `json:"offsetX,omitempty"`
	OffsetY  *int                  `json:"offsetY,omitempty"`
}

type TooltipConfig struct {
	Enabled         *bool                        `json:"enabled,omitempty"`
	EnabledOnSeries []int                        `json:"enabledOnSeries,omitempty"`
	Shared          *bool                        `json:"shared,omitempty"`
	FollowCursor    *bool                        `json:"followCursor,omitempty"`
	Intersect       *bool                        `json:"intersect,omitempty"`
	InverseOrder    *bool                        `json:"inverseOrder,omitempty"`
	Custom          interface{}                  `json:"custom,omitempty"`
	HideEmptySeries *bool                        `json:"hideEmptySeries,omitempty"`
	FillSeriesColor *bool                        `json:"fillSeriesColor,omitempty"`
	Theme           *string                      `json:"theme,omitempty"`
	Style           *TooltipStyleConfig          `json:"style,omitempty"`
	OnDatasetHover  *TooltipOnDatasetHoverConfig `json:"onDatasetHover,omitempty"`
	X               *TooltipXConfig              `json:"x,omitempty"`
	Y               interface{}                  `json:"y,omitempty"`
	Z               *TooltipZConfig              `json:"z,omitempty"`
	Marker          *TooltipMarkerConfig         `json:"marker,omitempty"`
	Items           *TooltipItemsConfig          `json:"items,omitempty"`
	Fixed           *TooltipFixedConfig          `json:"fixed,omitempty"`
}

type TitleAlign string

const (
	TitleAlignLeft   TitleAlign = "left"
	TitleAlignCenter TitleAlign = "center"
	TitleAlignRight  TitleAlign = "right"
)

type TitleStyleConfig struct {
	FontSize   *string     `json:"fontSize,omitempty"`
	FontWeight interface{} `json:"fontWeight,omitempty"`
	FontFamily *string     `json:"fontFamily,omitempty"`
	Color      *string     `json:"color,omitempty"`
}

type TitleConfig struct {
	Text     *string           `json:"text,omitempty"`
	Align    *TitleAlign       `json:"align,omitempty"`
	Margin   *int              `json:"margin,omitempty"`
	OffsetX  *int              `json:"offsetX,omitempty"`
	OffsetY  *int              `json:"offsetY,omitempty"`
	Floating *bool             `json:"floating,omitempty"`
	Style    *TitleStyleConfig `json:"style,omitempty"`
}

type ThemeMode string

const (
	ThemeModeLight ThemeMode = "light"
	ThemeModeDark  ThemeMode = "dark"
)

type ThemePalette string

const (
	ThemePalette1  ThemePalette = "palette1"
	ThemePalette2  ThemePalette = "palette2"
	ThemePalette3  ThemePalette = "palette3"
	ThemePalette4  ThemePalette = "palette4"
	ThemePalette5  ThemePalette = "palette5"
	ThemePalette6  ThemePalette = "palette6"
	ThemePalette7  ThemePalette = "palette7"
	ThemePalette8  ThemePalette = "palette8"
	ThemePalette9  ThemePalette = "palette9"
	ThemePalette10 ThemePalette = "palette10"
)

type ThemeMonochromeShadeTo string

const (
	ThemeMonochromeShadeToLight ThemeMonochromeShadeTo = "light"
	ThemeMonochromeShadeToDark  ThemeMonochromeShadeTo = "dark"
)

type ThemeMonochromeConfig struct {
	Enabled        *bool                   `json:"enabled,omitempty"`
	Color          *string                 `json:"color,omitempty"`
	ShadeTo        *ThemeMonochromeShadeTo `json:"shadeTo,omitempty"`
	ShadeIntensity *float64                `json:"shadeIntensity,omitempty"`
}

type ThemeConfig struct {
	Mode       *ThemeMode             `json:"mode,omitempty"`
	Palette    *ThemePalette          `json:"palette,omitempty"`
	Monochrome *ThemeMonochromeConfig `json:"monochrome,omitempty"`
}

type StrokeCurve string

const (
	StrokeCurveSmooth   StrokeCurve = "smooth"
	StrokeCurveStraight StrokeCurve = "straight"
	StrokeCurveStepline StrokeCurve = "stepline"
)

type StrokeLineCap string

const (
	StrokeLineCapButt   StrokeLineCap = "butt"
	StrokeLineCapSquare StrokeLineCap = "square"
	StrokeLineCapRound  StrokeLineCap = "round"
)

type StrokeConfig struct {
	Show      *bool         `json:"show,omitempty"`
	Curve     interface{}   `json:"curve,omitempty"` // StrokeCurve or []StrokeCurve
	LineCap   StrokeLineCap `json:"lineCap,omitempty"`
	Colors    []string      `json:"colors,omitempty"`
	Width     interface{}   `json:"width,omitempty"`     // number or []number
	DashArray interface{}   `json:"dashArray,omitempty"` // number or []number
}

type MarkersConfig struct {
	Size               interface{}        `json:"size,omitempty"` // number or []number
	Colors             []string           `json:"colors,omitempty"`
	StrokeColor        interface{}        `json:"strokeColor,omitempty"`     // string or []string
	StrokeWidth        interface{}        `json:"strokeWidth,omitempty"`     // number or []number
	StrokeOpacity      interface{}        `json:"strokeOpacity,omitempty"`   // number or []number
	StrokeDashArray    interface{}        `json:"strokeDashArray,omitempty"` // number or []number
	FillOpacity        interface{}        `json:"fillOpacity,omitempty"`     // number or []number
	Discrete           []MarkerDiscrete   `json:"discrete,omitempty"`
	Shape              interface{}        `json:"shape,omitempty"` // string or []string ('circle', 'square', 'rect')
	Radius             *int               `json:"radius,omitempty"`
	OffsetX            *int               `json:"offsetX,omitempty"`
	OffsetY            *int               `json:"offsetY,omitempty"`
	OnClick            templ.JSExpression `json:"onClick,omitempty"`
	OnDblClick         templ.JSExpression `json:"onDblClick,omitempty"`
	ShowNullDataPoints *bool              `json:"showNullDataPoints,omitempty"`
	Hover              *MarkerHover       `json:"hover,omitempty"`
}

type MarkerDiscrete struct {
	SeriesIndex    *int    `json:"seriesIndex,omitempty"`
	DataPointIndex *int    `json:"dataPointIndex,omitempty"`
	FillColor      *string `json:"fillColor,omitempty"`
	StrokeColor    *string `json:"strokeColor,omitempty"`
	Size           *int    `json:"size,omitempty"`
	Shape          *string `json:"shape,omitempty"` // 'circle', 'square', 'rect'
}

type MarkerHover struct {
	Size       *int `json:"size,omitempty"`
	SizeOffset *int `json:"sizeOffset,omitempty"`
}

type LegendPosition string

const (
	LegendPositionTop    LegendPosition = "top"
	LegendPositionRight  LegendPosition = "right"
	LegendPositionBottom LegendPosition = "bottom"
	LegendPositionLeft   LegendPosition = "left"
)

type LegendHorizontalAlign string

const (
	LegendHorizontalAlignLeft   LegendHorizontalAlign = "left"
	LegendHorizontalAlignCenter LegendHorizontalAlign = "center"
	LegendHorizontalAlignRight  LegendHorizontalAlign = "right"
)

type LegendConfig struct {
	Show                  *bool                  `json:"show,omitempty"`
	ShowForSingleSeries   *bool                  `json:"showForSingleSeries,omitempty"`
	ShowForNullSeries     *bool                  `json:"showForNullSeries,omitempty"`
	ShowForZeroSeries     *bool                  `json:"showForZeroSeries,omitempty"`
	Position              *LegendPosition        `json:"position,omitempty"`
	HorizontalAlign       *LegendHorizontalAlign `json:"horizontalAlign,omitempty"`
	Floating              *bool                  `json:"floating,omitempty"`
	FontSize              *string                `json:"fontSize,omitempty"`
	FontFamily            *string                `json:"fontFamily,omitempty"`
	FontWeight            interface{}            `json:"fontWeight,omitempty"`
	Formatter             templ.JSExpression     `json:"formatter,omitempty"`
	InverseOrder          *bool                  `json:"inverseOrder,omitempty"`
	Width                 *int                   `json:"width,omitempty"`
	Height                *int                   `json:"height,omitempty"`
	TooltipHoverFormatter templ.JSExpression     `json:"tooltipHoverFormatter,omitempty"`
	CustomLegendItems     []string               `json:"customLegendItems,omitempty"`
	OffsetX               *int                   `json:"offsetX,omitempty"`
	OffsetY               *int                   `json:"offsetY,omitempty"`
	Labels                *LegendLabelsConfig    `json:"labels,omitempty"`
	Markers               *LegendMarkersConfig   `json:"markers,omitempty"`
	ItemMargin            *LegendItemMargin      `json:"itemMargin,omitempty"`
	OnItemClick           *LegendOnItemClick     `json:"onItemClick,omitempty"`
	OnItemHover           *LegendOnItemHover     `json:"onItemHover,omitempty"`
	ContainerMargin       *Padding               `json:"containerMargin,omitempty"`
}

type LegendLabelsConfig struct {
	Colors          interface{} `json:"colors,omitempty"` // string or []string
	UseSeriesColors *bool       `json:"useSeriesColors,omitempty"`
}

type LegendMarkersConfig struct {
	Width       *int               `json:"width,omitempty"`
	Height      *int               `json:"height,omitempty"`
	StrokeWidth *int               `json:"strokeWidth,omitempty"`
	StrokeColor *string            `json:"strokeColor,omitempty"`
	FillColors  []string           `json:"fillColors,omitempty"`
	Radius      *int               `json:"radius,omitempty"`
	CustomHTML  templ.JSExpression `json:"customHTML,omitempty"`
	OnClick     templ.JSExpression `json:"onClick,omitempty"`
	OffsetX     *int               `json:"offsetX,omitempty"`
	OffsetY     *int               `json:"offsetY,omitempty"`
}

type LegendItemMargin struct {
	Horizontal *int `json:"horizontal,omitempty"`
	Vertical   *int `json:"vertical,omitempty"`
}

type LegendOnItemClick struct {
	ToggleDataSeries *bool `json:"toggleDataSeries,omitempty"`
}

type LegendOnItemHover struct {
	HighlightDataSeries *bool `json:"highlightDataSeries,omitempty"`
}

type NoDataConfig struct {
	Text          *string            `json:"text,omitempty"`
	Align         *TitleAlign        `json:"align,omitempty"`         // Re-use TitleAlign
	VerticalAlign *string            `json:"verticalAlign,omitempty"` // 'top', 'middle', 'bottom'
	OffsetX       *int               `json:"offsetX,omitempty"`
	OffsetY       *int               `json:"offsetY,omitempty"`
	Style         *NoDataStyleConfig `json:"style,omitempty"`
}

type NoDataStyleConfig struct {
	Color      *string `json:"color,omitempty"`
	FontSize   *string `json:"fontSize,omitempty"`
	FontFamily *string `json:"fontFamily,omitempty"`
}

type StatesConfig struct {
	Normal *StateFilterConfig `json:"normal,omitempty"`
	Hover  *StateFilterConfig `json:"hover,omitempty"`
	Active *StateActiveConfig `json:"active,omitempty"`
}

type StateFilterConfig struct {
	Filter *StateFilter `json:"filter,omitempty"`
}

type StateFilter struct {
	Type  *string  `json:"type,omitempty"` // 'none', 'lighten', 'darken'
	Value *float64 `json:"value,omitempty"`
}

type StateActiveConfig struct {
	AllowMultipleDataPointsSelection *bool        `json:"allowMultipleDataPointsSelection,omitempty"`
	Filter                           *StateFilter `json:"filter,omitempty"`
}

type FillType string

const (
	FillTypeSolid    FillType = "solid"
	FillTypeGradient FillType = "gradient"
	FillTypePattern  FillType = "pattern"
	FillTypeImage    FillType = "image"
)

type FillConfig struct {
	Colors   []string      `json:"colors,omitempty"`
	Opacity  interface{}   `json:"opacity,omitempty"` // number or []number
	Type     interface{}   `json:"type,omitempty"`    // FillType or []FillType
	Gradient *FillGradient `json:"gradient,omitempty"`
	Image    *FillImage    `json:"image,omitempty"`
	Pattern  *FillPattern  `json:"pattern,omitempty"`
}

type FillGradient struct {
	Shade            *string     `json:"shade,omitempty"` // 'light', 'dark'
	Type             *string     `json:"type,omitempty"`  // 'horizontal', 'vertical', 'diagonal1', 'diagonal2'
	ShadeIntensity   *float64    `json:"shadeIntensity,omitempty"`
	GradientToColors []string    `json:"gradientToColors,omitempty"`
	InverseColors    *bool       `json:"inverseColors,omitempty"`
	OpacityFrom      *float64    `json:"opacityFrom,omitempty"`
	OpacityTo        *float64    `json:"opacityTo,omitempty"`
	Stops            []float64   `json:"stops,omitempty"`
	ColorStops       []ColorStop `json:"colorStops,omitempty"`
}

type ColorStop struct {
	Offset  *int     `json:"offset,omitempty"`
	Color   *string  `json:"color,omitempty"`
	Opacity *float64 `json:"opacity,omitempty"`
}

type FillImage struct {
	Src    interface{} `json:"src,omitempty"` // string or []string
	Width  *int        `json:"width,omitempty"`
	Height *int        `json:"height,omitempty"`
}

type FillPattern struct {
	Style       interface{} `json:"style,omitempty"` // string or []string ('verticalLines', 'horizontalLines', 'slantedLines', 'squares', 'circles')
	Width       *int        `json:"width,omitempty"`
	Height      *int        `json:"height,omitempty"`
	StrokeWidth *int        `json:"strokeWidth,omitempty"`
}

type Annotations struct {
	YAxis  []YAxisAnnotation `json:"yaxis,omitempty"`
	XAxis  []XAxisAnnotation `json:"xaxis,omitempty"`
	Points []PointAnnotation `json:"points,omitempty"`
	Texts  []TextAnnotation  `json:"texts,omitempty"`
	Images []ImageAnnotation `json:"images,omitempty"`
}

type AnnotationLabel struct {
	BorderColor  *string               `json:"borderColor,omitempty"`
	BorderWidth  *int                  `json:"borderWidth,omitempty"`
	BorderRadius *int                  `json:"borderRadius,omitempty"`
	Text         *string               `json:"text,omitempty"`
	TextAnchor   *string               `json:"textAnchor,omitempty"`  // 'start', 'middle', 'end'
	Position     *string               `json:"position,omitempty"`    // 'top', 'bottom', 'left', 'right'
	Orientation  *string               `json:"orientation,omitempty"` // 'horizontal', 'vertical'
	OffsetX      *int                  `json:"offsetX,omitempty"`
	OffsetY      *int                  `json:"offsetY,omitempty"`
	Style        *AnnotationLabelStyle `json:"style,omitempty"`
}

type AnnotationLabelStyle struct {
	Background *string  `json:"background,omitempty"`
	Color      *string  `json:"color,omitempty"`
	FontSize   *string  `json:"fontSize,omitempty"`
	FontWeight *string  `json:"fontWeight,omitempty"`
	FontFamily *string  `json:"fontFamily,omitempty"`
	CSSClass   *string  `json:"cssClass,omitempty"`
	Padding    *Padding `json:"padding,omitempty"`
}

type YAxisAnnotation struct {
	Y               *float64         `json:"y,omitempty"`
	Y2              *float64         `json:"y2,omitempty"`
	StrokeDashArray *int             `json:"strokeDashArray,omitempty"`
	FillColor       *string          `json:"fillColor,omitempty"`
	BorderColor     *string          `json:"borderColor,omitempty"`
	BorderWidth     *int             `json:"borderWidth,omitempty"`
	Opacity         *float64         `json:"opacity,omitempty"`
	OffsetX         *int             `json:"offsetX,omitempty"`
	OffsetY         *int             `json:"offsetY,omitempty"`
	YAxisIndex      *int             `json:"yAxisIndex,omitempty"`
	Label           *AnnotationLabel `json:"label,omitempty"`
}

type XAxisAnnotation struct {
	X               *float64         `json:"x,omitempty"` // Can be string for category or number for datetime
	X2              *float64         `json:"x2,omitempty"`
	StrokeDashArray *int             `json:"strokeDashArray,omitempty"`
	FillColor       *string          `json:"fillColor,omitempty"`
	BorderColor     *string          `json:"borderColor,omitempty"`
	BorderWidth     *int             `json:"borderWidth,omitempty"`
	Opacity         *float64         `json:"opacity,omitempty"`
	OffsetX         *int             `json:"offsetX,omitempty"`
	OffsetY         *int             `json:"offsetY,omitempty"`
	Label           *AnnotationLabel `json:"label,omitempty"`
}

type PointAnnotation struct {
	X           interface{}      `json:"x,omitempty"` // number or string
	Y           *float64         `json:"y,omitempty"`
	YAxisIndex  *int             `json:"yAxisIndex,omitempty"`
	SeriesIndex *int             `json:"seriesIndex,omitempty"`
	Marker      *PointMarker     `json:"marker,omitempty"`
	Label       *AnnotationLabel `json:"label,omitempty"`
	Image       *PointImage      `json:"image,omitempty"`
}

type PointMarker struct {
	Size        *int    `json:"size,omitempty"`
	FillColor   *string `json:"fillColor,omitempty"`
	StrokeColor *string `json:"strokeColor,omitempty"`
	StrokeWidth *int    `json:"strokeWidth,omitempty"`
	Shape       *string `json:"shape,omitempty"` // 'circle', 'square'
	Radius      *int    `json:"radius,omitempty"`
	OffsetX     *int    `json:"offsetX,omitempty"`
	OffsetY     *int    `json:"offsetY,omitempty"`
	CSSClass    *string `json:"cssClass,omitempty"`
}

type PointImage struct {
	Path    *string `json:"path,omitempty"`
	Width   *int    `json:"width,omitempty"`
	Height  *int    `json:"height,omitempty"`
	OffsetX *int    `json:"offsetX,omitempty"`
	OffsetY *int    `json:"offsetY,omitempty"`
}

type TextAnnotation struct {
	X               *float64 `json:"x,omitempty"`
	Y               *float64 `json:"y,omitempty"`
	Text            *string  `json:"text,omitempty"`
	TextAnchor      *string  `json:"textAnchor,omitempty"` // 'start', 'middle', 'end'
	ForeColor       *string  `json:"foreColor,omitempty"`
	FontSize        *string  `json:"fontSize,omitempty"`
	FontFamily      *string  `json:"fontFamily,omitempty"`
	FontWeight      *string  `json:"fontWeight,omitempty"`
	BackgroundColor *string  `json:"backgroundColor,omitempty"`
	BorderColor     *string  `json:"borderColor,omitempty"`
	BorderRadius    *int     `json:"borderRadius,omitempty"`
	BorderWidth     *int     `json:"borderWidth,omitempty"`
	PaddingLeft     *int     `json:"paddingLeft,omitempty"`
	PaddingRight    *int     `json:"paddingRight,omitempty"`
	PaddingTop      *int     `json:"paddingTop,omitempty"`
	PaddingBottom   *int     `json:"paddingBottom,omitempty"`
}

type ImageAnnotation struct {
	Path   *string  `json:"path,omitempty"`
	X      *float64 `json:"x,omitempty"`
	Y      *float64 `json:"y,omitempty"`
	Width  *int     `json:"width,omitempty"`
	Height *int     `json:"height,omitempty"`
}
