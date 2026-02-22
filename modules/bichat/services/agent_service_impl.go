package services

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	bichatctx "github.com/iota-uz/iota-sdk/pkg/bichat/context"
	"github.com/iota-uz/iota-sdk/pkg/bichat/context/codecs"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/hooks"
	"github.com/iota-uz/iota-sdk/pkg/bichat/hooks/events"
	"github.com/iota-uz/iota-sdk/pkg/bichat/schema"
	"github.com/iota-uz/iota-sdk/pkg/bichat/services"
	bichatskills "github.com/iota-uz/iota-sdk/pkg/bichat/skills"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/sirupsen/logrus"
)

const (
	defaultSkillsCatalogLimit = 50
	defaultSkillsMaxChars     = 8000
)

// agentServiceImpl is the production implementation of AgentService.
// It bridges the chat domain with the Agent Framework, handling context building,
// agent execution, and event streaming.
type agentServiceImpl struct {
	agent                  agents.ExtendedAgent
	model                  agents.Model
	policy                 bichatctx.ContextPolicy
	renderer               bichatctx.Renderer
	checkpointer           agents.Checkpointer
	eventBus               hooks.EventBus
	chatRepo               domain.ChatRepository
	agentRegistry          *agents.AgentRegistry   // Optional for multi-agent delegation
	schemaMetadata         schema.MetadataProvider // Optional for table metadata
	projectPromptExtension string
	skillsCatalog          *bichatskills.Catalog
	skillsCatalogLimit     int
	skillsMaxChars         int
	runtimeTools           []agents.Tool
	logger                 *logrus.Logger
	formatterRegistry      *bichatctx.FormatterRegistry // Optional for StructuredTool support
}

// AgentServiceConfig holds configuration for creating an AgentService.
type AgentServiceConfig struct {
	Agent                  agents.ExtendedAgent
	Model                  agents.Model
	Policy                 bichatctx.ContextPolicy
	Renderer               bichatctx.Renderer
	Checkpointer           agents.Checkpointer
	EventBus               hooks.EventBus          // Required
	ChatRepo               domain.ChatRepository   // Repository for loading messages
	AgentRegistry          *agents.AgentRegistry   // Optional for multi-agent delegation
	SchemaMetadata         schema.MetadataProvider // Optional for table metadata
	ProjectPromptExtension string
	SkillsCatalog          *bichatskills.Catalog
	SkillsCatalogLimit     int
	SkillsMaxChars         int
	RuntimeTools           []agents.Tool
	Logger                 *logrus.Logger
	FormatterRegistry      *bichatctx.FormatterRegistry // Optional for StructuredTool support
}

// NewAgentService creates a production implementation of AgentService.
//
// Example:
//
//	service := NewAgentService(AgentServiceConfig{
//	    Agent:        baseAgent,
//	    Model:        model,
//	    Policy:       policy,
//	    Renderer:     renderer,
//	    Checkpointer: checkpointer,
//	    EventBus:     eventBus,
//	    ChatRepo:     chatRepo,
//	    AgentRegistry: agentRegistry,  // Optional for multi-agent
//	    SchemaMetadata: schemaProvider, // Optional for table metadata
//	})
func NewAgentService(cfg AgentServiceConfig) services.AgentService {
	eventBus := cfg.EventBus
	if eventBus == nil {
		eventBus = hooks.NewEventBus()
	}
	logger := cfg.Logger
	if logger == nil {
		logger = logrus.New()
	}
	limit := cfg.SkillsCatalogLimit
	if limit <= 0 {
		limit = defaultSkillsCatalogLimit
	}
	maxChars := cfg.SkillsMaxChars
	if maxChars <= 0 {
		maxChars = defaultSkillsMaxChars
	}
	return &agentServiceImpl{
		agent:                  cfg.Agent,
		model:                  cfg.Model,
		policy:                 cfg.Policy,
		renderer:               cfg.Renderer,
		checkpointer:           cfg.Checkpointer,
		eventBus:               eventBus,
		chatRepo:               cfg.ChatRepo,
		agentRegistry:          cfg.AgentRegistry,
		schemaMetadata:         cfg.SchemaMetadata,
		projectPromptExtension: strings.TrimSpace(cfg.ProjectPromptExtension),
		skillsCatalog:          cfg.SkillsCatalog,
		skillsCatalogLimit:     limit,
		skillsMaxChars:         maxChars,
		runtimeTools:           append([]agents.Tool(nil), cfg.RuntimeTools...),
		logger:                 logger,
		formatterRegistry:      cfg.FormatterRegistry,
	}
}

// ProcessMessage executes the agent for a user message and returns streaming events.
// This method:
//  1. Builds context using the Context Builder (system prompt, history, user message)
//  2. Compiles context with Renderer and Policy
//  3. Creates an Executor and runs the agent
//  4. Returns a Generator for streaming events to the caller
func (s *agentServiceImpl) ProcessMessage(
	ctx context.Context,
	sessionID uuid.UUID,
	content string,
	attachments []domain.Attachment,
) (types.Generator[agents.ExecutorEvent], error) {
	const op serrors.Op = "agentServiceImpl.ProcessMessage"
	ctx = agents.WithRuntimeSessionID(ctx, sessionID)

	// Get tenant ID for multi-tenant isolation
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	session, err := s.chatRepo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	// Load session messages from repository
	opts := domain.ListOptions{Limit: 100, Offset: 0}
	sessionMessages, err := s.chatRepo.GetSessionMessages(ctx, sessionID, opts)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	// Build context graph using Context Builder
	builder := bichatctx.NewBuilder()

	// 1. System prompt (KindPinned)
	systemPrompt := s.agent.SystemPrompt(ctx)
	if s.projectPromptExtension != "" {
		if systemPrompt == "" {
			systemPrompt = "PROJECT DOMAIN EXTENSION:\n" + s.projectPromptExtension
		} else {
			systemPrompt = systemPrompt + "\n\nPROJECT DOMAIN EXTENSION:\n" + s.projectPromptExtension
		}
	}
	if services.UseDebugMode(ctx) {
		debugPrompt := `DEBUG MODE ENABLED:
You are assisting a developer in diagnostic mode. Provide complete and explicit technical reasoning.
- Include exact tool calls you make (tool name, call id, arguments, and outcomes).
- Include exact SQL queries you execute and explain why each query is needed.
- Explain intermediate reasoning steps, assumptions, and error handling decisions.
- Do not hide implementation details behind business-safe summaries.`
		if systemPrompt == "" {
			systemPrompt = debugPrompt
		} else {
			systemPrompt = strings.TrimSpace(systemPrompt + "\n\n" + debugPrompt)
		}
	}
	if systemPrompt != "" {
		systemCodec := codecs.NewSystemRulesCodec()
		builder.System(systemCodec, systemPrompt)
	}

	// 1.5. Schema metadata (KindReference) - if provider configured
	if s.schemaMetadata != nil {
		metadata, err := s.schemaMetadata.ListMetadata(ctx)
		if err == nil && len(metadata) > 0 {
			metadataCodec := codecs.NewSchemaMetadataCodec()
			builder.Reference(metadataCodec, codecs.SchemaMetadataPayload{Tables: metadata})
		}
	}
	if reference := s.buildSkillsCatalogReference(); reference != "" {
		builder.Reference(codecs.NewSystemRulesCodec(), reference)
	}

	// 2. Session history (KindHistory)
	if len(sessionMessages) > 0 {
		historyCodec := codecs.NewConversationHistoryCodec()
		historyPayload := convertToHistoryPayload(sessionMessages)
		builder.History(historyCodec, historyPayload)
	}

	// 3. Current user turn (KindTurn)
	turnCodec := codecs.NewTurnCodec()
	turnPayload := codecs.TurnPayload{
		Content: content,
	}
	// Add attachments if present
	if len(attachments) > 0 {
		turnPayload.Attachments = codecs.ConvertAttachmentsToTurnAttachments(convertToTypeAttachments(attachments))
	}
	builder.Turn(turnCodec, turnPayload)

	// Compile with renderer and policy
	compiled, err := builder.Compile(s.renderer, s.policy)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	// Emit context.compile event
	// Convert TokensByKind map keys from BlockKind to string
	tokensByKindStr := make(map[string]int)
	for kind, tokens := range compiled.TokensByKind {
		tokensByKindStr[string(kind)] = tokens
	}

	event := events.NewContextCompileEvent(
		sessionID,
		tenantID,
		s.renderer.Provider(),
		compiled.TotalTokens,
		tokensByKindStr,
		len(compiled.Messages),
		compiled.Compacted,
		compiled.Truncated,
		compiled.ExcludedBlocks,
	)
	_ = s.eventBus.Publish(ctx, event)

	// Use compiled.Messages directly (now canonical []types.Message)
	executorMessages := compiled.Messages

	executor := s.buildExecutor(sessionID, tenantID)

	// Execute agent and get event generator
	input := agents.Input{
		Messages:           executorMessages,
		SessionID:          sessionID,
		TenantID:           tenantID,
		PreviousResponseID: session.LLMPreviousResponseID(),
	}

	// Return executor generator directly — no conversion needed.
	return executor.Execute(ctx, input), nil
}

// ResumeWithAnswer resumes agent execution after user answers questions (HITL).
// This method:
//  1. Loads the checkpoint via the checkpointer
//  2. Creates an Executor with restored state
//  3. Calls executor.Resume() with the user's answers
//  4. Returns a Generator for streaming the resumed execution
func (s *agentServiceImpl) ResumeWithAnswer(
	ctx context.Context,
	sessionID uuid.UUID,
	checkpointID string,
	answers map[string]types.Answer,
) (types.Generator[agents.ExecutorEvent], error) {
	const op serrors.Op = "agentServiceImpl.ResumeWithAnswer"
	ctx = agents.WithRuntimeSessionID(ctx, sessionID)

	// Validate inputs
	if checkpointID == "" {
		return nil, serrors.E(op, serrors.KindValidation, "checkpointID is required")
	}

	// Get tenant ID for multi-tenant isolation (validates it exists in context)
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	executor := s.buildExecutor(sessionID, tenantID)

	// Return resume generator directly — no conversion needed.
	return executor.Resume(ctx, checkpointID, answers), nil
}

// buildExecutor creates an Executor with shared options (delegation, formatter, checkpointer).
// Used by both ProcessMessage and ResumeWithAnswer to avoid duplicating setup logic.
func (s *agentServiceImpl) buildExecutor(sessionID, tenantID uuid.UUID) *agents.Executor {
	opts := []agents.ExecutorOption{
		agents.WithCheckpointer(s.checkpointer),
		agents.WithEventBus(s.eventBus),
		agents.WithMaxIterations(100),
	}

	if s.formatterRegistry != nil {
		opts = append(opts, agents.WithFormatterRegistry(s.formatterRegistry))
	}

	if tools := s.composeExecutorTools(sessionID, tenantID); len(tools) > 0 {
		opts = append(opts, agents.WithExecutorTools(tools))
	}

	return agents.NewExecutor(s.agent, s.model, opts...)
}

func (s *agentServiceImpl) buildSkillsCatalogReference() string {
	if s.skillsCatalog == nil {
		return ""
	}
	return bichatskills.RenderCatalogReference(
		s.skillsCatalog,
		s.skillsCatalogLimit,
		s.skillsMaxChars,
	)
}

func (s *agentServiceImpl) composeExecutorTools(sessionID, tenantID uuid.UUID) []agents.Tool {
	hasDelegationTargets := s.agentRegistry != nil && len(s.agentRegistry.All()) > 0
	if len(s.runtimeTools) == 0 && !hasDelegationTargets {
		return nil
	}

	tools := append([]agents.Tool(nil), s.agent.Tools()...)
	tools = append(tools, s.runtimeTools...)
	if hasDelegationTargets {
		tools = append(tools, agents.NewDelegationTool(
			s.agentRegistry,
			s.model,
			sessionID,
			tenantID,
		))
	}
	return deduplicateToolsByName(tools)
}

func deduplicateToolsByName(tools []agents.Tool) []agents.Tool {
	if len(tools) == 0 {
		return nil
	}

	// Preserve order while preferring later definitions (runtime overrides base).
	reversed := make([]agents.Tool, 0, len(tools))
	seen := make(map[string]struct{}, len(tools))

	for i := len(tools) - 1; i >= 0; i-- {
		tool := tools[i]
		if tool == nil {
			continue
		}
		name := strings.TrimSpace(tool.Name())
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		reversed = append(reversed, tool)
	}

	result := make([]agents.Tool, len(reversed))
	for i := range reversed {
		result[i] = reversed[len(reversed)-1-i]
	}
	return result
}

// convertToHistoryPayload converts types.Message slice to ConversationHistoryPayload
func convertToHistoryPayload(messages []types.Message) codecs.ConversationHistoryPayload {
	historyMessages := make([]codecs.ConversationMessage, 0, len(messages))
	for _, msg := range messages {
		// Skip messages with empty content (e.g., HITL question messages)
		// These messages have question_data in the database but should not
		// be included in the conversation history sent to the LLM
		if msg.Content() == "" {
			continue
		}
		historyMessages = append(historyMessages, codecs.ConversationMessage{
			Role:    string(msg.Role()),
			Content: msg.Content(),
		})
	}
	return codecs.ConversationHistoryPayload{
		Messages: historyMessages,
	}
}

// convertToTypeAttachments converts domain attachments to types.Attachment slice
func convertToTypeAttachments(domainAttachments []domain.Attachment) []types.Attachment {
	result := make([]types.Attachment, len(domainAttachments))
	for i, a := range domainAttachments {
		result[i] = types.Attachment{
			ID:        a.ID(),
			MessageID: a.MessageID(),
			UploadID:  a.UploadID(),
			FileName:  a.FileName(),
			MimeType:  a.MimeType(),
			SizeBytes: a.SizeBytes(),
			FilePath:  a.FilePath(),
			CreatedAt: a.CreatedAt(),
		}
	}
	return result
}
