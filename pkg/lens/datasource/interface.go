package datasource

import (
	"context"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens"
)

// DataSource defines the interface for data sources
type DataSource interface {
	// Query executes a query and returns the result
	Query(ctx context.Context, query Query) (*QueryResult, error)

	// TestConnection tests if the datasource is reachable
	TestConnection(ctx context.Context) error

	// GetMetadata returns datasource metadata
	GetMetadata() DataSourceMetadata

	// ValidateQuery validates a query before execution
	ValidateQuery(query Query) error

	// Close closes the datasource connection
	Close() error
}

// Query represents a query to be executed
type Query struct {
	ID            string                 // Unique query identifier
	Raw           string                 // Raw query string
	Variables     map[string]interface{} // Query variables
	TimeRange     lens.TimeRange         // Time range for the query
	RefreshRate   time.Duration          // How often to refresh
	MaxDataPoints int                    // Maximum data points to return
	Format        QueryFormat            // Expected result format
}

// QueryFormat represents the expected query result format
type QueryFormat string

const (
	FormatTimeSeries QueryFormat = "timeseries"
	FormatTable      QueryFormat = "table"
	FormatLogs       QueryFormat = "logs"
	FormatMetrics    QueryFormat = "metrics"
	FormatTrace      QueryFormat = "trace"
)

// QueryResult represents the result of a query execution
type QueryResult struct {
	Data     []DataPoint    // The actual data points
	Metadata ResultMetadata // Query execution metadata
	Error    *QueryError    // Error information if query failed
	Columns  []ColumnInfo   // Column information for table format
	ExecTime time.Duration  // Query execution time
}

// DataPoint represents a single data point
type DataPoint struct {
	Timestamp time.Time              // When the data point was recorded
	Value     interface{}            // The actual value (number, string, etc.)
	Labels    map[string]string      // Associated labels/tags
	Fields    map[string]interface{} // Additional fields
}

// ColumnInfo describes a column in table format results
type ColumnInfo struct {
	Name string   // Column name
	Type DataType // Column data type
	Unit string   // Unit of measurement (optional)
}

// DataType represents the type of data in a column
type DataType string

const (
	DataTypeString    DataType = "string"
	DataTypeNumber    DataType = "number"
	DataTypeBoolean   DataType = "boolean"
	DataTypeTimestamp DataType = "timestamp"
)

// ResultMetadata contains metadata about query execution
type ResultMetadata struct {
	QueryID        string        // Original query ID
	ExecutedAt     time.Time     // When query was executed
	RowCount       int           // Number of rows returned
	DataSource     string        // Data source identifier
	ProcessingTime time.Duration // Time spent processing
}

// QueryError represents an error that occurred during query execution
type QueryError struct {
	Code    ErrorCode // Error code
	Message string    // Human-readable error message
	Details string    // Additional error details
	Query   string    // The query that caused the error
}

func (e *QueryError) Error() string {
	return e.Message
}

// ErrorCode represents different types of query errors
type ErrorCode string

const (
	ErrorCodeSyntax     ErrorCode = "SYNTAX_ERROR"
	ErrorCodeTimeout    ErrorCode = "TIMEOUT"
	ErrorCodeConnection ErrorCode = "CONNECTION_ERROR"
	ErrorCodePermission ErrorCode = "PERMISSION_DENIED"
	ErrorCodeNotFound   ErrorCode = "NOT_FOUND"
	ErrorCodeRateLimit  ErrorCode = "RATE_LIMIT"
	ErrorCodeInternal   ErrorCode = "INTERNAL_ERROR"
)

// DataSourceMetadata contains information about a data source
type DataSourceMetadata struct {
	ID           string            // Unique identifier
	Name         string            // Display name
	Type         DataSourceType    // Type of data source
	Version      string            // Version information
	Description  string            // Description
	Capabilities []Capability      // Supported capabilities
	Config       map[string]string // Configuration options
}

// DataSourceType represents different types of data sources
type DataSourceType string

const (
	TypePostgreSQL DataSourceType = "postgresql"
	TypeMongoDB    DataSourceType = "mongodb"
	TypeGraphQL    DataSourceType = "graphql"
	TypeREST       DataSourceType = "rest"
	TypeCSV        DataSourceType = "csv"
	TypeJSON       DataSourceType = "json"
)

// Capability represents what a data source can do
type Capability string

const (
	CapabilityQuery       Capability = "query"
	CapabilityAnnotations Capability = "annotations"
	CapabilityVariables   Capability = "variables"
	CapabilityMetrics     Capability = "metrics"
	CapabilityLogs        Capability = "logs"
	CapabilityTraces      Capability = "traces"
)

// QueryBuilder provides a fluent interface for building queries
type QueryBuilder interface {
	WithQuery(query string) QueryBuilder
	WithVariable(key string, value interface{}) QueryBuilder
	WithTimeRange(start, end time.Time) QueryBuilder
	WithRefreshRate(rate time.Duration) QueryBuilder
	WithMaxDataPoints(maxPoints int) QueryBuilder
	WithFormat(format QueryFormat) QueryBuilder
	Build() Query
}

// DataSourceConfig represents configuration for creating a data source
type DataSourceConfig struct {
	Type       DataSourceType         // Type of data source
	Name       string                 // Display name
	URL        string                 // Connection URL
	Timeout    time.Duration          // Query timeout
	MaxRetries int                    // Maximum retry attempts
	Options    map[string]interface{} // Additional options
}
