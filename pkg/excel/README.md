# Excel Export Package

A flexible and efficient Excel export package for Go applications that supports various data sources and customizable formatting options.

## Features

- **Generic DataSource interface** - Export data from any source
- **Built-in PostgreSQL support** - Export query results directly to Excel
- **PGX/PGXPool support** - Native support for pgx connection pools
- **Customizable styling** - Headers, data rows, alternating colors
- **Export options** - Control headers, filtering, freezing, and row limits
- **Type-aware formatting** - Automatic formatting for dates, numbers, etc.
- **Memory efficient** - Streaming data processing for large datasets

## Installation

```bash
go get github.com/iota-uz/iota-sdk/pkg/excel
```

## Quick Start

### Basic Usage with PostgreSQL

```go
package main

import (
    "context"
    "database/sql"
    "log"
    "os"
    
    "github.com/iota-uz/iota-sdk/pkg/excel"
    _ "github.com/lib/pq"
)

func main() {
    // Open database connection
    db, err := sql.Open("postgres", "postgresql://...")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()
    
    // Create data source with SQL query
    datasource := excel.NewPostgresDataSource(
        db, 
        "SELECT id, name, email, created_at FROM users WHERE active = $1",
        true,
    ).WithSheetName("Active Users")
    
    // Create exporter with default options
    exporter := excel.NewExcelExporter(nil, nil)
    
    // Export to Excel
    ctx := context.Background()
    data, err := exporter.Export(ctx, datasource)
    if err != nil {
        log.Fatal(err)
    }
    
    // Write to file
    err = os.WriteFile("users.xlsx", data, 0644)
    if err != nil {
        log.Fatal(err)
    }
}
```

### Usage with PGX/PGXPool

```go
package main

import (
    "context"
    "log"
    "os"
    
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/iota-uz/iota-sdk/pkg/excel"
)

func main() {
    // Create pgxpool connection
    pool, err := pgxpool.New(context.Background(), "postgresql://...")
    if err != nil {
        log.Fatal(err)
    }
    defer pool.Close()
    
    // Create data source with SQL query
    datasource := excel.NewPgxDataSource(
        pool, 
        "SELECT id, name, email, created_at FROM users WHERE active = $1",
        true,
    ).WithSheetName("Active Users")
    
    // Create exporter with default options
    exporter := excel.NewExcelExporter(nil, nil)
    
    // Export to Excel
    ctx := context.Background()
    data, err := exporter.Export(ctx, datasource)
    if err != nil {
        log.Fatal(err)
    }
    
    // Write to file
    err = os.WriteFile("users.xlsx", data, 0644)
    if err != nil {
        log.Fatal(err)
    }
}
```

### Custom DataSource Implementation

```go
type CustomDataSource struct {
    data [][]string
}

func (c *CustomDataSource) GetHeaders() []string {
    return []string{"Column1", "Column2", "Column3"}
}

func (c *CustomDataSource) GetSheetName() string {
    return "CustomData"
}

func (c *CustomDataSource) GetRows(ctx context.Context) (func() ([]interface{}, error), error) {
    index := 0
    return func() ([]interface{}, error) {
        if index >= len(c.data) {
            return nil, nil // EOF
        }
        row := make([]interface{}, len(c.data[index]))
        for i, v := range c.data[index] {
            row[i] = v
        }
        index++
        return row, nil
    }, nil
}
```

### Advanced Options

```go
// Configure export options
exportOpts := &excel.ExportOptions{
    IncludeHeaders: true,
    AutoFilter:     true,
    FreezeHeader:   true,
    DateFormat:     "2006-01-02",
    TimeFormat:     "15:04:05",
    DateTimeFormat: "2006-01-02 15:04:05",
    MaxRows:        10000, // Limit export to 10k rows
}

// Configure styling
styleOpts := &excel.StyleOptions{
    HeaderStyle: &excel.CellStyle{
        Font: &excel.FontStyle{
            Bold: true,
            Size: 12,
        },
        Fill: &excel.FillStyle{
            Type:    "pattern",
            Pattern: 1,
            Color:   "#4CAF50",
        },
    },
    AlternateRow: true,
}

exporter := excel.NewExcelExporter(exportOpts, styleOpts)
```

## API Reference

### DataSource Interface

```go
type DataSource interface {
    GetHeaders() []string
    GetRows(ctx context.Context) (func() ([]interface{}, error), error)
    GetSheetName() string
}
```

### PostgresDataSource

```go
// Create new PostgreSQL data source (standard database/sql)
func NewPostgresDataSource(db *sql.DB, query string, args ...interface{}) *PostgresDataSource

// Set custom sheet name
func (p *PostgresDataSource) WithSheetName(name string) *PostgresDataSource
```

### PgxDataSource

```go
// Create new pgx data source (pgx/pgxpool)
func NewPgxDataSource(db *pgxpool.Pool, query string, args ...interface{}) *PgxDataSource

// Set custom sheet name
func (p *PgxDataSource) WithSheetName(name string) *PgxDataSource
```

### ExcelExporter

```go
// Create new Excel exporter
func NewExcelExporter(opts *ExportOptions, styleOpts *StyleOptions) *ExcelExporter

// Export data to Excel format
func (e *ExcelExporter) Export(ctx context.Context, datasource DataSource) ([]byte, error)
```

### Export Options

```go
type ExportOptions struct {
    IncludeHeaders bool   // Include header row
    AutoFilter     bool   // Add auto-filter to headers
    FreezeHeader   bool   // Freeze header row
    DateFormat     string // Format for date values
    TimeFormat     string // Format for time values
    DateTimeFormat string // Format for datetime values
    MaxRows        int    // Maximum rows to export (0 = unlimited)
}
```

## Testing

The package includes comprehensive tests for all components:

```bash
go test ./pkg/excel/...
```

## Performance Considerations

- The package uses streaming for data processing to handle large datasets efficiently
- PostgresDataSource fetches rows on-demand rather than loading all data into memory
- For very large exports, consider setting `MaxRows` to limit the output size
- Use context cancellation for long-running exports

## Error Handling

The package returns descriptive errors for common issues:

- Database connection errors
- Query execution errors
- Excel generation errors
- Context cancellation

Always check returned errors and handle them appropriately in your application.

## License

This package is part of the IOTA SDK and follows the same license terms.