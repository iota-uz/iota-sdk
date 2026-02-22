package agents

import (
	"context"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/kb"
	"github.com/iota-uz/iota-sdk/pkg/bichat/learning"
	"github.com/iota-uz/iota-sdk/pkg/bichat/permissions"
	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
	"github.com/iota-uz/iota-sdk/pkg/bichat/storage"
	"github.com/iota-uz/iota-sdk/pkg/bichat/tools/chart"
	"github.com/iota-uz/iota-sdk/pkg/bichat/tools/export"
	"github.com/iota-uz/iota-sdk/pkg/bichat/tools/hitl"
	toolkb "github.com/iota-uz/iota-sdk/pkg/bichat/tools/kb"
	toollearning "github.com/iota-uz/iota-sdk/pkg/bichat/tools/learning"
	toolsql "github.com/iota-uz/iota-sdk/pkg/bichat/tools/sql"
	"github.com/iota-uz/iota-sdk/pkg/bichat/tools/utility"
	"github.com/iota-uz/iota-sdk/pkg/composables"
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
	webFetchStorage       storage.FileStorage          // Optional storage for web_fetch save_to_artifacts
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

// WithWebFetchStorage sets storage used by web_fetch when save_to_artifacts=true.
func WithWebFetchStorage(fileStorage storage.FileStorage) BIAgentOption {
	return func(a *DefaultBIAgent) {
		a.webFetchStorage = fileStorage
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
			a.insightDepth = ""
			return
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
		model:    "gpt-5.2", // Default model (SOTA)
	}

	// Apply options first to configure dependencies
	for _, opt := range opts {
		opt(agent)
	}

	// Create schema adapters using the query executor
	schemaLister := bichatsql.NewQueryExecutorSchemaLister(executor,
		bichatsql.WithCountCacheTTL(10*time.Minute),
		bichatsql.WithCacheKeyFunc(tenantCacheKey),
	)
	schemaDescriber := bichatsql.NewQueryExecutorSchemaDescriber(executor)

	// Build core tools list with optional view access control
	agentTools := []agents.Tool{
		utility.NewGetCurrentTimeTool(),
		utility.NewWebFetchTool(utility.WithWebFetchStorage(agent.webFetchStorage)),
		toolsql.NewSchemaListTool(schemaLister, toolsql.WithSchemaListViewAccess(agent.viewAccess)),
		toolsql.NewSchemaDescribeTool(schemaDescriber, toolsql.WithSchemaDescribeViewAccess(agent.viewAccess)),
		toolsql.NewSQLExecuteTool(executor, toolsql.WithViewAccessControl(agent.viewAccess)),
		export.NewRenderTableTool(executor),
		export.NewExportQueryToExcelTool(executor),
		chart.NewDrawChartTool(),
		hitl.NewAskUserQuestionTool(),
	}

	// Add optional tools based on configuration
	if agent.kbSearcher != nil {
		agentTools = append(agentTools, toolkb.NewKBSearchTool(toolkb.NewKBSearcherAdapter(agent.kbSearcher)))
	}

	if agent.learningStore != nil {
		agentTools = append(agentTools,
			toollearning.NewSearchLearningsTool(agent.learningStore),
			toollearning.NewSaveLearningTool(agent.learningStore),
		)
	}

	if agent.validatedQueryStore != nil {
		agentTools = append(agentTools,
			toollearning.NewSearchValidatedQueriesTool(agent.validatedQueryStore),
			toollearning.NewSaveValidatedQueryTool(agent.validatedQueryStore),
		)
	}

	if agent.enableCodeInterpreter {
		agentTools = append(agentTools, utility.NewCodeInterpreterTool())
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
Your mission is to help users analyze data, generate reports, and answer business questions.`
	prompt += `

RESPONSE BEHAVIOR:
- Summarize findings clearly and concisely; highlight key insights and trends.
- Use plain English business terms only — never expose technical names (table names, column slugs, schema prefixes, internal IDs) to the user.
- Format monetary values with appropriate units and separators.
- Never expose sensitive data, credentials, or internal technical identifiers.
- Ask questions when uncertain rather than making assumptions.
- Empower users with data insights: provide context, explanations, and actionable recommendations.`

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

// tenantCacheKey extracts the tenant ID from context as a cache key string.
func tenantCacheKey(ctx context.Context) (string, error) {
	tid, err := composables.UseTenantID(ctx)
	if err != nil {
		return "", err
	}
	return tid.String(), nil
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
