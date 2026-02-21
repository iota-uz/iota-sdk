// Package tools provides reusable tools for BI-Chat agents.
//
// This package contains common tools that can be used by agents to interact with
// databases, knowledge bases, and external services. Tools implement the
// agents.StructuredTool interface (returning structured payloads) or the
// simpler agents.Tool interface for tools that return plain strings.
//
// # StructuredTool Pattern
//
// Most tools implement agents.StructuredTool, which adds a CallStructured
// method returning a ToolResult with a codec ID and typed payload. The
// executor's FormatterRegistry formats payloads into LLM-readable text.
// The Call method delegates to CallStructured for backward compatibility.
//
// Formatters live in pkg/bichat/context/formatters/ and are registered
// via formatters.DefaultFormatterRegistry().
//
// # Available Tools
//
// SQL Tools:
//   - SQLExecuteTool: Execute read-only SQL queries
//   - SchemaListTool: List all tables and views
//   - SchemaDescribeTool: Describe table schema details
//
// Utility Tools:
//   - GetCurrentTimeTool: Get current time in various formats
//   - DrawChartTool: Generate chart specifications
//   - render_table: Render interactive table payloads from SQL
//
// Knowledge Base:
//   - KBSearchTool: Search knowledge base for documents
//
// Export Tools:
//   - ExportToExcelTool: Export query results to Excel
//   - ExportToPDFTool: Export content to PDF via Gotenberg
//
// HITL (Human-in-the-Loop):
//   - AskUserQuestionTool: Ask user for clarification (triggers interrupt automatically)
//
// # Dependency Injection Pattern
//
// Tools accept dependencies via constructors, allowing consumers to provide
// custom implementations:
//
//	// SQL executor service
//	executor := tools.NewDefaultQueryExecutor(pool)
//	sqlTool := tools.NewSQLExecuteTool(executor)
//
//	// Knowledge base searcher
//	searcher := kb.NewBleveSearcher(indexPath)
//	kbTool := tools.NewKBSearchTool(searcher)
//
//	// Excel exporter
//	exporter := tools.NewDefaultExcelExporter("/exports", "https://example.com/exports")
//	excelTool := tools.NewExportToExcelTool(exporter)
//
// # Custom Implementations
//
// Consumers can implement custom versions of tool dependencies:
//
//	type CustomQueryExecutor struct {
//	    db *sql.DB
//	}
//
//	func (e *CustomQueryExecutor) ExecuteQuery(ctx context.Context, sql string, params []any, timeoutMs int) (*tools.QueryResult, error) {
//	    // Custom implementation
//	}
//
//	executor := &CustomQueryExecutor{db: db}
//	sqlTool := tools.NewSQLExecuteTool(executor)
//
// # HITL Interrupts
//
// The ask_user_question tool automatically triggers a HITL interrupt when called.
// Register it as a regular tool with the agent - the executor handles interrupts automatically.
//
// When the agent calls this tool, execution pauses and a checkpoint is saved.
// Resume execution with the user's answers (map from question ID to types.Answer):
//
//	answers := map[string]types.Answer{
//	    "question_1": types.NewAnswer("Q1 2024"),
//	    "question_2": types.NewAnswer("revenue"),
//	}
//	gen := executor.Resume(ctx, checkpointID, answers)
//	for {
//	    event, err := gen.Next(ctx)
//	    if err == types.ErrGeneratorDone { break }
//	    // Process event
//	}
//
// # Security Considerations
//
// - SQL tools validate queries are read-only (SELECT only)
// - Table/column names are validated against SQL injection
// - Query results are limited to prevent memory issues
// - Timeouts are enforced on all database operations
//
// # Performance Notes
//
// - SQL queries have a default 30-second timeout
// - Query results are limited to 1000 rows maximum
// - Chart data is limited to 1000 points per series
// - KB search results are limited to 20 documents maximum
package tools
