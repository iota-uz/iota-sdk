# BiChat Agents

Default agent implementations for the BiChat module.

## DefaultBIAgent

Pre-configured BI agent with SQL, schema, visualization, and HITL tools.

### Quick Start

```go
import bichatagents "github.com/iota-uz/iota-sdk/modules/bichat/agents"

executor := toolsql.NewDefaultQueryExecutor(dbPool)
agent, err := bichatagents.NewDefaultBIAgent(executor)
```

### With Optional Features

```go
import (
    bichatagents "github.com/iota-uz/iota-sdk/modules/bichat/agents"
    "github.com/iota-uz/iota-sdk/pkg/bichat/storage"
    "github.com/iota-uz/iota-sdk/pkg/bichat/tools/export"
)

// Create file storage for exports
fileStorage, _ := storage.NewLocalFileStorage("/var/lib/bichat/exports", "https://example.com/exports")

// Create export tools
excelTool := export.NewExportToExcelTool(
    export.WithOutputDir("/var/lib/bichat/exports"),
    export.WithBaseURL("https://example.com/exports"),
)
pdfTool := export.NewExportToPDFTool("http://gotenberg:3000", fileStorage)

// Create agent with optional features
agent, _ := bichatagents.NewDefaultBIAgent(
    executor,
    bichatagents.WithKBSearcher(kbSearcher),           // Add KB search
    bichatagents.WithExportTools(excelTool, pdfTool),  // Add Excel/PDF export
    bichatagents.WithModel("gpt-5.2"),                   // Custom model
    bichatagents.WithCodeInterpreter(true),            // Enable Python execution
    bichatagents.WithAgentRegistry(registry),          // Multi-agent delegation
)
```

## Available Tools

**Core (always enabled)**:
- `get_current_time` - Current date/time
- `schema_list` - List database tables
- `schema_describe` - Describe table schema
- `sql_execute` - Execute read-only SQL (max 1000 rows, 30s timeout)
- `draw_chart` - Create chart visualizations
- `ask_user_question` - HITL questions

**Optional**:
- `kb_search` - Requires `WithKBSearcher(kbSearcher)`
- `export_data_to_excel` - Requires `WithExportTools(excelTool, ...)`
- `export_to_pdf` - Requires `WithExportTools(..., pdfTool)`
- `code_interpreter` - Requires `WithCodeInterpreter(true)`
- `task` - Requires `WithAgentRegistry(registry)` for delegation

## Configuration

- **Name**: `bi_agent`
- **Model**: `gpt-5.2` (customizable)
- **Isolation**: Isolated (no parent context access)
- **Completion**: implicit stop (model returns no tool calls)

## Safety Constraints

- Only SELECT and WITH...SELECT queries allowed
- Results limited to 1000 rows
- Query timeout: 30 seconds
- Table/column names validated before execution
- No INSERT/UPDATE/DELETE via sql_execute

## Testing

```bash
go test ./modules/bichat/agents -v
go test ./modules/bichat/agents -cover
```

## Integration

```go
biAgent, _ := bichatagents.NewDefaultBIAgent(executor)

// Register with registry
registry := agents.NewAgentRegistry()
registry.Register(biAgent)

// Use with executor
executor := agents.NewExecutor(biAgent, modelClient)
result, err := executor.Execute(ctx, "Show sales trends")
```
