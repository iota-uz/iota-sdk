package agents

import (
	_ "embed"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/storage"
	"github.com/iota-uz/iota-sdk/pkg/bichat/tools"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

//go:embed excel_agent.prompt
var excelAgentPrompt string

// ExcelAgent is a specialized agent for working with spreadsheet attachments.
// It focuses on reading attachment artifacts and producing spreadsheet-specific summaries.
type ExcelAgent struct {
	*agents.BaseAgent
	artifactReaderTool agents.Tool
	model              string
}

// ExcelAgentOption is a functional option for configuring ExcelAgent.
type ExcelAgentOption func(*ExcelAgent)

// WithExcelAgentModel sets the LLM model for the Excel agent.
func WithExcelAgentModel(model string) ExcelAgentOption {
	return func(a *ExcelAgent) {
		a.model = model
	}
}

// WithExcelAgentArtifactReaderTool sets a custom artifact reader tool.
func WithExcelAgentArtifactReaderTool(tool agents.Tool) ExcelAgentOption {
	return func(a *ExcelAgent) {
		a.artifactReaderTool = tool
	}
}

// NewExcelAgent creates a new Excel specialist agent with the specified options.
// chatRepo and fileStorage are required so the agent can read session attachments.
func NewExcelAgent(
	chatRepo domain.ChatRepository,
	fileStorage storage.FileStorage,
	opts ...ExcelAgentOption,
) (*ExcelAgent, error) {
	const op serrors.Op = "NewExcelAgent"

	if chatRepo == nil {
		return nil, serrors.E(op, serrors.KindValidation, "chat repository is required")
	}
	if fileStorage == nil {
		return nil, serrors.E(op, serrors.KindValidation, "file storage is required")
	}

	agent := &ExcelAgent{
		model: "gpt-5.2-2025-12-11", // Default model
	}

	for _, opt := range opts {
		opt(agent)
	}

	if agent.artifactReaderTool == nil {
		agent.artifactReaderTool = tools.NewArtifactReaderTool(chatRepo, fileStorage)
	}

	agentTools := []agents.Tool{
		agent.artifactReaderTool,
		tools.NewAskUserQuestionTool(),
	}

	agent.BaseAgent = agents.NewBaseAgent(
		agents.WithName("excel-analyst"),
		agents.WithDescription("Specialized agent for spreadsheet attachments and large attachment-driven analysis"),
		agents.WithWhenToUse("Use when users upload large spreadsheets or attachments and need structured analysis, validation, and summarization"),
		agents.WithModel(agent.model),
		agents.WithTools(agentTools...),
		agents.WithSystemPrompt(excelAgentPrompt),
		agents.WithTerminationTools(agents.ToolFinalAnswer),
	)

	return agent, nil
}
