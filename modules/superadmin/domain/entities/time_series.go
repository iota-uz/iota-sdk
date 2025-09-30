package entities

import "time"

// TimeSeriesDataPoint represents a single data point in a time series
type TimeSeriesDataPoint struct {
	Date  time.Time
	Count int
}
