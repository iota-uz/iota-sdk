package agents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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

// EventEmitter is a callback that pushes an ExecutorEvent into the parent
// generator's yield stream. It returns false if the consumer has stopped.
// Used by streaming tools (e.g., delegation) to propagate child events.
type EventEmitter = func(ExecutorEvent) bool

type eventEmitterKey struct{}

// Deprecated: WithEventEmitter stores an EventEmitter in the context.
// Prefer implementing the StreamingTool interface instead.
func WithEventEmitter(ctx context.Context, emitter EventEmitter) context.Context {
	return context.WithValue(ctx, eventEmitterKey{}, emitter)
}

// Deprecated: EventEmitterFromContext retrieves the EventEmitter from the context.
// Prefer implementing the StreamingTool interface instead.
func EventEmitterFromContext(ctx context.Context) (EventEmitter, bool) {
	emitter, ok := ctx.Value(eventEmitterKey{}).(EventEmitter)
	return emitter, ok
}

// ExecutorEventType identifies different types of executor events.
type ExecutorEventType string

const (
	// EventTypeContent is emitted for streaming text chunks from the LLM.
	// Content is available in both Chunk.Delta (raw) and Content (convenience field).
	EventTypeContent ExecutorEventType = "content"

	// EventTypeChunk is an alias for EventTypeContent for backward compatibility.
	// Prefer EventTypeContent in new code.
	EventTypeChunk ExecutorEventType = "content"

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

	// EventTypeThinking is emitted for reasoning/thinking content from the LLM.
	// Thinking content is ephemeral — it is shown to the user during generation
	// but is not persisted as part of the final assistant message.
	EventTypeThinking ExecutorEventType = "thinking"
)

// Question represents a structured HITL question carried in ExecutorEvent.
// It is populated from the raw JSON interrupt payload so that consumers
// do not need to parse AskUserQuestionPayload themselves.
type Question struct {
	ID      string
	Text    string
	Type    QuestionType
	Options []QuestionOption
}

// QuestionType indicates whether a question is single- or multiple-choice.
type QuestionType string

const (
	QuestionTypeSingleChoice   QuestionType = "single_choice"
	QuestionTypeMultipleChoice QuestionType = "multiple_choice"
)

// QuestionOption is one selectable choice within a Question.
type QuestionOption struct {
	ID    string
	Label string
}

// ParsedInterrupt holds the structured representation of an interrupt event.
// It is derived from InterruptEvent.Data so that consumers do not need to
// unmarshal the raw JSON themselves.
type ParsedInterrupt struct {
	CheckpointID       string
	AgentName          string
	ProviderResponseID string
	Questions          []Question
}

// ExecutorEvent represents a single event during ReAct loop execution.
// Events are yielded as the executor progresses through reasoning and action steps.
type ExecutorEvent struct {
	// Type identifies what kind of event this is.
	Type ExecutorEventType

	// Content holds the text delta for EventTypeContent events.
	// It is identical to Chunk.Delta and provided as a convenience so that
	// callers do not need to nil-check Chunk.
	Content string

	// Chunk contains streaming text data (for EventTypeContent).
	Chunk *Chunk

	// Tool contains tool execution data (for EventTypeToolStart/EventTypeToolEnd).
	Tool *ToolEvent

	// Interrupt contains HITL interrupt data (for EventTypeInterrupt).
	// Use ParsedInterrupt for the structured representation.
	Interrupt *InterruptEvent

	// ParsedInterrupt holds the structured representation of Interrupt.
	// It is always populated when Type == EventTypeInterrupt.
	ParsedInterrupt *ParsedInterrupt

	// Error contains error information (for EventTypeError).
	Error error

	// Done indicates execution has completed (for EventTypeDone).
	Done bool

	// Result contains the final response (for EventTypeDone).
	Result *Response

	// Usage holds token-consumption metrics extracted from Result.Usage.
	// It is populated when Type == EventTypeDone and token usage is available.
	Usage *types.DebugUsage

	// ProviderResponseID is the provider continuity token, extracted from Result.
	// It is populated when Type == EventTypeDone.
	ProviderResponseID string

	// CodeInterpreter holds code interpreter results extracted from Result.
	// It is populated when Type == EventTypeDone and code interpreter was used.
	CodeInterpreter []types.CodeInterpreterResult

	// FileAnnotations holds file references extracted from Result.
	// It is populated when Type == EventTypeDone and files were generated.
	FileAnnotations []types.FileAnnotation
}

// ToolEvent represents a tool execution event.
type ToolEvent struct {
	// CallID uniquely identifies this tool call.
	CallID string

	// Name is the tool being executed.
	Name string

	// AgentName identifies which agent this tool call belongs to.
	// Empty for the primary agent; set to the sub-agent's name when
	// tool events are forwarded from a child executor via delegation.
	AgentName string

	// Arguments is the JSON-encoded input.
	Arguments string

	// Result is the tool output (only present in EventTypeToolEnd).
	Result string

	// Error is the tool error (only present in EventTypeToolEnd if failed).
	Error error

	// DurationMs is the execution time in milliseconds (only present in EventTypeToolEnd).
	DurationMs int64

	// Artifacts are generated outputs returned by the tool execution.
	Artifacts []types.ToolArtifact
}

type toolExecutionResult struct {
	output    string
	artifacts []types.ToolArtifact
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
	tools             []Tool                  // Optional override for agent tools (e.g., filtered for child executors)
	tokenEstimator    TokenEstimator          // Optional token estimator for cost tracking
	speculativeTools  bool                    // Start executing ready tool calls during streaming (best-effort)
	formatterRegistry types.FormatterRegistry // Optional formatter registry for StructuredTool support
}

// Input represents the input to Execute or Resume.
type Input struct {
	// Messages is the conversation history to start with.
	Messages []types.Message

	// SessionID identifies the chat session for observability and checkpointing.
	SessionID uuid.UUID

	// TenantID identifies the tenant for observability and checkpointing.
	TenantID uuid.UUID

	// TraceID identifies a single execution run for observability.
	// If empty, Execute() will generate one.
	TraceID string

	// ThreadID identifies the conversation thread for checkpointing.
	// If empty, a new thread ID will be generated.
	ThreadID string

	// PreviousResponseID is a provider continuity token for multi-turn state.
	// For OpenAI Responses API this maps to previous_response_id.
	PreviousResponseID *string

	// isResume is set internally by Resume() so that AgentStartEvent.IsResume
	// is emitted correctly. Callers should not set this directly.
	isResume bool
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
// Passing nil is ignored and the default event bus will be kept.
func WithEventBus(eventBus hooks.EventBus) ExecutorOption {
	return func(e *Executor) {
		if eventBus != nil {
			e.eventBus = eventBus
		}
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
func WithFormatterRegistry(registry types.FormatterRegistry) ExecutorOption {
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
		eventBus:          hooks.NewEventBus(),
	}

	for _, opt := range opts {
		opt(executor)
	}

	return executor
}

// callTool executes a tool, using StructuredTool + FormatterRegistry when available.
// Accepts a tool directly to avoid O(n) lookup. If tool is nil or has nil handler,
// falls back to agent.OnToolCall for backward compatibility.
// After invoking StructuredTool.CallStructured we always return and never fall through to Tool.Call() to avoid double execution.
func (e *Executor) callTool(ctx context.Context, tool Tool, toolName, arguments string) (toolExecutionResult, error) {
	if e.formatterRegistry != nil && tool != nil {
		if st, ok := tool.(StructuredTool); ok {
			result, err := st.CallStructured(ctx, arguments)
			if err != nil && result == nil {
				return toolExecutionResult{}, err
			}
			if result == nil {
				return toolExecutionResult{}, nil
			}
			execResult := toolExecutionResult{
				artifacts: result.Artifacts,
			}
			// result != nil: format or fallback, then return (never fall through to tool.Call())
			if f := e.formatterRegistry.Get(result.CodecID); f != nil {
				formatted, fmtErr := f.Format(result.Payload, types.DefaultFormatOptions())
				if fmtErr == nil {
					execResult.output = formatted
					if err != nil && errors.Is(err, ErrStructuredToolOutput) {
						return execResult, nil
					}
					return execResult, err
				}
			}
			// Formatter missing or format failed: return raw payload stringified
			fallback, _ := FormatToolOutput(result.Payload)
			if fallback == "" {
				fallback = fmt.Sprintf("%v", result.Payload)
			}
			execResult.output = fallback
			if err != nil && errors.Is(err, ErrStructuredToolOutput) {
				return execResult, nil
			}
			return execResult, err
		}
	}

	// Non-StructuredTool path: ToolFunc nil-handler fallback and agent.OnToolCall
	if tool != nil {
		if tf, ok := tool.(*ToolFunc); ok && tf.Fn == nil {
			output, err := e.agent.OnToolCall(ctx, toolName, arguments)
			return toolExecutionResult{output: output}, err
		}
		output, err := tool.Call(ctx, arguments)
		return toolExecutionResult{output: output}, err
	}
	output, err := e.agent.OnToolCall(ctx, toolName, arguments)
	return toolExecutionResult{output: output}, err
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

		// Agent lifecycle tracking
		agentStartTime := time.Now()
		agentName := e.agent.Metadata().Name
		totalAgentTokens := 0
		traceID := strings.TrimSpace(input.TraceID)
		if traceID == "" {
			traceID = uuid.New().String()
		}

		// Emit agent.start event
		_ = e.eventBus.Publish(ctx, events.NewAgentStartEventWithTrace(
			input.SessionID,
			input.TenantID,
			traceID,
			agentName,
			input.isResume,
		))

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
			modelInfo := e.model.Info()
			requestID := uuid.New().String()

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

			_ = e.eventBus.Publish(ctx, events.NewLLMRequestEventWithTrace(
				input.SessionID,
				input.TenantID,
				traceID,
				requestID,
				modelInfo.Name,
				modelInfo.Provider,
				len(req.Messages),
				len(req.Tools),
				estimatedTokens,
				userInput,
			))

			// Call model (streaming)
			var streamOpts []GenerateOption
			if e.model.HasCapability(CapabilityThinking) {
				streamOpts = append(streamOpts, WithReasoningEffort(ReasoningMedium))
			}
			gen, err := e.model.Stream(ctx, req, streamOpts...)
			if err != nil {
				// Emit agent.error event
				_ = e.eventBus.Publish(ctx, events.NewAgentErrorEventWithTrace(
					input.SessionID, input.TenantID, traceID, agentName,
					iteration, err.Error(),
					time.Since(agentStartTime).Milliseconds(),
				))

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
			var thinkingChunks []string
			startTime := time.Now()

			// Speculative tool execution state (best-effort).
			type specToolResult struct {
				call       types.ToolCall
				result     toolExecutionResult
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

				toolOutput := tr.result.output
				if tr.err != nil {
					toolOutput = fmt.Sprintf("Error: %v", tr.err)
				}

				// Emit tool completion/error event
				if tr.err != nil {
					_ = e.eventBus.Publish(ctx, events.NewToolErrorEventWithTrace(
						input.SessionID,
						input.TenantID,
						traceID,
						tr.call.Name,
						tr.call.Arguments,
						tr.call.ID,
						tr.err.Error(),
						tr.durationMs,
					))
				} else {
					_ = e.eventBus.Publish(ctx, events.NewToolCompleteEventWithTrace(
						input.SessionID,
						input.TenantID,
						traceID,
						tr.call.Name,
						tr.call.Arguments,
						tr.call.ID,
						toolOutput,
						tr.result.artifacts,
						tr.durationMs,
					))
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
						Artifacts:  tr.result.artifacts,
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
				_ = e.eventBus.Publish(specToolCtx, events.NewToolStartEventWithTrace(
					input.SessionID,
					input.TenantID,
					traceID,
					tc.Name,
					tc.Arguments,
					tc.ID,
				))

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

				go func(call types.ToolCall, concurrencyKey string, t Tool) {
					startedAt := time.Now()

					if concurrencyKey != "" {
						lock := getSpecKeyLock(concurrencyKey)
						lock.Lock()
						defer lock.Unlock()
					}

					res, err := e.callTool(specToolCtx, t, call.Name, call.Arguments)
					durationMs := time.Since(startedAt).Milliseconds()

					select {
					case specResultsCh <- specToolResult{call: call, result: res, err: err, durationMs: durationMs}:
					case <-specToolCtx.Done():
						return
					}
				}(tc, key, toolByName[tc.Name])

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

				// Yield thinking event (ephemeral reasoning content)
				if chunk.Thinking != "" {
					thinkingChunks = append(thinkingChunks, chunk.Thinking)
					if !yield(ExecutorEvent{
						Type:    EventTypeThinking,
						Content: chunk.Thinking,
					}) {
						gen.Close()
						return nil // Consumer stopped
					}
				}

				// Yield chunk event
				if chunk.Delta != "" {
					chunks = append(chunks, chunk.Delta)
					if !yield(ExecutorEvent{
						Type:    EventTypeContent,
						Content: chunk.Delta,
						Chunk:   &chunk,
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

			// Accumulate tokens for agent lifecycle tracking
			if usage != nil {
				totalAgentTokens += usage.TotalTokens
			}

			// Emit LLM response event
			{
				modelInfo := e.model.Info()
				var usageTokens types.TokenUsage
				if usage != nil {
					usageTokens = *usage
				}

				// Get accumulated response text for trace Output
				responseText := joinStrings(chunks)
				thinkingText := joinStrings(thinkingChunks)
				toolSummary := make([]events.LLMToolCallSummary, 0, len(toolCalls))
				for _, tc := range toolCalls {
					toolSummary = append(toolSummary, events.LLMToolCallSummary{
						CallID: tc.ID,
						Name:   tc.Name,
					})
				}

				// Use appropriate event constructor based on cache token presence
				var responseEvent *events.LLMResponseEvent
				if usageTokens.CacheWriteTokens > 0 || usageTokens.CacheReadTokens > 0 {
					responseEvent = events.NewLLMResponseEventWithCacheAndTrace(
						input.SessionID,
						input.TenantID,
						traceID,
						requestID,
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
						thinkingText,
						"",
						toolSummary,
					)
				} else {
					responseEvent = events.NewLLMResponseEventWithTrace(
						input.SessionID,
						input.TenantID,
						traceID,
						requestID,
						modelInfo.Name,
						modelInfo.Provider,
						usageTokens.PromptTokens,
						usageTokens.CompletionTokens,
						usageTokens.TotalTokens,
						time.Since(startTime).Milliseconds(),
						finishReason,
						len(toolCalls),
						responseText,
						thinkingText,
						"",
						toolSummary,
					)
				}
				// Attach model parameters if the model reports them.
				if reporter, ok := e.model.(ModelParameterReporter); ok {
					responseEvent.ModelParameters = reporter.ModelParameters()
				}

				_ = e.eventBus.Publish(ctx, responseEvent)
			}

			// Check for tool calls
			if len(toolCalls) == 0 {
				// Emit agent.complete event
				_ = e.eventBus.Publish(ctx, events.NewAgentCompleteEventWithTrace(
					input.SessionID, input.TenantID, traceID, agentName,
					iteration, totalAgentTokens,
					time.Since(agentStartTime).Milliseconds(),
				))

				// No tools, execution complete
				result := &Response{
					TraceID:            traceID,
					RequestID:          requestID,
					Model:              modelInfo.Name,
					Provider:           modelInfo.Provider,
					Message:            responseMessage,
					FinishReason:       finishReason,
					Thinking:           joinStrings(thinkingChunks),
					ProviderResponseID: providerResponseID,
				}
				if usage != nil {
					result.Usage = *usage
				}

				if !yield(buildDoneEvent(result)) {
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
				toolResults, interrupt, toolErr = e.executeToolCalls(ctx, input.SessionID, input.TenantID, traceID, tools, toolCalls, yield)
			}

			if toolErr != nil {
				// Emit agent.error event
				_ = e.eventBus.Publish(ctx, events.NewAgentErrorEventWithTrace(
					input.SessionID, input.TenantID, traceID, agentName,
					iteration, toolErr.Error(),
					time.Since(agentStartTime).Milliseconds(),
				))

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

				// Emit agent.complete event (interrupted, will resume later)
				_ = e.eventBus.Publish(ctx, events.NewAgentCompleteEventWithTrace(
					input.SessionID, input.TenantID, traceID, agentName,
					iteration, totalAgentTokens,
					time.Since(agentStartTime).Milliseconds(),
				))

				// Yield interrupt event to generator consumer
				if !yield(ExecutorEvent{
					Type:            EventTypeInterrupt,
					Interrupt:       interrupt,
					ParsedInterrupt: parseInterruptEvent(interrupt),
				}) {
					return nil
				}

				// Stop execution (will be resumed later via Resume())
				return nil
			}

			// Add tool results to messages
			messages = append(messages, toolResults...)

			// Continue to next iteration
		}

		// Emit agent.error event (max iterations)
		_ = e.eventBus.Publish(ctx, events.NewAgentErrorEventWithTrace(
			input.SessionID, input.TenantID, traceID, agentName,
			iteration, ErrMaxIterations.Error(),
			time.Since(agentStartTime).Milliseconds(),
		))

		// Max iterations reached
		return serrors.E(op, ErrMaxIterations)
	}, types.WithBufferSize(32))
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
			TraceID:            uuid.New().String(),
			ThreadID:           checkpoint.ThreadID,
			PreviousResponseID: checkpoint.PreviousResponseID,
			isResume:           true,
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
	}, types.WithBufferSize(32))
}

// executeToolCalls executes all tool calls in parallel and returns their results.
// If any tool triggers an interrupt, returns the interrupt event.
func (e *Executor) executeToolCalls(
	ctx context.Context,
	sessionID, tenantID uuid.UUID,
	traceID string,
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
		_ = e.eventBus.Publish(ctx, events.NewToolStartEventWithTrace(
			sessionID,
			tenantID,
			traceID,
			tc.Name,
			tc.Arguments,
			tc.ID,
		))

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
		result     toolExecutionResult
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
		_ = e.eventBus.Publish(toolCtx, events.NewToolStartEventWithTrace(
			sessionID,
			tenantID,
			traceID,
			tc.Name,
			tc.Arguments,
			tc.ID,
		))

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

		go func(idx int, call types.ToolCall, concurrencyKey string, t Tool, emitFn EventEmitter) {
			startTime := time.Now()

			if concurrencyKey != "" {
				lock := getKeyLock(concurrencyKey)
				lock.Lock()
				defer lock.Unlock()
			}

			var res toolExecutionResult
			var err error

			// Prefer StreamingTool for event-emitting tools (e.g., delegation).
			if st, ok := t.(StreamingTool); ok {
				var result string
				result, err = st.CallStreaming(toolCtx, call.Arguments, emitFn)
				if err == nil {
					res = toolExecutionResult{output: result}
				}
			} else {
				res, err = e.callTool(toolCtx, t, call.Name, call.Arguments)
			}

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
		}(i, tc, key, toolByName[tc.Name], yield)
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

		toolOutput := tr.result.output
		if tr.err != nil {
			toolOutput = fmt.Sprintf("Error: %v", tr.err)
		}

		// Emit tool completion/error event
		if tr.err != nil {
			_ = e.eventBus.Publish(toolCtx, events.NewToolErrorEventWithTrace(
				sessionID,
				tenantID,
				traceID,
				tr.call.Name,
				tr.call.Arguments,
				tr.call.ID,
				tr.err.Error(),
				tr.durationMs,
			))
		} else {
			_ = e.eventBus.Publish(toolCtx, events.NewToolCompleteEventWithTrace(
				sessionID,
				tenantID,
				traceID,
				tr.call.Name,
				tr.call.Arguments,
				tr.call.ID,
				toolOutput,
				tr.result.artifacts,
				tr.durationMs,
			))
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
				Artifacts:  tr.result.artifacts,
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
		if len(q.Header) > 50 {
			return types.AskUserQuestionPayload{}, serrors.E(op, serrors.KindValidation, fmt.Sprintf("question[%d]: header exceeds 50 characters", i))
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

// parseInterruptEvent converts an InterruptEvent's raw JSON payload into a
// ParsedInterrupt that callers can consume without parsing JSON themselves.
// The function is best-effort: if parsing fails, Questions is left nil.
func parseInterruptEvent(interrupt *InterruptEvent) *ParsedInterrupt {
	if interrupt == nil {
		return nil
	}

	parsed := &ParsedInterrupt{
		CheckpointID:       interrupt.CheckpointID,
		AgentName:          interrupt.AgentName,
		ProviderResponseID: interrupt.ProviderResponseID,
	}

	if len(interrupt.Data) > 0 {
		var payload types.AskUserQuestionPayload
		if err := json.Unmarshal(interrupt.Data, &payload); err == nil {
			questions := make([]Question, 0, len(payload.Questions))
			for _, q := range payload.Questions {
				options := make([]QuestionOption, 0, len(q.Options))
				for _, opt := range q.Options {
					options = append(options, QuestionOption{
						ID:    opt.ID,
						Label: opt.Label,
					})
				}
				qt := QuestionTypeSingleChoice
				if q.MultiSelect {
					qt = QuestionTypeMultipleChoice
				}
				questions = append(questions, Question{
					ID:      q.ID,
					Text:    q.Question,
					Type:    qt,
					Options: options,
				})
			}
			parsed.Questions = questions
		}
	}

	return parsed
}

// buildDoneEvent constructs an ExecutorEvent for EventTypeDone, populating
// convenience fields (Usage, ProviderResponseID, CodeInterpreter, FileAnnotations)
// from the Response so that callers do not need to inspect Result directly.
func buildDoneEvent(result *Response) ExecutorEvent {
	ev := ExecutorEvent{
		Type:   EventTypeDone,
		Done:   true,
		Result: result,
	}
	if result == nil {
		return ev
	}
	ev.ProviderResponseID = result.ProviderResponseID
	ev.CodeInterpreter = result.CodeInterpreterResults
	ev.FileAnnotations = result.FileAnnotations

	u := result.Usage
	if u.PromptTokens > 0 || u.CompletionTokens > 0 || u.TotalTokens > 0 ||
		u.CachedTokens > 0 || u.CacheReadTokens > 0 || u.CacheWriteTokens > 0 {
		cachedTokens := u.CachedTokens
		if cachedTokens == 0 {
			cachedTokens = u.CacheReadTokens
		}
		ev.Usage = &types.DebugUsage{
			PromptTokens:     u.PromptTokens,
			CompletionTokens: u.CompletionTokens,
			TotalTokens:      u.TotalTokens,
			CachedTokens:     cachedTokens,
		}
	}
	return ev
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
