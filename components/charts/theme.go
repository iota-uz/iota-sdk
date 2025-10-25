package charts

// Common color constants used across charts
const (
	// Slate colors (text and borders)
	ColorSlateGray  = "#64748b"
	ColorSlateLight = "#e2e8f0"

	// Primary colors (data series)
	ColorBlue   = "#3B82F6"
	ColorGreen  = "#10B981"
	ColorOrange = "#F59E0B"
	ColorRed    = "#EF4444"
	ColorPurple = "#8B5CF6"
	ColorPink   = "#EC4899"
	ColorCyan   = "#06B6D4"
	ColorIndigo = "#4338ca"
	ColorGray   = "#6b7280"
)

// Common font size constants
const (
	FontSizeSmall  = "12px"
	FontSizeMedium = "14px"
	FontSizeLarge  = "16px"
)

// Common border radius values
const (
	BorderRadiusNone     = 0
	BorderRadiusStandard = 8
	BorderRadiusLarge    = 12
)

// Common donut chart sizes
const (
	DonutSizeStandard = "70%"
	DonutSizeLarge    = "80%"
	DonutSizeSmall    = "60%"
)

// DefaultChartColors returns the standard color palette used across charts
func DefaultChartColors() []string {
	return []string{
		ColorBlue,
		ColorGreen,
		ColorOrange,
		ColorRed,
		ColorPurple,
		ColorPink,
		ColorCyan,
	}
}
