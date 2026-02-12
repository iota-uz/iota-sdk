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
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/stretchr/testify/require"
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
	toolCalls    []types.ToolCall
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
		Message: types.AssistantMessage(resp.content, types.WithToolCalls(resp.toolCalls...)),
		Usage: types.TokenUsage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
		FinishReason: resp.finishReason,
	}, nil
}

func (m *mockModel) Stream(ctx context.Context, req agents.Request, opts ...agents.GenerateOption) (types.Generator[agents.Chunk], error) {
	return types.NewGenerator(ctx, func(ctx context.Context, yield func(agents.Chunk) bool) error {
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
			Usage:        &types.TokenUsage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30},
			FinishReason: resp.finishReason,
			Done:         true,
		}

		if !yield(finalChunk) {
			return nil
		}

		return nil
	}), nil
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

func (m *mockModel) Pricing() agents.ModelPricing {
	return agents.ModelPricing{
		Currency:        "USD",
		InputPer1M:      1.00,
		OutputPer1M:     2.00,
		CacheWritePer1M: 0.50,
		CacheReadPer1M:  0.25,
	}
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

// funcAgent is a small ExtendedAgent implementation for tests.
type funcAgent struct {
	name             string
	tools            []agents.Tool
	systemPrompt     string
	terminationTools []string
	onToolCall       func(ctx context.Context, toolName, input string) (string, error)
}

func (a *funcAgent) Name() string { return a.name }

func (a *funcAgent) Description() string { return "Test agent" }

func (a *funcAgent) Metadata() agents.AgentMetadata {
	term := a.terminationTools
	if term == nil {
		term = []string{agents.ToolFinalAnswer}
	}
	return agents.AgentMetadata{
		Name:             a.name,
		Description:      "Test agent",
		WhenToUse:        "For testing",
		Model:            "mock-model",
		TerminationTools: term,
	}
}

func (a *funcAgent) Tools() []agents.Tool { return a.tools }

func (a *funcAgent) SystemPrompt(ctx context.Context) string { return a.systemPrompt }

func (a *funcAgent) OnToolCall(ctx context.Context, toolName, input string) (string, error) {
	if a.onToolCall == nil {
		return fmt.Sprintf("Tool %s executed with: %s", toolName, input), nil
	}
	return a.onToolCall(ctx, toolName, input)
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
		Messages: []types.Message{
			types.UserMessage("Hello"),
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
		event, err := gen.Next(ctx)
		if err != nil {
			if err == types.ErrGeneratorDone {
				break
			}
			t.Fatalf("Unexpected error: %v", err)
		}

		switch event.Type {
		case agents.EventTypeChunk:
			chunks = append(chunks, event.Chunk.Delta)
		case agents.EventTypeDone:
			doneCount++
			finalResult = event.Result
		case agents.EventTypeError:
			t.Fatalf("Unexpected error event: %v", event.Error)
		case agents.EventTypeToolStart, agents.EventTypeToolEnd, agents.EventTypeInterrupt:
			// no-op for this test
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
	if finalResult.Message.Content() != "Hello! How can I help you today?" {
		t.Errorf("Expected result content 'Hello! How can I help you today?', got '%s'", finalResult.Message.Content())
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
			toolCalls: []types.ToolCall{
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
		Messages: []types.Message{
			types.UserMessage("What's the weather in San Francisco?"),
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
		event, err := gen.Next(ctx)
		if err != nil {
			if err == types.ErrGeneratorDone {
				break
			}
			t.Fatalf("Unexpected error: %v", err)
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
		case agents.EventTypeChunk, agents.EventTypeInterrupt, agents.EventTypeDone:
			// no-op for this test
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
			toolCalls: []types.ToolCall{
				{ID: "call_1", Name: "search", Arguments: `{"query": "price"}`},
			},
			finishReason: "tool_calls",
		},
		// Turn 2: Calculate
		mockResponse{
			content: "Now let me calculate.",
			toolCalls: []types.ToolCall{
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
		Messages: []types.Message{
			types.UserMessage("Calculate double the current price"),
		},
		SessionID: uuid.New(),
		TenantID:  uuid.New(),
	}

	gen := executor.Execute(ctx, input)
	defer gen.Close()

	// Collect events
	toolExecutions := 0
	for {
		event, err := gen.Next(ctx)
		if err != nil {
			if err == types.ErrGeneratorDone {
				break
			}
			t.Fatalf("Unexpected error: %v", err)
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
			toolCalls: []types.ToolCall{
				{
					ID:   "call_1",
					Name: agents.ToolAskUserQuestion,
					Arguments: `{
						"questions": [
							{
								"question": "What is your favorite color?",
								"header": "Color",
								"multiSelect": false,
								"options": [
									{"label": "Red", "description": "Warm and vibrant"},
									{"label": "Blue", "description": "Cool and calming"}
								]
							}
						]
					}`,
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
		Messages: []types.Message{
			types.UserMessage("Hello"),
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
		event, err := gen.Next(ctx)
		if err != nil {
			if err == types.ErrGeneratorDone {
				break
			}
			t.Fatalf("Unexpected error: %v", err)
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

	// Parse interrupt data (canonical payload)
	var payload types.AskUserQuestionPayload
	if err := json.Unmarshal(interruptEvent.Data, &payload); err != nil {
		t.Fatalf("Failed to parse interrupt data: %v", err)
	}
	if payload.Type != types.InterruptTypeAskUserQuestion {
		t.Fatalf("Expected payload type '%s', got '%s'", types.InterruptTypeAskUserQuestion, payload.Type)
	}
	if len(payload.Questions) != 1 {
		t.Fatalf("Expected 1 question, got %d", len(payload.Questions))
	}
	q := payload.Questions[0]
	if q.ID == "" {
		t.Fatal("Expected generated question ID, got empty")
	}
	if q.Question != "What is your favorite color?" {
		t.Errorf("Expected question 'What is your favorite color?', got '%s'", q.Question)
	}
	if q.Header != "Color" {
		t.Errorf("Expected header 'Color', got '%s'", q.Header)
	}
	if len(q.Options) != 2 {
		t.Errorf("Expected 2 options, got %d", len(q.Options))
	}
	for i, opt := range q.Options {
		if opt.ID == "" {
			t.Fatalf("Expected generated option ID for option[%d], got empty", i)
		}
	}
}

// TestExecutor_Resume tests resuming from a checkpoint.
func TestExecutor_Resume(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Create checkpointer with a saved checkpoint
	checkpointer := agents.NewInMemoryCheckpointer()

	// Create a checkpoint
	interruptData, err := json.Marshal(types.AskUserQuestionPayload{
		Type: types.InterruptTypeAskUserQuestion,
		Questions: []types.AskUserQuestion{
			{
				ID:          "q1",
				Question:    "What is your favorite color?",
				Header:      "Color",
				MultiSelect: false,
				Options: []types.QuestionOption{
					{ID: "opt1", Label: "Red", Description: "Warm and vibrant"},
					{ID: "opt2", Label: "Blue", Description: "Cool and calming"},
				},
			},
		},
	})
	require.NoError(t, err)

	checkpoint := agents.NewCheckpoint(
		"thread-123",
		"test-agent",
		[]types.Message{
			types.UserMessage("Hello"),
			types.AssistantMessage("I need to ask you something."),
		},
		agents.WithPendingTools([]types.ToolCall{
			{
				ID:   "call_1",
				Name: agents.ToolAskUserQuestion,
				Arguments: `{
					"questions": [
						{
							"id": "q1",
							"question": "What is your favorite color?",
							"header": "Color",
							"multiSelect": false,
							"options": [
								{"id": "opt1", "label": "Red", "description": "Warm and vibrant"},
								{"id": "opt2", "label": "Blue", "description": "Cool and calming"}
							]
						}
					]
				}`,
			},
		}),
		agents.WithInterruptType(agents.ToolAskUserQuestion),
		agents.WithInterruptData(interruptData),
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
	answers := map[string]types.Answer{}
	answers["q1"] = types.NewAnswer("blue")

	gen := executor.Resume(ctx, checkpointID, answers)
	defer gen.Close()

	// Collect events
	var finalResult *agents.Response
	for {
		event, err := gen.Next(ctx)
		if err != nil {
			if err == types.ErrGeneratorDone {
				break
			}
			t.Fatalf("Unexpected error: %v", err)
		}

		if event.Type == agents.EventTypeDone {
			finalResult = event.Result
		}
	}

	// Verify final result
	if finalResult == nil {
		t.Fatal("Expected final result, got nil")
	}
	if finalResult.Message.Content() != "Your favorite color is blue!" {
		t.Errorf("Expected result 'Your favorite color is blue!', got '%s'", finalResult.Message.Content())
	}

	// Verify checkpoint was deleted
	_, err = checkpointer.Load(ctx, checkpointID)
	if !errors.Is(err, agents.ErrCheckpointNotFound) {
		t.Error("Expected checkpoint to be deleted after resume")
	}
}

// TestExecutor_Resume_MultipleQuestions tests resuming with multiple questions.
func TestExecutor_Resume_MultipleQuestions(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Create checkpointer with a saved checkpoint containing multiple questions
	checkpointer := agents.NewInMemoryCheckpointer()

	// Create a checkpoint with 3 questions
	interruptData, err := json.Marshal(types.AskUserQuestionPayload{
		Type: types.InterruptTypeAskUserQuestion,
		Questions: []types.AskUserQuestion{
			{
				ID:          "q1",
				Question:    "Which time period?",
				Header:      "Time Period",
				MultiSelect: false,
				Options: []types.QuestionOption{
					{ID: "q1_opt1", Label: "Q1 2024", Description: "First quarter"},
					{ID: "q1_opt2", Label: "Q2 2024", Description: "Second quarter"},
				},
			},
			{
				ID:          "q2",
				Question:    "Which metric?",
				Header:      "Metric",
				MultiSelect: false,
				Options: []types.QuestionOption{
					{ID: "q2_opt1", Label: "Revenue", Description: "Total revenue"},
					{ID: "q2_opt2", Label: "Profit", Description: "Net profit"},
				},
			},
			{
				ID:          "q3",
				Question:    "Which region?",
				Header:      "Region",
				MultiSelect: false,
				Options: []types.QuestionOption{
					{ID: "q3_opt1", Label: "North America", Description: "US and Canada"},
					{ID: "q3_opt2", Label: "Europe", Description: "EU countries"},
				},
			},
		},
	})
	require.NoError(t, err)

	checkpoint := agents.NewCheckpoint(
		"thread-multi",
		"test-agent",
		[]types.Message{
			types.UserMessage("I need to analyze data"),
			types.AssistantMessage("I need some clarifications."),
		},
		agents.WithPendingTools([]types.ToolCall{
			{
				ID:   "call_multi",
				Name: agents.ToolAskUserQuestion,
				Arguments: `{
					"questions": [
						{
							"id": "q1",
							"question": "Which time period?",
							"header": "Time Period",
							"multiSelect": false,
							"options": [
								{"label": "Q1 2024", "description": "First quarter"},
								{"label": "Q2 2024", "description": "Second quarter"}
							]
						},
						{
							"id": "q2",
							"question": "Which metric?",
							"header": "Metric",
							"multiSelect": false,
							"options": [
								{"label": "Revenue", "description": "Total revenue"},
								{"label": "Profit", "description": "Net profit"}
							]
						},
						{
							"id": "q3",
							"question": "Which region?",
							"header": "Region",
							"multiSelect": false,
							"options": [
								{"label": "North America", "description": "US and Canada"},
								{"label": "Europe", "description": "EU countries"}
							]
						}
					]
				}`,
			},
		}),
		agents.WithInterruptType(agents.ToolAskUserQuestion),
		agents.WithInterruptData(interruptData),
	)

	checkpointID, err := checkpointer.Save(ctx, checkpoint)
	if err != nil {
		t.Fatalf("Failed to save checkpoint: %v", err)
	}

	// Create agent and model for resume
	agent := newMockAgent("test-agent")

	model := newMockModel(
		mockResponse{
			content:      "Analyzing Q1 2024 revenue for North America...",
			finishReason: "stop",
		},
	)

	executor := agents.NewExecutor(agent, model,
		agents.WithCheckpointer(checkpointer),
	)

	// Resume execution with all 3 answers
	answers := map[string]types.Answer{
		"q1": types.NewAnswer("Q1 2024"),
		"q2": types.NewAnswer("Revenue"),
		"q3": types.NewAnswer("North America"),
	}

	gen := executor.Resume(ctx, checkpointID, answers)
	defer gen.Close()

	// Collect events
	var finalResult *agents.Response
	for {
		event, err := gen.Next(ctx)
		if err != nil {
			if err == types.ErrGeneratorDone {
				break
			}
			t.Fatalf("Unexpected error: %v", err)
		}

		if event.Type == agents.EventTypeDone {
			finalResult = event.Result
		}
	}

	// Verify final result
	if finalResult == nil {
		t.Fatal("Expected final result, got nil")
	}
	if finalResult.Message.Content() != "Analyzing Q1 2024 revenue for North America..." {
		t.Errorf("Expected result 'Analyzing Q1 2024 revenue for North America...', got '%s'", finalResult.Message.Content())
	}

	// Verify checkpoint was deleted
	_, err = checkpointer.Load(ctx, checkpointID)
	if !errors.Is(err, agents.ErrCheckpointNotFound) {
		t.Error("Expected checkpoint to be deleted after resume")
	}
}

// TestExecutor_Resume_MissingAnswer tests resuming with missing answers fails.
func TestExecutor_Resume_MissingAnswer(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Create checkpointer with a saved checkpoint
	checkpointer := agents.NewInMemoryCheckpointer()

	// Create a checkpoint with 2 questions
	interruptData, err := json.Marshal(types.AskUserQuestionPayload{
		Type: types.InterruptTypeAskUserQuestion,
		Questions: []types.AskUserQuestion{
			{
				ID:          "q1",
				Question:    "Which year?",
				Header:      "Year",
				MultiSelect: false,
				Options: []types.QuestionOption{
					{ID: "q1_opt1", Label: "2023", Description: "Last year"},
					{ID: "q1_opt2", Label: "2024", Description: "This year"},
				},
			},
			{
				ID:          "q2",
				Question:    "Which quarter?",
				Header:      "Quarter",
				MultiSelect: false,
				Options: []types.QuestionOption{
					{ID: "q2_opt1", Label: "Q1", Description: "First quarter"},
					{ID: "q2_opt2", Label: "Q2", Description: "Second quarter"},
				},
			},
		},
	})
	require.NoError(t, err)

	checkpoint := agents.NewCheckpoint(
		"thread-missing",
		"test-agent",
		[]types.Message{
			types.UserMessage("Show me data"),
			types.AssistantMessage("I need to ask you something."),
		},
		agents.WithPendingTools([]types.ToolCall{
			{
				ID:   "call_missing",
				Name: agents.ToolAskUserQuestion,
				Arguments: `{
					"questions": [
						{
							"id": "q1",
							"question": "Which year?",
							"header": "Year",
							"multiSelect": false,
							"options": [
								{"label": "2023", "description": "Last year"},
								{"label": "2024", "description": "This year"}
							]
						},
						{
							"id": "q2",
							"question": "Which quarter?",
							"header": "Quarter",
							"multiSelect": false,
							"options": [
								{"label": "Q1", "description": "First quarter"},
								{"label": "Q2", "description": "Second quarter"}
							]
						}
					]
				}`,
			},
		}),
		agents.WithInterruptType(agents.ToolAskUserQuestion),
		agents.WithInterruptData(interruptData),
	)

	checkpointID, err := checkpointer.Save(ctx, checkpoint)
	if err != nil {
		t.Fatalf("Failed to save checkpoint: %v", err)
	}

	// Create agent and model
	agent := newMockAgent("test-agent")
	model := newMockModel(
		mockResponse{
			content:      "This should not execute",
			finishReason: "stop",
		},
	)

	executor := agents.NewExecutor(agent, model,
		agents.WithCheckpointer(checkpointer),
	)

	// Resume with only one answer (missing q2)
	answers := map[string]types.Answer{
		"q1": types.NewAnswer("2024"),
		// Missing "q2"
	}

	gen := executor.Resume(ctx, checkpointID, answers)
	defer gen.Close()

	// Expect error for missing answer
	var gotError bool
	for {
		event, err := gen.Next(ctx)
		if err != nil {
			// Check if it's the expected missing answer error
			if err != types.ErrGeneratorDone {
				if errStr := err.Error(); errStr != "" {
					gotError = true
				}
			}
			break
		}

		if event.Type == agents.EventTypeError {
			gotError = true
			break
		}
	}

	if !gotError {
		t.Error("Expected error for missing answer, but got none")
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
		Messages: []types.Message{
			types.UserMessage("Hello"),
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
		_, err := gen.Next(ctx)
		if err != nil {
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
			toolCalls: []types.ToolCall{
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
		Messages: []types.Message{
			types.UserMessage("Start looping"),
		},
		SessionID: uuid.New(),
		TenantID:  uuid.New(),
	}

	gen := executor.Execute(ctx, input)
	defer gen.Close()

	// Consume all events
	var gotMaxIterationsError bool
	for {
		event, err := gen.Next(ctx)
		if err != nil {
			if errors.Is(err, agents.ErrMaxIterations) {
				gotMaxIterationsError = true
			}
			if err == types.ErrGeneratorDone {
				break
			}
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
		Messages: []types.Message{
			types.UserMessage("Hello"),
		},
		SessionID: uuid.New(),
		TenantID:  uuid.New(),
	}

	gen := executor.Execute(ctx, input)
	defer gen.Close()

	// Collect chunks in order
	var chunks []string
	for {
		event, err := gen.Next(ctx)
		if err != nil {
			if err == types.ErrGeneratorDone {
				break
			}
			t.Fatalf("Unexpected error: %v", err)
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
			toolCalls: []types.ToolCall{
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
		Messages: []types.Message{
			types.UserMessage("What is the answer?"),
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
		event, err := gen.Next(ctx)
		if err != nil {
			if err == types.ErrGeneratorDone {
				break
			}
			t.Fatalf("Unexpected error: %v", err)
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

	if finalResult.Message.Content() != "The answer is 42" {
		t.Errorf("Expected result 'The answer is 42', got '%s'", finalResult.Message.Content())
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
			toolCalls: []types.ToolCall{
				{
					ID:   "call_1",
					Name: agents.ToolAskUserQuestion,
					Arguments: `{
						"questions": [
							{
								"id": "q1",
								"question": "Test?",
								"header": "Test",
								"multiSelect": false,
								"options": [
									{"id": "opt1", "label": "Yes", "description": "Affirmative"},
									{"id": "opt2", "label": "No", "description": "Negative"}
								]
							}
						]
					}`,
				},
			},
			finishReason: "tool_calls",
		},
	)

	// Create executor WITHOUT checkpointer
	executor := agents.NewExecutor(agent, model)

	input := agents.Input{
		Messages: []types.Message{
			types.UserMessage("Hello"),
		},
		SessionID: uuid.New(),
		TenantID:  uuid.New(),
	}

	gen := executor.Execute(ctx, input)
	defer gen.Close()

	// Consume events - should get error
	var gotCheckpointError bool
	for {
		event, err := gen.Next(ctx)
		if err != nil {
			if errors.Is(err, agents.ErrCheckpointSaveFailed) {
				gotCheckpointError = true
			}
			if err == types.ErrGeneratorDone {
				break
			}
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
					toolCalls: []types.ToolCall{
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
				Messages: []types.Message{
					types.UserMessage(fmt.Sprintf("Test %d", idx)),
				},
				SessionID: uuid.New(),
				TenantID:  uuid.New(),
			}

			gen := executor.Execute(ctx, input)
			defer gen.Close()

			for {
				_, err := gen.Next(ctx)
				if err != nil {
					if err != types.ErrGeneratorDone {
						t.Errorf("Execution %d error: %v", idx, err)
					}
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

func TestExecutor_ToolCalls_RunInParallelWithinTurn(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	var active atomic.Int32
	var maxActive atomic.Int32

	trackConcurrency := func(toolName string) func(ctx context.Context, input string) (string, error) {
		return func(ctx context.Context, input string) (string, error) {
			count := active.Add(1)
			defer active.Add(-1)

			for {
				prev := maxActive.Load()
				if count <= prev {
					break
				}
				if maxActive.CompareAndSwap(prev, count) {
					break
				}
			}

			time.Sleep(40 * time.Millisecond)
			return fmt.Sprintf("ok:%s", toolName), nil
		}
	}

	agent := &funcAgent{
		name:         "parallel-agent",
		systemPrompt: "You are a helpful assistant.",
		tools: []agents.Tool{
			&agents.ToolFunc{ToolName: "tool_a", ToolDescription: "a", ToolParameters: map[string]any{}, Fn: trackConcurrency("tool_a")},
			&agents.ToolFunc{ToolName: "tool_b", ToolDescription: "b", ToolParameters: map[string]any{}, Fn: trackConcurrency("tool_b")},
		},
	}

	model := newMockModel(
		mockResponse{
			content: "run tools",
			toolCalls: []types.ToolCall{
				{ID: "call_a", Name: "tool_a", Arguments: "{}"},
				{ID: "call_b", Name: "tool_b", Arguments: "{}"},
			},
			finishReason: "tool_calls",
		},
		mockResponse{
			content:      "done",
			finishReason: "stop",
		},
	)

	executor := agents.NewExecutor(agent, model)
	input := agents.Input{
		Messages:  []types.Message{types.UserMessage("test")},
		SessionID: uuid.New(),
		TenantID:  uuid.New(),
	}

	gen := executor.Execute(ctx, input)
	defer gen.Close()

	for {
		_, err := gen.Next(ctx)
		if err != nil {
			if err == types.ErrGeneratorDone {
				break
			}
			t.Fatalf("unexpected generator error: %v", err)
		}
	}

	if maxActive.Load() < 2 {
		t.Fatalf("expected tool calls to overlap (maxActive >= 2), got %d", maxActive.Load())
	}
}

func TestExecutor_InterruptTool_IsExclusive(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	var executed atomic.Bool

	agent := &funcAgent{
		name:         "interrupt-agent",
		systemPrompt: "You are a helpful assistant.",
		tools: []agents.Tool{
			&agents.ToolFunc{ToolName: "other_tool", ToolDescription: "other", ToolParameters: map[string]any{}, Fn: func(ctx context.Context, input string) (string, error) { return "ok", nil }},
		},
		onToolCall: func(ctx context.Context, toolName, input string) (string, error) {
			executed.Store(true)
			return "should_not_run", nil
		},
	}

	model := newMockModel(mockResponse{
		content: "",
		toolCalls: []types.ToolCall{
			{ID: "call_other", Name: "other_tool", Arguments: "{}"},
			{
				ID:        "call_interrupt",
				Name:      agents.ToolAskUserQuestion,
				Arguments: `{"questions":[{"question":"Choose?","header":"Choose","multiSelect":false,"options":[{"label":"A","description":"Option A"},{"label":"B","description":"Option B"}]}]}`,
			},
		},
		finishReason: "tool_calls",
	})

	checkpointer := agents.NewInMemoryCheckpointer()
	executor := agents.NewExecutor(agent, model, agents.WithCheckpointer(checkpointer))

	input := agents.Input{
		Messages:  []types.Message{types.UserMessage("test")},
		SessionID: uuid.New(),
		TenantID:  uuid.New(),
	}

	gen := executor.Execute(ctx, input)
	defer gen.Close()

	gotInterrupt := false
	for {
		ev, err := gen.Next(ctx)
		if err != nil {
			if err == types.ErrGeneratorDone {
				break
			}
			t.Fatalf("unexpected generator error: %v", err)
		}
		if ev.Type == agents.EventTypeInterrupt {
			gotInterrupt = true
			break
		}
	}

	if !gotInterrupt {
		t.Fatalf("expected interrupt event")
	}
	if executed.Load() {
		t.Fatalf("expected other tool not to execute in same batch as interrupt")
	}
}
