package services

// Default pagination and limit constants for superadmin services
const (
	// DefaultPageSize is the default number of items per page
	DefaultPageSize = 20

	// MaxPageSize is the maximum allowed page size to prevent excessive queries
	MaxPageSize = 1000

	// DefaultLargeLimit is used for export operations that need to fetch many records
	DefaultLargeLimit = 10000

	// DefaultDateRangeDays is the default number of days to look back for analytics
	DefaultDateRangeDays = -30
)
