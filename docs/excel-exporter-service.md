# Excel Exporter Service Guide

The Excel Exporter Service provides functionality to export data from your application to Excel files.

## Overview

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

## Go Function Data Sources

The Excel exporter provides two powerful function-based data sources for generating custom Excel exports without requiring database queries.

### 1. FunctionDataSource

Use `FunctionDataSource` for dynamic data generation where you need to compute or fetch data programmatically:

```go
func (c *MyController) ExportComputedReport(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    // Define headers
    headers := []string{"Employee", "Department", "Base Salary", "Bonus", "Total Compensation"}
    
    // Create a function that generates data dynamically
    dataFunc := func(ctx context.Context) ([][]interface{}, error) {
        // Fetch base data from multiple sources
        employees, err := c.employeeService.GetActiveEmployees(ctx)
        if err != nil {
            return nil, err
        }
        
        var rows [][]interface{}
        for _, emp := range employees {
            // Calculate bonus based on business logic
            bonus := c.calculateBonus(emp.Performance, emp.Department)
            total := emp.BaseSalary + bonus
            
            row := []interface{}{
                emp.Name,
                emp.Department,
                emp.BaseSalary,
                bonus,
                total,
            }
            rows = append(rows, row)
        }
        
        return rows, nil
    }
    
    // Create data source with custom sheet name
    datasource := excel.NewFunctionDataSource(headers, dataFunc).
        WithSheetName("Compensation Report")
    
    // Configure export options
    config := exportconfig.New(
        exportconfig.WithFilename("compensation_report"),
        exportconfig.WithExportOptions(&excel.ExportOptions{
            IncludeHeaders: true,
            AutoFilter:     true,
            FreezeHeader:   true,
        }),
    )
    
    // Export using the data source
    upload, err := c.excelService.ExportFromDataSource(ctx, datasource, config)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    // Return download URL
    json.NewEncoder(w).Encode(map[string]interface{}{
        "downloadUrl": upload.URL().String(),
        "filename":    upload.Name(),
    })
}
```

### 2. SliceDataSource

Use `SliceDataSource` for static or pre-computed data:

```go
func (c *MyController) ExportStaticReport(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    // Prepare static data
    headers := []string{"Quarter", "Revenue", "Growth Rate", "Target Met"}
    data := [][]interface{}{
        {"Q1 2024", 1250000.00, "15.3%", "Yes"},
        {"Q2 2024", 1380000.00, "10.4%", "Yes"},
        {"Q3 2024", 1420000.00, "2.9%", "No"},
        {"Q4 2024", 1650000.00, "16.2%", "Yes"},
    }
    
    // Create slice data source
    datasource := excel.NewSliceDataSource(headers, data).
        WithSheetName("Quarterly Performance")
    
    // Configure with styling
    config := exportconfig.New(
        exportconfig.WithFilename("quarterly_report"),
        exportconfig.WithStyleOptions(&excel.StyleOptions{
            HeaderStyle: &excel.CellStyle{
                Font: &excel.FontStyle{
                    Bold: true,
                    Size: 12,
                },
                Fill: &excel.FillStyle{
                    Type:    "pattern",
                    Pattern: 1,
                    Color:   "#2196F3",
                },
            },
            AlternateRow: true,
        }),
    )
    
    // Export
    upload, err := c.excelService.ExportFromDataSource(ctx, datasource, config)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    // Return result
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{
        "url": upload.URL().String(),
    })
}
```

### 3. Complex Data Aggregation

Combine multiple data sources and complex business logic:

```go
func (c *MyController) ExportAggregatedMetrics(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    // Define comprehensive headers
    headers := []string{
        "Product", "Category", "Units Sold", "Revenue", 
        "Cost", "Profit", "Margin %", "Market Share %",
    }
    
    // Create function for complex data aggregation
    dataFunc := func(ctx context.Context) ([][]interface{}, error) {
        // Fetch data from multiple services
        products, err := c.productService.GetAllProducts(ctx)
        if err != nil {
            return nil, err
        }
        
        sales, err := c.salesService.GetProductSales(ctx, time.Now().AddDate(0, -1, 0))
        if err != nil {
            return nil, err
        }
        
        marketData, err := c.marketService.GetMarketShare(ctx)
        if err != nil {
            return nil, err
        }
        
        var rows [][]interface{}
        for _, product := range products {
            // Aggregate sales data
            salesData := sales[product.ID]
            if salesData == nil {
                continue // Skip products with no sales
            }
            
            // Calculate metrics
            revenue := salesData.UnitsSold * product.Price
            cost := salesData.UnitsSold * product.Cost
            profit := revenue - cost
            margin := (profit / revenue) * 100
            marketShare := marketData[product.ID]
            
            row := []interface{}{
                product.Name,
                product.Category,
                salesData.UnitsSold,
                revenue,
                cost,
                profit,
                fmt.Sprintf("%.2f%%", margin),
                fmt.Sprintf("%.2f%%", marketShare),
            }
            rows = append(rows, row)
        }
        
        // Sort by revenue (descending)
        sort.Slice(rows, func(i, j int) bool {
            return rows[i][3].(float64) > rows[j][3].(float64)
        })
        
        return rows, nil
    }
    
    // Create data source
    datasource := excel.NewFunctionDataSource(headers, dataFunc).
        WithSheetName("Product Performance Analysis")
    
    // Configure with advanced options
    config := exportconfig.New(
        exportconfig.WithFilename("product_analysis"),
        exportconfig.WithExportOptions(&excel.ExportOptions{
            IncludeHeaders: true,
            AutoFilter:     true,
            FreezeHeader:   true,
            MaxRows:        5000,
            DateFormat:     "2006-01-02",
        }),
        exportconfig.WithStyleOptions(&excel.StyleOptions{
            HeaderStyle: &excel.CellStyle{
                Font: &excel.FontStyle{
                    Bold: true,
                    Size: 11,
                },
                Fill: &excel.FillStyle{
                    Type:    "pattern",
                    Pattern: 1,
                    Color:   "#4CAF50",
                },
            },
            AlternateRow: true,
        }),
    )
    
    // Export
    upload, err := c.excelService.ExportFromDataSource(ctx, datasource, config)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    // Return detailed response
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "success":     true,
        "downloadUrl": upload.URL().String(),
        "filename":    upload.Name(),
        "size":        upload.Size(),
        "generatedAt": time.Now().Format(time.RFC3339),
    })
}
```

### 4. Time Series Data Export

Export time-based data with custom formatting:

```go
func (c *MyController) ExportTimeSeriesData(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    // Parse date range from request
    startDate := r.URL.Query().Get("start_date")
    endDate := r.URL.Query().Get("end_date")
    
    headers := []string{"Date", "Daily Users", "Page Views", "Bounce Rate", "Conversion Rate"}
    
    dataFunc := func(ctx context.Context) ([][]interface{}, error) {
        // Fetch analytics data
        analytics, err := c.analyticsService.GetDailyMetrics(ctx, startDate, endDate)
        if err != nil {
            return nil, err
        }
        
        var rows [][]interface{}
        for _, metric := range analytics {
            row := []interface{}{
                metric.Date.Format("2006-01-02"),
                metric.DailyUsers,
                metric.PageViews,
                fmt.Sprintf("%.2f%%", metric.BounceRate*100),
                fmt.Sprintf("%.2f%%", metric.ConversionRate*100),
            }
            rows = append(rows, row)
        }
        
        return rows, nil
    }
    
    datasource := excel.NewFunctionDataSource(headers, dataFunc).
        WithSheetName("Daily Analytics")
    
    config := exportconfig.New(
        exportconfig.WithFilename(fmt.Sprintf("analytics_%s_to_%s", startDate, endDate)),
        exportconfig.WithExportOptions(&excel.ExportOptions{
            IncludeHeaders: true,
            AutoFilter:     true,
            FreezeHeader:   true,
            DateFormat:     "2006-01-02",
        }),
    )
    
    upload, err := c.excelService.ExportFromDataSource(ctx, datasource, config)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    json.NewEncoder(w).Encode(map[string]interface{}{
        "downloadUrl": upload.URL().String(),
        "dateRange":   fmt.Sprintf("%s to %s", startDate, endDate),
    })
}
```

### 5. Error Handling and Context Cancellation

Both data sources properly handle context cancellation and errors:

```go
func (c *MyController) ExportWithErrorHandling(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    headers := []string{"ID", "Name", "Status"}
    
    dataFunc := func(ctx context.Context) ([][]interface{}, error) {
        // Check for context cancellation
        select {
        case <-ctx.Done():
            return nil, ctx.Err()
        default:
        }
        
        // Simulate long-running operation
        data, err := c.longRunningService.FetchData(ctx)
        if err != nil {
            return nil, fmt.Errorf("failed to fetch data: %w", err)
        }
        
        // Convert to Excel format
        var rows [][]interface{}
        for _, item := range data {
            // Check context again for large datasets
            select {
            case <-ctx.Done():
                return nil, ctx.Err()
            default:
            }
            
            row := []interface{}{item.ID, item.Name, item.Status}
            rows = append(rows, row)
        }
        
        return rows, nil
    }
    
    datasource := excel.NewFunctionDataSource(headers, dataFunc)
    
    config := exportconfig.New(
        exportconfig.WithFilename("processed_data"),
    )
    
    upload, err := c.excelService.ExportFromDataSource(ctx, datasource, config)
    if err != nil {
        switch {
        case errors.Is(err, context.Canceled):
            http.Error(w, "Export cancelled", http.StatusRequestTimeout)
        case errors.Is(err, context.DeadlineExceeded):
            http.Error(w, "Export timeout", http.StatusRequestTimeout)
        default:
            http.Error(w, "Export failed: "+err.Error(), http.StatusInternalServerError)
        }
        return
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{
        "downloadUrl": upload.URL().String(),
    })
}
```

### 6. Best Practices for Function Data Sources

```go
// ✅ Good practices
func createOptimizedDataSource() excel.DataSource {
    headers := []string{"ID", "Name", "Value"}
    
    dataFunc := func(ctx context.Context) ([][]interface{}, error) {
        const batchSize = 1000
        
        // Process in batches to avoid memory issues
        var allRows [][]interface{}
        offset := 0
        
        for {
            select {
            case <-ctx.Done():
                return nil, ctx.Err()
            default:
            }
            
            // Fetch batch
            batch, err := fetchBatch(ctx, offset, batchSize)
            if err != nil {
                return nil, err
            }
            
            if len(batch) == 0 {
                break // No more data
            }
            
            // Convert batch to rows
            for _, item := range batch {
                row := []interface{}{item.ID, item.Name, item.Value}
                allRows = append(allRows, row)
            }
            
            offset += batchSize
            
            // Limit total rows to prevent memory issues
            if len(allRows) >= 50000 {
                break
            }
        }
        
        return allRows, nil
    }
    
    return excel.NewFunctionDataSource(headers, dataFunc).
        WithSheetName("Optimized Data")
}

// ❌ Avoid loading all data at once without limits
func createProblematicDataSource() excel.DataSource {
    headers := []string{"ID", "Name"}
    
    dataFunc := func(ctx context.Context) ([][]interface{}, error) {
        // Don't do this - could load millions of rows
        allData, err := fetchAllDataFromDatabase(ctx)
        if err != nil {
            return nil, err
        }
        
        var rows [][]interface{}
        for _, item := range allData {
            row := []interface{}{item.ID, item.Name}
            rows = append(rows, row)
        }
        
        return rows, nil
    }
    
    return excel.NewFunctionDataSource(headers, dataFunc)
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
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{
        "downloadUrl": upload.URL().String(),
        "message": "Download your report",
    })
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
        "fileSize": upload.Size().String(),
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

## Performance Considerations

- Use SQL LIMIT or configure MaxRows for large datasets
- Ensure database queries are optimized with proper indexes
- The service automatically streams data for memory efficiency
- Pass request context for proper cancellation support

## Security Considerations

- Always use parameterized queries
- Verify user permissions before exporting sensitive data
- Use MaxRows to prevent resource exhaustion
- Upload URLs are secured by hash

## Troubleshooting

### Common Issues

- **Out of Memory**: Reduce MaxRows or optimize query
- **Slow Exports**: Add database indexes, limit columns
- **File Not Found**: Check upload service configuration
- **Permission Denied**: Verify upload directory permissions

## Complete Example

```go
package controllers

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

