package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	bichatctx "github.com/iota-uz/iota-sdk/pkg/bichat/context"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockAgent is a test implementation of agents.ExtendedAgent
type mockAgent struct {
	systemPrompt string
	tools        []agents.Tool
	metadata     agents.AgentMetadata
}

func newMockAgent() *mockAgent {
	return &mockAgent{
		systemPrompt: "You are a helpful test agent.",
		tools:        []agents.Tool{},
		metadata: agents.AgentMetadata{
			Name:             "test_agent",
			Description:      "A test agent for unit testing",
			Model:            "test-model",
			TerminationTools: []string{agents.ToolFinalAnswer},
		},
	}
}

func (m *mockAgent) Name() string {
	return m.metadata.Name
}

func (m *mockAgent) Description() string {
	return m.metadata.Description
}

func (m *mockAgent) Metadata() agents.AgentMetadata {
	return m.metadata
}

func (m *mockAgent) Tools() []agents.Tool {
	return m.tools
}

func (m *mockAgent) SystemPrompt(ctx context.Context) string {
	return m.systemPrompt
}

func (m *mockAgent) OnToolCall(ctx context.Context, toolName, input string) (string, error) {
	return "mock tool result", nil
}

// mockModel is a test implementation of agents.Model
type mockModel struct {
	response *agents.Response
	chunks   []agents.Chunk
	err      error
}

func newMockModel() *mockModel {
	return &mockModel{
		response: &agents.Response{
			Message: *types.AssistantMessage("Test response"),
			Usage: types.TokenUsage{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			},
			FinishReason: "stop",
		},
		chunks: []agents.Chunk{
			{Delta: "Test ", Done: false},
			{Delta: "response", Done: false},
			{
				Delta:        "",
				FinishReason: "stop",
				Done:         true,
				Usage: &types.TokenUsage{
					PromptTokens:     10,
					CompletionTokens: 20,
					TotalTokens:      30,
				},
			},
		},
	}
}

func (m *mockModel) Generate(ctx context.Context, req agents.Request, opts ...agents.GenerateOption) (*agents.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func (m *mockModel) Stream(ctx context.Context, req agents.Request, opts ...agents.GenerateOption) types.Generator[agents.Chunk] {
	return types.NewGenerator(ctx, func(ctx context.Context, yield func(agents.Chunk) bool) error {
		if m.err != nil {
			return m.err
		}
		for _, chunk := range m.chunks {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			if !yield(chunk) {
				return nil
			}
		}
		return nil
	})
}

func (m *mockModel) Info() agents.ModelInfo {
	return agents.ModelInfo{
		Name:     "test-model",
		Provider: "mock",
		Capabilities: []agents.Capability{
			agents.CapabilityStreaming,
			agents.CapabilityTools,
		},
	}
}

func (m *mockModel) HasCapability(capability agents.Capability) bool {
	for _, c := range m.Info().Capabilities {
		if c == capability {
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

// mockRenderer is a test implementation of bichatctx.Renderer
type mockRenderer struct{}

func (m *mockRenderer) Render(block bichatctx.ContextBlock) (bichatctx.RenderedBlock, error) {
	return bichatctx.RenderedBlock{
		SystemContent: "test system content",
		Message:       map[string]any{"role": "user", "content": "test"},
	}, nil
}

func (m *mockRenderer) EstimateTokens(block bichatctx.ContextBlock) (int, error) {
	return 10, nil
}

func (m *mockRenderer) Provider() string {
	return "mock"
}

// mockCheckpointer is a test implementation of agents.Checkpointer
type mockCheckpointer struct {
	checkpoints map[string]*agents.Checkpoint
}

func newMockCheckpointer() *mockCheckpointer {
	return &mockCheckpointer{
		checkpoints: make(map[string]*agents.Checkpoint),
	}
}

func (m *mockCheckpointer) Save(ctx context.Context, checkpoint *agents.Checkpoint) (string, error) {
	id := uuid.New().String()
	m.checkpoints[id] = checkpoint
	return id, nil
}

func (m *mockCheckpointer) Load(ctx context.Context, id string) (*agents.Checkpoint, error) {
	checkpoint, exists := m.checkpoints[id]
	if !exists {
		return nil, agents.ErrCheckpointNotFound
	}
	return checkpoint, nil
}

func (m *mockCheckpointer) LoadByThreadID(ctx context.Context, threadID string) (*agents.Checkpoint, error) {
	// Simple implementation: find first checkpoint with matching threadID
	for _, cp := range m.checkpoints {
		if cp.ThreadID == threadID {
			return cp, nil
		}
	}
	return nil, agents.ErrCheckpointNotFound
}

func (m *mockCheckpointer) Delete(ctx context.Context, id string) error {
	if _, exists := m.checkpoints[id]; !exists {
		return agents.ErrCheckpointNotFound
	}
	delete(m.checkpoints, id)
	return nil
}

func (m *mockCheckpointer) LoadAndDelete(ctx context.Context, checkpointID string) (*agents.Checkpoint, error) {
	checkpoint, exists := m.checkpoints[checkpointID]
	if !exists {
		return nil, agents.ErrCheckpointNotFound
	}
	delete(m.checkpoints, checkpointID)
	return checkpoint, nil
}

// mockChatRepository is a test implementation of domain.ChatRepository
type mockChatRepository struct {
	sessions    map[uuid.UUID]*domain.Session
	messages    map[uuid.UUID][]*types.Message
	attachments map[uuid.UUID]*domain.Attachment
}

func newMockChatRepository() *mockChatRepository {
	return &mockChatRepository{
		sessions:    make(map[uuid.UUID]*domain.Session),
		messages:    make(map[uuid.UUID][]*types.Message),
		attachments: make(map[uuid.UUID]*domain.Attachment),
	}
}

func (m *mockChatRepository) CreateSession(ctx context.Context, session *domain.Session) error {
	if session.ID == uuid.Nil {
		session.ID = uuid.New()
	}
	m.sessions[session.ID] = session
	return nil
}

func (m *mockChatRepository) GetSession(ctx context.Context, id uuid.UUID) (*domain.Session, error) {
	session, exists := m.sessions[id]
	if !exists {
		return nil, errors.New("session not found")
	}
	return session, nil
}

func (m *mockChatRepository) UpdateSession(ctx context.Context, session *domain.Session) error {
	m.sessions[session.ID] = session
	return nil
}

func (m *mockChatRepository) ListUserSessions(ctx context.Context, userID int64, opts domain.ListOptions) ([]*domain.Session, error) {
	var sessions []*domain.Session
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}
	return sessions, nil
}

func (m *mockChatRepository) DeleteSession(ctx context.Context, id uuid.UUID) error {
	delete(m.sessions, id)
	delete(m.messages, id)
	return nil
}

func (m *mockChatRepository) SaveMessage(ctx context.Context, msg *types.Message) error {
	if msg.ID == uuid.Nil {
		msg.ID = uuid.New()
	}
	m.messages[msg.SessionID] = append(m.messages[msg.SessionID], msg)
	return nil
}

func (m *mockChatRepository) GetMessage(ctx context.Context, id uuid.UUID) (*types.Message, error) {
	for _, msgs := range m.messages {
		for _, msg := range msgs {
			if msg.ID == id {
				return msg, nil
			}
		}
	}
	return nil, errors.New("message not found")
}

func (m *mockChatRepository) GetSessionMessages(ctx context.Context, sessionID uuid.UUID, opts domain.ListOptions) ([]*types.Message, error) {
	msgs, exists := m.messages[sessionID]
	if !exists {
		return []*types.Message{}, nil
	}
	return msgs, nil
}

func (m *mockChatRepository) TruncateMessagesFrom(ctx context.Context, sessionID uuid.UUID, from time.Time) (int64, error) {
	return 0, nil
}

func (m *mockChatRepository) SaveAttachment(ctx context.Context, attachment *domain.Attachment) error {
	if attachment.ID == uuid.Nil {
		attachment.ID = uuid.New()
	}
	m.attachments[attachment.ID] = attachment
	return nil
}

func (m *mockChatRepository) GetAttachment(ctx context.Context, id uuid.UUID) (*domain.Attachment, error) {
	att, exists := m.attachments[id]
	if !exists {
		return nil, errors.New("attachment not found")
	}
	return att, nil
}

func (m *mockChatRepository) GetMessageAttachments(ctx context.Context, messageID uuid.UUID) ([]*domain.Attachment, error) {
	var atts []*domain.Attachment
	for _, att := range m.attachments {
		atts = append(atts, att)
	}
	return atts, nil
}

func (m *mockChatRepository) DeleteAttachment(ctx context.Context, id uuid.UUID) error {
	delete(m.attachments, id)
	return nil
}

func (m *mockChatRepository) SaveArtifact(ctx context.Context, artifact *domain.Artifact) error {
	return nil
}

func (m *mockChatRepository) GetArtifact(ctx context.Context, id uuid.UUID) (*domain.Artifact, error) {
	return nil, nil
}

func (m *mockChatRepository) GetSessionArtifacts(ctx context.Context, sessionID uuid.UUID, opts domain.ListOptions) ([]*domain.Artifact, error) {
	return nil, nil
}

func (m *mockChatRepository) DeleteArtifact(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *mockChatRepository) UpdateArtifact(ctx context.Context, id uuid.UUID, name, description string) error {
	return nil
}

func TestNewAgentService(t *testing.T) {
	t.Parallel()

	agent := newMockAgent()
	model := newMockModel()
	renderer := &mockRenderer{}
	checkpointer := newMockCheckpointer()
	chatRepo := newMockChatRepository()

	policy := bichatctx.ContextPolicy{
		ContextWindow:     4096,
		CompletionReserve: 1024,
		MaxSensitivity:    bichatctx.SensitivityPublic,
		OverflowStrategy:  bichatctx.OverflowTruncate,
	}

	service := NewAgentService(AgentServiceConfig{
		Agent:        agent,
		Model:        model,
		Policy:       policy,
		Renderer:     renderer,
		Checkpointer: checkpointer,
		ChatRepo:     chatRepo,
	})

	assert.NotNil(t, service)
	impl, ok := service.(*agentServiceImpl)
	require.True(t, ok)
	assert.Equal(t, agent, impl.agent)
	assert.Equal(t, model, impl.model)
}

func TestProcessMessage_Success(t *testing.T) {
	t.Parallel()

	// Setup
	agent := newMockAgent()
	model := newMockModel()
	renderer := &mockRenderer{}
	checkpointer := newMockCheckpointer()
	chatRepo := newMockChatRepository()

	policy := bichatctx.ContextPolicy{
		ContextWindow:     4096,
		CompletionReserve: 1024,
		MaxSensitivity:    bichatctx.SensitivityPublic,
		OverflowStrategy:  bichatctx.OverflowTruncate,
	}

	service := NewAgentService(AgentServiceConfig{
		Agent:        agent,
		Model:        model,
		Policy:       policy,
		Renderer:     renderer,
		Checkpointer: checkpointer,
		ChatRepo:     chatRepo,
	})

	// Create context with tenant ID
	ctx := context.Background()
	tenantID := uuid.New()
	ctx = composables.WithTenantID(ctx, tenantID)

	sessionID := uuid.New()
	content := "Hello, test agent!"
	var attachments []domain.Attachment

	// Execute
	gen, err := service.ProcessMessage(ctx, sessionID, content, attachments)
	require.NoError(t, err)
	require.NotNil(t, gen)
	defer gen.Close()

	// Collect all events
	var events []services.Event
	for {
		event, err := gen.Next()
		if errors.Is(err, services.ErrGeneratorDone) {
			break
		}
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		events = append(events, event)
	}

	// Verify we got events
	assert.NotEmpty(t, events)

	// Should have at least content chunks and a done event
	hasContent := false
	hasDone := false
	for _, event := range events {
		if event.Type == services.EventTypeContent {
			hasContent = true
		}
		if event.Type == services.EventTypeDone {
			hasDone = true
		}
	}

	assert.True(t, hasContent, "Expected content events")
	assert.True(t, hasDone, "Expected done event")
}

func TestProcessMessage_MissingTenantID(t *testing.T) {
	t.Parallel()

	// Setup
	agent := newMockAgent()
	model := newMockModel()
	renderer := &mockRenderer{}
	checkpointer := newMockCheckpointer()
	chatRepo := newMockChatRepository()

	policy := bichatctx.ContextPolicy{
		ContextWindow:     4096,
		CompletionReserve: 1024,
		MaxSensitivity:    bichatctx.SensitivityPublic,
		OverflowStrategy:  bichatctx.OverflowTruncate,
	}

	service := NewAgentService(AgentServiceConfig{
		Agent:        agent,
		Model:        model,
		Policy:       policy,
		Renderer:     renderer,
		Checkpointer: checkpointer,
		ChatRepo:     chatRepo,
	})

	// Create context WITHOUT tenant ID
	ctx := context.Background()
	sessionID := uuid.New()
	content := "Hello, test agent!"
	var attachments []domain.Attachment

	// Execute
	gen, err := service.ProcessMessage(ctx, sessionID, content, attachments)

	// Should fail without tenant ID
	assert.Error(t, err)
	assert.Nil(t, gen)
}

func TestResumeWithAnswer_Success(t *testing.T) {
	t.Parallel()

	// Setup
	agent := newMockAgent()
	model := newMockModel()
	renderer := &mockRenderer{}
	checkpointer := newMockCheckpointer()
	chatRepo := newMockChatRepository()

	policy := bichatctx.ContextPolicy{
		ContextWindow:     4096,
		CompletionReserve: 1024,
		MaxSensitivity:    bichatctx.SensitivityPublic,
		OverflowStrategy:  bichatctx.OverflowTruncate,
	}

	service := NewAgentService(AgentServiceConfig{
		Agent:        agent,
		Model:        model,
		Policy:       policy,
		Renderer:     renderer,
		Checkpointer: checkpointer,
		ChatRepo:     chatRepo,
	})

	// Create context with tenant ID
	ctx := context.Background()
	tenantID := uuid.New()
	ctx = composables.WithTenantID(ctx, tenantID)

	// Create a checkpoint first
	checkpoint := agents.NewCheckpoint(
		"test-thread",
		"test_agent",
		[]types.Message{
			*types.UserMessage("What is 2+2?"),
		},
	)
	checkpointID, err := checkpointer.Save(ctx, checkpoint)
	require.NoError(t, err)

	sessionID := uuid.New()

	// Test with multiple answers to verify map support
	answers := map[string]string{
		"question_1": "Q1 2024",
		"question_2": "revenue",
		"question_3": "increase",
	}

	// Execute
	gen, err := service.ResumeWithAnswer(ctx, sessionID, checkpointID, answers)
	require.NoError(t, err)
	require.NotNil(t, gen)
	defer gen.Close()

	// Collect all events
	var events []services.Event
	for {
		event, err := gen.Next()
		if errors.Is(err, services.ErrGeneratorDone) {
			break
		}
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		events = append(events, event)
	}

	// Verify we got events
	assert.NotEmpty(t, events)
}

func TestResumeWithAnswer_EmptyCheckpointID(t *testing.T) {
	t.Parallel()

	// Setup
	agent := newMockAgent()
	model := newMockModel()
	renderer := &mockRenderer{}
	checkpointer := newMockCheckpointer()
	chatRepo := newMockChatRepository()

	policy := bichatctx.ContextPolicy{
		ContextWindow:     4096,
		CompletionReserve: 1024,
		MaxSensitivity:    bichatctx.SensitivityPublic,
		OverflowStrategy:  bichatctx.OverflowTruncate,
	}

	service := NewAgentService(AgentServiceConfig{
		Agent:        agent,
		Model:        model,
		Policy:       policy,
		Renderer:     renderer,
		Checkpointer: checkpointer,
		ChatRepo:     chatRepo,
	})

	// Create context with tenant ID
	ctx := context.Background()
	tenantID := uuid.New()
	ctx = composables.WithTenantID(ctx, tenantID)

	sessionID := uuid.New()
	answers := map[string]string{"q1": "answer"}

	// Execute with empty checkpoint ID
	gen, err := service.ResumeWithAnswer(ctx, sessionID, "", answers)

	// Should fail with validation error
	assert.Error(t, err)
	assert.Nil(t, gen)
}

func TestResumeWithAnswer_MissingTenantID(t *testing.T) {
	t.Parallel()

	// Setup
	agent := newMockAgent()
	model := newMockModel()
	renderer := &mockRenderer{}
	checkpointer := newMockCheckpointer()
	chatRepo := newMockChatRepository()

	policy := bichatctx.ContextPolicy{
		ContextWindow:     4096,
		CompletionReserve: 1024,
		MaxSensitivity:    bichatctx.SensitivityPublic,
		OverflowStrategy:  bichatctx.OverflowTruncate,
	}

	service := NewAgentService(AgentServiceConfig{
		Agent:        agent,
		Model:        model,
		Policy:       policy,
		Renderer:     renderer,
		Checkpointer: checkpointer,
		ChatRepo:     chatRepo,
	})

	// Create context WITHOUT tenant ID
	ctx := context.Background()
	sessionID := uuid.New()
	checkpointID := "test-checkpoint"
	answers := map[string]string{"q1": "answer"}

	// Execute
	gen, err := service.ResumeWithAnswer(ctx, sessionID, checkpointID, answers)

	// Should fail without tenant ID
	assert.Error(t, err)
	assert.Nil(t, gen)
}

func TestConvertExecutorEvent_Chunk(t *testing.T) {
	t.Parallel()

	execEvent := agents.ExecutorEvent{
		Type: agents.EventTypeChunk,
		Chunk: &agents.Chunk{
			Delta: "Hello world",
		},
	}

	serviceEvent := convertExecutorEvent(execEvent)

	assert.Equal(t, services.EventTypeContent, serviceEvent.Type)
	assert.Equal(t, "Hello world", serviceEvent.Content)
}

func TestConvertExecutorEvent_ToolStart(t *testing.T) {
	t.Parallel()

	execEvent := agents.ExecutorEvent{
		Type: agents.EventTypeToolStart,
		Tool: &agents.ToolEvent{
			Name:      "test_tool",
			Arguments: `{"param": "value"}`,
		},
	}

	serviceEvent := convertExecutorEvent(execEvent)

	assert.Equal(t, services.EventTypeToolStart, serviceEvent.Type)
	require.NotNil(t, serviceEvent.Tool)
	assert.Equal(t, "test_tool", serviceEvent.Tool.Name)
	assert.Equal(t, `{"param": "value"}`, serviceEvent.Tool.Arguments)
}

func TestConvertExecutorEvent_ToolEnd(t *testing.T) {
	t.Parallel()

	execEvent := agents.ExecutorEvent{
		Type: agents.EventTypeToolEnd,
		Tool: &agents.ToolEvent{
			Name:      "test_tool",
			Arguments: `{"param": "value"}`,
			Result:    "tool result",
			Error:     nil,
		},
	}

	serviceEvent := convertExecutorEvent(execEvent)

	assert.Equal(t, services.EventTypeToolEnd, serviceEvent.Type)
	require.NotNil(t, serviceEvent.Tool)
	assert.Equal(t, "test_tool", serviceEvent.Tool.Name)
	assert.Equal(t, "tool result", serviceEvent.Tool.Result)
	assert.Nil(t, serviceEvent.Tool.Error)
}

func TestConvertExecutorEvent_Interrupt(t *testing.T) {
	t.Parallel()

	sessionID := uuid.New()
	checkpointID := "checkpoint_" + sessionID.String()
	// Use new array format: {questions: [...]}
	interruptData := []byte(`{"questions": [{"id": "q1", "question": "What is your name?"}]}`)

	execEvent := agents.ExecutorEvent{
		Type: agents.EventTypeInterrupt,
		Interrupt: &agents.InterruptEvent{
			Type:         agents.ToolAskUserQuestion,
			SessionID:    sessionID,
			Data:         interruptData,
			CheckpointID: checkpointID,
		},
	}

	serviceEvent := convertExecutorEvent(execEvent)

	assert.Equal(t, services.EventTypeInterrupt, serviceEvent.Type)
	require.NotNil(t, serviceEvent.Interrupt)
	assert.NotEmpty(t, serviceEvent.Interrupt.CheckpointID)
	require.Len(t, serviceEvent.Interrupt.Questions, 1)
	assert.Equal(t, "q1", serviceEvent.Interrupt.Questions[0].ID)
	assert.Equal(t, "What is your name?", serviceEvent.Interrupt.Questions[0].Text)
	assert.Equal(t, services.QuestionTypeText, serviceEvent.Interrupt.Questions[0].Type)
}

func TestConvertExecutorEvent_Done(t *testing.T) {
	t.Parallel()

	execEvent := agents.ExecutorEvent{
		Type: agents.EventTypeDone,
		Done: true,
		Result: &agents.Response{
			Message: *types.AssistantMessage("Final response"),
			Usage: types.TokenUsage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
			},
			FinishReason: "stop",
		},
	}

	serviceEvent := convertExecutorEvent(execEvent)

	assert.Equal(t, services.EventTypeDone, serviceEvent.Type)
	assert.True(t, serviceEvent.Done)
	require.NotNil(t, serviceEvent.Usage)
	assert.Equal(t, 100, serviceEvent.Usage.PromptTokens)
	assert.Equal(t, 50, serviceEvent.Usage.CompletionTokens)
	assert.Equal(t, 150, serviceEvent.Usage.TotalTokens)
}

func TestConvertExecutorEvent_Error(t *testing.T) {
	t.Parallel()

	testErr := errors.New("test error")
	execEvent := agents.ExecutorEvent{
		Type:  agents.EventTypeError,
		Error: testErr,
	}

	serviceEvent := convertExecutorEvent(execEvent)

	assert.Equal(t, services.EventTypeError, serviceEvent.Type)
	assert.Equal(t, testErr, serviceEvent.Error)
}

func TestGeneratorAdapter_Close(t *testing.T) {
	t.Parallel()

	// Create a simple executor event generator for testing
	execGen := types.NewGenerator(context.Background(), func(ctx context.Context, yield func(agents.ExecutorEvent) bool) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		yield(agents.ExecutorEvent{
			Type:  agents.EventTypeChunk,
			Chunk: &agents.Chunk{Delta: "test"},
		})
		return nil
	})

	adapter := &generatorAdapter{
		ctx:   context.Background(),
		inner: execGen,
	}
	adapter.Close()

	// After close, Next should return ErrGeneratorDone
	_, err := adapter.Next()
	assert.Error(t, err) // Should get an error (either ErrGeneratorDone or closed error)
}
