package services

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	bichatctx "github.com/iota-uz/iota-sdk/pkg/bichat/context"
	"github.com/iota-uz/iota-sdk/pkg/bichat/context/codecs"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/hooks"
	"github.com/iota-uz/iota-sdk/pkg/bichat/hooks/events"
	"github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// agentServiceImpl is the production implementation of AgentService.
// It bridges the chat domain with the Agent Framework, handling context building,
// agent execution, and event streaming.
type agentServiceImpl struct {
	agent         agents.ExtendedAgent
	model         agents.Model
	policy        bichatctx.ContextPolicy
	renderer      bichatctx.Renderer
	checkpointer  agents.Checkpointer
	eventBus      hooks.EventBus
	chatRepo      domain.ChatRepository
	agentRegistry *agents.AgentRegistry // Optional for multi-agent delegation
}

// AgentServiceConfig holds configuration for creating an AgentService.
type AgentServiceConfig struct {
	Agent         agents.ExtendedAgent
	Model         agents.Model
	Policy        bichatctx.ContextPolicy
	Renderer      bichatctx.Renderer
	Checkpointer  agents.Checkpointer
	EventBus      hooks.EventBus        // Optional
	ChatRepo      domain.ChatRepository // Repository for loading messages
	AgentRegistry *agents.AgentRegistry // Optional for multi-agent delegation
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
//	})
func NewAgentService(cfg AgentServiceConfig) services.AgentService {
	return &agentServiceImpl{
		agent:         cfg.Agent,
		model:         cfg.Model,
		policy:        cfg.Policy,
		renderer:      cfg.Renderer,
		checkpointer:  cfg.Checkpointer,
		eventBus:      cfg.EventBus,
		chatRepo:      cfg.ChatRepo,
		agentRegistry: cfg.AgentRegistry,
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
) (types.Generator[services.Event], error) {
	const op serrors.Op = "agentServiceImpl.ProcessMessage"

	// Get tenant ID for multi-tenant isolation
	tenantID, err := composables.UseTenantID(ctx)
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
	if systemPrompt != "" {
		systemCodec := codecs.NewSystemRulesCodec()
		builder.System(systemCodec, systemPrompt)
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
	if s.eventBus != nil {
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
	}

	// Use compiled.Messages directly (now canonical []types.Message)
	executorMessages := compiled.Messages

	// Build executor options
	executorOpts := []agents.ExecutorOption{
		agents.WithCheckpointer(s.checkpointer),
		agents.WithEventBus(s.eventBus),
		agents.WithMaxIterations(10),
	}

	// Add delegation tool if registry is configured
	if s.agentRegistry != nil && len(s.agentRegistry.All()) > 0 {
		// Get agent's default tools
		agentTools := s.agent.Tools()

		// Create delegation tool with runtime session/tenant IDs
		delegationTool := agents.NewDelegationTool(
			s.agentRegistry,
			s.model,
			sessionID,
			tenantID,
		)

		// Append delegation tool to agent tools
		extendedTools := append(agentTools, delegationTool)

		// Add tools to executor options
		executorOpts = append(executorOpts, agents.WithExecutorTools(extendedTools))
	}

	// Create executor with agent, model, and options
	executor := agents.NewExecutor(s.agent, s.model, executorOpts...)

	// Execute agent and get event generator
	input := agents.Input{
		Messages:  executorMessages,
		SessionID: sessionID,
		TenantID:  tenantID,
	}

	execGen := executor.Execute(ctx, input)

	// Convert executor generator to service event generator
	return s.convertExecutorGenerator(ctx, execGen), nil
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
) (types.Generator[services.Event], error) {
	const op serrors.Op = "agentServiceImpl.ResumeWithAnswer"

	// Validate inputs
	if checkpointID == "" {
		return nil, serrors.E(op, serrors.KindValidation, "checkpointID is required")
	}

	// Get tenant ID for multi-tenant isolation (validates it exists in context)
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	// Build executor options (same as ProcessMessage)
	executorOpts := []agents.ExecutorOption{
		agents.WithCheckpointer(s.checkpointer),
		agents.WithEventBus(s.eventBus),
		agents.WithMaxIterations(10),
	}

	// Add delegation tool if registry is configured
	if s.agentRegistry != nil && len(s.agentRegistry.All()) > 0 {
		// Get agent's default tools
		agentTools := s.agent.Tools()

		// Create delegation tool with runtime session/tenant IDs
		delegationTool := agents.NewDelegationTool(
			s.agentRegistry,
			s.model,
			sessionID,
			tenantID,
		)

		// Append delegation tool to agent tools
		extendedTools := append(agentTools, delegationTool)

		// Add tools to executor options
		executorOpts = append(executorOpts, agents.WithExecutorTools(extendedTools))
	}

	// Create executor (same configuration as ProcessMessage)
	executor := agents.NewExecutor(s.agent, s.model, executorOpts...)

	// Resume execution from checkpoint with user answers
	execGen := executor.Resume(ctx, checkpointID, answers)

	// Convert executor generator to service event generator
	return s.convertExecutorGenerator(ctx, execGen), nil
}

// convertExecutorGenerator converts an executor event generator to a service event generator.
// This converts agents.ExecutorEvent to services.Event for the service layer.
func (s *agentServiceImpl) convertExecutorGenerator(
	ctx context.Context,
	execGen types.Generator[agents.ExecutorEvent],
) types.Generator[services.Event] {
	return types.NewGenerator(ctx, func(ctx context.Context, yield func(services.Event) bool) error {
		for {
			execEvent, err := execGen.Next(ctx)
			if err != nil {
				if err == types.ErrGeneratorDone {
					return nil // Normal completion
				}
				return err
			}

			// Convert executor event to service event
			serviceEvent := convertExecutorEvent(execEvent)
			if !yield(serviceEvent) {
				return nil // Consumer stopped
			}
		}
	})
}

// convertExecutorEvent converts an agents.ExecutorEvent to a services.Event.
func convertExecutorEvent(execEvent agents.ExecutorEvent) services.Event {
	event := services.Event{}

	switch execEvent.Type {
	case agents.EventTypeChunk:
		event.Type = services.EventTypeContent
		if execEvent.Chunk != nil {
			event.Content = execEvent.Chunk.Delta
		}

	case agents.EventTypeToolStart:
		event.Type = services.EventTypeToolStart
		if execEvent.Tool != nil {
			event.Tool = &services.ToolEvent{
				Name:      execEvent.Tool.Name,
				Arguments: execEvent.Tool.Arguments,
			}
		}

	case agents.EventTypeToolEnd:
		event.Type = services.EventTypeToolEnd
		if execEvent.Tool != nil {
			event.Tool = &services.ToolEvent{
				Name:      execEvent.Tool.Name,
				Arguments: execEvent.Tool.Arguments,
				Result:    execEvent.Tool.Result,
				Error:     execEvent.Tool.Error,
			}
		}

	case agents.EventTypeInterrupt:
		event.Type = services.EventTypeInterrupt
		if execEvent.Interrupt != nil {
			event.Interrupt = convertInterruptEvent(execEvent.Interrupt)
		}

	case agents.EventTypeDone:
		event.Type = services.EventTypeDone
		event.Done = true
		// Extract usage from result if available
		if execEvent.Result != nil {
			event.Usage = &services.TokenUsage{
				PromptTokens:     execEvent.Result.Usage.PromptTokens,
				CompletionTokens: execEvent.Result.Usage.CompletionTokens,
				TotalTokens:      execEvent.Result.Usage.TotalTokens,
			}
		}

	case agents.EventTypeError:
		event.Type = services.EventTypeError
		event.Error = execEvent.Error
	}

	return event
}

// convertInterruptEvent converts an agents.InterruptEvent to a services.InterruptEvent.
func convertInterruptEvent(agentInterrupt *agents.InterruptEvent) *services.InterruptEvent {
	if agentInterrupt == nil {
		return nil
	}

	// Parse the canonical interrupt payload
	var questions []services.Question
	if len(agentInterrupt.Data) > 0 {
		var payload types.AskUserQuestionPayload
		if err := json.Unmarshal(agentInterrupt.Data, &payload); err == nil {
			questions = make([]services.Question, 0, len(payload.Questions))
			for _, q := range payload.Questions {
				// Convert options
				options := make([]services.QuestionOption, 0, len(q.Options))
				for _, opt := range q.Options {
					options = append(options, services.QuestionOption{
						ID:    opt.ID,
						Label: opt.Label,
					})
				}

				// Determine question type.
				// Questions are always single_choice or multiple_choice (no standalone free-text type).
				questionType := services.QuestionTypeSingleChoice
				if q.MultiSelect {
					questionType = services.QuestionTypeMultipleChoice
				}

				questions = append(questions, services.Question{
					ID:      q.ID,
					Text:    q.Question,
					Type:    questionType,
					Options: options,
				})
			}
		}
	}

	// Use the real checkpoint ID and agent name from the interrupt event
	checkpointID := agentInterrupt.CheckpointID
	agentName := agentInterrupt.AgentName

	return &services.InterruptEvent{
		CheckpointID: checkpointID,
		AgentName:    agentName,
		Questions:    questions,
	}
}

// convertToHistoryPayload converts types.Message slice to ConversationHistoryPayload
func convertToHistoryPayload(messages []types.Message) codecs.ConversationHistoryPayload {
	historyMessages := make([]codecs.ConversationMessage, 0, len(messages))
	for _, msg := range messages {
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
			FileName:  a.FileName(),
			MimeType:  a.MimeType(),
			SizeBytes: a.SizeBytes(),
			FilePath:  a.FilePath(),
			CreatedAt: a.CreatedAt(),
		}
	}
	return result
}
