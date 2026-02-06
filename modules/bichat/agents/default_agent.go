package agents

import (
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
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
	kbSearcher            tools.KBSearcher
	exportTools           []agents.Tool // Optional export tools (Excel, PDF)
	model                 string        // Store model separately to apply during initialization
	enableCodeInterpreter bool
	agentRegistry         *agents.AgentRegistry         // Optional registry for multi-agent delegation
	viewAccess            permissions.ViewAccessControl // Optional view permission control for SQL
}

// BIAgentOption is a functional option for configuring DefaultBIAgent.
type BIAgentOption func(*DefaultBIAgent)

// WithKBSearcher adds knowledge base search capability to the agent.
func WithKBSearcher(searcher tools.KBSearcher) BIAgentOption {
	return func(a *DefaultBIAgent) {
		a.kbSearcher = searcher
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

// WithViewAccessControl enables permission-based view access control for SQL execution.
// When configured, schema_list, schema_describe, and sql_execute tools will validate
// user permissions against analytics schema views before execution.
func WithViewAccessControl(vac permissions.ViewAccessControl) BIAgentOption {
	return func(a *DefaultBIAgent) {
		a.viewAccess = vac
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
		tools.NewDrawChartTool(),
		tools.NewAskUserQuestionTool(),
	}

	// Add optional tools based on configuration
	if agent.kbSearcher != nil {
		agentTools = append(agentTools, tools.NewKBSearchTool(agent.kbSearcher))
	}

	if agent.enableCodeInterpreter {
		agentTools = append(agentTools, tools.NewCodeInterpreterTool())
	}

	// Add export tools if provided
	if len(agent.exportTools) > 0 {
		agentTools = append(agentTools, agent.exportTools...)
	}

	// Build system prompt with optional registry information
	systemPrompt := buildBISystemPrompt(agent.enableCodeInterpreter, agent.agentRegistry)

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
func buildBISystemPrompt(codeInterpreter bool, registry *agents.AgentRegistry) string {
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

IMPORTANT CONSTRAINTS:
- All SQL queries MUST be read-only (SELECT or WITH...SELECT)
- Results are limited to 1000 rows maximum
- Query timeout is 30 seconds
- Always validate table/column names using schema tools first
- Never expose sensitive data or credentials
- Ask questions when uncertain rather than making assumptions

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

	return prompt
}
