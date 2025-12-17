---
layout: default
title: Excel Exporter
parent: Advanced
nav_order: 3
description: "Excel export functionality for IOTA SDK"
---

# Excel Exporter

The Excel Exporter Service provides functionality to export data from your application to Excel files with automatic formatting, styling, and upload management.

## Overview

The Excel Exporter enables:

- **Query Export**: Export SQL query results directly to Excel
- **Custom Data Sources**: Implement custom data providers
- **Automatic Styling**: Apply formatting, colors, and fonts
- **Sheet Configuration**: Multiple sheets, frozen headers, auto-filter
- **File Management**: Automatic upload and versioning
- **Streaming Export**: Memory-efficient handling of large datasets

## Basic Usage

### Importing the Service

```go
import "github.com/iota-uz/iota-sdk/modules/core/services"

type MyController struct {
    excelService services.ExcelExportService
}
```

### Export from SQL Query

```go
func (c *MyController) ExportUsers(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    upload, err := c.excelService.ExportFromQuery(
        ctx,
        "SELECT id, first_name, last_name, email, created_at FROM users WHERE active = true",
        "active_users", // filename without .xlsx
        true,           // include headers
    )
    if err != nil {
        http.Error(w, "Export failed", http.StatusInternalServerError)
        return
    }

    // Redirect to download
    http.Redirect(w, r, upload.URL().String(), http.StatusSeeOther)
}
```

## Advanced Usage

### Export with Options

```go
// Configure export behavior
exportOpts := &excel.ExportOptions{
    IncludeHeaders: true,
    AutoFilter:     true,     // Enable filter buttons
    FreezeHeader:   true,     // Freeze first row
    DateFormat:     "2006-01-02",
    MaxRows:        10000,    // Limit to 10k rows
    SheetName:      "Users",
}

upload, err := c.excelService.ExportFromQueryWithOptions(
    ctx,
    "SELECT * FROM users",
    "users_export",
    exportOpts,
    nil,
)
```

### Custom Styling

```go
styleOpts := &excel.StyleOptions{
    HeaderStyle: &excel.CellStyle{
        Font: &excel.FontStyle{
            Bold:  true,
            Size:  12,
            Color: "#FFFFFF",
        },
        Fill: &excel.FillStyle{
            Type:    "pattern",
            Pattern: 1,
            Color:   "#4CAF50",
        },
        Alignment: &excel.AlignmentStyle{
            Horizontal: "center",
            Vertical:   "center",
        },
    },
    AlternateRow: true,
    AltRowColor:  "#F0F0F0",
}

upload, err := c.excelService.ExportFromQueryWithOptions(
    ctx,
    "SELECT * FROM orders",
    "orders_report",
    exportOpts,
    styleOpts,
    startDate, endDate,
)
```

### Export from Custom Data Source

Implement the `DataSource` interface for custom data:

```go
type ReportDataSource struct {
    reportService *ReportService
    reportType    string
}

func (r *ReportDataSource) GetHeaders() []string {
    return []string{"Month", "Revenue", "Expenses", "Profit", "Growth %"}
}

func (r *ReportDataSource) GetSheetName() string {
    return "Monthly Report"
}

func (r *ReportDataSource) GetRows(ctx context.Context) (func() ([]interface{}, error), error) {
    // Fetch data
    data, err := r.reportService.GetMonthlyReport(ctx, r.reportType)
    if err != nil {
        return nil, err
    }

    index := 0
    return func() ([]interface{}, error) {
        if index >= len(data) {
            return nil, nil // EOF
        }

        row := data[index]
        index++

        return []interface{}{
            row.Month,
            row.Revenue,
            row.Expenses,
            row.Profit,
            row.GrowthPercent,
        }, nil
    }, nil
}

// Use custom data source
upload, err := c.excelService.ExportFromDataSource(
    ctx,
    &ReportDataSource{
        reportService: c.reportService,
        reportType: "monthly",
    },
    "monthly_report",
    exportOpts,
)
```

### Multiple Sheets

```go
// Export multiple sheets
sheets := []excel.SheetData{
    {
        Name: "Summary",
        DataSource: summaryDataSource,
    },
    {
        Name: "Details",
        DataSource: detailsDataSource,
    },
    {
        Name: "Metadata",
        DataSource: metadataDataSource,
    },
}

upload, err := c.excelService.ExportMultiSheet(
    ctx,
    sheets,
    "comprehensive_report",
)
```

## Field Types and Formatting

### Numeric Formatting

```go
// Currency
styleOpts.ColumnFormats = map[string]string{
    "amount": "#,##0.00",
    "total":  "[$$-409]#,##0.00;-[$$-409]#,##0.00",
}

// Percentage
styleOpts.ColumnFormats["growth_percent"] = "0.00%"

// Integer
styleOpts.ColumnFormats["quantity"] = "0"
```

### Date/Time Formatting

```go
// Date
styleOpts.ColumnFormats["order_date"] = "YYYY-MM-DD"

// DateTime
styleOpts.ColumnFormats["created_at"] = "YYYY-MM-DD HH:MM:SS"

// Time
styleOpts.ColumnFormats["delivery_time"] = "HH:MM:SS"
```

### Conditional Formatting

```go
styleOpts.ConditionalFormats = []excel.ConditionalFormat{
    {
        Range: "D:D", // Amount column
        Type:  "colorScale",
        Min: excel.ColorScaleValue{
            Value: 0,
            Color: "#FF0000", // Red for low amounts
        },
        Mid: excel.ColorScaleValue{
            Value: 50,
            Color: "#FFFF00", // Yellow for medium
        },
        Max: excel.ColorScaleValue{
            Value: 100,
            Color: "#00FF00", // Green for high amounts
        },
    },
}
```

## Column Configuration

### Column Widths

```go
styleOpts.ColumnWidths = map[string]int{
    "A": 20,  // First column 20 chars wide
    "B": 30,
    "C": 15,
}
```

### Column Formatting

```go
styleOpts.ColumnStyles = map[string]*excel.CellStyle{
    "C": {
        Font: &excel.FontStyle{Color: "#0000FF"},
    },
    "D": {
        Fill: &excel.FillStyle{Color: "#FFFFCC"},
    },
}
```

## Performance Optimization

### Streaming Large Datasets

```go
// Process large exports in chunks
func (c *MyController) ExportLargeDataset(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // Stream rows to avoid loading all in memory
    dataSource := &StreamingDataSource{
        query: "SELECT * FROM large_table",
        batchSize: 1000,
    }

    upload, err := c.excelService.ExportFromDataSource(
        ctx,
        dataSource,
        "large_export",
        &excel.ExportOptions{
            MaxRows: 100000, // Limit for safety
        },
    )
}
```

### Progress Tracking

```go
// Track export progress
upload, err := c.excelService.ExportWithProgress(
    ctx,
    dataSource,
    "report",
    func(processed, total int64) {
        // Update progress
        log.Printf("Exported %d/%d rows", processed, total)
    },
)
```

## Error Handling

### Common Errors

```go
upload, err := c.excelService.ExportFromQuery(ctx, query, filename, true)

if err != nil {
    switch err {
    case excel.ErrMaxRowsExceeded:
        // Handle too many rows
        http.Error(w, "Too many rows to export", http.StatusBadRequest)
    case excel.ErrInvalidQuery:
        // Handle SQL errors
        http.Error(w, "Invalid query", http.StatusBadRequest)
    case excel.ErrUploadFailed:
        // Handle upload failure
        http.Error(w, "Failed to save file", http.StatusInternalServerError)
    default:
        http.Error(w, err.Error(), http.StatusInternalServerError)
    }
    return
}
```

## Controller Integration

### As a Standalone Endpoint

```go
func (c *UserController) ExportUsers(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    logger := composables.UseLogger(ctx)

    logger.Info("Starting user export")

    upload, err := c.excelService.ExportFromQuery(
        ctx,
        `SELECT u.id, u.first_name, u.last_name, u.email,
                COUNT(o.id) as order_count
         FROM users u
         LEFT JOIN orders o ON u.id = o.user_id
         WHERE u.active = true
         GROUP BY u.id`,
        "users_with_orders",
        true,
    )

    if err != nil {
        logger.WithField("error", err.Error()).Error("Export failed")
        http.Error(w, "Export failed", http.StatusInternalServerError)
        return
    }

    logger.Info("Export completed successfully")

    // Option 1: Redirect to download
    http.Redirect(w, r, upload.URL().String(), http.StatusSeeOther)

    // Option 2: Return URL as JSON
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{
        "downloadUrl": upload.URL().String(),
        "filename": upload.Name(),
    })
}
```

### With Filters

```go
func (c *OrderController) ExportOrders(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // Get filter parameters
    startDate := r.FormValue("start_date")
    endDate := r.FormValue("end_date")
    status := r.FormValue("status")

    // Build dynamic query
    query := `SELECT id, order_number, customer_name, total, status, created_at
              FROM orders WHERE 1=1`

    params := []interface{}{}

    if startDate != "" {
        query += " AND created_at >= $" + fmt.Sprint(len(params)+1)
        params = append(params, startDate)
    }

    if endDate != "" {
        query += " AND created_at <= $" + fmt.Sprint(len(params)+1)
        params = append(params, endDate)
    }

    if status != "" {
        query += " AND status = $" + fmt.Sprint(len(params)+1)
        params = append(params, status)
    }

    upload, err := c.excelService.ExportFromQueryWithParams(
        ctx,
        query,
        "orders",
        true,
        params...,
    )

    // Return download...
}
```

## Testing Exports

```go
func TestExportUsers(t *testing.T) {
    suite := controllertest.New(t, userModule)

    // Test export endpoint
    response := suite.GET("/users/export").
        Expect(t).
        Status(302)

    // Verify redirect to file URL
    location := response.Header("Location")
    if location == "" {
        t.Error("Expected redirect location")
    }

    // Verify file was created
    if !strings.HasSuffix(location, ".xlsx") {
        t.Error("Expected Excel file download")
    }
}
```

## Configuration

### Environment Variables

```bash
# Maximum file size
EXCEL_MAX_ROWS=100000
EXCEL_MAX_COLUMNS=100

# Temporary storage location
EXCEL_TEMP_DIR=/tmp/excel_exports

# Auto-cleanup duration
EXCEL_CLEANUP_AGE_HOURS=24
```

## Best Practices

1. **Limit Row Count**: Always set `MaxRows` to prevent memory exhaustion
   ```go
   exportOpts.MaxRows = 50000
   ```

2. **Include Headers**: Always use `IncludeHeaders: true` for usability
   ```go
   exportOpts.IncludeHeaders = true
   ```

3. **Use Freeze for Headers**: Freeze header rows for better usability
   ```go
   exportOpts.FreezeHeader = true
   ```

4. **Format Numbers Properly**: Use appropriate formatting for numeric columns
   ```go
   styleOpts.ColumnFormats["amount"] = "#,##0.00"
   ```

5. **Add Validation**: Validate queries and parameters
   ```go
   if len(query) > 10000 {
       return errors.New("query too large")
   }
   ```

6. **Log Operations**: Log all export operations for audit trails
   ```go
   logger.WithField("filename", filename).Info("Excel export completed")
   ```

## Limitations and Solutions

| Limitation | Solution |
|-----------|----------|
| Large datasets | Use streaming and row limits |
| Complex queries | Pre-aggregate or use custom data source |
| Column constraints | Use dynamic field mapping |
| Memory usage | Process in batches |

---

For more information, see the [Advanced Features Overview](./index.md) or the [Excel Exporter Service documentation](../excel-exporter-service.md).
