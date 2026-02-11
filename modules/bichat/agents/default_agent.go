package agents

import (
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/kb"
	"github.com/iota-uz/iota-sdk/pkg/bichat/learning"
	"github.com/iota-uz/iota-sdk/pkg/bichat/permissions"
	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
	"github.com/iota-uz/iota-sdk/pkg/bichat/tools"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// DefaultBIAgent is the default Business Intelligence agent with all BI tools wired.
// It provides SQL querying, schema exploration, charting, and optional export capabilities.
type DefaultBIAgent struct {
	*agents.BaseAgent
	executor              bichatsql.QueryExecutor
	kbSearcher            kb.KBSearcher
	learningStore         learning.LearningStore       // Optional learning store for dynamic learnings
	validatedQueryStore   learning.ValidatedQueryStore // Optional validated query store for query library
	exportTools           []agents.Tool                // Optional export tools (Excel, PDF)
	artifactReaderTool    agents.Tool                  // Optional artifact reader tool
	model                 string                       // Store model separately to apply during initialization
	enableCodeInterpreter bool
	agentRegistry         *agents.AgentRegistry         // Optional registry for multi-agent delegation
	viewAccess            permissions.ViewAccessControl // Optional view permission control for SQL
	insightDepth          string                        // Insight prompting depth: "", "brief", "standard", "detailed"
}

// BIAgentOption is a functional option for configuring DefaultBIAgent.
type BIAgentOption func(*DefaultBIAgent)

// WithKBSearcher adds knowledge base search capability to the agent.
func WithKBSearcher(searcher kb.KBSearcher) BIAgentOption {
	return func(a *DefaultBIAgent) {
		a.kbSearcher = searcher
	}
}

// WithLearningStore adds dynamic learning capability to the agent.
// When configured, the agent can save and retrieve learnings from SQL errors,
// type mismatches, and user corrections to avoid repeating mistakes.
func WithLearningStore(store learning.LearningStore) BIAgentOption {
	return func(a *DefaultBIAgent) {
		a.learningStore = store
	}
}

// WithValidatedQueryStore adds validated query library capability to the agent.
// When configured, the agent can search and save validated SQL query patterns
// to reuse proven solutions for similar questions.
func WithValidatedQueryStore(store learning.ValidatedQueryStore) BIAgentOption {
	return func(a *DefaultBIAgent) {
		a.validatedQueryStore = store
	}
}

// WithModel sets the LLM model for the agent.
func WithModel(model string) BIAgentOption {
	return func(a *DefaultBIAgent) {
		a.model = model
	}
}

// WithCodeInterpreter enables code interpreter capability for Python execution.
// This uses OpenAI's code_interpreter tool to execute Python code in a sandboxed environment.
func WithCodeInterpreter(enabled bool) BIAgentOption {
	return func(a *DefaultBIAgent) {
		a.enableCodeInterpreter = enabled
	}
}

// WithAgentRegistry sets the agent registry for multi-agent delegation.
// If provided, the agent can use the 'task' tool to delegate to specialized sub-agents.
func WithAgentRegistry(registry *agents.AgentRegistry) BIAgentOption {
	return func(a *DefaultBIAgent) {
		a.agentRegistry = registry
	}
}

// WithExportTools adds export tools (Excel, PDF) to the agent.
// This accepts pre-configured export tools that will be appended to the agent's tool list.
func WithExportTools(exportTools ...agents.Tool) BIAgentOption {
	return func(a *DefaultBIAgent) {
		a.exportTools = append(a.exportTools, exportTools...)
	}
}

// WithArtifactReaderTool adds the artifact_reader tool to the agent when configured.
func WithArtifactReaderTool(tool agents.Tool) BIAgentOption {
	return func(a *DefaultBIAgent) {
		a.artifactReaderTool = tool
	}
}

// WithViewAccessControl enables permission-based view access control for SQL execution.
// When configured, schema_list, schema_describe, and sql_execute tools will validate
// user permissions against analytics schema views before execution.
func WithViewAccessControl(vac permissions.ViewAccessControl) BIAgentOption {
	return func(a *DefaultBIAgent) {
		a.viewAccess = vac
	}
}

// WithInsightPrompting enables insight-focused response prompting with the specified depth.
// Valid values: "" (disabled), "brief", "standard", "detailed".
//   - "brief": 2-3 sentence summary after data
//   - "standard": Key findings + trends + anomalies + comparison
//   - "detailed": Full analysis with recommendations and next steps
func WithInsightPrompting(depth string) BIAgentOption {
	return func(a *DefaultBIAgent) {
		validDepths := map[string]bool{
			"":         true,
			"brief":    true,
			"standard": true,
			"detailed": true,
		}
		if !validDepths[depth] {
			panic(fmt.Sprintf("invalid insight depth: %s (must be one of: \"\", \"brief\", \"standard\", \"detailed\")", depth))
		}
		a.insightDepth = depth
	}
}

// NewDefaultBIAgent creates a new default BI agent with the specified options.
// The executor parameter is required for SQL querying capabilities.
// Additional tools (KB search) can be added via options.
func NewDefaultBIAgent(
	executor bichatsql.QueryExecutor,
	opts ...BIAgentOption,
) (*DefaultBIAgent, error) {
	const op serrors.Op = "NewDefaultBIAgent"

	// Validate required parameters
	if executor == nil {
		return nil, serrors.E(op, serrors.KindValidation, "executor is required")
	}

	agent := &DefaultBIAgent{
		executor: executor,
		model:    "gpt-4", // Default model
	}

	// Apply options first to configure dependencies
	for _, opt := range opts {
		opt(agent)
	}

	// Create schema adapters using the query executor
	schemaLister := bichatsql.NewQueryExecutorSchemaLister(executor)
	schemaDescriber := bichatsql.NewQueryExecutorSchemaDescriber(executor)

	// Build core tools list with optional view access control
	agentTools := []agents.Tool{
		tools.NewGetCurrentTimeTool(),
		tools.NewSchemaListTool(schemaLister, tools.WithSchemaListViewAccess(agent.viewAccess)),
		tools.NewSchemaDescribeTool(schemaDescriber, tools.WithSchemaDescribeViewAccess(agent.viewAccess)),
		tools.NewSQLExecuteTool(executor, tools.WithViewAccessControl(agent.viewAccess)),
		tools.NewExportQueryToExcelTool(executor),
		tools.NewDrawChartTool(),
		tools.NewAskUserQuestionTool(),
	}

	// Add optional tools based on configuration
	if agent.kbSearcher != nil {
		agentTools = append(agentTools, tools.NewKBSearchTool(tools.NewKBSearcherAdapter(agent.kbSearcher)))
	}

	if agent.learningStore != nil {
		agentTools = append(agentTools,
			tools.NewSearchLearningsTool(agent.learningStore),
			tools.NewSaveLearningTool(agent.learningStore),
		)
	}

	if agent.validatedQueryStore != nil {
		agentTools = append(agentTools,
			tools.NewSearchValidatedQueriesTool(agent.validatedQueryStore),
			tools.NewSaveValidatedQueryTool(agent.validatedQueryStore),
		)
	}

	if agent.enableCodeInterpreter {
		agentTools = append(agentTools, tools.NewCodeInterpreterTool())
	}
	if agent.artifactReaderTool != nil {
		agentTools = append(agentTools, agent.artifactReaderTool)
	}

	// Add export tools if provided
	if len(agent.exportTools) > 0 {
		agentTools = append(agentTools, agent.exportTools...)
	}

	// Build system prompt with optional registry information, learning store, validated query store, and insight depth
	systemPrompt := buildBISystemPrompt(
		agent.enableCodeInterpreter,
		agent.agentRegistry,
		agent.learningStore != nil,
		agent.validatedQueryStore != nil,
		agent.insightDepth,
		agent.artifactReaderTool != nil,
	)

	// Create base agent with configured model
	agent.BaseAgent = agents.NewBaseAgent(
		agents.WithName("bi_agent"),
		agents.WithDescription("Business Intelligence assistant with SQL and KB access"),
		agents.WithWhenToUse("Use for data analysis, reporting, and BI queries"),
		agents.WithModel(agent.model),
		agents.WithTools(agentTools...),
		agents.WithSystemPrompt(systemPrompt),
		agents.WithTerminationTools(agents.ToolFinalAnswer),
	)

	return agent, nil
}

// buildBISystemPrompt constructs the system prompt for the BI agent.
// If registry is provided, it appends available sub-agents information for delegation.
// If learningEnabled is true, it appends learning system instructions.
// If validatedQueryEnabled is true, it appends validated query library instructions.
// If insightDepth is set, it appends insight-focused response instructions.
func buildBISystemPrompt(codeInterpreter bool, registry *agents.AgentRegistry, learningEnabled bool, validatedQueryEnabled bool, insightDepth string, artifactReaderEnabled bool) string {
	prompt := `You are a Business Intelligence assistant with access to a SQL database and knowledge base.
Your mission is to help users analyze data, generate reports, and answer business questions.
`
	prompt += `

WORKFLOW GUIDELINES:
1. UNDERSTAND THE REQUEST
   - For date-related queries, use get_current_time first to establish context
   - If requirements are ambiguous, use ask_user_question to clarify before querying
   - Break down complex questions into simpler sub-questions
   - If you need business rules/procedures and KB is available, use kb_search

2. EXPLORE THE SCHEMA
   - Always use schema_list to see available tables before writing SQL
   - Use schema_describe to understand table structure, relationships, and constraints
   - Pay attention to foreign keys and indexes for optimal query performance

3. WRITE SAFE SQL
   - Only SELECT queries are allowed (no INSERT, UPDATE, DELETE, DROP, etc.)
   - Validate queries before execution
   - Use appropriate JOINs based on foreign key relationships
   - Add LIMIT clauses for large result sets
   - Use indexes for WHERE and JOIN conditions when available

4. VISUALIZE DATA
   - Use draw_chart for data visualization when appropriate
   - Choose chart type based on data: line (trends), bar (comparisons), pie (proportions)
   - Pie and donut charts require exactly one series
   - Max 1000 data points per series

5. PROVIDE CLEAR ANSWERS
   - Summarize findings clearly and concisely
   - Highlight key insights and trends
   - Use charts to make data more understandable
   - Call final_answer with your complete response
   - Use plain English business terms only — NEVER expose technical names (table names, column slugs, schema prefixes, internal IDs) to the user
   - Format monetary values with appropriate units and separators

SEARCHING BY USER-PROVIDED DATA:
Users often provide approximate or misspelled names, IDs, or keywords.
Match strategy depends on the identifier type:

Structured identifiers (UUIDs, order IDs, ISO codes, enum values, and other canonical IDs):
- ALWAYS use exact equality (=). These are machine-generated and must match precisely.

Soft human-facing identifiers (names, addresses, policy numbers, license plates, nicknames):
- Use ILIKE with wildcards: WHERE name ILIKE '%ali%'
- For names, search by parts: WHERE last_name ILIKE '%alibaev%' (ignore first name typos)
- For codes/numbers (policy numbers, license plates), use ILIKE or LIKE with wildcards for partial matches
- similarity() requires the pg_trgm extension; only use it if the database supports it.
  Example: WHERE similarity(name, 'input') > 0.3 ORDER BY similarity DESC

General rules:
- When a query returns 0 rows but the user clearly expects results, try a broader search
- If you find close but not exact matches, use ask_user_question to confirm: "Did you mean X?"
- NEVER tell the user "no results found" without first trying at least one broader/fuzzy search

RESOLVE-THEN-QUERY PATTERN:
When a user refers to an entity by name or partial identifier, first resolve it to a concrete ID:
1. Search broadly (ILIKE/wildcards) to find matching records with their IDs
2. If multiple matches, use ask_user_question to let the user pick
3. Once resolved, use the concrete ID (exact equality) for all subsequent queries in the conversation
This avoids repeated fuzzy matching and ensures consistency across follow-up questions.

IMPORTANT CONSTRAINTS:
- All SQL queries MUST be read-only (SELECT or WITH...SELECT)
- Results are limited to 1000 rows maximum
- Query timeout is 30 seconds
- Always validate table/column names using schema tools first
- Never expose sensitive data, credentials, or internal technical identifiers to users
- Ask questions when uncertain rather than making assumptions

ERROR RECOVERY:
When SQL execution fails, the error response includes a structured diagnosis with code, table, column, and suggestions.
Follow these recovery steps based on error code (max 2 retries before asking user for help):

1. COLUMN_NOT_FOUND: Column does not exist
   - Use schema_describe on the referenced table to verify available columns
   - Fix column name (check for typos, case sensitivity, or different naming)
   - Retry with corrected column name

2. TABLE_NOT_FOUND: Table does not exist
   - Use schema_list to find available tables
   - Identify correct table name (check for typos, schema prefix, or different naming)
   - Retry with corrected table name

3. TYPE_MISMATCH: Column type does not match expected type
   - Use schema_describe to check actual column types
   - Add appropriate type cast (e.g., column_name::text, column_name::integer)
   - Retry with type-corrected query

4. SYNTAX_ERROR: Invalid SQL syntax
   - Review syntax for missing commas, parentheses, or keywords
   - Fix syntax errors
   - Retry with corrected syntax

5. AMBIGUOUS_COLUMN: Column exists in multiple tables
   - Qualify ambiguous column with table alias (e.g., t1.column_name)
   - Retry with qualified column references

If error persists after 2 retries, explain the issue to the user and ask for clarification.

EXAMPLE WORKFLOW:
User: "Show me sales trends for last quarter"
1. get_current_time (establish current date and quarter)
2. schema_list (find sales-related tables)
3. schema_describe (understand sales table structure)
4. sql_execute (query sales data for last quarter)
5. draw_chart (visualize trend as line chart)
6. final_answer (summarize findings with chart)

Remember: You are here to empower users with data insights, not just execute queries.
Provide context, explanations, and actionable recommendations based on the data.`

	if artifactReaderEnabled {
		prompt += `

ATTACHMENT ANALYSIS:
- When user files are attached, inspect them with artifact_reader before answering.
- Use artifact_reader action="list" to discover available artifacts in the current session.
- Use artifact_reader action="read" with artifact_id (or artifact_name) to inspect file content.
- For chart artifacts, use mode="spec" to read chart metadata/spec.`
	}

	// Append available agents for delegation if registry is configured
	if registry != nil && len(registry.All()) > 0 {
		prompt += "\n\n" + registry.Describe()
		prompt += `

DELEGATION GUIDELINES:
- Use the 'task' tool to delegate complex SQL tasks to specialized agents
- Provide clear instructions and context in the delegation prompt
- The sub-agent will execute independently and return results
- Use delegation for multi-step database queries or complex schema analysis
- After receiving results from sub-agent, you can visualize with draw_chart or provide insights`
	}

	// Append learning system instructions if enabled
	if learningEnabled {
		prompt += `

LEARNING SYSTEM:
Search and save learnings to avoid repeating mistakes across conversations.

1. BEFORE SQL: Use search_learnings to check known patterns, gotchas, and fixes for relevant tables
2. AFTER ERRORS: Use save_learning with the error pattern and solution
   - Categories: sql_error, type_mismatch, user_correction, business_rule
   - Include table_name and sql_patch when applicable
3. Learnings persist across conversations — your saves help future queries succeed on first try`
	}

	// Append validated query library instructions if enabled
	if validatedQueryEnabled {
		prompt += `

QUERY LIBRARY:
Search and save validated SQL query patterns to reuse proven solutions.

1. BEFORE writing SQL: Use search_validated_queries with question keywords or table names
2. AFTER successful queries: Use save_validated_query for queries answering meaningful business questions
   - Include clear question, summary, tables used, and data quality notes
3. DO NOT save: Simple single-table lookups, one-off exploratory queries, or queries with errors
4. Saved queries grow the library — future conversations start with proven patterns`
	}

	if learningEnabled || validatedQueryEnabled {
		prompt += `

PRE-QUERY RETRIEVAL POLICY:
Before your first SQL attempt for each user question:
1. If query library is available: call search_validated_queries to find proven SQL patterns
2. If learning system is available: call search_learnings for known errors, type gotchas, and fixes
3. Use the retrieved context to shape your first SQL query
Do not skip this retrieval phase unless the question clearly needs no SQL.`
	}

	// Append insight-focused response instructions if enabled
	if insightDepth != "" {
		prompt += "\n\n" + buildInsightInstructions(insightDepth)
	}

	return prompt
}

// buildInsightInstructions generates insight-focused response guidelines based on depth level.
func buildInsightInstructions(depth string) string {
	base := `INSIGHT-FOCUSED RESPONSES:
Your answers should go beyond raw data presentation to provide business intelligence.
Always interpret query results in business context and highlight actionable insights.

CORE PRINCIPLES:
- Ground all observations in actual query results (no hallucination)
- Compare values when meaningful (vs previous period, vs average, vs target)
- Identify trends, patterns, and anomalies
- Explain what the data means for business decisions
`

	switch depth {
	case "brief":
		return base + `
BRIEF MODE (2-3 sentences):
After presenting data or charts, add a concise summary:
- One key finding or trend
- One comparison or context point
- Keep it actionable and grounded in results

Example: "Sales increased 15% vs last quarter, driven primarily by Product A.
This outpaces the industry average of 8% growth."`

	case "standard":
		return base + `
STANDARD MODE (structured analysis):
After presenting data, provide:
1. KEY FINDINGS: 2-3 most important observations from the data
2. TRENDS: Notable patterns over time or across categories
3. ANOMALIES: Unexpected values or outliers worth investigating
4. COMPARISONS: How results compare to baselines, targets, or prior periods

Keep each section to 1-2 sentences. Focus on business implications.

Example:
"KEY FINDINGS: Revenue grew 15% YoY with Product A leading growth. Customer retention improved to 92%.
TRENDS: Growth accelerating in Q3-Q4 vs Q1-Q2. Enterprise segment outpacing SMB.
ANOMALIES: Region APAC declined 5% despite overall growth - investigate market conditions.
COMPARISONS: Performance exceeds target by 8pp and industry benchmark by 12pp."`

	case "detailed":
		return base + `
DETAILED MODE (comprehensive analysis):
After presenting data, provide:
1. EXECUTIVE SUMMARY: One-paragraph overview of key findings
2. TRENDS & PATTERNS: Detailed analysis of changes over time or across segments
3. ANOMALIES & RISKS: Unexpected results and potential concerns
4. COMPARISONS & CONTEXT: Performance vs benchmarks, targets, prior periods
5. RECOMMENDATIONS: 2-3 actionable next steps based on the data
6. SUGGESTED ANALYSIS: Additional queries that could deepen understanding

Each section should be concise but thorough (2-4 sentences). Prioritize business value.

Example:
"EXECUTIVE SUMMARY: Q4 revenue grew 18% YoY to $5.2M, exceeding targets by 12%.
Growth driven by Product A (35% YoY) and strong enterprise segment performance (42% YoY).

TRENDS & PATTERNS: Monthly growth accelerating from 10% in Jan to 22% in Dec.
Enterprise segment consistently outperforming SMB by 2.5x. Product A market share increased from 22% to 31%.

ANOMALIES & RISKS: APAC region declined 8% despite global growth - primarily due to two major
customer churns in Q3. Marketing spend efficiency dropped 15% in Nov-Dec.

COMPARISONS & CONTEXT: Performance exceeds industry benchmark (6% YoY) by 3x.
Retention rate (92%) now matches top quartile. Customer acquisition cost up 20% vs last year.

RECOMMENDATIONS:
1. Investigate APAC customer churn root causes and implement retention program
2. Audit Q4 marketing spend to identify efficiency drop drivers
3. Expand Product A investment given strong market traction

SUGGESTED ANALYSIS:
- Customer cohort retention by acquisition channel
- Product A vs B profitability breakdown
- APAC customer satisfaction scores over past 6 months"`

	default:
		return ""
	}
}

// DefaultBISystemPromptOpts configures which optional sections are included in the default BI system prompt.
// Use this when building a custom parent agent that should share the SDK's base BI instructions.
type DefaultBISystemPromptOpts struct {
	CodeInterpreter       bool
	AgentRegistry         *agents.AgentRegistry
	LearningEnabled       bool
	ValidatedQueryEnabled bool
	InsightDepth          string // "", "brief", "standard", "detailed"
	ArtifactReaderEnabled bool
}

// DefaultBISystemPrompt returns the default Business Intelligence system prompt used by the SDK's DefaultBIAgent.
// Consumers (e.g. EAI) can use this for custom parent agents and append project-specific guidance via
// the project prompt extension in AgentServiceConfig / ModuleConfig.
func DefaultBISystemPrompt(opts DefaultBISystemPromptOpts) string {
	return buildBISystemPrompt(
		opts.CodeInterpreter,
		opts.AgentRegistry,
		opts.LearningEnabled,
		opts.ValidatedQueryEnabled,
		opts.InsightDepth,
		opts.ArtifactReaderEnabled,
	)
}
