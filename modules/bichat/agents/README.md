# BiChat Agents

This package provides default agent implementations for the BiChat module.

## Overview

The **DefaultBIAgent** is a fully-configured Business Intelligence agent that provides:

- **SQL Querying**: Execute read-only SELECT queries against your database
- **Schema Exploration**: List tables and describe their structure
- **Data Visualization**: Create charts (line, bar, pie, area, donut)
- **Knowledge Base Search** (optional): Search documentation and business rules
- **Data Export** (optional): Export results to Excel or PDF
- **Human-in-the-Loop**: Ask clarifying questions when needed

## Quick Start

### Basic Usage

```go
import (
    bichatagents "github.com/iota-uz/iota-sdk/modules/bichat/agents"
    "github.com/iota-uz/iota-sdk/pkg/bichat/tools"
)

// Create a query executor
executor := tools.NewDefaultQueryExecutor(dbPool)

// Create the default BI agent
agent, err := bichatagents.NewDefaultBIAgent(executor)
if err != nil {
    // handle error
}
```

### With Optional Features

```go
// Add knowledge base search
agent, err := bichatagents.NewDefaultBIAgent(
    executor,
    bichatagents.WithKBSearcher(kbSearcher),
)

// Add Excel export
agent, err := bichatagents.NewDefaultBIAgent(
    executor,
    bichatagents.WithExcelExporter(excelExporter),
)

// Use a different model
agent, err := bichatagents.NewDefaultBIAgent(
    executor,
    bichatagents.WithModel("gpt-3.5-turbo"),
)

// Combine multiple options
agent, err := bichatagents.NewDefaultBIAgent(
    executor,
    bichatagents.WithKBSearcher(kbSearcher),
    bichatagents.WithExcelExporter(excelExporter),
    bichatagents.WithPDFExporter(pdfExporter),
    bichatagents.WithModel("gpt-4"),
)
```

## Available Tools

### Core Tools (Always Available)

- **get_current_time**: Get current date/time for relative date queries
- **schema_list**: List all database tables with metadata
- **schema_describe**: Get detailed table schema (columns, types, indexes)
- **sql_execute**: Execute read-only SQL queries (max 1000 rows)
- **draw_chart**: Create chart visualizations
- **ask_user_question**: Ask clarifying questions (HITL)

### Optional Tools

- **kb_search**: Search knowledge base (requires `WithKBSearcher`)
- **export_to_excel**: Export results to Excel (requires `WithExcelExporter`)
- **export_to_pdf**: Export content to PDF (requires `WithPDFExporter`)

## Configuration Options

### WithKBSearcher

Add knowledge base search capability:

```go
type MyKBSearcher struct{}

func (s *MyKBSearcher) Search(ctx context.Context, query string, limit int) ([]tools.SearchResult, error) {
    // Your implementation
}

func (s *MyKBSearcher) IsAvailable() bool {
    return true
}

agent, _ := bichatagents.NewDefaultBIAgent(
    executor,
    bichatagents.WithKBSearcher(&MyKBSearcher{}),
)
```

### WithExcelExporter

Add Excel export capability:

```go
type MyExcelExporter struct{}

func (e *MyExcelExporter) ExportToExcel(ctx context.Context, data *tools.QueryResult, filename string) (string, error) {
    // Your implementation
    return "/exports/file.xlsx", nil
}

agent, _ := bichatagents.NewDefaultBIAgent(
    executor,
    bichatagents.WithExcelExporter(&MyExcelExporter{}),
)
```

### WithModel

Customize the LLM model:

```go
agent, _ := bichatagents.NewDefaultBIAgent(
    executor,
    bichatagents.WithModel("gpt-3.5-turbo"),
)
```

## Agent Configuration

The agent is configured with the following metadata:

- **Name**: `bi_agent`
- **Description**: Business Intelligence assistant with SQL and KB access
- **WhenToUse**: Use for data analysis, reporting, and BI queries
- **Model**: `gpt-4` (default, customizable)
- **Isolation**: Isolated (no access to parent context)
- **Termination**: `final_answer` tool

## System Prompt

The agent comes with a comprehensive system prompt that guides the LLM through:

1. **Understanding Requests**: Parse user questions and ask for clarification if needed
2. **Exploring Schemas**: List tables and describe structures before querying
3. **Writing Safe SQL**: Only SELECT queries, with validation and timeouts
4. **Visualizing Data**: Choose appropriate chart types based on data
5. **Exporting Results**: Offer Excel/PDF export for further analysis
6. **Providing Answers**: Clear, actionable insights with context

### Safety Constraints

- Only SELECT and WITH...SELECT queries allowed
- Results limited to 1000 rows maximum
- Query timeout is 30 seconds
- Table/column names validated before use
- No exposure of sensitive data or credentials

## Testing

The package includes comprehensive tests with 100% coverage:

```bash
go test ./modules/bichat/agents -v
go test ./modules/bichat/agents -cover
```

## Integration with BiChat Module

This agent is designed to be used with the BiChat module's service layer:

```go
import (
    bichatagents "github.com/iota-uz/iota-sdk/modules/bichat/agents"
    "github.com/iota-uz/iota-sdk/pkg/bichat/agents"
)

// Create agent
biAgent, _ := bichatagents.NewDefaultBIAgent(executor)

// Register with agent registry (if using multi-agent system)
registry := agents.NewAgentRegistry()
registry.Register(biAgent)

// Use with executor
executor := agents.NewExecutor(biAgent, modelClient)
result, err := executor.Execute(ctx, "Show me sales trends for last quarter")
```

## Examples

See `example_test.go` for runnable examples:

```bash
go test ./modules/bichat/agents -run ^Example
```

## License

Part of the IOTA SDK project.
