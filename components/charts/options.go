package charts

// ChartOption is a function that modifies chart options
type ChartOption func(*ChartOptions)

// WithChartHeight sets the chart height
func WithChartHeight(height string) ChartOption {
	return func(opts *ChartOptions) {
		opts.Chart.Height = height
	}
}

// WithChartColors sets the chart colors
func WithChartColors(colors ...string) ChartOption {
	return func(opts *ChartOptions) {
		opts.Colors = colors
	}
}

// WithDataLabelsEnabled enables or disables data labels
func WithDataLabelsEnabled(enabled bool) ChartOption {
	return func(opts *ChartOptions) {
		if opts.DataLabels == nil {
			opts.DataLabels = &DataLabels{}
		}
		opts.DataLabels.Enabled = enabled
	}
}

// WithLegendHidden hides the legend
func WithLegendHidden() ChartOption {
	return func(opts *ChartOptions) {
		hidden := false
		opts.Legend = &LegendConfig{
			Show: &hidden,
		}
	}
}

// WithTitle sets the chart title
func WithChartTitle(title string) ChartOption {
	return func(opts *ChartOptions) {
		opts.Title = &TitleConfig{
			Text: &title,
		}
	}
}

// WithGridDisabled disables the chart grid
func WithGridDisabled() ChartOption {
	return func(opts *ChartOptions) {
		opts.Grid = &GridConfig{
			BorderColor: "transparent",
		}
	}
}

// WithStroke sets stroke configuration
func WithStroke(width int, curve string) ChartOption {
	return func(opts *ChartOptions) {
		opts.Stroke = &StrokeConfig{
			Width: []int{width},
			Curve: &curve,
		}
	}
}

// WithTheme sets the chart theme
func WithTheme(mode string, palette string) ChartOption {
	return func(opts *ChartOptions) {
		var themeMode ThemeMode
		switch mode {
		case "light":
			themeMode = ThemeModeLight
		case "dark":
			themeMode = ThemeModeDark
		}

		var themePalette ThemePalette
		switch palette {
		case "palette1":
			themePalette = ThemePalette1
		case "palette2":
			themePalette = ThemePalette2
		}

		opts.Theme = &ThemeConfig{
			Mode:    &themeMode,
			Palette: &themePalette,
		}
	}
}

// ApplyOptions applies functional options to chart options
func ApplyOptions(baseOpts ChartOptions, options ...ChartOption) ChartOptions {
	opts := baseOpts
	for _, option := range options {
		option(&opts)
	}
	return opts
}

// Common preset configurations

// DarkThemeOption applies dark theme settings
func DarkThemeOption() ChartOption {
	return WithTheme("dark", "palette1")
}

// MinimalOption applies minimal styling (no toolbar, grid, legend)
func MinimalOption() ChartOption {
	return func(opts *ChartOptions) {
		opts.Chart.Toolbar.Show = false
		WithGridDisabled()(opts)
		WithLegendHidden()(opts)
	}
}

// ResponsiveOption makes the chart responsive
func ResponsiveOption() ChartOption {
	return func(opts *ChartOptions) {
		opts.Chart.Height = "100%"
	}
}
