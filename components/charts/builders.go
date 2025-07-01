package charts

// LineChartBuilder provides a fluent interface for building line charts
type LineChartBuilder struct {
	options LineChartOptions
}

// LineChartOptions represents type-safe options for line charts
type LineChartOptions struct {
	Height     string
	Series     []NamedSeries
	Categories []string
	YAxis      *YAxisConfig
	Colors     []string
	DataLabels bool
	ShowGrid   bool
	Stroke     *StrokeConfig
	Markers    *MarkersConfig
	Title      string
}

// NamedSeries represents a data series with a name
type NamedSeries struct {
	Name string
	Data []float64
}

// NewLineChart creates a new line chart builder
func NewLineChart() *LineChartBuilder {
	return &LineChartBuilder{
		options: LineChartOptions{
			Height:     "300px",
			DataLabels: false,
			ShowGrid:   true,
		},
	}
}

// WithSeries adds a data series to the line chart
func (b *LineChartBuilder) WithSeries(name string, data []float64) *LineChartBuilder {
	b.options.Series = append(b.options.Series, NamedSeries{Name: name, Data: data})
	return b
}

// WithCategories sets the X-axis categories
func (b *LineChartBuilder) WithCategories(categories []string) *LineChartBuilder {
	b.options.Categories = categories
	return b
}

// WithHeight sets the chart height
func (b *LineChartBuilder) WithHeight(height string) *LineChartBuilder {
	b.options.Height = height
	return b
}

// WithColors sets the chart colors
func (b *LineChartBuilder) WithColors(colors ...string) *LineChartBuilder {
	b.options.Colors = colors
	return b
}

// WithDataLabels enables or disables data labels
func (b *LineChartBuilder) WithDataLabels(enabled bool) *LineChartBuilder {
	b.options.DataLabels = enabled
	return b
}

// WithTitle sets the chart title
func (b *LineChartBuilder) WithTitle(title string) *LineChartBuilder {
	b.options.Title = title
	return b
}

// Build converts the builder to standard ChartOptions
func (b *LineChartBuilder) Build() ChartOptions {
	series := make([]Series, len(b.options.Series))
	for i, s := range b.options.Series {
		data := make([]interface{}, len(s.Data))
		for j, v := range s.Data {
			data[j] = v
		}
		series[i] = Series{Name: s.Name, Data: data}
	}

	opts := ChartOptions{
		Chart: ChartConfig{
			Type:   LineChartType,
			Height: b.options.Height,
			Toolbar: Toolbar{
				Show: false,
			},
		},
		Series: series,
		XAxis: XAxisConfig{
			Categories: b.options.Categories,
		},
		Colors: b.options.Colors,
		DataLabels: &DataLabels{
			Enabled: b.options.DataLabels,
		},
	}

	if b.options.YAxis != nil {
		opts.YAxis = []YAxisConfig{*b.options.YAxis}
	}

	if b.options.Title != "" {
		opts.Title = &TitleConfig{
			Text: &b.options.Title,
		}
	}

	if b.options.Stroke != nil {
		opts.Stroke = b.options.Stroke
	}

	if b.options.Markers != nil {
		opts.Markers = b.options.Markers
	}

	return opts
}

// BarChartBuilder provides a fluent interface for building bar charts
type BarChartBuilder struct {
	options BarChartOptions
}

// BarChartOptions represents type-safe options for bar charts
type BarChartOptions struct {
	Height       string
	Series       []NamedSeries
	Categories   []string
	Colors       []string
	DataLabels   bool
	Horizontal   bool
	Stacked      bool
	BorderRadius int
	ColumnWidth  string
	Title        string
}

// NewBarChart creates a new bar chart builder
func NewBarChart() *BarChartBuilder {
	return &BarChartBuilder{
		options: BarChartOptions{
			Height:       "300px",
			DataLabels:   false,
			Horizontal:   false,
			Stacked:      false,
			BorderRadius: 0,
			ColumnWidth:  "70%",
		},
	}
}

// WithSeries adds a data series to the bar chart
func (b *BarChartBuilder) WithSeries(name string, data []float64) *BarChartBuilder {
	b.options.Series = append(b.options.Series, NamedSeries{Name: name, Data: data})
	return b
}

// WithCategories sets the X-axis categories
func (b *BarChartBuilder) WithCategories(categories []string) *BarChartBuilder {
	b.options.Categories = categories
	return b
}

// WithHeight sets the chart height
func (b *BarChartBuilder) WithHeight(height string) *BarChartBuilder {
	b.options.Height = height
	return b
}

// WithColors sets the chart colors
func (b *BarChartBuilder) WithColors(colors ...string) *BarChartBuilder {
	b.options.Colors = colors
	return b
}

// AsHorizontal makes the bar chart horizontal
func (b *BarChartBuilder) AsHorizontal() *BarChartBuilder {
	b.options.Horizontal = true
	return b
}

// AsStacked makes the bar chart stacked
func (b *BarChartBuilder) AsStacked() *BarChartBuilder {
	b.options.Stacked = true
	return b
}

// WithBorderRadius sets the border radius for bars
func (b *BarChartBuilder) WithBorderRadius(radius int) *BarChartBuilder {
	b.options.BorderRadius = radius
	return b
}

// Build converts the builder to standard ChartOptions
func (b *BarChartBuilder) Build() ChartOptions {
	series := make([]Series, len(b.options.Series))
	for i, s := range b.options.Series {
		data := make([]interface{}, len(s.Data))
		for j, v := range s.Data {
			data[j] = v
		}
		series[i] = Series{Name: s.Name, Data: data}
	}

	opts := ChartOptions{
		Chart: ChartConfig{
			Type:    BarChartType,
			Height:  b.options.Height,
			Stacked: b.options.Stacked,
			Toolbar: Toolbar{
				Show: false,
			},
		},
		Series: series,
		XAxis: XAxisConfig{
			Categories: b.options.Categories,
		},
		Colors: b.options.Colors,
		DataLabels: &DataLabels{
			Enabled: b.options.DataLabels,
		},
		PlotOptions: &PlotOptions{
			Bar: &BarConfig{
				BorderRadius: b.options.BorderRadius,
				ColumnWidth:  b.options.ColumnWidth,
				DataLabels: BarLabels{
					Position: "top",
				},
			},
		},
	}

	if b.options.Horizontal {
		opts.PlotOptions.Bar.Horizontal = &b.options.Horizontal
	}

	if b.options.Title != "" {
		opts.Title = &TitleConfig{
			Text: &b.options.Title,
		}
	}

	return opts
}

// PieChartBuilder provides a fluent interface for building pie charts
type PieChartBuilder struct {
	data    []float64
	labels  []string
	options PieChartOptions
}

// PieChartOptions represents type-safe options for pie charts
type PieChartOptions struct {
	Height     string
	Colors     []string
	DataLabels bool
	ShowLegend bool
	Title      string
	DonutSize  string // For converting to donut chart
}

// NewPieChart creates a new pie chart builder with required data and labels
func NewPieChart(data []float64, labels []string) *PieChartBuilder {
	if len(data) != len(labels) {
		panic("data and labels must have the same length")
	}
	return &PieChartBuilder{
		data:   data,
		labels: labels,
		options: PieChartOptions{
			Height:     "300px",
			DataLabels: true,
			ShowLegend: true,
		},
	}
}

// WithHeight sets the chart height
func (b *PieChartBuilder) WithHeight(height string) *PieChartBuilder {
	b.options.Height = height
	return b
}

// WithColors sets the chart colors
func (b *PieChartBuilder) WithColors(colors ...string) *PieChartBuilder {
	b.options.Colors = colors
	return b
}

// WithDataLabels enables or disables data labels
func (b *PieChartBuilder) WithDataLabels(enabled bool) *PieChartBuilder {
	b.options.DataLabels = enabled
	return b
}

// WithLegend enables or disables the legend
func (b *PieChartBuilder) WithLegend(enabled bool) *PieChartBuilder {
	b.options.ShowLegend = enabled
	return b
}

// AsDonut converts the pie chart to a donut chart with the specified size
func (b *PieChartBuilder) AsDonut(size string) *PieChartBuilder {
	b.options.DonutSize = size
	return b
}

// Build converts the builder to standard ChartOptions
func (b *PieChartBuilder) Build() ChartOptions {
	chartType := PieChartType
	if b.options.DonutSize != "" {
		chartType = DonutChartType
	}

	series := make([]interface{}, len(b.data))
	for i, v := range b.data {
		series[i] = v
	}

	opts := ChartOptions{
		Chart: ChartConfig{
			Type:   chartType,
			Height: b.options.Height,
			Toolbar: Toolbar{
				Show: false,
			},
		},
		Series: series,
		Labels: b.labels,
		Colors: b.options.Colors,
		DataLabels: &DataLabels{
			Enabled: b.options.DataLabels,
		},
	}

	if !b.options.ShowLegend {
		opts.Legend = &LegendConfig{
			Show: &b.options.ShowLegend,
		}
	}

	if b.options.Title != "" {
		opts.Title = &TitleConfig{
			Text: &b.options.Title,
		}
	}

	if b.options.DonutSize != "" {
		opts.PlotOptions = &PlotOptions{
			Donut: &PieDonutConfig{
				Size: &b.options.DonutSize,
			},
		}
	}

	return opts
}

// RadialBarChartBuilder provides a fluent interface for building radial bar charts
type RadialBarChartBuilder struct {
	data    []float64
	labels  []string
	options RadialBarChartOptions
}

// RadialBarChartOptions represents type-safe options for radial bar charts
type RadialBarChartOptions struct {
	Height           string
	Colors           []string
	StartAngle       int
	EndAngle         int
	Hollow           string
	Track            bool
	Size             string
	TrackStrokeWidth string
	DataLabels       bool
	Title            string
}

// NewRadialBarChart creates a new radial bar chart builder
func NewRadialBarChart(data []float64, labels []string) *RadialBarChartBuilder {
	return &RadialBarChartBuilder{
		data:   data,
		labels: labels,
		options: RadialBarChartOptions{
			Height:           "300px",
			StartAngle:       -90,
			EndAngle:         90,
			Hollow:           "50%",
			Track:            true,
			Size:             "85%",
			TrackStrokeWidth: "15px",
			DataLabels:       true,
		},
	}
}

// WithHeight sets the chart height
func (b *RadialBarChartBuilder) WithHeight(height string) *RadialBarChartBuilder {
	b.options.Height = height
	return b
}

// WithColors sets the chart colors
func (b *RadialBarChartBuilder) WithColors(colors ...string) *RadialBarChartBuilder {
	b.options.Colors = colors
	return b
}

// WithAngles sets the start and end angles
func (b *RadialBarChartBuilder) WithAngles(start, end int) *RadialBarChartBuilder {
	b.options.StartAngle = start
	b.options.EndAngle = end
	return b
}

// WithSize sets the radial bar size (thickness)
func (b *RadialBarChartBuilder) WithSize(size string) *RadialBarChartBuilder {
	b.options.Size = size
	return b
}

// WithTrackStrokeWidth sets the track stroke width (bar thickness)
func (b *RadialBarChartBuilder) WithTrackStrokeWidth(width string) *RadialBarChartBuilder {
	b.options.TrackStrokeWidth = width
	return b
}

// Build converts the builder to standard ChartOptions
func (b *RadialBarChartBuilder) Build() ChartOptions {
	series := make([]interface{}, len(b.data))
	for i, v := range b.data {
		series[i] = v
	}

	opts := ChartOptions{
		Chart: ChartConfig{
			Type:   RadialBarChartType,
			Height: b.options.Height,
			Toolbar: Toolbar{
				Show: false,
			},
		},
		Series: series,
		Labels: b.labels,
		Colors: b.options.Colors,
		DataLabels: &DataLabels{
			Enabled: b.options.DataLabels,
		},
		PlotOptions: &PlotOptions{
			RadialBar: &RadialBarConfig{
				Size:       &b.options.Size,
				StartAngle: &b.options.StartAngle,
				EndAngle:   &b.options.EndAngle,
				Hollow: &RadialBarHollow{
					Size: &b.options.Hollow,
				},
				Track: &RadialBarTrack{
					Show:        &b.options.Track,
					StrokeWidth: &b.options.TrackStrokeWidth,
				},
			},
		},
	}

	if b.options.Title != "" {
		opts.Title = &TitleConfig{
			Text: &b.options.Title,
		}
	}

	return opts
}

// AreaChartBuilder provides a fluent interface for building area charts
type AreaChartBuilder struct {
	options AreaChartOptions
}

// AreaChartOptions represents type-safe options for area charts
type AreaChartOptions struct {
	Height     string
	Series     []NamedSeries
	Categories []string
	Colors     []string
	DataLabels bool
	Stacked    bool
	Title      string
}

// NewAreaChart creates a new area chart builder
func NewAreaChart() *AreaChartBuilder {
	return &AreaChartBuilder{
		options: AreaChartOptions{
			Height:     "300px",
			DataLabels: false,
			Stacked:    false,
		},
	}
}

// WithSeries adds a data series to the area chart
func (b *AreaChartBuilder) WithSeries(name string, data []float64) *AreaChartBuilder {
	b.options.Series = append(b.options.Series, NamedSeries{Name: name, Data: data})
	return b
}

// WithCategories sets the X-axis categories
func (b *AreaChartBuilder) WithCategories(categories []string) *AreaChartBuilder {
	b.options.Categories = categories
	return b
}

// WithHeight sets the chart height
func (b *AreaChartBuilder) WithHeight(height string) *AreaChartBuilder {
	b.options.Height = height
	return b
}

// WithColors sets the chart colors
func (b *AreaChartBuilder) WithColors(colors ...string) *AreaChartBuilder {
	b.options.Colors = colors
	return b
}

// AsStacked makes the area chart stacked
func (b *AreaChartBuilder) AsStacked() *AreaChartBuilder {
	b.options.Stacked = true
	return b
}

// WithTitle sets the chart title
func (b *AreaChartBuilder) WithTitle(title string) *AreaChartBuilder {
	b.options.Title = title
	return b
}

// Build converts the builder to standard ChartOptions
func (b *AreaChartBuilder) Build() ChartOptions {
	series := make([]Series, len(b.options.Series))
	for i, s := range b.options.Series {
		data := make([]interface{}, len(s.Data))
		for j, v := range s.Data {
			data[j] = v
		}
		series[i] = Series{Name: s.Name, Data: data}
	}

	opts := ChartOptions{
		Chart: ChartConfig{
			Type:    AreaChartType,
			Height:  b.options.Height,
			Stacked: b.options.Stacked,
			Toolbar: Toolbar{
				Show: false,
			},
		},
		Series: series,
		XAxis: XAxisConfig{
			Categories: b.options.Categories,
		},
		Colors: b.options.Colors,
		DataLabels: &DataLabels{
			Enabled: b.options.DataLabels,
		},
	}

	if b.options.Title != "" {
		opts.Title = &TitleConfig{
			Text: &b.options.Title,
		}
	}

	return opts
}
