# Excel Exporter Service Guide

This guide explains how to use the Excel Exporter Service in the IOTA SDK to export data from your application to Excel files.

## Overview

The Excel Exporter Service provides a convenient way to:
- Export SQL query results directly to Excel files
- Use custom data sources for Excel generation
- Automatically manage file uploads through the Upload Service
- Apply custom styling and formatting options

## Basic Usage

### 1. Service Injection

The ExcelExportService is automatically registered in the core module. You can inject it into your controllers or services:

```go
type MyController struct {
    excelService *services.ExcelExportService
}

func NewMyController(excelService *services.ExcelExportService) *MyController {
    return &MyController{
        excelService: excelService,
    }
}
```

### 2. Export from SQL Query

The simplest way to export data is using a SQL query:

```go
func (c *MyController) ExportUsers(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    // Export active users to Excel
    upload, err := c.excelService.ExportFromQuery(
        ctx,
        "SELECT id, name, email, created_at FROM users WHERE active = $1",
        "active_users", // filename (without .xlsx)
        true, // query parameter
    )
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    // Redirect to download URL
    http.Redirect(w, r, upload.URL().String(), http.StatusSeeOther)
}
```

### 3. Export with Custom Options

For more control over the export format:

```go
func (c *MyController) ExportSalesReport(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    // Configure export options
    exportOpts := &excel.ExportOptions{
        IncludeHeaders: true,
        AutoFilter:     true,
        FreezeHeader:   true,
        DateFormat:     "2006-01-02",
        MaxRows:        10000, // Limit to 10k rows
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
    
    // Export with options
    upload, err := c.excelService.ExportFromQueryWithOptions(
        ctx,
        `SELECT 
            order_id,
            customer_name,
            product_name,
            quantity,
            unit_price,
            total_amount,
            order_date
        FROM sales_orders
        WHERE order_date BETWEEN $1 AND $2
        ORDER BY order_date DESC`,
        "sales_report",
        exportOpts,
        styleOpts,
        startDate,
        endDate,
    )
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    // Return download URL as JSON
    response := map[string]string{
        "downloadUrl": upload.URL().String(),
        "filename":    upload.Name(),
    }
    json.NewEncoder(w).Encode(response)
}
```

### 4. Export from Custom Data Source

For complex data transformations or non-SQL sources:

```go
// Implement custom data source
type ReportDataSource struct {
    reportService *ReportService
    reportType    string
}

func (r *ReportDataSource) GetHeaders() []string {
    return []string{"Month", "Revenue", "Expenses", "Profit", "Growth %"}
}

func (r *ReportDataSource) GetSheetName() string {
    return "Financial Summary"
}

func (r *ReportDataSource) GetRows(ctx context.Context) (func() ([]interface{}, error), error) {
    // Fetch report data
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
            fmt.Sprintf("%.2f%%", row.GrowthRate),
        }, nil
    }, nil
}

// Use in controller
func (c *MyController) ExportFinancialReport(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    // Create custom data source
    datasource := &ReportDataSource{
        reportService: c.reportService,
        reportType:    r.URL.Query().Get("type"),
    }
    
    // Export using custom data source
    upload, err := c.excelService.ExportFromDataSource(
        ctx,
        datasource,
        "financial_report",
        nil, // use default export options
        excel.DefaultStyleOptions(), // use default styling
    )
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    // Return result
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "success": true,
        "upload": map[string]string{
            "id":   fmt.Sprintf("%d", upload.ID()),
            "url":  upload.URL().String(),
            "name": upload.Name(),
        },
    })
}
```

## Advanced Examples

### Dynamic Column Selection

```go
func (c *MyController) ExportCustomReport(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    // Parse selected columns from request
    selectedColumns := r.Form["columns[]"]
    
    // Build dynamic query
    columns := strings.Join(selectedColumns, ", ")
    query := fmt.Sprintf("SELECT %s FROM products WHERE category = $1", columns)
    
    upload, err := c.excelService.ExportFromQuery(
        ctx,
        query,
        "custom_product_report",
        r.URL.Query().Get("category"),
    )
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    // Return download link
    fmt.Fprintf(w, "Download your report: %s", upload.URL())
}
```

### Batch Export with Progress

```go
func (c *MyController) ExportLargeDataset(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    // Configure for large dataset
    exportOpts := &excel.ExportOptions{
        IncludeHeaders: true,
        MaxRows:        50000, // Limit to 50k rows per file
    }
    
    // Export with pagination handled internally
    upload, err := c.excelService.ExportFromQueryWithOptions(
        ctx,
        "SELECT * FROM large_table ORDER BY id",
        "large_dataset",
        exportOpts,
        nil,
    )
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    // Return result
    json.NewEncoder(w).Encode(map[string]interface{}{
        "fileUrl": upload.URL().String(),
        "fileSize": upload.Size(),
    })
}
```

### Multi-Sheet Export

```go
// Custom data source supporting multiple sheets
type MultiSheetDataSource struct {
    sheets []SheetData
    currentSheet int
    currentRow int
}

type SheetData struct {
    Name    string
    Headers []string
    Rows    [][]interface{}
}

// Implement DataSource interface for each sheet...

// Then use multiple exports and combine:
func (c *MyController) ExportMultiSheetReport(w http.ResponseWriter, r *http.Request) {
    // Note: Current implementation exports single sheets
    // For multi-sheet, export multiple files or extend the Excel package
    
    sheets := []struct{
        query string
        name  string
    }{
        {"SELECT * FROM sales", "sales"},
        {"SELECT * FROM inventory", "inventory"},
        {"SELECT * FROM customers", "customers"},
    }
    
    var uploads []upload.Upload
    for _, sheet := range sheets {
        upload, err := c.excelService.ExportFromQuery(
            r.Context(),
            sheet.query,
            sheet.name,
        )
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        uploads = append(uploads, upload)
    }
    
    // Return all download links
    var urls []string
    for _, u := range uploads {
        urls = append(urls, u.URL().String())
    }
    
    json.NewEncoder(w).Encode(map[string]interface{}{
        "files": urls,
    })
}
```

## Integration with HTMX

For HTMX-based UIs, trigger downloads seamlessly:

```go
func (c *MyController) ExportWithHTMX(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    // Export data
    upload, err := c.excelService.ExportFromQuery(
        ctx,
        "SELECT * FROM orders WHERE status = $1",
        "pending_orders",
        "pending",
    )
    if err != nil {
        if htmx.IsHxRequest(r) {
            htmx.SetTrigger(w, "export-error", `{"message": "`+err.Error()+`"}`)
        }
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    // For HTMX requests, trigger download
    if htmx.IsHxRequest(r) {
        htmx.SetTrigger(w, "download-ready", `{"url": "`+upload.URL().String()+`", "filename": "`+upload.Name()+`"}`)
        w.WriteHeader(http.StatusNoContent)
        return
    }
    
    // For regular requests, redirect
    http.Redirect(w, r, upload.URL().String(), http.StatusSeeOther)
}
```

## Error Handling

Always handle errors appropriately:

```go
upload, err := excelService.ExportFromQuery(ctx, query, filename, args...)
if err != nil {
    switch {
    case errors.Is(err, context.Canceled):
        // Handle cancellation
        http.Error(w, "Export cancelled", http.StatusRequestTimeout)
    case strings.Contains(err.Error(), "failed to execute query"):
        // Database error
        http.Error(w, "Database error", http.StatusInternalServerError)
    case strings.Contains(err.Error(), "failed to save"):
        // Upload service error
        http.Error(w, "Failed to save file", http.StatusInternalServerError)
    default:
        // Generic error
        http.Error(w, "Export failed", http.StatusInternalServerError)
    }
    return
}
```

## Performance Tips

1. **Use Query Limits**: For large datasets, use SQL LIMIT or configure MaxRows
2. **Index Your Queries**: Ensure database queries are optimized with proper indexes
3. **Stream Large Results**: The service automatically streams data for memory efficiency
4. **Use Context Cancellation**: Pass request context for proper cancellation support
5. **Cache Common Exports**: Consider caching frequently requested exports

## Security Considerations

1. **Validate SQL Inputs**: Always use parameterized queries
2. **Check Permissions**: Verify user permissions before exporting sensitive data
3. **Limit Export Size**: Use MaxRows to prevent resource exhaustion
4. **Audit Exports**: Log export activities for compliance
5. **Secure URLs**: Upload URLs are secured by hash

## Troubleshooting

### Common Issues

1. **Out of Memory**: Reduce MaxRows or optimize query
2. **Slow Exports**: Add database indexes, limit columns
3. **File Not Found**: Check upload service configuration
4. **Permission Denied**: Verify upload directory permissions

### Debug Mode

Enable debug logging to troubleshoot issues:

```go
// In your service initialization
logger.SetLevel(logrus.DebugLevel)
```

## Complete Example

Here's a complete controller example:

```go
package controllers

import (
    "encoding/json"
    "net/http"
    "time"
    
    "github.com/iota-uz/iota-sdk/modules/core/services"
    "github.com/iota-uz/iota-sdk/pkg/excel"
)

type ReportController struct {
    excelService *services.ExcelExportService
}

func NewReportController(excelService *services.ExcelExportService) *ReportController {
    return &ReportController{
        excelService: excelService,
    }
}

func (c *ReportController) RegisterRoutes(router *http.ServeMux) {
    router.HandleFunc("/api/reports/export/users", c.ExportUsers)
    router.HandleFunc("/api/reports/export/sales", c.ExportSales)
}

func (c *ReportController) ExportUsers(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    // Parse filters
    status := r.URL.Query().Get("status")
    if status == "" {
        status = "active"
    }
    
    // Export with filters
    upload, err := c.excelService.ExportFromQuery(
        ctx,
        `SELECT 
            id,
            username,
            email,
            status,
            created_at,
            last_login_at
        FROM users 
        WHERE status = $1
        ORDER BY created_at DESC`,
        "users_" + status,
        status,
    )
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    // Return JSON response
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "success": true,
        "download_url": upload.URL().String(),
        "filename": upload.Name(),
        "size": upload.Size(),
    })
}

func (c *ReportController) ExportSales(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    // Parse date range
    startDate := r.URL.Query().Get("start_date")
    endDate := r.URL.Query().Get("end_date")
    
    if startDate == "" || endDate == "" {
        http.Error(w, "start_date and end_date are required", http.StatusBadRequest)
        return
    }
    
    // Configure styling for financial data
    styleOpts := &excel.StyleOptions{
        HeaderStyle: &excel.CellStyle{
            Font: &excel.FontStyle{Bold: true},
            Fill: &excel.FillStyle{
                Type: "pattern",
                Pattern: 1,
                Color: "#E3F2FD",
            },
        },
        AlternateRow: true,
    }
    
    // Export sales data
    upload, err := c.excelService.ExportFromQueryWithOptions(
        ctx,
        `SELECT 
            order_id,
            order_date,
            customer_name,
            product_name,
            quantity,
            unit_price,
            quantity * unit_price as total
        FROM sales_orders so
        JOIN customers c ON so.customer_id = c.id
        JOIN products p ON so.product_id = p.id
        WHERE order_date BETWEEN $1 AND $2
        ORDER BY order_date DESC`,
        fmt.Sprintf("sales_%s_to_%s", startDate, endDate),
        nil,
        styleOpts,
        startDate,
        endDate,
    )
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    // Return response
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "url": upload.URL().String(),
        "generated_at": time.Now().Format(time.RFC3339),
    })
}
```

## Next Steps

- Review the [Excel Package Documentation](/pkg/excel/README.md) for advanced DataSource implementations
- Check the [Upload Service Documentation](upload-service.md) for file management details
- See [Example Projects](../examples/) for more usage patterns