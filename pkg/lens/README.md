# Lens Package

Lens is a flexible data visualization and dashboard framework for Go applications. It provides a comprehensive solution for building dashboards, executing queries against multiple data sources, and rendering interactive visualizations.

## Features

- **Multi-Data Source Support**: PostgreSQL (with extensible data source architecture for future additions)
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

func monitorCache(memCache *cache.MemoryCache) {
    stats := memCache.Stats()
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
        <style>/* CSS is generated automatically by the dashboard component */</style>
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
- `datasource.TypeMongoDB` - MongoDB collections (interface defined, implementation planned)

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

## Drilldown and Interactive Features

The lens package provides comprehensive interactive functionality through an event-driven system that enables drilldown, navigation, modals, and custom interactions when users click on chart elements.

### Event System Overview

The event system captures user interactions and executes configured actions based on the clicked context. It supports multiple event types and four distinct action types for maximum flexibility.

#### Supported Event Types

- `Click` - General chart area clicks
- `DataPoint` - Specific data point clicks (most common for drilldown)
- `Legend` - Legend item clicks
- `Marker` - Chart marker interactions
- `XAxisLabel` - X-axis label clicks

#### Action Types

1. **Navigation** - Redirect to URLs with variable substitution
2. **DrillDown** - Filter/update current dashboard contextually
3. **Modal** - Display detailed information in popups
4. **Custom** - Execute custom JavaScript functions

### Basic Drilldown Configuration

```go
import (
    "github.com/iota-uz/iota-sdk/pkg/lens"
    "github.com/iota-uz/iota-sdk/pkg/lens/builder"
)

// Simple drilldown using convenience method
panel := builder.BarChart().
    ID("sales-by-region").
    Title("Sales by Region").
    DataSource("main-db").
    Query("SELECT region, sales_amount FROM sales_data").
    Position(0, 0).
    Size(6, 4).
    OnDrillDown(map[string]string{
        "region": "{label}",        // Use clicked label as filter
        "period": "{seriesName}",   // Use series name
    }).
    Build()

// Advanced drilldown with specific event types
panel2 := builder.BarChart().
    ID("advanced-sales").
    Title("Advanced Sales Chart").
    DataSource("main-db").
    Query("SELECT region, sales_amount FROM sales_data").
    Position(0, 4).
    Size(6, 4).
    OnDataPointClick(lens.ActionConfig{
        Type: lens.ActionTypeDrillDown,
        DrillDown: &lens.DrillDownAction{
            Filters: map[string]string{
                "region": "{label}",
                "period": "{seriesName}",
            },
            Variables: map[string]string{
                "selectedRegion": "{label}",
                "selectedValue":  "{value}",
            },
        },
    }).
    Build()
```

### Navigation Drilldown

Navigate to different pages/dashboards when chart elements are clicked:

```go
// Simple navigation using convenience method
panel := builder.LineChart().
    ID("overview-metrics").
    Title("System Overview").
    OnNavigate("/dashboard/details?metric={label}&value={value}", "_blank").
    Build()

// Advanced navigation with specific event types
panel2 := builder.LineChart().
    ID("detailed-metrics").
    Title("Detailed System Overview").
    OnDataPointClick(lens.ActionConfig{
        Type: lens.ActionTypeNavigation,
        Navigation: &lens.NavigationAction{
            URL:    "/dashboard/details?metric={label}&value={value}&time={dataPoint.x}",
            Target: "_blank", // Open in new tab
            Variables: map[string]string{
                "source": "dashboard",
            },
        },
    }).
    OnLegendClick(lens.ActionConfig{
        Type: lens.ActionTypeNavigation,
        Navigation: &lens.NavigationAction{
            URL:    "/dashboard/series/{seriesName}",
            Target: "_self",
            Variables: map[string]string{},
        },
    }).
    Build()
```

### Modal Drilldown

Display detailed information in modal popups:

```go
// Simple modal using convenience method
panel := builder.PieChart().
    ID("category-breakdown").
    Title("Expense Categories").
    OnModal("Category Details: {label}", "", "/api/category-details?category={label}&value={value}").
    Build()

// Advanced modal with variables
panel2 := builder.PieChart().
    ID("advanced-categories").
    Title("Advanced Expense Categories").
    OnDataPointClick(lens.ActionConfig{
        Type: lens.ActionTypeModal,
        Modal: &lens.ModalAction{
            Title: "Category Details: {label}",
            URL:   "/api/category-details?category={label}&value={value}",
            Variables: map[string]string{
                "source": "pie-chart",
                "timestamp": "{dataPoint.x}",
            },
        },
    }).
    Build()
```

### Custom JavaScript Actions

Execute custom JavaScript functions with event context:

```go
// Simple custom action using convenience method
panel := builder.AreaChart().
    ID("realtime-data").
    Title("Real-time Metrics").
    OnCustom("handleCustomDrilldown", map[string]string{
        "panelId":    "{panelId}",
        "dataPoint":  "{value}",
        "timestamp":  "{dataPoint.x}",
        "customData": "additional-context",
    }).
    Build()

// Advanced custom action with specific event types
panel2 := builder.AreaChart().
    ID("advanced-realtime").
    Title("Advanced Real-time Metrics").
    OnDataPointClick(lens.ActionConfig{
        Type: lens.ActionTypeCustom,
        Custom: &lens.CustomAction{
            Function: "handleCustomDrilldown",
            Variables: map[string]string{
                "panelId":    "{panelId}",
                "dataPoint":  "{value}",
                "timestamp":  "{dataPoint.x}",
                "customData": "additional-context",
            },
        },
    }).
    Build()
```

### Variable Substitution

The event system supports rich variable substitution for dynamic actions:

#### Basic Variables
- `{panelId}` - Panel identifier
- `{chartType}` - Chart type (line, bar, pie, etc.)
- `{label}` - Data point label/category
- `{value}` - Data point value
- `{seriesName}` - Series name
- `{categoryName}` - Category name
- `{seriesIndex}` - Zero-based series index
- `{dataIndex}` - Zero-based data point index

#### Data Point Variables
- `{dataPoint.x}` - X-coordinate value
- `{dataPoint.y}` - Y-coordinate value
- `{dataPoint.label}` - Data point label

#### Dashboard Variables
- `{var.variableName}` - Access dashboard variables
- `{data.customField}` - Access custom data fields

### Multi-Level Drilldown Example

Create a complete drill-down experience across multiple dashboard levels:

```go
// Level 1: Sales Overview Dashboard
overviewDashboard := builder.NewDashboard().
    Title("Sales Overview").
    Variable("year", "2024").
    Panel(
        builder.BarChart().
            ID("sales-by-region").
            Title("Sales by Region").
            Query(`
                SELECT region, SUM(amount) as total_sales 
                FROM sales 
                WHERE year = $year 
                GROUP BY region
            `).
            OnDataPointClick(lens.ActionConfig{
                Type: lens.ActionTypeNavigation,
                Navigation: &lens.NavigationAction{
                    URL: "/dashboard/region-details?region={label}&year={var.year}",
                    Target: "_self",
                },
            }).
            Build(),
    ).
    Build()

// Level 2: Region Details Dashboard
regionDashboard := builder.NewDashboard().
    Title("Region Sales Details").
    Variable("region", "").
    Variable("year", "2024").
    Panel(
        builder.LineChart().
            ID("monthly-sales").
            Title("Monthly Sales Trend").
            Query(`
                SELECT month, SUM(amount) as monthly_sales 
                FROM sales 
                WHERE region = $region AND year = $year 
                GROUP BY month 
                ORDER BY month
            `).
            OnDataPointClick(lens.ActionConfig{
                Type: lens.ActionTypeModal,
                Modal: &lens.ModalAction{
                    Title: "Monthly Details: {dataPoint.x}",
                    URL: "/api/sales-details?region={var.region}&month={dataPoint.x}&year={var.year}",
                },
            }).
            Build(),
    ).
    Panel(
        builder.ColumnChart().
            ID("product-sales").
            Title("Product Performance").
            Query(`
                SELECT product, SUM(amount) as product_sales 
                FROM sales 
                WHERE region = $region AND year = $year 
                GROUP BY product
            `).
            OnDataPointClick(lens.ActionConfig{
                Type: lens.ActionTypeDrillDown,
                DrillDown: &lens.DrillDownAction{
                    Filters: map[string]string{
                        "product": "{label}",
                    },
                    Variables: map[string]string{
                        "selectedProduct": "{label}",
                    },
                },
            }).
            Build(),
    ).
    Build()
```

### Event Handler Implementation

For custom actions, implement JavaScript event handlers:

```javascript
// In your web application
function handleCustomDrilldown(context) {
    console.log('Custom drilldown triggered:', context);
    
    // Access event context
    const panelId = context.panelId;
    const value = context.dataPoint;
    const timestamp = context.timestamp;
    
    // Perform custom logic
    if (value > 1000) {
        // Show alert for high values
        showHighValueAlert(value, timestamp);
    } else {
        // Navigate to details page
        window.location.href = `/details/${panelId}/${timestamp}`;
    }
}

function showHighValueAlert(value, timestamp) {
    alert(`High value detected: ${value} at ${timestamp}`);
}
```

### Server-Side Event Processing

Handle drilldown events in your controllers:

```go
// In your controller handling drilldown requests
func (c *DashboardController) HandleLensEvent(ctx context.Context, panelID string, eventCtx lens.EventContext) error {
    switch eventCtx.Action.Type {
    case lens.ActionTypeDrillDown:
        // Apply filters and update dashboard
        return c.applyDrilldownFilters(ctx, eventCtx.Action.DrillDown)
        
    case lens.ActionTypeModal:
        // Return modal content
        return c.renderModalContent(ctx, eventCtx.Action.Modal, eventCtx)
        
    case lens.ActionTypeNavigation:
        // Handle navigation logic if needed
        return c.handleNavigation(ctx, eventCtx.Action.Navigation)
        
    default:
        return fmt.Errorf("unsupported action type: %s", eventCtx.Action.Type)
    }
}

func (c *DashboardController) applyDrilldownFilters(ctx context.Context, action *lens.DrillDownAction) error {
    // Update dashboard variables and filters
    for key, value := range action.Filters {
        c.dashboardState.SetFilter(key, value)
    }
    
    for key, value := range action.Variables {
        c.dashboardState.SetVariable(key, value)
    }
    
    // Re-execute dashboard with new filters
    return c.refreshDashboard(ctx)
}
```

### Best Practices

#### Performance Considerations
- Use specific event types (DataPoint, Legend) rather than generic Click for better performance
- Implement caching for frequently accessed drilldown data
- Consider pagination for large result sets in modals

#### User Experience
- Provide visual feedback (loading indicators) during drilldown operations
- Use consistent navigation patterns across dashboards
- Include breadcrumb navigation for multi-level drilldowns
- Offer "back" functionality to return to previous levels

#### Security
- Always validate and sanitize variables before using in queries
- Implement proper authorization checks for drilldown targets
- Use parameterized queries to prevent SQL injection

#### Error Handling
```go
// Graceful error handling in event processing
func (c *Controller) HandleEventError(err error, eventCtx lens.EventContext) {
    log.Printf("Event processing error for panel %s: %v", eventCtx.PanelID, err)
    
    // Return user-friendly error response
    c.renderErrorModal("Unable to load details. Please try again.")
}
```

## License

This package is part of the IOTA SDK and follows the same licensing terms.
