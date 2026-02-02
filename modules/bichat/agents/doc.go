// Package agents provides default agent implementations for the BiChat module.
//
// The default BI agent (DefaultBIAgent) is a fully-configured Business Intelligence
// agent with access to SQL querying, schema exploration, charting, and optional
// knowledge base search and export capabilities.
//
// # Usage
//
// Basic usage with only SQL capabilities:
//
//	executor := tools.NewDefaultQueryExecutor(dbPool)
//	agent, err := NewDefaultBIAgent(executor)
//	if err != nil {
//	    // handle error
//	}
//
// With optional knowledge base search:
//
//	kbSearcher := myapp.NewKBSearcher()
//	agent, err := NewDefaultBIAgent(
//	    executor,
//	    WithKBSearcher(kbSearcher),
//	)
//
// With export tools:
//
//	import "github.com/iota-uz/iota-sdk/pkg/bichat/storage"
//	import "github.com/iota-uz/iota-sdk/pkg/bichat/tools"
//
//	fileStorage, _ := storage.NewLocalFileStorage("/var/lib/bichat/exports", "https://example.com/exports")
//	excelTool := tools.NewExportToExcelTool(
//	    tools.WithOutputDir("/var/lib/bichat/exports"),
//	    tools.WithBaseURL("https://example.com/exports"),
//	)
//	pdfTool := tools.NewExportToPDFTool("http://gotenberg:3000", fileStorage)
//
//	agent, err := NewDefaultBIAgent(
//	    executor,
//	    WithKBSearcher(kbSearcher),
//	    WithExportTools(excelTool, pdfTool),
//	    WithModel("gpt-3.5-turbo"),
//	    WithCodeInterpreter(true),
//	    WithAgentRegistry(registry),
//	)
//
// # Tools Available
//
// Core tools (always available):
//   - get_current_time: Get current date/time for relative date queries
//   - schema_list: List all database tables
//   - schema_describe: Get detailed table schema information
//   - sql_execute: Execute read-only SQL queries
//   - draw_chart: Create chart visualizations
//   - ask_user_question: Ask clarifying questions (HITL)
//
// Optional tools (configured via options):
//   - kb_search: Search knowledge base (requires WithKBSearcher)
//   - export_data_to_excel: Export query results to Excel (requires WithExportTools)
//   - export_to_pdf: Export content to PDF (requires WithExportTools)
//   - code_interpreter: Execute Python for data analysis (requires WithCodeInterpreter)
//   - task: Delegate to specialized sub-agents (requires WithAgentRegistry)
//
// # Agent Configuration
//
// The agent is configured with:
//   - Name: "bi_agent"
//   - Model: "gpt-4" (default, customizable via WithModel)
//   - Isolation: Isolated (no access to parent context)
//   - Termination: final_answer tool
//
// # System Prompt
//
// The agent comes with a comprehensive system prompt that guides the LLM through:
//   - Understanding user requests
//   - Exploring database schemas
//   - Writing safe, read-only SQL queries
//   - Visualizing data with charts
//   - Exporting results
//   - Providing clear, actionable answers
//
// The system prompt includes safety constraints to ensure:
//   - Only SELECT queries are allowed
//   - Results are limited to 1000 rows maximum
//   - Query timeout is 30 seconds
//   - Table/column names are validated before use
package agents
