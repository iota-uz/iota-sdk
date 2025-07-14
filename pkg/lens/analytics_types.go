package lens

import (
	"time"
)

// AnalyticsCategory represents a category with its metrics
type AnalyticsCategory struct {
	// Name is the display name of the category
	Name string
	// Key is the translation key for the category name
	Key string
	// Value is the numeric value (revenue or count)
	Value float64
	// Percentage is the percentage of total
	Percentage string
	// Color is the hex color for visualization
	Color string
	// FormattedValue is the pre-formatted display value
	FormattedValue string
}

// AnalyticsDetailConfig represents configuration for analytics detail page
type AnalyticsDetailConfig struct {
	// Date is the date for which analytics are shown
	Date time.Time
	// DataType is either "revenue" or "count"
	DataType string
	// Categories is the list of categories with their data
	Categories []AnalyticsCategory
	// Title is the page title
	Title string
	// BaseURL is the base URL for navigation (e.g., "/crm/analytics")
	BaseURL string
	// BackURL is the URL to return to (e.g., "/crm/summary-report")
	BackURL string
	// Icons for different data types
	RevenueIcon string
	CountIcon   string
}

// AnalyticsPieChartOptions prepares pie chart options for analytics categories
func AnalyticsPieChartOptions(categories []AnalyticsCategory, dataType string) map[string]interface{} {
	if categories == nil || len(categories) == 0 {
		return map[string]interface{}{}
	}

	// Extract data for pie chart
	var series []float64
	var labels []string
	var colors []string

	for _, category := range categories {
		if category.Value > 0 {
			series = append(series, category.Value)
			labels = append(labels, category.Name)
			colors = append(colors, category.Color)
		}
	}

	// If no data, return empty options
	if len(series) == 0 {
		return map[string]interface{}{}
	}

	return map[string]interface{}{
		"series": series,
		"labels": labels,
		"colors": colors,
	}
}
