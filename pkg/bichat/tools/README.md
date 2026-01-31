# BI-Chat Tools

Reusable tools for BI-Chat agents, implementing the `agents.Tool` interface.

## Overview

This package provides common tools that agents can use to interact with databases, knowledge bases, and external services. All tools follow a provider-agnostic design, accepting dependencies via constructors.

## Available Tools

### SQL Tools

#### SQLExecuteTool
Execute read-only SQL queries against a database.

```go
executor := tools.NewDefaultQueryExecutor(pool)
sqlTool := tools.NewSQLExecuteTool(executor)
```

Features:
- Validates queries are read-only (SELECT only)
- Enforces row limits (max 1000 rows)
- 30-second query timeout
- Returns formatted JSON results

#### SchemaListTool
List all tables and views in a schema.

```go
schemaListTool := tools.NewSchemaListTool(executor)
```

#### SchemaDescribeTool
Get detailed schema information for a table.

```go
schemaDescribeTool := tools.NewSchemaDescribeTool(executor)
```

Returns:
- Column names and types
- Constraints
- Indexes
- Sample values

### Utility Tools

#### GetCurrentTimeTool
Get current time in various formats and timezones.

```go
timeTool := tools.NewGetCurrentTimeTool()
```

Returns:
- Current date and time (RFC3339)
- Date, time components
- Day of week, quarter, week of year
- Configurable timezone support

#### DrawChartTool
Generate chart specifications for visualization.

```go
chartTool := tools.NewDrawChartTool()
```

Supported chart types:
- Line, bar, area
- Pie, donut

Features:
- Max 1000 data points per series
- Hex color validation
- Configurable height (100-1000px)
- Returns specification (not rendered chart)

### Knowledge Base

#### KBSearchTool
Search knowledge base for relevant documents.

```go
searcher := kb.NewBleveSearcher(indexPath)
kbTool := tools.NewKBSearchTool(searcher)
```

Features:
- Full-text search with relevance scoring
- Configurable result limit (max 20)
- Returns excerpts and metadata

### Export Tools

#### ExportToExcelTool
Export query results to Excel format.

```go
exporter := tools.NewDefaultExcelExporter("/exports", "https://example.com/exports")
excelTool := tools.NewExportToExcelTool(exporter)
```

Features:
- Formatted headers with styling
- Auto-column sizing
- Returns download URL

#### ExportToPDFTool
Export HTML content to PDF via Gotenberg.

```go
pdfTool := tools.NewExportToPDFTool("http://gotenberg:3000")
```

Features:
- HTML to PDF conversion
- Landscape/portrait support
- Returns download URL

### HITL (Human-in-the-Loop)

#### AskUserQuestionTool
Ask user for clarification (special interrupt tool).

```go
// Register as interrupt handler
handler := tools.NewAskUserQuestionHandler()
executor.RegisterInterruptHandler(agents.ToolAskUserQuestion, handler)

// Tool is available to agent
questionTool := tools.NewAskUserQuestionTool()
```

Question types:
- Text input
- Single choice
- Multiple choice

## Custom Implementations

### Custom Query Executor

```go
type CustomQueryExecutor struct {
    db *sql.DB
}

func (e *CustomQueryExecutor) ExecuteQuery(ctx context.Context, sql string, params []any, timeoutMs int) (*tools.QueryResult, error) {
    // Custom implementation with tenant isolation, logging, etc.
}

executor := &CustomQueryExecutor{db: db}
sqlTool := tools.NewSQLExecuteTool(executor)
```

### Custom KB Searcher

```go
type CustomKBSearcher struct {
    index *bleve.Index
}

func (s *CustomKBSearcher) Search(ctx context.Context, query string, limit int) ([]tools.SearchResult, error) {
    // Custom search with boosting, filtering, etc.
}

func (s *CustomKBSearcher) IsAvailable() bool {
    return s.index != nil
}

searcher := &CustomKBSearcher{index: index}
kbTool := tools.NewKBSearchTool(searcher)
```

### Custom Excel Exporter

```go
type S3ExcelExporter struct {
    s3Client *s3.Client
    bucket   string
}

func (e *S3ExcelExporter) ExportToExcel(ctx context.Context, data *tools.QueryResult, filename string) (string, error) {
    // Generate Excel and upload to S3
    // Return presigned URL
}

exporter := &S3ExcelExporter{s3Client: client, bucket: "exports"}
excelTool := tools.NewExportToExcelTool(exporter)
```

## HITL Workflow

### 1. Register Interrupt Handler

```go
handler := tools.NewAskUserQuestionHandler()
executor.RegisterInterruptHandler(agents.ToolAskUserQuestion, handler)
```

### 2. Agent Calls Tool

Agent decides to ask user for clarification:

```json
{
  "question": "Which date range would you like to analyze?",
  "question_type": "single_choice",
  "choices": [
    {"id": "last_month", "label": "Last Month"},
    {"id": "last_quarter", "label": "Last Quarter"},
    {"id": "last_year", "label": "Last Year"}
  ]
}
```

### 3. Execution Pauses

Executor saves checkpoint and yields interrupt event:

```go
for {
    event, err, hasMore := gen.Next()
    if !hasMore { break }

    if event.Type == agents.EventTypeInterrupt {
        // Display question to user
        question := event.InterruptData
        checkpointID := event.CheckpointID
    }
}
```

### 4. Resume with Answer

User provides answer, resume execution:

```go
gen := executor.Resume(ctx, checkpointID, "last_quarter")
for {
    event, err, hasMore := gen.Next()
    if !hasMore { break }
    // Continue processing
}
```

## Security Best Practices

### SQL Injection Prevention

- Table/column names validated with regex: `^[a-zA-Z_][a-zA-Z0-9_]*$`
- Queries validated to be SELECT-only
- Dangerous keywords blacklisted (INSERT, UPDATE, DELETE, etc.)
- Parameterized queries recommended

### Resource Limits

- Query results: 1000 rows max
- Chart data: 1000 points per series max
- KB search: 20 results max
- Query timeout: 30 seconds default

### Tenant Isolation

Consumers should implement tenant isolation in custom executors:

```go
func (e *TenantAwareExecutor) ExecuteQuery(ctx context.Context, sql string, params []any, timeoutMs int) (*tools.QueryResult, error) {
    tenantID := composables.UseTenantID(ctx)

    // Ensure query includes tenant_id filter
    sql = e.addTenantFilter(sql, tenantID)

    return e.pool.Query(ctx, sql, params...)
}
```

## Dependencies

Required for all tools:
- `github.com/iota-uz/iota-sdk/pkg/bichat/agents`
- `github.com/iota-uz/iota-sdk/pkg/serrors`

SQL tools:
- `github.com/jackc/pgx/v5`
- `github.com/jackc/pgx/v5/pgxpool`

Excel export:
- `github.com/xuri/excelize/v2`

## Testing

See [testing guide](../../../.claude/guides/backend/testing.md) for testing patterns.

Example tool test:

```go
func TestSQLExecuteTool_Call(t *testing.T) {
    t.Parallel()

    // Setup
    executor := &mockQueryExecutor{
        result: &tools.QueryResult{
            Columns: []string{"id", "name"},
            Rows: []map[string]interface{}{
                {"id": 1, "name": "test"},
            },
            RowCount: 1,
        },
    }

    tool := tools.NewSQLExecuteTool(executor)

    // Execute
    input := `{"query": "SELECT * FROM users"}`
    result, err := tool.Call(context.Background(), input)

    // Assert
    assert.NoError(t, err)
    assert.Contains(t, result, "\"id\"")
    assert.Contains(t, result, "test")
}
```

## Examples

See reference implementations:
- Shyona: `/Users/diyorkhaydarov/Projects/sdk/shy-trucks/core/modules/shyona/tools/`
- Ali: `/Users/diyorkhaydarov/Projects/sdk/eai/back/modules/ali/agents/tools/`
