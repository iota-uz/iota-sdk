package agents_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
)

// mockModel is a test model that returns predefined responses.
type mockModel struct {
	responses     []mockResponse
	currentIndex  int
	streamingMode bool
	info          agents.ModelInfo
}

type mockResponse struct {
	content      string
	toolCalls    []agents.ToolCall
	finishReason string
	err          error
}

func newMockModel(responses ...mockResponse) *mockModel {
	return &mockModel{
		responses:     responses,
		streamingMode: true,
		info: agents.ModelInfo{
			Name:     "mock-model",
			Provider: "test",
			Capabilities: []agents.Capability{
				agents.CapabilityStreaming,
				agents.CapabilityTools,
			},
		},
	}
}

func (m *mockModel) Generate(ctx context.Context, req agents.Request, opts ...agents.GenerateOption) (*agents.Response, error) {
	if m.currentIndex >= len(m.responses) {
		return nil, fmt.Errorf("no more mock responses")
	}

	resp := m.responses[m.currentIndex]
	m.currentIndex++

	if resp.err != nil {
		return nil, resp.err
	}

	return &agents.Response{
		Message: agents.Message{
			Role:      agents.RoleAssistant,
			Content:   resp.content,
			ToolCalls: resp.toolCalls,
		},
		Usage: agents.TokenUsage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
		FinishReason: resp.finishReason,
	}, nil
}

func (m *mockModel) Stream(ctx context.Context, req agents.Request, opts ...agents.GenerateOption) agents.Generator[agents.Chunk] {
	return agents.NewGenerator(func(yield func(agents.Chunk) bool) error {
		if m.currentIndex >= len(m.responses) {
			return fmt.Errorf("no more mock responses")
		}

		resp := m.responses[m.currentIndex]
		m.currentIndex++

		if resp.err != nil {
			return resp.err
		}

		// Simulate streaming by splitting content into chunks
		content := resp.content
		chunkSize := 5
		for i := 0; i < len(content); i += chunkSize {
			end := i + chunkSize
			if end > len(content) {
				end = len(content)
			}

			chunk := agents.Chunk{
				Delta:     content[i:end],
				ToolCalls: nil,
				Done:      false,
			}

			if !yield(chunk) {
				return nil
			}
		}

		// Final chunk with tool calls and metadata
		finalChunk := agents.Chunk{
			Delta:        "",
			ToolCalls:    resp.toolCalls,
			Usage:        &agents.TokenUsage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30},
			FinishReason: resp.finishReason,
			Done:         true,
		}

		if !yield(finalChunk) {
			return nil
		}

		return nil
	})
}

func (m *mockModel) Info() agents.ModelInfo {
	return m.info
}

func (m *mockModel) HasCapability(capability agents.Capability) bool {
	for _, cap := range m.info.Capabilities {
		if cap == capability {
			return true
		}
	}
	return false
}

// mockAgent is a test agent with configurable tools.
// Implements both Agent and ExtendedAgent interfaces.
type mockAgent struct {
	name             string
	tools            []agents.Tool
	systemPrompt     string
	terminationTools []string
	toolResults      map[string]string // tool name -> result
	toolErrors       map[string]error  // tool name -> error
}

func newMockAgent(name string, tools ...agents.Tool) *mockAgent {
	return &mockAgent{
		name:             name,
		tools:            tools,
		systemPrompt:     "You are a helpful assistant.",
		terminationTools: []string{agents.ToolFinalAnswer},
		toolResults:      make(map[string]string),
		toolErrors:       make(map[string]error),
	}
}

// Name implements Agent interface.
func (a *mockAgent) Name() string {
	return a.name
}

// Description implements Agent interface.
func (a *mockAgent) Description() string {
	return "Test agent"
}

// Metadata implements ExtendedAgent interface.
func (a *mockAgent) Metadata() agents.AgentMetadata {
	return agents.AgentMetadata{
		Name:             a.name,
		Description:      "Test agent",
		WhenToUse:        "For testing",
		Isolation:        agents.Isolated,
		Model:            "mock-model",
		TerminationTools: a.terminationTools,
	}
}

// Tools implements ExtendedAgent interface.
func (a *mockAgent) Tools() []agents.Tool {
	return a.tools
}

func (a *mockAgent) SystemPrompt(ctx context.Context) string {
	return a.systemPrompt
}

func (a *mockAgent) OnToolCall(ctx context.Context, toolName, input string) (string, error) {
	// Check for configured error
	if err, exists := a.toolErrors[toolName]; exists {
		return "", err
	}

	// Check for configured result
	if result, exists := a.toolResults[toolName]; exists {
		return result, nil
	}

	// Default: echo the input
	return fmt.Sprintf("Tool %s executed with: %s", toolName, input), nil
}

func (a *mockAgent) setToolResult(toolName, result string) {
	a.toolResults[toolName] = result
}

func (a *mockAgent) setToolError(toolName string, err error) {
	a.toolErrors[toolName] = err
}

// TestExecutor_SingleTurn tests basic request/response without tools.
func TestExecutor_SingleTurn(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Create mock model with simple response
	model := newMockModel(mockResponse{
		content:      "Hello! How can I help you today?",
		finishReason: "stop",
	})

	// Create mock agent
	agent := newMockAgent("test-agent")

	// Create executor
	executor := agents.NewExecutor(agent, model)

	// Execute
	input := agents.Input{
		Messages: []agents.Message{
			{Role: agents.RoleUser, Content: "Hello"},
		},
		SessionID: uuid.New(),
		TenantID:  uuid.New(),
	}

	gen := executor.Execute(ctx, input)
	defer gen.Close()

	// Collect events
	var chunks []string
	var finalResult *agents.Response
	var doneCount int

	for {
		event, err, hasMore := gen.Next()
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !hasMore {
			break
		}

		switch event.Type {
		case agents.EventTypeChunk:
			chunks = append(chunks, event.Chunk.Delta)
		case agents.EventTypeDone:
			doneCount++
			finalResult = event.Result
		case agents.EventTypeError:
			t.Fatalf("Unexpected error event: %v", event.Error)
		}
	}

	// Verify chunks
	fullContent := ""
	for _, chunk := range chunks {
		fullContent += chunk
	}
	if fullContent != "Hello! How can I help you today?" {
		t.Errorf("Expected full content 'Hello! How can I help you today?', got '%s'", fullContent)
	}

	// Verify done event
	if doneCount != 1 {
		t.Errorf("Expected 1 done event, got %d", doneCount)
	}

	// Verify final result
	if finalResult == nil {
		t.Fatal("Expected final result, got nil")
	}
	if finalResult.Message.Content != "Hello! How can I help you today?" {
		t.Errorf("Expected result content 'Hello! How can I help you today?', got '%s'", finalResult.Message.Content)
	}
	if finalResult.FinishReason != "stop" {
		t.Errorf("Expected finish reason 'stop', got '%s'", finalResult.FinishReason)
	}
}

// TestExecutor_ToolCalls tests tool execution and result handling.
func TestExecutor_ToolCalls(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Create test tool
	weatherTool := agents.NewTool(
		"get_weather",
		"Gets weather for a location",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"location": map[string]any{"type": "string"},
			},
		},
		func(ctx context.Context, input string) (string, error) {
			return `{"temperature": 72, "condition": "sunny"}`, nil
		},
	)

	// Create mock agent
	agent := newMockAgent("test-agent", weatherTool)
	agent.setToolResult("get_weather", `{"temperature": 72, "condition": "sunny"}`)

	// Create mock model with tool call response
	model := newMockModel(
		mockResponse{
			content: "Let me check the weather for you.",
			toolCalls: []agents.ToolCall{
				{
					ID:        "call_1",
					Name:      "get_weather",
					Arguments: `{"location": "San Francisco"}`,
				},
			},
			finishReason: "tool_calls",
		},
		mockResponse{
			content:      "The weather in San Francisco is 72°F and sunny.",
			finishReason: "stop",
		},
	)

	// Create executor
	executor := agents.NewExecutor(agent, model)

	// Execute
	input := agents.Input{
		Messages: []agents.Message{
			{Role: agents.RoleUser, Content: "What's the weather in San Francisco?"},
		},
		SessionID: uuid.New(),
		TenantID:  uuid.New(),
	}

	gen := executor.Execute(ctx, input)
	defer gen.Close()

	// Collect events
	var toolStartCount, toolEndCount int
	var toolStartEvent, toolEndEvent *agents.ToolEvent

	for {
		event, err, hasMore := gen.Next()
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !hasMore {
			break
		}

		switch event.Type {
		case agents.EventTypeToolStart:
			toolStartCount++
			toolStartEvent = event.Tool
		case agents.EventTypeToolEnd:
			toolEndCount++
			toolEndEvent = event.Tool
		case agents.EventTypeError:
			t.Fatalf("Unexpected error event: %v", event.Error)
		}
	}

	// Verify tool events
	if toolStartCount != 1 {
		t.Errorf("Expected 1 tool start event, got %d", toolStartCount)
	}
	if toolEndCount != 1 {
		t.Errorf("Expected 1 tool end event, got %d", toolEndCount)
	}

	// Verify tool start event
	if toolStartEvent != nil {
		if toolStartEvent.Name != "get_weather" {
			t.Errorf("Expected tool name 'get_weather', got '%s'", toolStartEvent.Name)
		}
		if toolStartEvent.CallID != "call_1" {
			t.Errorf("Expected call ID 'call_1', got '%s'", toolStartEvent.CallID)
		}
	}

	// Verify tool end event
	if toolEndEvent != nil {
		if toolEndEvent.Result != `{"temperature": 72, "condition": "sunny"}` {
			t.Errorf("Unexpected tool result: %s", toolEndEvent.Result)
		}
		if toolEndEvent.Error != nil {
			t.Errorf("Expected no tool error, got: %v", toolEndEvent.Error)
		}
		if toolEndEvent.DurationMs < 0 {
			t.Error("Expected non-negative duration")
		}
	}
}

// TestExecutor_MultiTurn tests conversation with multiple tool rounds.
func TestExecutor_MultiTurn(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Create test tools
	searchTool := agents.NewTool("search", "Search the web", map[string]any{}, nil)
	calculatorTool := agents.NewTool("calculator", "Perform calculations", map[string]any{}, nil)

	agent := newMockAgent("test-agent", searchTool, calculatorTool)
	agent.setToolResult("search", "The current price is $100")
	agent.setToolResult("calculator", "Result: 200")

	// Create mock model with multiple turns
	model := newMockModel(
		// Turn 1: Search
		mockResponse{
			content: "Let me search for that.",
			toolCalls: []agents.ToolCall{
				{ID: "call_1", Name: "search", Arguments: `{"query": "price"}`},
			},
			finishReason: "tool_calls",
		},
		// Turn 2: Calculate
		mockResponse{
			content: "Now let me calculate.",
			toolCalls: []agents.ToolCall{
				{ID: "call_2", Name: "calculator", Arguments: `{"expr": "100 * 2"}`},
			},
			finishReason: "tool_calls",
		},
		// Turn 3: Final answer
		mockResponse{
			content:      "The result is 200.",
			finishReason: "stop",
		},
	)

	executor := agents.NewExecutor(agent, model)

	input := agents.Input{
		Messages: []agents.Message{
			{Role: agents.RoleUser, Content: "Calculate double the current price"},
		},
		SessionID: uuid.New(),
		TenantID:  uuid.New(),
	}

	gen := executor.Execute(ctx, input)
	defer gen.Close()

	// Collect events
	toolExecutions := 0
	for {
		event, err, hasMore := gen.Next()
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !hasMore {
			break
		}

		if event.Type == agents.EventTypeToolEnd {
			toolExecutions++
		}
	}

	// Verify multiple tool executions
	if toolExecutions != 2 {
		t.Errorf("Expected 2 tool executions, got %d", toolExecutions)
	}
}

// TestExecutor_Interrupt tests HITL checkpoint save/resume.
func TestExecutor_Interrupt(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Create mock agent with ask_user_question tool
	agent := newMockAgent("test-agent")

	// Create mock model that calls ask_user_question
	model := newMockModel(
		mockResponse{
			content: "I need to ask you a question.",
			toolCalls: []agents.ToolCall{
				{
					ID:        "call_1",
					Name:      agents.ToolAskUserQuestion,
					Arguments: `{"question": "What is your favorite color?"}`,
				},
			},
			finishReason: "tool_calls",
		},
	)

	// Create checkpointer
	checkpointer := agents.NewInMemoryCheckpointer()

	// Create executor with checkpointer
	executor := agents.NewExecutor(agent, model,
		agents.WithCheckpointer(checkpointer),
	)

	input := agents.Input{
		Messages: []agents.Message{
			{Role: agents.RoleUser, Content: "Hello"},
		},
		SessionID: uuid.New(),
		TenantID:  uuid.New(),
		ThreadID:  "thread-123",
	}

	gen := executor.Execute(ctx, input)
	defer gen.Close()

	// Collect events
	var interruptEvent *agents.InterruptEvent
	for {
		event, err, hasMore := gen.Next()
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !hasMore {
			break
		}

		if event.Type == agents.EventTypeInterrupt {
			interruptEvent = event.Interrupt
		}
	}

	// Verify interrupt event
	if interruptEvent == nil {
		t.Fatal("Expected interrupt event, got nil")
	}
	if interruptEvent.Type != agents.ToolAskUserQuestion {
		t.Errorf("Expected interrupt type '%s', got '%s'", agents.ToolAskUserQuestion, interruptEvent.Type)
	}

	// Parse interrupt data
	var data struct {
		Question string `json:"question"`
	}
	if err := json.Unmarshal(interruptEvent.Data, &data); err != nil {
		t.Fatalf("Failed to parse interrupt data: %v", err)
	}
	if data.Question != "What is your favorite color?" {
		t.Errorf("Expected question 'What is your favorite color?', got '%s'", data.Question)
	}
}

// TestExecutor_Resume tests resuming from a checkpoint.
func TestExecutor_Resume(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Create checkpointer with a saved checkpoint
	checkpointer := agents.NewInMemoryCheckpointer()

	// Create a checkpoint
	checkpoint := agents.NewCheckpoint(
		"thread-123",
		"test-agent",
		[]agents.Message{
			{Role: agents.RoleUser, Content: "Hello"},
			{Role: agents.RoleAssistant, Content: "I need to ask you something."},
		},
		agents.WithPendingTools([]agents.ToolCall{
			{
				ID:        "call_1",
				Name:      agents.ToolAskUserQuestion,
				Arguments: `{"question": "What is your favorite color?"}`,
			},
		}),
		agents.WithInterruptType(agents.ToolAskUserQuestion),
	)

	checkpointID, err := checkpointer.Save(ctx, checkpoint)
	if err != nil {
		t.Fatalf("Failed to save checkpoint: %v", err)
	}

	// Create agent and model for resume
	agent := newMockAgent("test-agent")

	model := newMockModel(
		mockResponse{
			content:      "Your favorite color is blue!",
			finishReason: "stop",
		},
	)

	executor := agents.NewExecutor(agent, model,
		agents.WithCheckpointer(checkpointer),
	)

	// Resume execution with user's answer
	gen := executor.Resume(ctx, checkpointID, "blue")
	defer gen.Close()

	// Collect events
	var finalResult *agents.Response
	for {
		event, err, hasMore := gen.Next()
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !hasMore {
			break
		}

		if event.Type == agents.EventTypeDone {
			finalResult = event.Result
		}
	}

	// Verify final result
	if finalResult == nil {
		t.Fatal("Expected final result, got nil")
	}
	if finalResult.Message.Content != "Your favorite color is blue!" {
		t.Errorf("Expected result 'Your favorite color is blue!', got '%s'", finalResult.Message.Content)
	}

	// Verify checkpoint was deleted
	_, err = checkpointer.Load(ctx, checkpointID)
	if !errors.Is(err, agents.ErrCheckpointNotFound) {
		t.Error("Expected checkpoint to be deleted after resume")
	}
}

// TestExecutor_Cancellation tests context cancellation stops execution.
func TestExecutor_Cancellation(t *testing.T) {
	t.Parallel()

	// Create cancelable context
	ctx, cancel := context.WithCancel(context.Background())

	// Create mock model that never completes
	model := &mockModel{
		responses: []mockResponse{
			{content: "This will never finish", finishReason: "stop"},
		},
		streamingMode: true,
		info: agents.ModelInfo{
			Name:     "mock-model",
			Provider: "test",
			Capabilities: []agents.Capability{
				agents.CapabilityStreaming,
			},
		},
	}

	agent := newMockAgent("test-agent")
	executor := agents.NewExecutor(agent, model)

	input := agents.Input{
		Messages: []agents.Message{
			{Role: agents.RoleUser, Content: "Hello"},
		},
		SessionID: uuid.New(),
		TenantID:  uuid.New(),
	}

	gen := executor.Execute(ctx, input)
	defer gen.Close()

	// Cancel context after a short delay
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	// Try to consume events (should stop when context is cancelled)
	eventCount := 0
	for {
		_, err, hasMore := gen.Next()
		if err != nil || !hasMore {
			break
		}
		eventCount++
	}

	// Verify we stopped (exact count depends on timing, just check we didn't hang)
	if eventCount > 1000 {
		t.Error("Expected cancellation to stop execution")
	}
}

// TestExecutor_MaxIterations tests max iterations prevent infinite loops.
func TestExecutor_MaxIterations(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Create mock model that always returns tool calls (infinite loop scenario)
	responses := make([]mockResponse, 20) // More than max iterations
	for i := range responses {
		responses[i] = mockResponse{
			content: fmt.Sprintf("Iteration %d", i),
			toolCalls: []agents.ToolCall{
				{ID: fmt.Sprintf("call_%d", i), Name: "loop_tool", Arguments: "{}"},
			},
			finishReason: "tool_calls",
		}
	}

	model := newMockModel(responses...)

	loopTool := agents.NewTool("loop_tool", "A tool that loops forever", map[string]any{}, nil)
	agent := newMockAgent("test-agent", loopTool)
	agent.setToolResult("loop_tool", "looped")

	// Create executor with max iterations
	executor := agents.NewExecutor(agent, model,
		agents.WithMaxIterations(5),
	)

	input := agents.Input{
		Messages: []agents.Message{
			{Role: agents.RoleUser, Content: "Start looping"},
		},
		SessionID: uuid.New(),
		TenantID:  uuid.New(),
	}

	gen := executor.Execute(ctx, input)
	defer gen.Close()

	// Consume all events
	var gotMaxIterationsError bool
	for {
		event, err, hasMore := gen.Next()
		if err != nil {
			if errors.Is(err, agents.ErrMaxIterations) {
				gotMaxIterationsError = true
			}
			break
		}
		if !hasMore {
			break
		}

		// Count iterations via tool executions
		if event.Type == agents.EventTypeError && errors.Is(event.Error, agents.ErrMaxIterations) {
			gotMaxIterationsError = true
		}
	}

	// Verify max iterations error
	if !gotMaxIterationsError {
		t.Error("Expected ErrMaxIterations, didn't get it")
	}
}

// TestExecutor_StreamingChunks tests streaming chunk ordering.
func TestExecutor_StreamingChunks(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	model := newMockModel(
		mockResponse{
			content:      "Hello world! This is a test.",
			finishReason: "stop",
		},
	)

	agent := newMockAgent("test-agent")
	executor := agents.NewExecutor(agent, model)

	input := agents.Input{
		Messages: []agents.Message{
			{Role: agents.RoleUser, Content: "Hello"},
		},
		SessionID: uuid.New(),
		TenantID:  uuid.New(),
	}

	gen := executor.Execute(ctx, input)
	defer gen.Close()

	// Collect chunks in order
	var chunks []string
	for {
		event, err, hasMore := gen.Next()
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !hasMore {
			break
		}

		if event.Type == agents.EventTypeChunk && event.Chunk.Delta != "" {
			chunks = append(chunks, event.Chunk.Delta)
		}
	}

	// Verify chunks are in order
	fullContent := ""
	for _, chunk := range chunks {
		fullContent += chunk
	}

	expected := "Hello world! This is a test."
	if fullContent != expected {
		t.Errorf("Expected content '%s', got '%s'", expected, fullContent)
	}
}

// TestExecutor_TerminationTool tests execution stops when termination tool is called.
func TestExecutor_TerminationTool(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	finalAnswerTool := agents.NewTool(
		agents.ToolFinalAnswer,
		"Provides the final answer",
		map[string]any{},
		nil,
	)

	agent := newMockAgent("test-agent", finalAnswerTool)
	agent.setToolResult(agents.ToolFinalAnswer, "The answer is 42")

	model := newMockModel(
		mockResponse{
			content: "Let me provide the answer.",
			toolCalls: []agents.ToolCall{
				{
					ID:        "call_1",
					Name:      agents.ToolFinalAnswer,
					Arguments: `{"answer": "42"}`,
				},
			},
			finishReason: "tool_calls",
		},
	)

	executor := agents.NewExecutor(agent, model)

	input := agents.Input{
		Messages: []agents.Message{
			{Role: agents.RoleUser, Content: "What is the answer?"},
		},
		SessionID: uuid.New(),
		TenantID:  uuid.New(),
	}

	gen := executor.Execute(ctx, input)
	defer gen.Close()

	// Collect events
	var finalResult *agents.Response
	toolExecutions := 0

	for {
		event, err, hasMore := gen.Next()
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !hasMore {
			break
		}

		if event.Type == agents.EventTypeToolEnd {
			toolExecutions++
		}

		if event.Type == agents.EventTypeDone {
			finalResult = event.Result
		}
	}

	// Verify execution stopped after termination tool
	if toolExecutions != 1 {
		t.Errorf("Expected 1 tool execution, got %d", toolExecutions)
	}

	if finalResult == nil {
		t.Fatal("Expected final result, got nil")
	}

	if finalResult.Message.Content != "The answer is 42" {
		t.Errorf("Expected result 'The answer is 42', got '%s'", finalResult.Message.Content)
	}
}

// TestExecutor_NoCheckpointerInterruptFails tests interrupt fails without checkpointer.
func TestExecutor_NoCheckpointerInterruptFails(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	agent := newMockAgent("test-agent")

	model := newMockModel(
		mockResponse{
			content: "I need to ask a question.",
			toolCalls: []agents.ToolCall{
				{
					ID:        "call_1",
					Name:      agents.ToolAskUserQuestion,
					Arguments: `{"question": "Test?"}`,
				},
			},
			finishReason: "tool_calls",
		},
	)

	// Create executor WITHOUT checkpointer
	executor := agents.NewExecutor(agent, model)

	input := agents.Input{
		Messages: []agents.Message{
			{Role: agents.RoleUser, Content: "Hello"},
		},
		SessionID: uuid.New(),
		TenantID:  uuid.New(),
	}

	gen := executor.Execute(ctx, input)
	defer gen.Close()

	// Consume events - should get error
	var gotCheckpointError bool
	for {
		event, err, hasMore := gen.Next()
		if err != nil {
			if errors.Is(err, agents.ErrCheckpointSaveFailed) {
				gotCheckpointError = true
			}
			break
		}
		if !hasMore {
			break
		}

		if event.Type == agents.EventTypeError {
			if errors.Is(event.Error, agents.ErrCheckpointSaveFailed) {
				gotCheckpointError = true
			}
		}
	}

	if !gotCheckpointError {
		t.Error("Expected ErrCheckpointSaveFailed when interrupt without checkpointer")
	}
}

// TestExecutor_ConcurrentExecution tests multiple concurrent executions.
func TestExecutor_ConcurrentExecution(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Track concurrent executions
	var activeCount atomic.Int32

	// Create tool that tracks concurrency
	concurrentTool := &agents.ToolFunc{
		ToolName:        "concurrent_tool",
		ToolDescription: "Test concurrent execution",
		ToolParameters:  map[string]any{},
		Fn: func(ctx context.Context, input string) (string, error) {
			count := activeCount.Add(1)
			defer activeCount.Add(-1)

			// Simulate work
			time.Sleep(10 * time.Millisecond)

			return fmt.Sprintf("Concurrent count: %d", count), nil
		},
	}

	agent := newMockAgent("test-agent", concurrentTool)

	// Run multiple executions in parallel
	const numExecutions = 5
	done := make(chan bool, numExecutions)

	for i := 0; i < numExecutions; i++ {
		go func(idx int) {
			model := newMockModel(
				mockResponse{
					content: "Testing concurrency",
					toolCalls: []agents.ToolCall{
						{ID: fmt.Sprintf("call_%d", idx), Name: "concurrent_tool", Arguments: "{}"},
					},
					finishReason: "tool_calls",
				},
				mockResponse{
					content:      "Done",
					finishReason: "stop",
				},
			)

			executor := agents.NewExecutor(agent, model)

			input := agents.Input{
				Messages: []agents.Message{
					{Role: agents.RoleUser, Content: fmt.Sprintf("Test %d", idx)},
				},
				SessionID: uuid.New(),
				TenantID:  uuid.New(),
			}

			gen := executor.Execute(ctx, input)
			defer gen.Close()

			for {
				_, err, hasMore := gen.Next()
				if err != nil {
					t.Errorf("Execution %d error: %v", idx, err)
					break
				}
				if !hasMore {
					break
				}
			}

			done <- true
		}(i)
	}

	// Wait for all executions to complete
	for i := 0; i < numExecutions; i++ {
		<-done
	}

	// Verify all executions completed (activeCount should be 0)
	if activeCount.Load() != 0 {
		t.Errorf("Expected 0 active executions after completion, got %d", activeCount.Load())
	}
}
