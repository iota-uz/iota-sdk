package entities

// Analytics represents system-wide metrics for superadmin dashboard
type Analytics struct {
	TenantCount  int
	UserCount    int
	DAU          int // Daily Active Users
	WAU          int // Weekly Active Users
	MAU          int // Monthly Active Users
	SessionCount int

	// Time series data for charts
	UserSignupsTimeSeries   []TimeSeriesDataPoint
	TenantSignupsTimeSeries []TimeSeriesDataPoint
}
