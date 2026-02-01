package agents

import (
	_ "embed"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	bichatservices "github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/bichat/tools"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

//go:embed sql_agent.prompt
var sqlAgentPrompt string

// SQLAgent is a specialized agent for SQL query generation and database analysis.
// It provides schema exploration, query generation, and data analysis capabilities.
//
// The agent is isolated from parent context and uses only SQL-related tools:
//   - schema_list: List all database tables
//   - schema_describe: Get detailed schema information
//   - sql_execute: Execute read-only SQL queries
//   - final_answer: Return results to parent agent
//
// Usage:
//
//	executor := tools.NewDefaultQueryExecutor(dbPool)
//	sqlAgent, err := NewSQLAgent(executor)
//	if err != nil {
//	    return err
//	}
//
//	// Register in agent registry
//	registry := agents.NewAgentRegistry()
//	registry.Register(sqlAgent)
type SQLAgent struct {
	*agents.BaseAgent
	executor bichatservices.QueryExecutorService
	model    string
}

// SQLAgentOption is a functional option for configuring SQLAgent.
type SQLAgentOption func(*SQLAgent)

// WithSQLAgentModel sets the LLM model for the SQL agent.
func WithSQLAgentModel(model string) SQLAgentOption {
	return func(a *SQLAgent) {
		a.model = model
	}
}

// NewSQLAgent creates a new SQL analyst agent with the specified options.
// The executor parameter is required for database access.
func NewSQLAgent(
	executor bichatservices.QueryExecutorService,
	opts ...SQLAgentOption,
) (*SQLAgent, error) {
	const op serrors.Op = "NewSQLAgent"

	// Validate required parameters
	if executor == nil {
		return nil, serrors.E(op, serrors.KindValidation, "executor is required")
	}

	agent := &SQLAgent{
		executor: executor,
		model:    "gpt-4", // Default model
	}

	// Apply options
	for _, opt := range opts {
		opt(agent)
	}

	// Create adapter to bridge bichatservices.QueryExecutorService -> tools.QueryExecutorService
	toolsExecutor := newQueryExecutorAdapter(executor)

	// Build core tools list (SQL-specific only)
	agentTools := []agents.Tool{
		tools.NewSchemaListTool(toolsExecutor),
		tools.NewSchemaDescribeTool(toolsExecutor),
		tools.NewSQLExecuteTool(toolsExecutor),
	}

	// Create base agent with configured model
	agent.BaseAgent = agents.NewBaseAgent(
		agents.WithName("sql-analyst"),
		agents.WithDescription("Specialized agent for SQL query generation and database analysis"),
		agents.WithWhenToUse("Use when you need to generate SQL queries, analyze database schemas, or execute complex multi-step database queries"),
		agents.WithModel(agent.model),
		agents.WithTools(agentTools...),
		agents.WithSystemPrompt(sqlAgentPrompt),
		agents.WithTerminationTools(agents.ToolFinalAnswer),
	)

	return agent, nil
}
