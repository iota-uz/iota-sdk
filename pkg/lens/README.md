# Lens Package

Lens is a flexible data visualization and dashboard framework for Go applications. It provides a comprehensive solution for building dashboards, executing queries against multiple data sources, and rendering interactive visualizations.

## Features

- **Multi-Data Source Support**: PostgreSQL, MongoDB, and extensible data source architecture
- **Query Execution Engine**: Concurrent query execution with timeout and error handling
- **Dashboard Builder**: Fluent API for building dashboards and panels programmatically
- **Caching System**: In-memory caching with TTL and LRU eviction for improved performance
- **Templating Engine**: Server-side rendering with templ for dynamic UI generation
- **Type Safety**: Strong typing throughout the API for better developer experience

## Quick Start

### 1. Setting up a Data Source

```go
package main

import (
    "context"
    "time"
    
    "github.com/iota-uz/iota-sdk/pkg/lens/datasource/postgres"
    "github.com/iota-uz/iota-sdk/pkg/lens/executor"
)

func main() {
    // Configure PostgreSQL data source
    pgConfig := postgres.Config{
        ConnectionString: "postgres://user:pass@localhost:5432/mydb",
        MaxConnections:   10,
        MinConnections:   2,
        QueryTimeout:     30 * time.Second,
    }
    
    // Create PostgreSQL data source
    pgDataSource, err := postgres.NewPostgreSQLDataSource(pgConfig)
    if err != nil {
        panic(err)
    }
    defer pgDataSource.Close()
    
    // Create executor and register data source
    exec := executor.NewExecutor(nil, 30*time.Second)
    err = exec.RegisterDataSource("main-db", pgDataSource)
    if err != nil {
        panic(err)
    }
}
```

### 2. Building a Dashboard

```go
import (
    "github.com/iota-uz/iota-sdk/pkg/lens"
    "github.com/iota-uz/iota-sdk/pkg/lens/builder"
)

func createDashboard() lens.DashboardConfig {
    dashboard := builder.NewDashboard().
        Title("System Metrics Dashboard").
        Description("Real-time system monitoring").
        Variable("environment", "production").
        Variable("timeRange", lens.TimeRange{
            Start: time.Now().Add(-24 * time.Hour),
            End:   time.Now(),
        }).
        Panel(
            builder.LineChart().
                ID("cpu-usage").
                Title("CPU Usage").
                DataSource("main-db").
                Query(`
                    SELECT timestamp, cpu_percent 
                    FROM metrics 
                    WHERE environment = $environment 
                    AND timestamp >= $timeRange
                    ORDER BY timestamp
                `).
                Position(0, 0).
                Size(6, 4).
                Option("maxRows", 1000).
                Option("timeout", "10s").
                Build(),
        ).
        Panel(
            builder.AreaChart().
                ID("memory-usage").
                Title("Memory Usage").
                DataSource("main-db").
                Query(`
                    SELECT timestamp, memory_percent 
                    FROM metrics 
                    WHERE environment = $environment 
                    AND timestamp >= $timeRange
                    ORDER BY timestamp
                `).
                Position(6, 0).
                Size(6, 4).
                Build(),
        ).
        Build()
    
    return dashboard
}
```

### 3. Executing Queries

```go
func executeQueries(ctx context.Context, exec executor.Executor) {
    // Execute a single query
    query := executor.ExecutionQuery{
        DataSourceID: "main-db",
        Query:        "SELECT COUNT(*) as total_users FROM users",
        Variables:    map[string]interface{}{"active": true},
        Format:       datasource.FormatTable,
        MaxRows:      100,
    }
    
    result, err := exec.Execute(ctx, query)
    if err != nil {
        log.Printf("Query failed: %v", err)
        return
    }
    
    fmt.Printf("Query returned %d rows in %v\n", 
        len(result.Data), result.ExecTime)
    
    // Execute entire dashboard
    dashboard := createDashboard()
    dashResult, err := exec.ExecuteDashboard(ctx, dashboard)
    if err != nil {
        log.Printf("Dashboard execution failed: %v", err)
        return
    }
    
    fmt.Printf("Dashboard executed in %v with %d panels\n", 
        dashResult.Duration, len(dashResult.PanelResults))
}
```

### 4. Using the Caching System

```go
import (
    "github.com/iota-uz/iota-sdk/pkg/lens/cache"
)

func setupCaching(exec executor.Executor) executor.Executor {
    // Create memory cache with 1000 entries, cleanup every 5 minutes
    memCache := cache.NewMemoryCache(1000, 5*time.Minute)
    
    // Wrap executor with caching, 5 minute default TTL
    cachingExec := cache.NewCachingExecutor(exec, memCache, 5*time.Minute)
    
    return cachingExec
}

func monitorCache() {
    stats := cache.Stats()
    fmt.Printf("Cache Stats: %d hits, %d misses, %.2f%% hit rate\n", 
        stats.Hits, stats.Misses, stats.HitRate)
}
```

### 5. Creating Custom Data Sources

```go
import (
    "github.com/iota-uz/iota-sdk/pkg/lens/datasource"
)

type CustomDataSource struct {
    // Your implementation fields
}

func (ds *CustomDataSource) Query(ctx context.Context, query datasource.Query) (*datasource.QueryResult, error) {
    // Implement query execution
    return &datasource.QueryResult{
        Data: []datasource.DataPoint{
            {
                Timestamp: time.Now(),
                Value:     42.0,
                Labels:    map[string]string{"source": "custom"},
                Fields:    map[string]interface{}{"metric": "example"},
            },
        },
        Columns: []datasource.ColumnInfo{
            {Name: "timestamp", Type: datasource.DataTypeTimestamp},
            {Name: "value", Type: datasource.DataTypeNumber},
        },
        Metadata: datasource.ResultMetadata{
            QueryID:    query.ID,
            RowCount:   1,
            DataSource: "custom",
        },
    }, nil
}

func (ds *CustomDataSource) TestConnection(ctx context.Context) error {
    // Implement connection test
    return nil
}

func (ds *CustomDataSource) GetMetadata() datasource.DataSourceMetadata {
    return datasource.DataSourceMetadata{
        Type:        "custom",
        Name:        "Custom Data Source",
        Version:     "1.0.0",
        Description: "Custom implementation example",
        Capabilities: []datasource.Capability{
            datasource.CapabilityQuery,
        },
    }
}

func (ds *CustomDataSource) ValidateQuery(query datasource.Query) error {
    // Implement query validation
    return nil
}

func (ds *CustomDataSource) Close() error {
    // Implement cleanup
    return nil
}
```

### 6. Rendering Dashboards in Templ Components

```go
import (
    "github.com/iota-uz/iota-sdk/pkg/lens/ui"
    "github.com/a-h/templ"
)

// Simple example: Render a dashboard config with data as templ component
templ DashboardPage(config lens.DashboardConfig, results *executor.DashboardResult) {
    <!DOCTYPE html>
    <html>
    <head>
        <title>{ config.Name }</title>
        <style>{ ui.GenerateCSS(config.Grid) }</style>
    </head>
    <body>
        @ui.DashboardWithData(config, results)
    </body>
    </html>
}

// Usage in your handler
func dashboardHandler(w http.ResponseWriter, r *http.Request) {
    // Create dashboard config
    dashboard := createDashboard()
    
    // Execute queries
    results, err := exec.ExecuteDashboard(r.Context(), dashboard)
    if err != nil {
        http.Error(w, err.Error(), 500)
        return
    }
    
    // Render with templ
    component := DashboardPage(dashboard, results)
    component.Render(r.Context(), w)
}
```

## Advanced Usage

### Variable Interpolation

Variables in queries are automatically interpolated with proper escaping:

```go
query := executor.ExecutionQuery{
    DataSourceID: "main-db",
    Query: `
        SELECT * FROM events 
        WHERE user_id = $userID 
        AND created_at >= $startTime 
        AND status = $status
    `,
    Variables: map[string]interface{}{
        "userID":    123,
        "startTime": time.Now().Add(-24 * time.Hour),
        "status":    "active",
    },
}
```

### Time Range Queries

Handle time-based data with built-in time range support:

```go
timeRange := lens.TimeRange{
    Start: time.Now().Add(-7 * 24 * time.Hour), // 7 days ago
    End:   time.Now(),
}

query := executor.ExecutionQuery{
    DataSourceID: "main-db",
    Query:        "SELECT timestamp, value FROM metrics WHERE timestamp BETWEEN $start AND $end",
    Variables: map[string]interface{}{
        "start": timeRange.Start,
        "end":   timeRange.End,
    },
    TimeRange: timeRange,
}
```

### Error Handling

The lens package provides detailed error information:

```go
result, err := exec.Execute(ctx, query)
if err != nil {
    if queryErr, ok := err.(*datasource.QueryError); ok {
        switch queryErr.Code {
        case datasource.ErrorCodeTimeout:
            log.Printf("Query timed out: %s", queryErr.Message)
        case datasource.ErrorCodeSyntax:
            log.Printf("SQL syntax error: %s", queryErr.Message)
        case datasource.ErrorCodePermission:
            log.Printf("Permission denied: %s", queryErr.Message)
        default:
            log.Printf("Query error: %s", queryErr.Message)
        }
    }
    return
}
```

## Configuration

### Data Source Types

Currently supported data source types:

- `datasource.TypePostgreSQL` - PostgreSQL databases
- `datasource.TypeMongoDB` - MongoDB collections (planned)

### Chart Types

Available visualization types:

- `lens.ChartTypeLine` - Line charts for time series
- `lens.ChartTypeArea` - Area charts
- `lens.ChartTypeBar` - Horizontal bar charts
- `lens.ChartTypeColumn` - Vertical column charts
- `lens.ChartTypePie` - Pie charts
- `lens.ChartTypeTable` - Data tables

### Query Formats

Supported result formats:

- `datasource.FormatTable` - Tabular data
- `datasource.FormatTimeSeries` - Time-based series data

## Testing

The package includes comprehensive test coverage. Run tests with:

```bash
go test ./pkg/lens/...
```

For integration tests with real PostgreSQL:

```bash
# Start PostgreSQL (using Docker)
docker run --name postgres-test -e POSTGRES_PASSWORD=password -p 5432:5432 -d postgres

# Run integration tests
go test ./pkg/lens/datasource/postgres -tags=integration
```

## Performance

- Use caching for frequently accessed data
- Set appropriate query timeouts
- Limit result sets with `MaxRows`
- Consider connection pooling for high-traffic scenarios
- Monitor cache hit rates and adjust TTL accordingly

## License

This package is part of the IOTA SDK and follows the same licensing terms.
