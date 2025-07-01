package postgres

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/datasource"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgreSQLDataSource implements DataSource for PostgreSQL databases
type PostgreSQLDataSource struct {
	pool     *pgxpool.Pool
	metadata datasource.DataSourceMetadata
	config   Config
}

// Config contains PostgreSQL-specific configuration
type Config struct {
	ConnectionString string        // PostgreSQL connection string
	MaxConnections   int32         // Maximum number of connections
	MinConnections   int32         // Minimum number of connections
	MaxConnLifetime  time.Duration // Maximum connection lifetime
	MaxConnIdleTime  time.Duration // Maximum connection idle time
	QueryTimeout     time.Duration // Default query timeout
}

// NewPostgreSQLDataSource creates a new PostgreSQL data source
func NewPostgreSQLDataSource(config Config) (*PostgreSQLDataSource, error) {
	if config.ConnectionString == "" {
		return nil, fmt.Errorf("connection string is required")
	}

	// Parse connection string and create pool config
	poolConfig, err := pgxpool.ParseConfig(config.ConnectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}

	// Apply configuration
	if config.MaxConnections > 0 {
		poolConfig.MaxConns = config.MaxConnections
	}
	if config.MinConnections > 0 {
		poolConfig.MinConns = config.MinConnections
	}
	if config.MaxConnLifetime > 0 {
		poolConfig.MaxConnLifetime = config.MaxConnLifetime
	}
	if config.MaxConnIdleTime > 0 {
		poolConfig.MaxConnIdleTime = config.MaxConnIdleTime
	}

	// Create connection pool
	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Set default timeout
	if config.QueryTimeout == 0 {
		config.QueryTimeout = 30 * time.Second
	}

	ds := &PostgreSQLDataSource{
		pool:   pool,
		config: config,
		metadata: datasource.DataSourceMetadata{
			Type:        datasource.TypePostgreSQL,
			Name:        "PostgreSQL",
			Version:     "1.0.0",
			Description: "PostgreSQL database data source using pgxpool",
			Capabilities: []datasource.Capability{
				datasource.CapabilityQuery,
				datasource.CapabilityMetrics,
			},
		},
	}

	return ds, nil
}

// Query executes a query and returns the result
func (ds *PostgreSQLDataSource) Query(ctx context.Context, query datasource.Query) (*datasource.QueryResult, error) {
	// Set timeout
	queryTimeout := ds.config.QueryTimeout
	if query.RefreshRate > 0 && query.RefreshRate < queryTimeout {
		queryTimeout = query.RefreshRate
	}

	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	// Interpolate variables in query
	interpolatedQuery, err := ds.interpolateVariables(query.Raw, query.Variables)
	if err != nil {
		return nil, &datasource.QueryError{
			Code:    datasource.ErrorCodeSyntax,
			Message: "Failed to interpolate variables",
			Details: err.Error(),
			Query:   query.Raw,
		}
	}

	// Execute query based on format
	switch query.Format {
	case datasource.FormatTable:
		return ds.executeTableQuery(queryCtx, interpolatedQuery, query)
	case datasource.FormatTimeSeries:
		return ds.executeTimeSeriesQuery(queryCtx, interpolatedQuery, query)
	case datasource.FormatLogs:
		return ds.executeTableQuery(queryCtx, interpolatedQuery, query)
	case datasource.FormatMetrics:
		return ds.executeTableQuery(queryCtx, interpolatedQuery, query)
	case datasource.FormatTrace:
		return ds.executeTableQuery(queryCtx, interpolatedQuery, query)
	default:
		return ds.executeTableQuery(queryCtx, interpolatedQuery, query)
	}
}

// executeTableQuery executes a query and returns table format results
func (ds *PostgreSQLDataSource) executeTableQuery(ctx context.Context, query string, originalQuery datasource.Query) (*datasource.QueryResult, error) {
	start := time.Now()

	// Apply row limit
	if originalQuery.MaxDataPoints > 0 {
		query = fmt.Sprintf("%s LIMIT %d", query, originalQuery.MaxDataPoints)
	}

	rows, err := ds.pool.Query(ctx, query)
	if err != nil {
		return nil, ds.handleQueryError(err, query)
	}
	defer rows.Close()

	// Get column information
	fieldDescriptions := rows.FieldDescriptions()
	columns := make([]datasource.ColumnInfo, len(fieldDescriptions))
	for i, desc := range fieldDescriptions {
		columns[i] = datasource.ColumnInfo{
			Name: desc.Name,
			Type: ds.pgTypeToDataType(desc.DataTypeOID),
		}
	}

	// Read all rows
	var dataPoints []datasource.DataPoint
	rowCount := 0

	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, ds.handleQueryError(err, query)
		}

		// Convert row to data point
		fields := make(map[string]interface{})
		for i, value := range values {
			columnName := columns[i].Name
			fields[columnName] = value
		}

		dataPoint := datasource.DataPoint{
			Timestamp: time.Now(), // Use current time for non-time series data
			Fields:    fields,
			Labels:    make(map[string]string),
		}

		// If first column looks like a timestamp, use it
		if len(values) > 0 {
			if timestamp, ok := values[0].(time.Time); ok {
				dataPoint.Timestamp = timestamp
			}
		}

		dataPoints = append(dataPoints, dataPoint)
		rowCount++
	}

	if err := rows.Err(); err != nil {
		return nil, ds.handleQueryError(err, query)
	}

	return &datasource.QueryResult{
		Data:    dataPoints,
		Columns: columns,
		Metadata: datasource.ResultMetadata{
			QueryID:        originalQuery.ID,
			ExecutedAt:     start,
			RowCount:       rowCount,
			DataSource:     string(ds.metadata.Type),
			ProcessingTime: time.Since(start),
		},
		ExecTime: time.Since(start),
	}, nil
}

// executeTimeSeriesQuery executes a query and returns time series format results
func (ds *PostgreSQLDataSource) executeTimeSeriesQuery(ctx context.Context, query string, originalQuery datasource.Query) (*datasource.QueryResult, error) {
	start := time.Now()

	// Apply row limit
	if originalQuery.MaxDataPoints > 0 {
		query = fmt.Sprintf("%s LIMIT %d", query, originalQuery.MaxDataPoints)
	}

	rows, err := ds.pool.Query(ctx, query)
	if err != nil {
		return nil, ds.handleQueryError(err, query)
	}
	defer rows.Close()

	// Get column information
	fieldDescriptions := rows.FieldDescriptions()
	columns := make([]datasource.ColumnInfo, len(fieldDescriptions))
	for i, desc := range fieldDescriptions {
		columns[i] = datasource.ColumnInfo{
			Name: desc.Name,
			Type: ds.pgTypeToDataType(desc.DataTypeOID),
		}
	}

	// Expect at least timestamp and value columns
	if len(columns) < 2 {
		return nil, &datasource.QueryError{
			Code:    datasource.ErrorCodeSyntax,
			Message: "Time series query must return at least timestamp and value columns",
			Query:   query,
		}
	}

	// Read all rows
	var dataPoints []datasource.DataPoint
	rowCount := 0

	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, ds.handleQueryError(err, query)
		}

		if len(values) < 2 {
			continue
		}

		// First column should be timestamp
		var timestamp time.Time
		switch t := values[0].(type) {
		case time.Time:
			timestamp = t
		case string:
			if parsed, err := time.Parse(time.RFC3339, t); err == nil {
				timestamp = parsed
			} else {
				timestamp = time.Now()
			}
		default:
			timestamp = time.Now()
		}

		// Second column should be the main value
		var value interface{} = values[1]

		// Additional columns become fields
		fields := make(map[string]interface{})
		labels := make(map[string]string)

		for i := 2; i < len(values); i++ {
			columnName := columns[i].Name
			columnValue := values[i]

			// String values go to labels, others to fields
			if strValue, ok := columnValue.(string); ok {
				labels[columnName] = strValue
			} else {
				fields[columnName] = columnValue
			}
		}

		dataPoint := datasource.DataPoint{
			Timestamp: timestamp,
			Value:     value,
			Fields:    fields,
			Labels:    labels,
		}

		dataPoints = append(dataPoints, dataPoint)
		rowCount++
	}

	if err := rows.Err(); err != nil {
		return nil, ds.handleQueryError(err, query)
	}

	return &datasource.QueryResult{
		Data:    dataPoints,
		Columns: columns,
		Metadata: datasource.ResultMetadata{
			QueryID:        originalQuery.ID,
			ExecutedAt:     start,
			RowCount:       rowCount,
			DataSource:     string(ds.metadata.Type),
			ProcessingTime: time.Since(start),
		},
		ExecTime: time.Since(start),
	}, nil
}

// TestConnection tests if the datasource is reachable
func (ds *PostgreSQLDataSource) TestConnection(ctx context.Context) error {
	testCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return ds.pool.Ping(testCtx)
}

// GetMetadata returns datasource metadata
func (ds *PostgreSQLDataSource) GetMetadata() datasource.DataSourceMetadata {
	return ds.metadata
}

// ValidateQuery validates a query before execution
func (ds *PostgreSQLDataSource) ValidateQuery(query datasource.Query) error {
	if query.Raw == "" {
		return fmt.Errorf("query cannot be empty")
	}

	// Basic SQL injection protection
	lowerQuery := strings.ToLower(strings.TrimSpace(query.Raw))

	// Allow only SELECT statements
	if !strings.HasPrefix(lowerQuery, "select") {
		return fmt.Errorf("only SELECT queries are allowed")
	}

	// Prevent dangerous operations
	dangerousKeywords := []string{
		"drop", "delete", "insert", "update", "create", "alter",
		"truncate", "grant", "revoke", "exec", "execute",
	}

	for _, keyword := range dangerousKeywords {
		if strings.Contains(lowerQuery, keyword) {
			return fmt.Errorf("query contains dangerous keyword: %s", keyword)
		}
	}

	return nil
}

// Close closes the datasource connection
func (ds *PostgreSQLDataSource) Close() error {
	if ds.pool != nil {
		ds.pool.Close()
	}
	return nil
}

// Helper methods

// interpolateVariables replaces variables in the query string
func (ds *PostgreSQLDataSource) interpolateVariables(query string, variables map[string]interface{}) (string, error) {
	result := query

	for key, value := range variables {
		placeholder := fmt.Sprintf("$%s", key)

		// Convert value to string safely
		var valueStr string
		switch v := value.(type) {
		case string:
			valueStr = fmt.Sprintf("'%s'", strings.ReplaceAll(v, "'", "''"))
		case int, int32, int64, float32, float64:
			valueStr = fmt.Sprintf("%v", v)
		case bool:
			valueStr = strconv.FormatBool(v)
		case time.Time:
			valueStr = fmt.Sprintf("'%s'", v.Format(time.RFC3339))
		case lens.TimeRange:
			// Handle time range specially - replace with start time for now
			valueStr = fmt.Sprintf("'%s'", v.Start.Format(time.RFC3339))
		default:
			valueStr = fmt.Sprintf("'%v'", v)
		}

		result = strings.ReplaceAll(result, placeholder, valueStr)
	}

	return result, nil
}

// pgTypeToDataType converts PostgreSQL type OIDs to DataType
func (ds *PostgreSQLDataSource) pgTypeToDataType(oid uint32) datasource.DataType {
	// PostgreSQL type OIDs - these are constants from pgx
	switch oid {
	case 25, 1043, 1042: // text, varchar, bpchar
		return datasource.DataTypeString
	case 21, 23, 20, 700, 701, 1700: // int2, int4, int8, float4, float8, numeric
		return datasource.DataTypeNumber
	case 16: // bool
		return datasource.DataTypeBoolean
	case 1114, 1184, 1082, 1083: // timestamp, timestamptz, date, time
		return datasource.DataTypeTimestamp
	default:
		return datasource.DataTypeString
	}
}

// handleQueryError converts PostgreSQL errors to QueryError
func (ds *PostgreSQLDataSource) handleQueryError(err error, query string) error {
	if err == nil {
		return nil
	}

	var code datasource.ErrorCode
	message := err.Error()

	switch {
	case strings.Contains(message, "timeout"):
		code = datasource.ErrorCodeTimeout
	case strings.Contains(message, "connection"):
		code = datasource.ErrorCodeConnection
	case strings.Contains(message, "syntax"):
		code = datasource.ErrorCodeSyntax
	case strings.Contains(message, "permission"):
		code = datasource.ErrorCodePermission
	default:
		code = datasource.ErrorCodeInternal
	}

	return &datasource.QueryError{
		Code:    code,
		Message: message,
		Query:   query,
	}
}

// Factory creates PostgreSQL data sources
type Factory struct{}

// NewFactory creates a new PostgreSQL data source factory
func NewFactory() *Factory {
	return &Factory{}
}

// Create creates a PostgreSQL data source from configuration
func (f *Factory) Create(config datasource.DataSourceConfig) (datasource.DataSource, error) {
	if config.Type != datasource.TypePostgreSQL {
		return nil, fmt.Errorf("unsupported data source type: %s", config.Type)
	}

	pgConfig := Config{
		ConnectionString: config.URL,
		QueryTimeout:     config.Timeout,
		MaxConnections:   10, // Default values
		MinConnections:   2,
		MaxConnLifetime:  time.Hour,
		MaxConnIdleTime:  30 * time.Minute,
	}

	// Extract additional options
	if maxConns, ok := config.Options["maxConnections"].(int); ok {
		pgConfig.MaxConnections = int32(maxConns)
	}
	if minConns, ok := config.Options["minConnections"].(int); ok {
		pgConfig.MinConnections = int32(minConns)
	}

	return NewPostgreSQLDataSource(pgConfig)
}

// SupportedTypes returns the data source types this factory supports
func (f *Factory) SupportedTypes() []datasource.DataSourceType {
	return []datasource.DataSourceType{datasource.TypePostgreSQL}
}

// ValidateConfig validates a PostgreSQL data source configuration
func (f *Factory) ValidateConfig(config datasource.DataSourceConfig) error {
	if config.Type != datasource.TypePostgreSQL {
		return fmt.Errorf("unsupported data source type: %s", config.Type)
	}

	if config.URL == "" {
		return fmt.Errorf("connection URL is required for PostgreSQL data source")
	}

	// Validate connection string format
	if !strings.Contains(config.URL, "postgres://") && !strings.Contains(config.URL, "postgresql://") {
		return fmt.Errorf("invalid PostgreSQL connection string format")
	}

	return nil
}
