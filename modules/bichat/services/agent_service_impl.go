package services

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	bichatctx "github.com/iota-uz/iota-sdk/pkg/bichat/context"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/hooks"
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
}

// AgentServiceConfig holds configuration for creating an AgentService.
type AgentServiceConfig struct {
	Agent        agents.ExtendedAgent
	Model        agents.Model
	Policy       bichatctx.ContextPolicy
	Renderer     bichatctx.Renderer
	Checkpointer agents.Checkpointer
	EventBus     hooks.EventBus // Optional
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
//	})
func NewAgentService(cfg AgentServiceConfig) services.AgentService {
	return &agentServiceImpl{
		agent:        cfg.Agent,
		model:        cfg.Model,
		policy:       cfg.Policy,
		renderer:     cfg.Renderer,
		checkpointer: cfg.Checkpointer,
		eventBus:     cfg.EventBus,
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

	// TODO: Load session messages from repository
	// For now, we'll create a simple message history placeholder
	var sessionMessages []types.Message

	// TODO: Use Context Builder when codecs are available
	// For now, compile a simple context directly
	compiled, err := s.compileSimpleContext(ctx, sessionMessages, content)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	// Create executor with agent, model, and options
	executor := agents.NewExecutor(
		s.agent,
		s.model,
		agents.WithCheckpointer(s.checkpointer),
		agents.WithEventBus(s.eventBus),
		agents.WithMaxIterations(10),
	)

	// Execute agent and get event generator
	input := agents.Input{
		Messages:  compiled,
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

	// Resume execution from checkpoint
	// Note: The executor's Resume method expects a single answer string,
	// but we receive a map. We'll join the answers for now.
	// TODO: Update executor.Resume to accept map[string]string
	var answer string
	if len(answers) > 0 {
		// For now, take the first answer
		for _, v := range answers {
			answer = v
			break
		}
	}

	execGen := executor.Resume(ctx, checkpointID, answer)

	// Wrap the executor generator into a service event generator
	return s.wrapExecutorGenerator(ctx, execGen), nil
}

// compileSimpleContext creates a simple message list without using the full context builder.
// This is a temporary implementation until codecs are available.
func (s *agentServiceImpl) compileSimpleContext(
	ctx context.Context,
	sessionMessages []types.Message,
	userContent string,
) ([]types.Message, error) {
	messages := make([]types.Message, 0, len(sessionMessages)+2)

	// Add system prompt if agent provides one
	systemPrompt := s.agent.SystemPrompt(ctx)
	if systemPrompt != "" {
		messages = append(messages, *types.SystemMessage(systemPrompt))
	}

	// Add session history
	messages = append(messages, sessionMessages...)

	// Add current user message
	messages = append(messages, *types.UserMessage(userContent))

	return messages, nil
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

	// The checkpointID should be set by the executor
	// TODO: Extract checkpoint ID from interrupt metadata when available
	checkpointID := fmt.Sprintf("checkpoint_%s", agentInterrupt.SessionID.String())

	return &services.InterruptEvent{
		CheckpointID: checkpointID,
		Questions:    questions,
	}
}
