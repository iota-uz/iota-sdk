package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/hooks"
	"github.com/iota-uz/iota-sdk/pkg/bichat/hooks/events"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// ExecutorEventType identifies different types of executor events.
type ExecutorEventType string

const (
	// EventTypeChunk is emitted for streaming text chunks from the LLM.
	EventTypeChunk ExecutorEventType = "chunk"

	// EventTypeToolStart is emitted when a tool execution begins.
	EventTypeToolStart ExecutorEventType = "tool_start"

	// EventTypeToolEnd is emitted when a tool execution completes.
	EventTypeToolEnd ExecutorEventType = "tool_end"

	// EventTypeInterrupt is emitted when execution is interrupted for HITL.
	EventTypeInterrupt ExecutorEventType = "interrupt"

	// EventTypeDone is emitted when execution completes successfully.
	EventTypeDone ExecutorEventType = "done"

	// EventTypeError is emitted when an error occurs during execution.
	EventTypeError ExecutorEventType = "error"
)

// ExecutorEvent represents a single event during ReAct loop execution.
// Events are yielded as the executor progresses through reasoning and action steps.
type ExecutorEvent struct {
	// Type identifies what kind of event this is.
	Type ExecutorEventType

	// Chunk contains streaming text data (for EventTypeChunk).
	Chunk *Chunk

	// Tool contains tool execution data (for EventTypeToolStart/EventTypeToolEnd).
	Tool *ToolEvent

	// Interrupt contains HITL interrupt data (for EventTypeInterrupt).
	Interrupt *InterruptEvent

	// Error contains error information (for EventTypeError).
	Error error

	// Done indicates execution has completed (for EventTypeDone).
	Done bool

	// Result contains the final response (for EventTypeDone).
	Result *Response
}

// ToolEvent represents a tool execution event.
type ToolEvent struct {
	// CallID uniquely identifies this tool call.
	CallID string

	// Name is the tool being executed.
	Name string

	// Arguments is the JSON-encoded input.
	Arguments string

	// Result is the tool output (only present in EventTypeToolEnd).
	Result string

	// Error is the tool error (only present in EventTypeToolEnd if failed).
	Error error

	// DurationMs is the execution time in milliseconds (only present in EventTypeToolEnd).
	DurationMs int64
}

// Executor executes the ReAct (Reason + Act) loop for an agent.
// It coordinates between the LLM model, tools, and interrupt handlers.
//
// The executor:
//   - Calls the model to generate responses
//   - Executes tool calls returned by the model
//   - Handles HITL interrupts (saving checkpoints and yielding interrupt events)
//   - Emits events to the EventBus for observability
//   - Returns a Generator that yields ExecutorEvent objects
//
// Example usage:
//
//	executor := NewExecutor(agent, model,
//	    WithCheckpointer(checkpointer),
//	    WithEventBus(eventBus),
//	    WithMaxIterations(10),
//	)
//
//	gen := executor.Execute(ctx, Input{
//	    Messages: []types.Message{{Role: RoleUser, Content: "What's the weather?"}},
//	    SessionID: sessionID,
//	    TenantID: tenantID,
//	})
//	defer gen.Close()
//
//	for {
//	    event, err, hasMore := gen.Next()
//	    if err != nil { return err }
//	    if !hasMore { break }
//	    handleEvent(event)
//	}
type Executor struct {
	agent             ExtendedAgent
	model             Model
	checkpointer      Checkpointer
	eventBus          hooks.EventBus
	maxIterations     int
	interruptRegistry *InterruptHandlerRegistry
}

// Input represents the input to Execute or Resume.
type Input struct {
	// Messages is the conversation history to start with.
	Messages []types.Message

	// SessionID identifies the chat session for observability and checkpointing.
	SessionID uuid.UUID

	// TenantID identifies the tenant for observability and checkpointing.
	TenantID uuid.UUID

	// ThreadID identifies the conversation thread for checkpointing.
	// If empty, a new thread ID will be generated.
	ThreadID string
}

// ExecutorOption configures an Executor.
type ExecutorOption func(*Executor)

// WithCheckpointer sets the checkpointer for HITL support.
// If nil, interrupts will fail with ErrCheckpointSaveFailed.
func WithCheckpointer(checkpointer Checkpointer) ExecutorOption {
	return func(e *Executor) {
		e.checkpointer = checkpointer
	}
}

// WithEventBus sets the event bus for observability.
// If nil, events will not be published.
func WithEventBus(eventBus hooks.EventBus) ExecutorOption {
	return func(e *Executor) {
		e.eventBus = eventBus
	}
}

// WithMaxIterations sets the maximum number of ReAct iterations.
// Default is 10. Use this to prevent infinite loops.
func WithMaxIterations(max int) ExecutorOption {
	return func(e *Executor) {
		e.maxIterations = max
	}
}

// WithInterruptRegistry sets the interrupt handler registry.
// If nil, a default registry will be created.
func WithInterruptRegistry(registry *InterruptHandlerRegistry) ExecutorOption {
	return func(e *Executor) {
		e.interruptRegistry = registry
	}
}

// NewExecutor creates a new Executor with the given agent and model.
func NewExecutor(agent ExtendedAgent, model Model, opts ...ExecutorOption) *Executor {
	executor := &Executor{
		agent:             agent,
		model:             model,
		maxIterations:     10,
		interruptRegistry: NewInterruptHandlerRegistry(),
	}

	for _, opt := range opts {
		opt(executor)
	}

	return executor
}

// Execute runs the ReAct loop and returns a Generator that yields ExecutorEvent objects.
// The generator will yield events for:
//   - Text chunks (from streaming LLM responses)
//   - Tool executions (start and end)
//   - HITL interrupts (when ask_user_question or similar tools are called)
//   - Final completion (with the complete response)
//   - Errors (if execution fails)
//
// The generator must be closed when done to release resources.
func (e *Executor) Execute(ctx context.Context, input Input) types.Generator[ExecutorEvent] {
	const op serrors.Op = "Executor.Execute"

	return types.NewGenerator(ctx, func(ctx context.Context, yield func(ExecutorEvent) bool) error {
		// Validate input
		if len(input.Messages) == 0 {
			return serrors.E(op, "input must contain at least one message")
		}

		// Generate thread ID if not provided
		threadID := input.ThreadID
		if threadID == "" {
			threadID = uuid.New().String()
		}

		// Build initial request with system prompt
		messages := make([]types.Message, 0, len(input.Messages)+1)

		// Add system prompt if agent provides one
		systemPrompt := e.agent.SystemPrompt(ctx)
		if systemPrompt != "" {
			messages = append(messages, *types.SystemMessage(systemPrompt))
		}

		// Add input messages
		messages = append(messages, input.Messages...)

		// Start ReAct loop
		iteration := 0
		for iteration < e.maxIterations {
			iteration++

			// Build model request
			req := Request{
				Messages: messages,
				Tools:    e.agent.Tools(),
			}

			// Emit LLM request event
			if e.eventBus != nil {
				modelInfo := e.model.Info()
				_ = e.eventBus.Publish(ctx, events.NewLLMRequestEvent(
					input.SessionID,
					input.TenantID,
					modelInfo.Name,
					modelInfo.Provider,
					len(req.Messages),
					len(req.Tools),
					0, // TODO: estimate tokens
				))
			}

			// Call model (streaming)
			gen := e.model.Stream(ctx, req)
			defer gen.Close()

			// Accumulate response
			var responseMessage types.Message
			var chunks []string
			var toolCalls []types.ToolCall
			var usage *types.TokenUsage
			var finishReason string
			startTime := time.Now()

			for {
				chunk, err := gen.Next(ctx)
				if err != nil {
					if err == types.ErrGeneratorDone {
						break
					}
					return serrors.E(op, err)
				}

				// Yield chunk event
				if chunk.Delta != "" {
					chunks = append(chunks, chunk.Delta)
					if !yield(ExecutorEvent{
						Type:  EventTypeChunk,
						Chunk: &chunk,
					}) {
						return nil // Consumer stopped
					}
				}

				// Accumulate tool calls
				if len(chunk.ToolCalls) > 0 {
					toolCalls = chunk.ToolCalls
				}

				// Capture final metadata
				if chunk.Done {
					usage = chunk.Usage
					finishReason = chunk.FinishReason
				}
			}

			// Build response message
			responseMessage = *types.AssistantMessage(joinStrings(chunks), types.WithToolCalls(toolCalls...))
			messages = append(messages, responseMessage)

			// Emit LLM response event
			if e.eventBus != nil {
				modelInfo := e.model.Info()
				var usageTokens types.TokenUsage
				if usage != nil {
					usageTokens = *usage
				}
				_ = e.eventBus.Publish(ctx, events.NewLLMResponseEvent(
					input.SessionID,
					input.TenantID,
					modelInfo.Name,
					modelInfo.Provider,
					usageTokens.PromptTokens,
					usageTokens.CompletionTokens,
					usageTokens.TotalTokens,
					time.Since(startTime).Milliseconds(),
					finishReason,
					len(toolCalls),
				))
			}

			// Check for tool calls
			if len(toolCalls) == 0 {
				// No tools, execution complete
				result := &Response{
					Message:      responseMessage,
					FinishReason: finishReason,
				}
				if usage != nil {
					result.Usage = *usage
				}

				if !yield(ExecutorEvent{
					Type:   EventTypeDone,
					Done:   true,
					Result: result,
				}) {
					return nil
				}
				return nil
			}

			// Execute tool calls
			toolResults, interrupt, err := e.executeToolCalls(ctx, input.SessionID, input.TenantID, toolCalls, yield)
			if err != nil {
				if !yield(ExecutorEvent{
					Type:  EventTypeError,
					Error: serrors.E(op, err),
				}) {
					return nil
				}
				return serrors.E(op, err)
			}

			// Check for interrupt
			if interrupt != nil {
				// Save checkpoint
				checkpointID, err := e.saveCheckpoint(ctx, threadID, messages, toolCalls, interrupt)
				if err != nil {
					return serrors.E(op, ErrCheckpointSaveFailed, err)
				}

				// Set checkpoint ID on interrupt event
				interrupt.AgentName = e.agent.Metadata().Name
				interrupt.SessionID = input.SessionID

				// Emit interrupt event to EventBus
				if e.eventBus != nil {
					var question string
					var data struct {
						Questions []struct {
							Question string `json:"question"`
						} `json:"questions"`
					}
					if err := json.Unmarshal(interrupt.Data, &data); err == nil && len(data.Questions) > 0 {
						question = data.Questions[0].Question
					}

					_ = e.eventBus.Publish(ctx, events.NewInterruptEvent(
						input.SessionID,
						input.TenantID,
						interrupt.Type,
						e.agent.Metadata().Name,
						question,
						checkpointID,
					))
				}

				// Yield interrupt event to generator consumer
				if !yield(ExecutorEvent{
					Type:      EventTypeInterrupt,
					Interrupt: interrupt,
				}) {
					return nil
				}

				// Stop execution (will be resumed later via Resume())
				return nil
			}

			// Add tool results to messages
			messages = append(messages, toolResults...)

			// Check for termination tools
			metadata := e.agent.Metadata()
			for _, tc := range toolCalls {
				for _, term := range metadata.TerminationTools {
					if tc.Name == term {
						// Find the tool result
						var resultContent string
						for _, tr := range toolResults {
							if tr.ToolCallID != nil && *tr.ToolCallID == tc.ID {
								resultContent = tr.Content
								break
							}
						}

						// Termination tool called, return result
						result := &Response{
							Message:      *types.AssistantMessage(resultContent),
							FinishReason: "tool_calls",
						}
						if usage != nil {
							result.Usage = *usage
						}

						if !yield(ExecutorEvent{
							Type:   EventTypeDone,
							Done:   true,
							Result: result,
						}) {
							return nil
						}
						return nil
					}
				}
			}

			// Continue to next iteration
		}

		// Max iterations reached
		return serrors.E(op, ErrMaxIterations)
	})
}

// Resume continues execution from a saved checkpoint after receiving user input.
// This is used for HITL (human-in-the-loop) workflows where execution was interrupted.
//
// The answers parameter is a map from question ID to user's answer string.
// All questions must have answers, or the resume will fail.
//
// Example:
//
//	// After interrupt event is received:
//	answers := map[string]string{
//	    "question_1": "Q1 2024",
//	    "question_2": "revenue",
//	}
//	gen := executor.Resume(ctx, checkpointID, answers)
//	defer gen.Close()
//	for {
//	    event, err, hasMore := gen.Next()
//	    if err != nil { return err }
//	    if !hasMore { break }
//	    handleEvent(event)
//	}
func (e *Executor) Resume(ctx context.Context, checkpointID string, answers map[string]string) types.Generator[ExecutorEvent] {
	const op serrors.Op = "Executor.Resume"

	return types.NewGenerator(ctx, func(ctx context.Context, yield func(ExecutorEvent) bool) error {
		// Load checkpoint
		if e.checkpointer == nil {
			return serrors.E(op, ErrCheckpointNotFound)
		}

		checkpoint, err := e.checkpointer.LoadAndDelete(ctx, checkpointID)
		if err != nil {
			return serrors.E(op, err)
		}

		// Restore messages
		messages := checkpoint.Messages

		// Add tool results with user's answers
		for _, tc := range checkpoint.PendingTools {
			// Check if this is an interrupt tool that needs answers
			if tc.Name == ToolAskUserQuestion {
				// Parse questions from tool call arguments
				var questionsData struct {
					Questions []struct {
						ID   string `json:"id"`
						Text string `json:"text"`
					} `json:"questions"`
				}
				if err := json.Unmarshal([]byte(tc.Arguments), &questionsData); err != nil {
					if !yield(ExecutorEvent{
						Type:  EventTypeError,
						Error: serrors.E(op, "failed to parse questions from checkpoint", err),
					}) {
						return nil
					}
					return serrors.E(op, "failed to parse questions from checkpoint", err)
				}

				// Build response with all answers
				responseData := make(map[string]string)
				for _, q := range questionsData.Questions {
					answer, exists := answers[q.ID]
					if !exists {
						if !yield(ExecutorEvent{
							Type:  EventTypeError,
							Error: serrors.E(op, serrors.KindValidation, fmt.Sprintf("missing answer for question %s", q.ID)),
						}) {
							return nil
						}
						return serrors.E(op, serrors.KindValidation, fmt.Sprintf("missing answer for question %s", q.ID))
					}
					responseData[q.ID] = answer
				}

				// Create tool response message with all answers
				encoded, _ := json.Marshal(responseData)
				messages = append(messages, *types.ToolResponse(tc.ID, string(encoded)))
			}
		}

		// Resume execution with restored state
		// TODO: Add SessionID and TenantID to Checkpoint struct to restore from metadata
		input := Input{
			Messages:  messages,
			SessionID: uuid.Nil, // TODO: restore from checkpoint metadata
			TenantID:  uuid.Nil, // TODO: restore from checkpoint metadata
			ThreadID:  checkpoint.ThreadID,
		}

		// Create a new generator that delegates to Execute
		resumeGen := e.Execute(ctx, input)
		defer resumeGen.Close()

		// Yield all events from the resumed execution
		for {
			event, err := resumeGen.Next(ctx)
			if err != nil {
				if err == types.ErrGeneratorDone {
					break
				}
				return serrors.E(op, err)
			}
			if !yield(event) {
				return nil
			}
		}

		return nil
	})
}

// executeToolCalls executes all tool calls in parallel and returns their results.
// If any tool triggers an interrupt, returns the interrupt event.
func (e *Executor) executeToolCalls(
	ctx context.Context,
	sessionID, tenantID uuid.UUID,
	toolCalls []types.ToolCall,
	yield func(ExecutorEvent) bool,
) ([]types.Message, *InterruptEvent, error) {
	const op serrors.Op = "Executor.executeToolCalls"

	results := make([]types.Message, 0, len(toolCalls))

	for _, tc := range toolCalls {
		// Emit tool start event
		if e.eventBus != nil {
			_ = e.eventBus.Publish(ctx, events.NewToolStartEvent(
				sessionID,
				tenantID,
				tc.Name,
				tc.Arguments,
				tc.ID,
			))
		}

		// Yield tool start event
		if !yield(ExecutorEvent{
			Type: EventTypeToolStart,
			Tool: &ToolEvent{
				CallID:    tc.ID,
				Name:      tc.Name,
				Arguments: tc.Arguments,
			},
		}) {
			return nil, nil, nil // Consumer stopped
		}

		// Check for interrupt tools
		if tc.Name == ToolAskUserQuestion {
			// The tool already returns formatted InterruptData in its result
			// Just parse it and create the interrupt event
			// tc.Arguments contains the full question data structure
			interruptData := json.RawMessage(tc.Arguments)

			interrupt := &InterruptEvent{
				Type: ToolAskUserQuestion,
				Data: interruptData,
			}

			return results, interrupt, nil
		}

		// Execute regular tool
		startTime := time.Now()
		result, err := e.agent.OnToolCall(ctx, tc.Name, tc.Arguments)
		durationMs := time.Since(startTime).Milliseconds()

		// Emit tool completion/error event
		if e.eventBus != nil {
			if err != nil {
				_ = e.eventBus.Publish(ctx, events.NewToolErrorEvent(
					sessionID,
					tenantID,
					tc.Name,
					tc.Arguments,
					tc.ID,
					err.Error(),
					durationMs,
				))
			} else {
				_ = e.eventBus.Publish(ctx, events.NewToolCompleteEvent(
					sessionID,
					tenantID,
					tc.Name,
					tc.Arguments,
					tc.ID,
					result,
					durationMs,
				))
			}
		}

		// Handle error
		if err != nil {
			result = fmt.Sprintf("Error: %v", err)
		}

		// Yield tool end event
		if !yield(ExecutorEvent{
			Type: EventTypeToolEnd,
			Tool: &ToolEvent{
				CallID:     tc.ID,
				Name:       tc.Name,
				Arguments:  tc.Arguments,
				Result:     result,
				Error:      err,
				DurationMs: durationMs,
			},
		}) {
			return nil, nil, nil // Consumer stopped
		}

		// Add tool result message
		results = append(results, *types.ToolResponse(tc.ID, result))
	}

	return results, nil, nil
}

// saveCheckpoint creates and saves a checkpoint for HITL resumption.
func (e *Executor) saveCheckpoint(
	ctx context.Context,
	threadID string,
	messages []types.Message,
	toolCalls []types.ToolCall,
	interrupt *InterruptEvent,
) (string, error) {
	const op serrors.Op = "Executor.saveCheckpoint"

	if e.checkpointer == nil {
		return "", serrors.E(op, ErrCheckpointSaveFailed)
	}

	checkpoint := NewCheckpoint(
		threadID,
		e.agent.Metadata().Name,
		messages,
		WithPendingTools(toolCalls),
		WithInterruptType(interrupt.Type),
		WithInterruptData(interrupt.Data),
	)

	checkpointID, err := e.checkpointer.Save(ctx, checkpoint)
	if err != nil {
		return "", serrors.E(op, err)
	}

	return checkpointID, nil
}

// joinStrings concatenates a slice of strings.
func joinStrings(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}

	// Estimate total length to minimize allocations
	totalLen := 0
	for _, p := range parts {
		totalLen += len(p)
	}

	result := make([]byte, 0, totalLen)
	for _, p := range parts {
		result = append(result, p...)
	}

	return string(result)
}
