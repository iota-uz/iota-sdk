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
	agent        agents.ExtendedAgent
	model        agents.Model
	policy       bichatctx.ContextPolicy
	renderer     bichatctx.Renderer
	checkpointer agents.Checkpointer
	eventBus     hooks.EventBus
	chatRepo     domain.ChatRepository
}

// AgentServiceConfig holds configuration for creating an AgentService.
type AgentServiceConfig struct {
	Agent        agents.ExtendedAgent
	Model        agents.Model
	Policy       bichatctx.ContextPolicy
	Renderer     bichatctx.Renderer
	Checkpointer agents.Checkpointer
	EventBus     hooks.EventBus        // Optional
	ChatRepo     domain.ChatRepository // Repository for loading messages
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
//	})
func NewAgentService(cfg AgentServiceConfig) services.AgentService {
	return &agentServiceImpl{
		agent:        cfg.Agent,
		model:        cfg.Model,
		policy:       cfg.Policy,
		renderer:     cfg.Renderer,
		checkpointer: cfg.Checkpointer,
		eventBus:     cfg.EventBus,
		chatRepo:     cfg.ChatRepo,
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
) (services.Generator[services.Event], error) {
	const op serrors.Op = "agentServiceImpl.ProcessMessage"

	// Get tenant ID for multi-tenant isolation
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	// Load session messages from repository
	opts := domain.ListOptions{Limit: 100, Offset: 0}
	domainMessages, err := s.chatRepo.GetSessionMessages(ctx, sessionID, opts)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	// Convert domain messages to types.Message for agent framework
	sessionMessages := make([]types.Message, 0, len(domainMessages))
	for _, dm := range domainMessages {
		msg := types.Message{
			ID:        dm.ID,
			SessionID: dm.SessionID,
			Role:      dm.Role,
			Content:   dm.Content,
		}

		// Handle tool calls if present
		if len(dm.ToolCalls) > 0 {
			msg.ToolCalls = dm.ToolCalls
		}

		// Handle tool call ID if present
		if dm.ToolCallID != nil {
			msg.ToolCallID = dm.ToolCallID
		}

		sessionMessages = append(sessionMessages, msg)
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
	userMsg := types.UserMessage(content)

	// Add attachments if present
	if len(attachments) > 0 {
		userMsg.Attachments = convertToTypeAttachments(attachments)
	}

	// Use systemCodec for turn (it accepts string payload via extractText)
	systemCodec := codecs.NewSystemRulesCodec()
	builder.Turn(systemCodec, userMsg.Content)

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
			false, // compacted (not implemented yet)
			compiled.Truncated,
			0, // excludedBlocks (not tracked yet)
		)
		_ = s.eventBus.Publish(ctx, event)
	}

	// Convert compiled.Messages to []types.Message for executor
	executorMessages := make([]types.Message, 0, len(compiled.Messages))
	for _, msg := range compiled.Messages {
		// Messages are provider-specific map[string]any
		msgMap, ok := msg.(map[string]any)
		if !ok {
			continue
		}

		// Extract role and content from message map
		role, _ := msgMap["role"].(string)
		content, _ := msgMap["content"].(string)

		executorMessages = append(executorMessages, types.Message{
			Role:    types.Role(role),
			Content: content,
		})
	}

	// Create executor with agent, model, and checkpointer
	executor := agents.NewExecutor(
		s.agent,
		s.model,
		agents.WithCheckpointer(s.checkpointer),
		agents.WithEventBus(s.eventBus),
		agents.WithMaxIterations(10),
	)

	// Execute agent and get event generator
	input := agents.Input{
		Messages:  executorMessages,
		SessionID: sessionID,
		TenantID:  tenantID,
	}

	execGen := executor.Execute(ctx, input)

	// Wrap the executor generator into a service event generator
	return s.wrapExecutorGenerator(ctx, execGen), nil
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
	answers map[string]string,
) (services.Generator[services.Event], error) {
	const op serrors.Op = "agentServiceImpl.ResumeWithAnswer"

	// Validate inputs
	if checkpointID == "" {
		return nil, serrors.E(op, serrors.KindValidation, "checkpointID is required")
	}

	// Get tenant ID for multi-tenant isolation (validates it exists in context)
	_, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	// Create executor (same configuration as ProcessMessage)
	executor := agents.NewExecutor(
		s.agent,
		s.model,
		agents.WithCheckpointer(s.checkpointer),
		agents.WithEventBus(s.eventBus),
		agents.WithMaxIterations(10),
	)

	// Resume execution from checkpoint with user answers
	execGen := executor.Resume(ctx, checkpointID, answers)

	// Wrap the executor generator into a service event generator
	return s.wrapExecutorGenerator(ctx, execGen), nil
}

// wrapExecutorGenerator wraps an executor event generator into a service event generator.
// This converts agents.ExecutorEvent to services.Event for the service layer.
func (s *agentServiceImpl) wrapExecutorGenerator(
	ctx context.Context,
	execGen types.Generator[agents.ExecutorEvent],
) services.Generator[services.Event] {
	return &generatorAdapter{
		ctx:   ctx,
		inner: execGen,
	}
}

// generatorAdapter adapts a types.Generator[ExecutorEvent] to services.Generator[Event].
type generatorAdapter struct {
	ctx   context.Context
	inner types.Generator[agents.ExecutorEvent]
}

// Next returns the next service event by converting executor events.
func (g *generatorAdapter) Next() (services.Event, error, bool) {
	// types.Generator.Next() requires a context, but services.Generator.Next() doesn't provide one
	// We use the context stored during adapter creation
	execEvent, err := g.inner.Next(g.ctx)
	if err != nil {
		// Check if generator is done (not an error)
		if err == types.ErrGeneratorDone {
			return services.Event{}, nil, false
		}
		return services.Event{}, err, false
	}

	// Convert executor event to service event
	serviceEvent := convertExecutorEvent(execEvent)
	return serviceEvent, nil, true
}

// Close releases resources held by the inner generator.
func (g *generatorAdapter) Close() {
	g.inner.Close()
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

	// Parse the interrupt data to extract questions
	var questions []services.Question
	if len(agentInterrupt.Data) > 0 {
		// Try to parse the data as a question
		var questionData struct {
			Question string `json:"question"`
		}
		if err := json.Unmarshal(agentInterrupt.Data, &questionData); err == nil {
			questions = []services.Question{
				{
					ID:   "q1", // Simple ID for single question
					Text: questionData.Question,
					Type: services.QuestionTypeText,
				},
			}
		}
	}

	// Use the real checkpoint ID from the interrupt event
	checkpointID := agentInterrupt.CheckpointID

	return &services.InterruptEvent{
		CheckpointID: checkpointID,
		Questions:    questions,
	}
}

// convertToHistoryPayload converts types.Message slice to ConversationHistoryPayload
func convertToHistoryPayload(messages []types.Message) codecs.ConversationHistoryPayload {
	historyMessages := make([]codecs.ConversationMessage, 0, len(messages))
	for _, msg := range messages {
		historyMessages = append(historyMessages, codecs.ConversationMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
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
			ID:        a.ID,
			MessageID: a.MessageID,
			FileName:  a.FileName,
			MimeType:  a.MimeType,
			SizeBytes: a.SizeBytes,
			FilePath:  a.FilePath,
			CreatedAt: a.CreatedAt,
		}
	}
	return result
}
