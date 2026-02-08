package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/hooks"
	"github.com/iota-uz/iota-sdk/pkg/bichat/hooks/events"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// askUserQuestionArgs matches the ask_user_question tool JSON schema.
// Note: tool-call arguments do NOT include a "type" field.
type askUserQuestionArgs struct {
	Questions []askUserQuestionArgsQuestion `json:"questions"`
	Metadata  *types.QuestionMetadata       `json:"metadata,omitempty"`
}

type askUserQuestionArgsQuestion struct {
	ID          string                      `json:"id,omitempty"`
	Question    string                      `json:"question"`
	Header      string                      `json:"header"`
	MultiSelect bool                        `json:"multiSelect"`
	Options     []askUserQuestionArgsOption `json:"options"`
}

type askUserQuestionArgsOption struct {
	ID          string `json:"id,omitempty"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

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
	tools             []Tool            // Optional override for agent tools (e.g., filtered for child executors)
	tokenEstimator    TokenEstimator    // Optional token estimator for cost tracking
	speculativeTools  bool              // Start executing ready tool calls during streaming (best-effort)
	formatterRegistry FormatterRegistry // Optional formatter registry for StructuredTool support
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

	// PreviousResponseID is a provider continuity token for multi-turn state.
	// For OpenAI Responses API this maps to previous_response_id.
	PreviousResponseID *string
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
func WithMaxIterations(maxIterations int) ExecutorOption {
	return func(e *Executor) {
		e.maxIterations = maxIterations
	}
}

// WithInterruptRegistry sets the interrupt handler registry.
// If nil, a default registry will be created.
func WithInterruptRegistry(registry *InterruptHandlerRegistry) ExecutorOption {
	return func(e *Executor) {
		e.interruptRegistry = registry
	}
}

// WithTokenEstimator sets the token estimator for accurate cost tracking.
// If nil, token estimation will be disabled (events will report 0 tokens).
//
// Recommended implementations:
//   - TiktokenEstimator: Accurate token counting using tiktoken library
//   - CharacterBasedEstimator: Fast approximation using character counts
//
// Example:
//
//	estimator := NewTiktokenEstimator("cl100k_base")
//	executor := NewExecutor(agent, model, WithTokenEstimator(estimator))
func WithTokenEstimator(estimator TokenEstimator) ExecutorOption {
	return func(e *Executor) {
		e.tokenEstimator = estimator
	}
}

// WithExecutorTools sets custom tools for the executor, overriding the agent's default tools.
// Use this to filter tools (e.g., removing delegation tool for child executors to prevent recursion).
func WithExecutorTools(tools []Tool) ExecutorOption {
	return func(e *Executor) {
		e.tools = tools
	}
}

// WithSpeculativeTools enables best-effort speculative tool execution during streaming.
// When enabled, the executor will start executing tool calls as soon as the model emits
// completed tool call items (before the final streaming chunk arrives).
func WithSpeculativeTools(enabled bool) ExecutorOption {
	return func(e *Executor) {
		e.speculativeTools = enabled
	}
}

// WithFormatterRegistry sets the formatter registry for StructuredTool support.
// When set, the executor will check if tools implement StructuredTool and use
// the registry to format their structured output.
func WithFormatterRegistry(registry FormatterRegistry) ExecutorOption {
	return func(e *Executor) {
		e.formatterRegistry = registry
	}
}

// NewExecutor creates a new Executor with the given agent and model.
func NewExecutor(agent ExtendedAgent, model Model, opts ...ExecutorOption) *Executor {
	executor := &Executor{
		agent:             agent,
		model:             model,
		maxIterations:     10,
		interruptRegistry: NewInterruptHandlerRegistry(),
		speculativeTools:  true,
	}

	for _, opt := range opts {
		opt(executor)
	}

	return executor
}

// callTool executes a tool, using StructuredTool + FormatterRegistry when available.
func (e *Executor) callTool(ctx context.Context, toolName, arguments string) (string, error) {
	// If formatter registry is set, check if the tool implements StructuredTool
	if e.formatterRegistry != nil {
		tools := e.tools
		if tools == nil {
			tools = e.agent.Tools()
		}
		for _, tool := range tools {
			if tool != nil && tool.Name() == toolName {
				if st, ok := tool.(StructuredTool); ok {
					result, err := st.CallStructured(ctx, arguments)
					if err != nil {
						return "", err
					}
					if result != nil {
						formatter := e.formatterRegistry.Get(result.CodecID)
						if formatter != nil {
							opts := FormatOptions{
								MaxRows:      25,
								MaxCellWidth: 80,
							}
							return formatter.Format(result.Payload, opts)
						}
					}
				}
				break
			}
		}
	}
	// Fallback to existing path
	return e.agent.OnToolCall(ctx, toolName, arguments)
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

		// Use input messages as-is (system prompt already included from context compilation)
		messages := input.Messages

		// Track provider continuity across iterations in a single execution.
		previousResponseID := input.PreviousResponseID

		// Start ReAct loop
		iteration := 0
		for iteration < e.maxIterations {
			iteration++

			// Determine which tools to use (override if e.tools is set)
			tools := e.tools
			if tools == nil {
				tools = e.agent.Tools()
			}

			// Build model request
			req := Request{
				Messages:           messages,
				Tools:              tools,
				PreviousResponseID: previousResponseID,
			}

			// Emit LLM request event
			if e.eventBus != nil {
				modelInfo := e.model.Info()

				// Estimate tokens if estimator is configured
				estimatedTokens := 0
				if e.tokenEstimator != nil {
					estimatedTokens, _ = e.tokenEstimator.EstimateMessages(ctx, req.Messages)
				}

				// Extract last user message content for trace Input
				userInput := ""
				for i := len(req.Messages) - 1; i >= 0; i-- {
					if req.Messages[i].Role() == types.RoleUser {
						userInput = req.Messages[i].Content()
						break
					}
				}

				_ = e.eventBus.Publish(ctx, events.NewLLMRequestEvent(
					input.SessionID,
					input.TenantID,
					modelInfo.Name,
					modelInfo.Provider,
					len(req.Messages),
					len(req.Tools),
					estimatedTokens,
					userInput,
				))
			}

			// Call model (streaming)
			gen, err := e.model.Stream(ctx, req)
			if err != nil {
				// Emit error event
				if !yield(ExecutorEvent{
					Type:  EventTypeError,
					Error: err,
				}) {
					return nil
				}
				return serrors.E(op, err)
			}

			// Accumulate response
			var responseMessage types.Message
			var chunks []string
			var toolCalls []types.ToolCall
			var citations []types.Citation
			var usage *types.TokenUsage
			var finishReason string
			var providerResponseID string
			startTime := time.Now()

			// Speculative tool execution state (best-effort).
			type specToolResult struct {
				call       types.ToolCall
				result     string
				err        error
				durationMs int64
			}

			specEnabled := e.speculativeTools
			specCancelled := false
			specStarted := make(map[string]struct{})
			specResults := make(map[string]types.Message)
			specPending := 0
			// Do not rely on a large buffer here: tool calls can exceed small fixed sizes.
			// We drain results opportunistically during streaming to avoid backpressure.
			specResultsCh := make(chan specToolResult, 16)
			specToolCtx, specCancel := context.WithCancel(ctx)
			defer specCancel()

			toolByName := make(map[string]Tool, len(tools))
			for _, tool := range tools {
				if tool == nil {
					continue
				}
				if _, exists := toolByName[tool.Name()]; !exists {
					toolByName[tool.Name()] = tool
				}
			}

			// Concurrency-keyed locks (serialize tools sharing the same key).
			specKeyLocks := make(map[string]*sync.Mutex)
			var specKeyLocksMu sync.Mutex
			getSpecKeyLock := func(key string) *sync.Mutex {
				specKeyLocksMu.Lock()
				defer specKeyLocksMu.Unlock()
				if m, ok := specKeyLocks[key]; ok {
					return m
				}
				m := &sync.Mutex{}
				specKeyLocks[key] = m
				return m
			}

			handleSpecResult := func(tr specToolResult) bool {
				if !specEnabled {
					return true
				}

				toolOutput := tr.result
				if tr.err != nil {
					toolOutput = fmt.Sprintf("Error: %v", tr.err)
				}

				// Emit tool completion/error event
				if e.eventBus != nil {
					if tr.err != nil {
						_ = e.eventBus.Publish(ctx, events.NewToolErrorEvent(
							input.SessionID,
							input.TenantID,
							tr.call.Name,
							tr.call.Arguments,
							tr.call.ID,
							tr.err.Error(),
							tr.durationMs,
						))
					} else {
						_ = e.eventBus.Publish(ctx, events.NewToolCompleteEvent(
							input.SessionID,
							input.TenantID,
							tr.call.Name,
							tr.call.Arguments,
							tr.call.ID,
							toolOutput,
							tr.durationMs,
						))
					}
				}

				// Yield tool end event
				if !yield(ExecutorEvent{
					Type: EventTypeToolEnd,
					Tool: &ToolEvent{
						CallID:     tr.call.ID,
						Name:       tr.call.Name,
						Arguments:  tr.call.Arguments,
						Result:     toolOutput,
						Error:      tr.err,
						DurationMs: tr.durationMs,
					},
				}) {
					specCancel()
					return false
				}

				// Store for ordered message append later.
				specResults[tr.call.ID] = types.ToolResponse(tr.call.ID, toolOutput)
				return true
			}

			drainSpecResults := func(block bool) bool {
				if !specEnabled {
					return true
				}
				for specPending > 0 {
					if !block {
						select {
						case tr := <-specResultsCh:
							specPending--
							if !handleSpecResult(tr) {
								return false
							}
							continue
						default:
							return true
						}
					}

					select {
					case tr := <-specResultsCh:
						specPending--
						if !handleSpecResult(tr) {
							return false
						}
					case <-specToolCtx.Done():
						return true
					}
				}
				return true
			}

			startSpecTool := func(tc types.ToolCall) bool {
				if !specEnabled {
					return true
				}
				if specCancelled {
					return true
				}
				callID := tc.ID
				if callID == "" || tc.Name == "" {
					return true
				}
				if tc.Name == ToolAskUserQuestion {
					// Interrupt tools are exclusive; cancel any in-flight speculative tools.
					specCancelled = true
					specCancel()
					return true
				}
				if _, exists := specStarted[callID]; exists {
					return true
				}
				specStarted[callID] = struct{}{}

				// Emit tool start event
				if e.eventBus != nil {
					_ = e.eventBus.Publish(specToolCtx, events.NewToolStartEvent(
						input.SessionID,
						input.TenantID,
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
					specCancel()
					return false
				}

				// Determine concurrency key (optional).
				key := ""
				if tool := toolByName[tc.Name]; tool != nil {
					if keyed, ok := tool.(ToolConcurrency); ok {
						key = keyed.ConcurrencyKey()
					}
				}

				specPending++

				go func(call types.ToolCall, concurrencyKey string) {
					startedAt := time.Now()

					if concurrencyKey != "" {
						lock := getSpecKeyLock(concurrencyKey)
						lock.Lock()
						defer lock.Unlock()
					}

					res, err := e.callTool(specToolCtx, call.Name, call.Arguments)
					durationMs := time.Since(startedAt).Milliseconds()

					select {
					case specResultsCh <- specToolResult{call: call, result: res, err: err, durationMs: durationMs}:
					case <-specToolCtx.Done():
						return
					}
				}(tc, key)

				return true
			}

			for {
				chunk, err := gen.Next(ctx)
				if err != nil {
					if err == types.ErrGeneratorDone {
						break
					}
					gen.Close()
					return serrors.E(op, err)
				}

				// Yield chunk event
				if chunk.Delta != "" {
					chunks = append(chunks, chunk.Delta)
					if !yield(ExecutorEvent{
						Type:  EventTypeChunk,
						Chunk: &chunk,
					}) {
						gen.Close()
						return nil // Consumer stopped
					}
				}

				// Accumulate tool calls
				if len(chunk.ToolCalls) > 0 {
					toolCalls = chunk.ToolCalls
					// Best-effort: start executing ready tool calls before streaming completes.
					if specEnabled && !specCancelled {
						// If an interrupt tool is present in this batch, it is exclusive.
						// Do not start any other tools in the same batch.
						for _, tc := range chunk.ToolCalls {
							if tc.Name == ToolAskUserQuestion {
								specCancelled = true
								specCancel()
								break
							}
						}
						if !specCancelled {
							for _, tc := range chunk.ToolCalls {
								if !startSpecTool(tc) {
									gen.Close()
									return nil // Consumer stopped
								}
							}
						}
					}
				}

				// Drain any completed speculative tool results to avoid backpressure.
				if !drainSpecResults(false) {
					gen.Close()
					return nil
				}

				// Capture final metadata
				if chunk.Done {
					usage = chunk.Usage
					finishReason = chunk.FinishReason
					citations = chunk.Citations
					providerResponseID = chunk.ProviderResponseID
				}
			}

			// Close generator immediately after exhausting
			gen.Close()

			// Build response message with all metadata
			msgOpts := []types.MessageOption{types.WithToolCalls(toolCalls...)}
			if len(citations) > 0 {
				msgOpts = append(msgOpts, types.WithCitations(citations...))
			}
			responseMessage = types.AssistantMessage(joinStrings(chunks), msgOpts...)
			messages = append(messages, responseMessage)
			if providerResponseID != "" {
				id := providerResponseID
				previousResponseID = &id
			}

			// Emit LLM response event
			if e.eventBus != nil {
				modelInfo := e.model.Info()
				var usageTokens types.TokenUsage
				if usage != nil {
					usageTokens = *usage
				}

				// Get accumulated response text for trace Output
				responseText := joinStrings(chunks)

				// Use appropriate event constructor based on cache token presence
				var responseEvent *events.LLMResponseEvent
				if usageTokens.CacheWriteTokens > 0 || usageTokens.CacheReadTokens > 0 {
					responseEvent = events.NewLLMResponseEventWithCache(
						input.SessionID,
						input.TenantID,
						modelInfo.Name,
						modelInfo.Provider,
						usageTokens.PromptTokens,
						usageTokens.CompletionTokens,
						usageTokens.TotalTokens,
						usageTokens.CacheWriteTokens,
						usageTokens.CacheReadTokens,
						time.Since(startTime).Milliseconds(),
						finishReason,
						len(toolCalls),
						responseText,
					)
				} else {
					responseEvent = events.NewLLMResponseEvent(
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
						responseText,
					)
				}
				_ = e.eventBus.Publish(ctx, responseEvent)
			}

			// Check for tool calls
			if len(toolCalls) == 0 {
				// No tools, execution complete
				result := &Response{
					Message:            responseMessage,
					FinishReason:       finishReason,
					ProviderResponseID: providerResponseID,
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
			var toolResults []types.Message
			var interrupt *InterruptEvent
			var toolErr error

			if specEnabled {
				// If interrupt tool exists in the final set, treat it as exclusive.
				for _, tc := range toolCalls {
					if tc.Name == ToolAskUserQuestion {
						specCancelled = true
						specCancel()
						break
					}
				}
			}

			if specEnabled && !specCancelled {
				// Ensure all tool calls are started (some may only be known at the final chunk).
				for _, tc := range toolCalls {
					if !startSpecTool(tc) {
						return nil // Consumer stopped
					}
				}

				// Drain remaining tool results (blocking) until all speculative executions finish.
				if !drainSpecResults(true) {
					return nil
				}

				toolResults = make([]types.Message, 0, len(toolCalls))
				for _, tc := range toolCalls {
					if msg, ok := specResults[tc.ID]; ok && msg != nil {
						toolResults = append(toolResults, msg)
					}
				}
			} else {
				toolResults, interrupt, toolErr = e.executeToolCalls(ctx, input.SessionID, input.TenantID, tools, toolCalls, yield)
			}

			if toolErr != nil {
				if !yield(ExecutorEvent{
					Type:  EventTypeError,
					Error: serrors.E(op, toolErr),
				}) {
					return nil
				}
				return serrors.E(op, toolErr)
			}

			// Check for interrupt
			if interrupt != nil {
				// Save checkpoint
				checkpointID, err := e.saveCheckpoint(ctx, threadID, messages, toolCalls, interrupt, input.SessionID, input.TenantID, previousResponseID)
				if err != nil {
					return serrors.E(op, ErrCheckpointSaveFailed, err)
				}

				// Set checkpoint ID on interrupt event
				interrupt.AgentName = e.agent.Metadata().Name
				interrupt.SessionID = input.SessionID
				interrupt.CheckpointID = checkpointID
				if previousResponseID != nil {
					interrupt.ProviderResponseID = *previousResponseID
				}

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
							if tr.ToolCallID() != nil && *tr.ToolCallID() == tc.ID {
								resultContent = tr.Content()
								break
							}
						}

						// Termination tool called, return result
						result := &Response{
							Message:            types.AssistantMessage(resultContent),
							FinishReason:       "tool_calls",
							ProviderResponseID: providerResponseID,
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
func (e *Executor) Resume(ctx context.Context, checkpointID string, answers map[string]types.Answer) types.Generator[ExecutorEvent] {
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

		// Prefer canonical interrupt payload from checkpoint (source of truth).
		var interruptPayload types.AskUserQuestionPayload
		if checkpoint.InterruptType == ToolAskUserQuestion {
			if len(checkpoint.InterruptData) == 0 {
				return serrors.E(op, serrors.KindValidation, "missing interrupt payload in checkpoint")
			}
			if err := json.Unmarshal(checkpoint.InterruptData, &interruptPayload); err != nil {
				return serrors.E(op, err, "failed to parse interrupt payload from checkpoint")
			}
			if interruptPayload.Type != types.InterruptTypeAskUserQuestion {
				return serrors.E(op, serrors.KindValidation, "invalid interrupt payload type in checkpoint")
			}
		}

		// Add tool results with user's answers
		for _, tc := range checkpoint.PendingTools {
			// Check if this is an interrupt tool that needs answers
			if tc.Name == ToolAskUserQuestion {
				payload := interruptPayload

				// Validate answers cover all required question IDs
				responseData := make(map[string]json.RawMessage)
				for _, q := range payload.Questions {
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

					// Store answer as JSON (supports both string and []string)
					responseData[q.ID] = answer.Value
				}

				// Create tool response message with all answers (JSON-encoded)
				encoded, _ := json.Marshal(responseData)
				messages = append(messages, types.ToolResponse(tc.ID, string(encoded)))
			}
		}

		// Resume execution with restored state
		input := Input{
			Messages:           messages,
			SessionID:          checkpoint.SessionID,
			TenantID:           checkpoint.TenantID,
			ThreadID:           checkpoint.ThreadID,
			PreviousResponseID: checkpoint.PreviousResponseID,
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
				resumeGen.Close()
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
	tools []Tool,
	toolCalls []types.ToolCall,
	yield func(ExecutorEvent) bool,
) ([]types.Message, *InterruptEvent, error) {
	const op serrors.Op = "Executor.executeToolCalls"

	if len(toolCalls) == 0 {
		return nil, nil, nil
	}

	// Interrupt tool handling is exclusive (do not execute other tools in the same batch).
	var interruptIdx = -1
	for i, tc := range toolCalls {
		if tc.Name == ToolAskUserQuestion {
			if interruptIdx != -1 {
				return nil, nil, serrors.E(op, serrors.KindValidation, "multiple interrupt tool calls in one batch are not supported")
			}
			interruptIdx = i
		}
	}
	if interruptIdx != -1 {
		tc := toolCalls[interruptIdx]

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

		payload, err := parseAndCanonicalizeAskUserQuestionArgs(tc.Arguments)
		if err != nil {
			return nil, nil, serrors.E(op, err)
		}

		interruptData, err := json.Marshal(payload)
		if err != nil {
			return nil, nil, serrors.E(op, err, "failed to marshal interrupt payload")
		}

		interrupt := &InterruptEvent{
			Type: ToolAskUserQuestion,
			Data: interruptData,
		}

		return nil, interrupt, nil
	}

	toolByName := make(map[string]Tool, len(tools))
	for _, tool := range tools {
		if tool == nil {
			continue
		}
		if _, exists := toolByName[tool.Name()]; !exists {
			toolByName[tool.Name()] = tool
		}
	}

	type toolResult struct {
		idx        int
		call       types.ToolCall
		result     string
		err        error
		durationMs int64
	}

	toolCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	resultsCh := make(chan toolResult, len(toolCalls))

	// Concurrency-keyed locks (serialize tools sharing the same key).
	keyLocks := make(map[string]*sync.Mutex)
	var keyLocksMu sync.Mutex
	getKeyLock := func(key string) *sync.Mutex {
		keyLocksMu.Lock()
		defer keyLocksMu.Unlock()
		if m, ok := keyLocks[key]; ok {
			return m
		}
		m := &sync.Mutex{}
		keyLocks[key] = m
		return m
	}

	// Emit tool start events before launching each tool execution.
	for i, tc := range toolCalls {
		// Emit tool start event
		if e.eventBus != nil {
			_ = e.eventBus.Publish(toolCtx, events.NewToolStartEvent(
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
			cancel()
			return nil, nil, nil // Consumer stopped
		}

		// Determine concurrency key (optional).
		key := ""
		if tool := toolByName[tc.Name]; tool != nil {
			if keyed, ok := tool.(ToolConcurrency); ok {
				key = keyed.ConcurrencyKey()
			}
		}

		go func(idx int, call types.ToolCall, concurrencyKey string) {
			startTime := time.Now()

			if concurrencyKey != "" {
				lock := getKeyLock(concurrencyKey)
				lock.Lock()
				defer lock.Unlock()
			}

			res, err := e.callTool(toolCtx, call.Name, call.Arguments)
			durationMs := time.Since(startTime).Milliseconds()

			select {
			case resultsCh <- toolResult{
				idx:        idx,
				call:       call,
				result:     res,
				err:        err,
				durationMs: durationMs,
			}:
			case <-toolCtx.Done():
				return
			}
		}(i, tc, key)
	}

	ordered := make([]types.Message, len(toolCalls))
	received := 0

	for received < len(toolCalls) {
		var tr toolResult
		select {
		case tr = <-resultsCh:
		case <-toolCtx.Done():
			return nil, nil, serrors.E(op, toolCtx.Err())
		}

		received++

		toolOutput := tr.result
		if tr.err != nil {
			toolOutput = fmt.Sprintf("Error: %v", tr.err)
		}

		// Emit tool completion/error event
		if e.eventBus != nil {
			if tr.err != nil {
				_ = e.eventBus.Publish(toolCtx, events.NewToolErrorEvent(
					sessionID,
					tenantID,
					tr.call.Name,
					tr.call.Arguments,
					tr.call.ID,
					tr.err.Error(),
					tr.durationMs,
				))
			} else {
				_ = e.eventBus.Publish(toolCtx, events.NewToolCompleteEvent(
					sessionID,
					tenantID,
					tr.call.Name,
					tr.call.Arguments,
					tr.call.ID,
					toolOutput,
					tr.durationMs,
				))
			}
		}

		// Yield tool end event
		if !yield(ExecutorEvent{
			Type: EventTypeToolEnd,
			Tool: &ToolEvent{
				CallID:     tr.call.ID,
				Name:       tr.call.Name,
				Arguments:  tr.call.Arguments,
				Result:     toolOutput,
				Error:      tr.err,
				DurationMs: tr.durationMs,
			},
		}) {
			cancel()
			return nil, nil, nil // Consumer stopped
		}

		ordered[tr.idx] = types.ToolResponse(tr.call.ID, toolOutput)
	}

	results := make([]types.Message, 0, len(ordered))
	for _, msg := range ordered {
		if msg != nil {
			results = append(results, msg)
		}
	}

	return results, nil, nil
}

func parseAndCanonicalizeAskUserQuestionArgs(args string) (types.AskUserQuestionPayload, error) {
	const op serrors.Op = "Executor.parseAndCanonicalizeAskUserQuestionArgs"

	parsed, err := ParseToolInput[askUserQuestionArgs](args)
	if err != nil {
		return types.AskUserQuestionPayload{}, serrors.E(op, err, "failed to parse ask_user_question arguments")
	}

	if len(parsed.Questions) == 0 {
		return types.AskUserQuestionPayload{}, serrors.E(op, serrors.KindValidation, "at least one question required")
	}
	if len(parsed.Questions) > 4 {
		return types.AskUserQuestionPayload{}, serrors.E(op, serrors.KindValidation, "maximum 4 questions allowed")
	}

	questionIDs := make(map[string]bool)
	canonicalQuestions := make([]types.AskUserQuestion, 0, len(parsed.Questions))

	for i, q := range parsed.Questions {
		if q.Question == "" {
			return types.AskUserQuestionPayload{}, serrors.E(op, serrors.KindValidation, fmt.Sprintf("question[%d]: question text is required", i))
		}
		if q.Header == "" {
			return types.AskUserQuestionPayload{}, serrors.E(op, serrors.KindValidation, fmt.Sprintf("question[%d]: header is required", i))
		}
		if len(q.Header) > 12 {
			return types.AskUserQuestionPayload{}, serrors.E(op, serrors.KindValidation, fmt.Sprintf("question[%d]: header exceeds 12 characters", i))
		}
		if len(q.Options) < 2 {
			return types.AskUserQuestionPayload{}, serrors.E(op, serrors.KindValidation, fmt.Sprintf("question[%d]: at least 2 options required", i))
		}
		if len(q.Options) > 4 {
			return types.AskUserQuestionPayload{}, serrors.E(op, serrors.KindValidation, fmt.Sprintf("question[%d]: maximum 4 options allowed", i))
		}

		qid := q.ID
		if qid == "" {
			qid = fmt.Sprintf("q%d", i+1)
		}
		if questionIDs[qid] {
			return types.AskUserQuestionPayload{}, serrors.E(op, serrors.KindValidation, fmt.Sprintf("duplicate question ID: %s", qid))
		}
		questionIDs[qid] = true

		optionIDs := make(map[string]bool)
		canonicalOptions := make([]types.QuestionOption, 0, len(q.Options))
		for j, opt := range q.Options {
			if opt.Label == "" {
				return types.AskUserQuestionPayload{}, serrors.E(op, serrors.KindValidation, fmt.Sprintf("question[%d].option[%d]: label is required", i, j))
			}
			if opt.Description == "" {
				return types.AskUserQuestionPayload{}, serrors.E(op, serrors.KindValidation, fmt.Sprintf("question[%d].option[%d]: description is required", i, j))
			}

			oid := opt.ID
			if oid == "" {
				oid = fmt.Sprintf("%s_opt%d", qid, j+1)
			}
			if optionIDs[oid] {
				return types.AskUserQuestionPayload{}, serrors.E(op, serrors.KindValidation, fmt.Sprintf("question[%d]: duplicate option ID: %s", i, oid))
			}
			optionIDs[oid] = true

			canonicalOptions = append(canonicalOptions, types.QuestionOption{
				ID:          oid,
				Label:       opt.Label,
				Description: opt.Description,
			})
		}

		canonicalQuestions = append(canonicalQuestions, types.AskUserQuestion{
			ID:          qid,
			Question:    q.Question,
			Header:      q.Header,
			MultiSelect: q.MultiSelect,
			Options:     canonicalOptions,
		})
	}

	return types.AskUserQuestionPayload{
		Type:      types.InterruptTypeAskUserQuestion,
		Questions: canonicalQuestions,
		Metadata:  parsed.Metadata,
	}, nil
}

// saveCheckpoint creates and saves a checkpoint for HITL resumption.
func (e *Executor) saveCheckpoint(
	ctx context.Context,
	threadID string,
	messages []types.Message,
	toolCalls []types.ToolCall,
	interrupt *InterruptEvent,
	sessionID uuid.UUID,
	tenantID uuid.UUID,
	previousResponseID *string,
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
		WithSessionID(sessionID),
		WithTenantID(tenantID),
		WithCheckpointPreviousResponseID(previousResponseID),
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
