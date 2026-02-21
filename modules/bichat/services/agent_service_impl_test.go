package services

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	bichatctx "github.com/iota-uz/iota-sdk/pkg/bichat/context"
	"github.com/iota-uz/iota-sdk/pkg/bichat/context/codecs"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/hooks"
	"github.com/iota-uz/iota-sdk/pkg/bichat/services"
	bichatskills "github.com/iota-uz/iota-sdk/pkg/bichat/skills"
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
	requests []agents.Request
}

func newMockModel() *mockModel {
	return &mockModel{
		response: &agents.Response{
			Message: types.AssistantMessage("Test response"),
			Usage: types.TokenUsage{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			},
			FinishReason:       "stop",
			ProviderResponseID: "resp_mock_final",
		},
		chunks: []agents.Chunk{
			{Delta: "Test ", Done: false},
			{Delta: "response", Done: false},
			{
				Delta:              "",
				FinishReason:       "stop",
				Done:               true,
				ProviderResponseID: "resp_mock_final",
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
	m.requests = append(m.requests, req)
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func (m *mockModel) Stream(ctx context.Context, req agents.Request, opts ...agents.GenerateOption) (types.Generator[agents.Chunk], error) {
	m.requests = append(m.requests, req)
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
	}), nil
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

// spyRenderer records blocks passed through compilation and renders canonical messages.
// It allows tests to assert on block kinds/payloads without depending on internal compiler behavior.
type spyRenderer struct {
	mu              sync.Mutex
	estimatedBlocks []bichatctx.ContextBlock
	renderedBlocks  []bichatctx.ContextBlock
}

func (s *spyRenderer) Render(block bichatctx.ContextBlock) (bichatctx.RenderedBlock, error) {
	s.mu.Lock()
	s.renderedBlocks = append(s.renderedBlocks, block)
	s.mu.Unlock()

	switch block.Meta.Kind {
	case bichatctx.KindPinned:
		if v, ok := block.Payload.(string); ok && v != "" {
			return bichatctx.RenderedBlock{Messages: []types.Message{types.SystemMessage(v)}}, nil
		}
		return bichatctx.RenderedBlock{Messages: []types.Message{types.SystemMessage("system")}}, nil

	case bichatctx.KindHistory:
		if h, ok := block.Payload.(codecs.ConversationHistoryPayload); ok && len(h.Messages) > 0 {
			msgs := make([]types.Message, 0, len(h.Messages))
			for _, m := range h.Messages {
				role := types.RoleUser
				switch m.Role {
				case "system":
					role = types.RoleSystem
				case "assistant":
					role = types.RoleAssistant
				case "tool":
					role = types.RoleTool
				case "user":
					role = types.RoleUser
				}
				msgs = append(msgs, types.NewMessage(
					types.WithRole(role),
					types.WithContent(m.Content),
				))
			}
			return bichatctx.RenderedBlock{Messages: msgs}, nil
		}
		return bichatctx.RenderedBlock{Messages: []types.Message{types.SystemMessage("history")}}, nil

	case bichatctx.KindTurn:
		if t, ok := block.Payload.(codecs.TurnPayload); ok {
			return bichatctx.RenderedBlock{Messages: []types.Message{types.UserMessage(t.Content)}}, nil
		}
		return bichatctx.RenderedBlock{Messages: []types.Message{types.UserMessage("turn")}}, nil

	case bichatctx.KindReference:
		return bichatctx.RenderedBlock{Messages: []types.Message{types.SystemMessage("reference")}}, nil

	case bichatctx.KindMemory:
		return bichatctx.RenderedBlock{Messages: []types.Message{types.SystemMessage("memory")}}, nil

	case bichatctx.KindState:
		return bichatctx.RenderedBlock{Messages: []types.Message{types.SystemMessage("state")}}, nil

	case bichatctx.KindToolOutput:
		return bichatctx.RenderedBlock{Messages: []types.Message{types.SystemMessage("tool_output")}}, nil
	}

	return bichatctx.RenderedBlock{}, nil
}

func (s *spyRenderer) EstimateTokens(block bichatctx.ContextBlock) (int, error) {
	s.mu.Lock()
	s.estimatedBlocks = append(s.estimatedBlocks, block)
	s.mu.Unlock()
	return 10, nil
}

func (s *spyRenderer) Provider() string {
	return "spy"
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
	sessions    map[uuid.UUID]domain.Session
	messages    map[uuid.UUID][]types.Message
	attachments map[uuid.UUID]domain.Attachment
	artifacts   map[uuid.UUID]domain.Artifact
}

func newMockChatRepository() *mockChatRepository {
	return &mockChatRepository{
		sessions:    make(map[uuid.UUID]domain.Session),
		messages:    make(map[uuid.UUID][]types.Message),
		attachments: make(map[uuid.UUID]domain.Attachment),
		artifacts:   make(map[uuid.UUID]domain.Artifact),
	}
}

func (m *mockChatRepository) CreateSession(ctx context.Context, session domain.Session) error {
	m.sessions[session.ID()] = session
	return nil
}

func (m *mockChatRepository) GetSession(ctx context.Context, id uuid.UUID) (domain.Session, error) {
	session, exists := m.sessions[id]
	if !exists {
		return nil, errors.New("session not found")
	}
	return session, nil
}

func (m *mockChatRepository) UpdateSession(ctx context.Context, session domain.Session) error {
	m.sessions[session.ID()] = session
	return nil
}

func (m *mockChatRepository) UpdateSessionTitle(ctx context.Context, id uuid.UUID, title string) error {
	session, exists := m.sessions[id]
	if !exists {
		return errors.New("session not found")
	}
	m.sessions[id] = session.UpdateTitle(title)
	return nil
}

func (m *mockChatRepository) UpdateSessionTitleIfEmpty(ctx context.Context, id uuid.UUID, title string) (bool, error) {
	session, exists := m.sessions[id]
	if !exists {
		return false, errors.New("session not found")
	}
	if strings.TrimSpace(session.Title()) != "" {
		return false, nil
	}
	m.sessions[id] = session.UpdateTitle(title)
	return true, nil
}

func (m *mockChatRepository) ListUserSessions(ctx context.Context, userID int64, opts domain.ListOptions) ([]domain.Session, error) {
	sessions := make([]domain.Session, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}
	return sessions, nil
}

func (m *mockChatRepository) CountUserSessions(ctx context.Context, userID int64, opts domain.ListOptions) (int, error) {
	return len(m.sessions), nil
}

func (m *mockChatRepository) DeleteSession(ctx context.Context, id uuid.UUID) error {
	delete(m.sessions, id)
	delete(m.messages, id)
	return nil
}

func (m *mockChatRepository) SaveMessage(ctx context.Context, msg types.Message) error {
	sessionID := msg.SessionID()
	m.messages[sessionID] = append(m.messages[sessionID], msg)
	return nil
}

func (m *mockChatRepository) GetMessage(ctx context.Context, id uuid.UUID) (types.Message, error) {
	for _, msgs := range m.messages {
		for _, msg := range msgs {
			if msg.ID() == id {
				return msg, nil
			}
		}
	}
	return nil, errors.New("message not found")
}

func (m *mockChatRepository) GetSessionMessages(ctx context.Context, sessionID uuid.UUID, opts domain.ListOptions) ([]types.Message, error) {
	msgs, exists := m.messages[sessionID]
	if !exists {
		return []types.Message{}, nil
	}
	return msgs, nil
}

func (m *mockChatRepository) TruncateMessagesFrom(ctx context.Context, sessionID uuid.UUID, from time.Time) (int64, error) {
	messages := m.messages[sessionID]
	filtered := make([]types.Message, 0, len(messages))
	var deleted int64
	for _, msg := range messages {
		if msg.CreatedAt().Before(from) {
			filtered = append(filtered, msg)
			continue
		}
		deleted++
	}
	m.messages[sessionID] = filtered
	return deleted, nil
}

func (m *mockChatRepository) SaveAttachment(ctx context.Context, attachment domain.Attachment) error {
	m.attachments[attachment.ID()] = attachment
	return nil
}

func (m *mockChatRepository) GetAttachment(ctx context.Context, id uuid.UUID) (domain.Attachment, error) {
	att, exists := m.attachments[id]
	if !exists {
		return nil, errors.New("attachment not found")
	}
	return att, nil
}

func (m *mockChatRepository) GetMessageAttachments(ctx context.Context, messageID uuid.UUID) ([]domain.Attachment, error) {
	atts := make([]domain.Attachment, 0, len(m.attachments))
	for _, att := range m.attachments {
		atts = append(atts, att)
	}
	return atts, nil
}

func (m *mockChatRepository) DeleteAttachment(ctx context.Context, id uuid.UUID) error {
	delete(m.attachments, id)
	return nil
}

func (m *mockChatRepository) SaveArtifact(ctx context.Context, artifact domain.Artifact) error {
	m.artifacts[artifact.ID()] = artifact
	return nil
}

func (m *mockChatRepository) GetArtifact(ctx context.Context, id uuid.UUID) (domain.Artifact, error) {
	artifact, exists := m.artifacts[id]
	if !exists {
		return nil, errors.New("artifact not found")
	}
	return artifact, nil
}

func (m *mockChatRepository) GetSessionArtifacts(ctx context.Context, sessionID uuid.UUID, opts domain.ListOptions) ([]domain.Artifact, error) {
	result := make([]domain.Artifact, 0)
	for _, artifact := range m.artifacts {
		if artifact.SessionID() == sessionID {
			result = append(result, artifact)
		}
	}
	return result, nil
}

func (m *mockChatRepository) DeleteSessionArtifacts(ctx context.Context, sessionID uuid.UUID) (int64, error) {
	var deleted int64
	for id, artifact := range m.artifacts {
		if artifact.SessionID() == sessionID {
			delete(m.artifacts, id)
			deleted++
		}
	}
	return deleted, nil
}

func (m *mockChatRepository) DeleteArtifact(ctx context.Context, id uuid.UUID) error {
	delete(m.artifacts, id)
	return nil
}

func (m *mockChatRepository) UpdateArtifact(ctx context.Context, id uuid.UUID, name, description string) error {
	return nil
}

func (m *mockChatRepository) UpdateMessageQuestionData(ctx context.Context, msgID uuid.UUID, qd *types.QuestionData) error {
	msgs := m.messages
	for sid, sessionMsgs := range msgs {
		for i, msg := range sessionMsgs {
			if msg.ID() == msgID {
				updated := types.NewMessage(
					types.WithMessageID(msg.ID()),
					types.WithSessionID(msg.SessionID()),
					types.WithRole(msg.Role()),
					types.WithContent(msg.Content()),
					types.WithQuestionData(qd),
					types.WithCreatedAt(msg.CreatedAt()),
				)
				m.messages[sid][i] = updated
				return nil
			}
		}
	}
	return errors.New("message not found")
}

func (m *mockChatRepository) GetPendingQuestionMessage(ctx context.Context, sessionID uuid.UUID) (types.Message, error) {
	for _, msg := range m.messages[sessionID] {
		if msg.HasPendingQuestion() {
			return msg, nil
		}
	}
	return nil, errors.New("no pending question found")
}

func TestProcessMessage_Success(t *testing.T) {
	t.Parallel()

	// Setup
	agent := newMockAgent()
	model := newMockModel()
	renderer := &spyRenderer{}
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
		EventBus:     hooks.NewEventBus(),
		ChatRepo:     chatRepo,
	})

	// Create context with tenant ID
	ctx := context.Background()
	tenantID := uuid.New()
	ctx = composables.WithTenantID(ctx, tenantID)

	sessionID := uuid.New()
	session := domain.NewSession(
		domain.WithID(sessionID),
		domain.WithTenantID(tenantID),
		domain.WithUserID(1),
		domain.WithTitle("test"),
	)
	require.NoError(t, chatRepo.CreateSession(ctx, session))
	content := "Hello, test agent!"
	require.NoError(t, chatRepo.SaveMessage(ctx, types.UserMessage(
		"previous",
		types.WithSessionID(sessionID),
		types.WithCreatedAt(time.Now().Add(-1*time.Minute)),
	)))

	attachments := []domain.Attachment{
		domain.NewAttachment(
			domain.WithAttachmentID(uuid.New()),
			domain.WithAttachmentMessageID(uuid.New()),
			domain.WithFileName("report.png"),
			domain.WithMimeType("image/png"),
			domain.WithSizeBytes(1234),
			domain.WithFilePath("/uploads/report.png"),
			domain.WithAttachmentCreatedAt(time.Now()),
		),
	}

	// Execute
	gen, err := service.ProcessMessage(ctx, sessionID, content, attachments)
	require.NoError(t, err)
	require.NotNil(t, gen)
	defer gen.Close()

	// Verify compilation contract: Pinned -> History -> Turn blocks with expected payloads.
	renderer.mu.Lock()
	rendered := append([]bichatctx.ContextBlock(nil), renderer.renderedBlocks...)
	renderer.mu.Unlock()

	require.Len(t, rendered, 3)
	assert.Equal(t, bichatctx.KindPinned, rendered[0].Meta.Kind)
	assert.Equal(t, bichatctx.KindHistory, rendered[1].Meta.Kind)
	assert.Equal(t, bichatctx.KindTurn, rendered[2].Meta.Kind)

	pinnedPayload, ok := rendered[0].Payload.(string)
	require.True(t, ok)
	assert.Equal(t, agent.systemPrompt, pinnedPayload)

	historyPayload, ok := rendered[1].Payload.(codecs.ConversationHistoryPayload)
	require.True(t, ok)
	require.Len(t, historyPayload.Messages, 1)
	assert.Equal(t, "user", historyPayload.Messages[0].Role)
	assert.Equal(t, "previous", historyPayload.Messages[0].Content)

	turnPayload, ok := rendered[2].Payload.(codecs.TurnPayload)
	require.True(t, ok)
	assert.Equal(t, content, turnPayload.Content)
	require.Len(t, turnPayload.Attachments, 1)
	assert.Equal(t, "report.png", turnPayload.Attachments[0].FileName)
	assert.Equal(t, "image/png", turnPayload.Attachments[0].MimeType)
	assert.Equal(t, int64(1234), turnPayload.Attachments[0].SizeBytes)
	assert.Equal(t, "/uploads/report.png", turnPayload.Attachments[0].Reference)

	// Collect all events
	var events []agents.ExecutorEvent
	for {
		event, err := gen.Next(ctx)
		if errors.Is(err, types.ErrGeneratorDone) {
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
	var done *agents.ExecutorEvent
	for _, event := range events {
		if event.Type == agents.EventTypeContent {
			hasContent = true
		}
		if event.Type == agents.EventTypeDone {
			e := event
			done = &e
		}
	}

	assert.True(t, hasContent, "Expected content events")
	require.NotNil(t, done, "Expected done event")
	require.NotNil(t, done.Usage)
	assert.Equal(t, 10, done.Usage.PromptTokens)
	assert.Equal(t, 20, done.Usage.CompletionTokens)
	assert.Equal(t, 30, done.Usage.TotalTokens)
	assert.Equal(t, "resp_mock_final", done.ProviderResponseID)
}

func TestProcessMessage_AppendsProjectPromptExtension(t *testing.T) {
	t.Parallel()

	agent := newMockAgent()
	model := newMockModel()
	renderer := &spyRenderer{}
	checkpointer := newMockCheckpointer()
	chatRepo := newMockChatRepository()

	policy := bichatctx.ContextPolicy{
		ContextWindow:     4096,
		CompletionReserve: 1024,
		MaxSensitivity:    bichatctx.SensitivityPublic,
		OverflowStrategy:  bichatctx.OverflowTruncate,
	}

	projectExtension := "You are operating in insurance BI domain."

	service := NewAgentService(AgentServiceConfig{
		Agent:                  agent,
		Model:                  model,
		Policy:                 policy,
		Renderer:               renderer,
		Checkpointer:           checkpointer,
		EventBus:               hooks.NewEventBus(),
		ChatRepo:               chatRepo,
		ProjectPromptExtension: projectExtension,
	})

	ctx := context.Background()
	tenantID := uuid.New()
	ctx = composables.WithTenantID(ctx, tenantID)

	sessionID := uuid.New()
	session := domain.NewSession(
		domain.WithID(sessionID),
		domain.WithTenantID(tenantID),
		domain.WithUserID(1),
		domain.WithTitle("test"),
	)
	require.NoError(t, chatRepo.CreateSession(ctx, session))

	gen, err := service.ProcessMessage(ctx, sessionID, "hello", nil)
	require.NoError(t, err)
	require.NotNil(t, gen)
	defer gen.Close()

	renderer.mu.Lock()
	rendered := append([]bichatctx.ContextBlock(nil), renderer.renderedBlocks...)
	renderer.mu.Unlock()

	require.NotEmpty(t, rendered)
	pinnedPayload, ok := rendered[0].Payload.(string)
	require.True(t, ok)
	assert.Equal(t, agent.systemPrompt+"\n\nPROJECT DOMAIN EXTENSION:\n"+projectExtension, pinnedPayload)
}

func TestProcessMessage_AppendsDebugPromptAfterProjectPromptExtension(t *testing.T) {
	t.Parallel()

	agent := newMockAgent()
	model := newMockModel()
	renderer := &spyRenderer{}
	checkpointer := newMockCheckpointer()
	chatRepo := newMockChatRepository()

	policy := bichatctx.ContextPolicy{
		ContextWindow:     4096,
		CompletionReserve: 1024,
		MaxSensitivity:    bichatctx.SensitivityPublic,
		OverflowStrategy:  bichatctx.OverflowTruncate,
	}

	projectExtension := "Insurance domain extension."

	service := NewAgentService(AgentServiceConfig{
		Agent:                  agent,
		Model:                  model,
		Policy:                 policy,
		Renderer:               renderer,
		Checkpointer:           checkpointer,
		EventBus:               hooks.NewEventBus(),
		ChatRepo:               chatRepo,
		ProjectPromptExtension: projectExtension,
	})

	ctx := context.Background()
	tenantID := uuid.New()
	ctx = composables.WithTenantID(ctx, tenantID)
	ctx = services.WithDebugMode(ctx, true)

	sessionID := uuid.New()
	session := domain.NewSession(
		domain.WithID(sessionID),
		domain.WithTenantID(tenantID),
		domain.WithUserID(1),
		domain.WithTitle("test"),
	)
	require.NoError(t, chatRepo.CreateSession(ctx, session))

	gen, err := service.ProcessMessage(ctx, sessionID, "hello", nil)
	require.NoError(t, err)
	require.NotNil(t, gen)
	defer gen.Close()

	renderer.mu.Lock()
	rendered := append([]bichatctx.ContextBlock(nil), renderer.renderedBlocks...)
	renderer.mu.Unlock()

	require.NotEmpty(t, rendered)
	pinnedPayload, ok := rendered[0].Payload.(string)
	require.True(t, ok)
	assert.Contains(t, pinnedPayload, "PROJECT DOMAIN EXTENSION:\n"+projectExtension)
	assert.Contains(t, pinnedPayload, "DEBUG MODE ENABLED:")
	assert.Less(
		t,
		strings.Index(pinnedPayload, "PROJECT DOMAIN EXTENSION:"),
		strings.Index(pinnedPayload, "DEBUG MODE ENABLED:"),
	)
}

func TestProcessMessage_IncludesSelectedSkillsReference(t *testing.T) {
	t.Parallel()

	skillsRoot := t.TempDir()
	writeServiceTestSkillFile(t, skillsRoot, "analytics/sql-debug", `---
name: SQL Debugging
description: Recover quickly from SQL failures
when_to_use:
  - sql error
  - missing column
tags:
  - sql
---
Inspect schema and retry safely.
`)

	catalog, err := bichatskills.LoadCatalog(skillsRoot)
	require.NoError(t, err)
	selector := bichatskills.NewSelector(catalog)

	agent := newMockAgent()
	model := newMockModel()
	renderer := &spyRenderer{}
	checkpointer := newMockCheckpointer()
	chatRepo := newMockChatRepository()

	policy := bichatctx.ContextPolicy{
		ContextWindow:     4096,
		CompletionReserve: 1024,
		MaxSensitivity:    bichatctx.SensitivityPublic,
		OverflowStrategy:  bichatctx.OverflowTruncate,
	}

	service := NewAgentService(AgentServiceConfig{
		Agent:          agent,
		Model:          model,
		Policy:         policy,
		Renderer:       renderer,
		Checkpointer:   checkpointer,
		EventBus:       hooks.NewEventBus(),
		ChatRepo:       chatRepo,
		SkillsSelector: selector,
	})

	ctx := context.Background()
	tenantID := uuid.New()
	ctx = composables.WithTenantID(ctx, tenantID)

	sessionID := uuid.New()
	session := domain.NewSession(
		domain.WithID(sessionID),
		domain.WithTenantID(tenantID),
		domain.WithUserID(1),
		domain.WithTitle("test"),
	)
	require.NoError(t, chatRepo.CreateSession(ctx, session))

	gen, err := service.ProcessMessage(ctx, sessionID, "I have sql error in my query", nil)
	require.NoError(t, err)
	require.NotNil(t, gen)
	defer gen.Close()

	renderer.mu.Lock()
	rendered := append([]bichatctx.ContextBlock(nil), renderer.renderedBlocks...)
	renderer.mu.Unlock()

	var hasSkillsReference bool
	for _, block := range rendered {
		if block.Meta.Kind != bichatctx.KindReference {
			continue
		}
		payload, ok := block.Payload.(string)
		if !ok {
			continue
		}
		if strings.Contains(payload, "@analytics/sql-debug") {
			hasSkillsReference = true
			break
		}
	}
	assert.True(t, hasSkillsReference, "expected skills reference block with selected skill")
}

func TestProcessMessage_NoSkillsMatchDoesNotInjectReference(t *testing.T) {
	t.Parallel()

	skillsRoot := t.TempDir()
	writeServiceTestSkillFile(t, skillsRoot, "analytics/sql-debug", `---
name: SQL Debugging
description: Recover quickly from SQL failures
when_to_use:
  - sql error
tags:
  - sql
---
Inspect schema and retry safely.
`)

	catalog, err := bichatskills.LoadCatalog(skillsRoot)
	require.NoError(t, err)
	selector := bichatskills.NewSelector(catalog)

	agent := newMockAgent()
	model := newMockModel()
	renderer := &spyRenderer{}
	checkpointer := newMockCheckpointer()
	chatRepo := newMockChatRepository()

	policy := bichatctx.ContextPolicy{
		ContextWindow:     4096,
		CompletionReserve: 1024,
		MaxSensitivity:    bichatctx.SensitivityPublic,
		OverflowStrategy:  bichatctx.OverflowTruncate,
	}

	service := NewAgentService(AgentServiceConfig{
		Agent:          agent,
		Model:          model,
		Policy:         policy,
		Renderer:       renderer,
		Checkpointer:   checkpointer,
		EventBus:       hooks.NewEventBus(),
		ChatRepo:       chatRepo,
		SkillsSelector: selector,
	})

	ctx := context.Background()
	tenantID := uuid.New()
	ctx = composables.WithTenantID(ctx, tenantID)

	sessionID := uuid.New()
	session := domain.NewSession(
		domain.WithID(sessionID),
		domain.WithTenantID(tenantID),
		domain.WithUserID(1),
		domain.WithTitle("test"),
	)
	require.NoError(t, chatRepo.CreateSession(ctx, session))

	gen, err := service.ProcessMessage(ctx, sessionID, "hello there", nil)
	require.NoError(t, err)
	require.NotNil(t, gen)
	defer gen.Close()

	renderer.mu.Lock()
	rendered := append([]bichatctx.ContextBlock(nil), renderer.renderedBlocks...)
	renderer.mu.Unlock()

	for _, block := range rendered {
		if block.Meta.Kind != bichatctx.KindReference {
			continue
		}
		payload, ok := block.Payload.(string)
		if !ok {
			continue
		}
		assert.NotContains(t, payload, "SKILLS CONTEXT:")
	}
}

func TestProcessMessage_MentionSelectsSkill(t *testing.T) {
	t.Parallel()

	skillsRoot := t.TempDir()
	writeServiceTestSkillFile(t, skillsRoot, "finance/month-end", `---
name: Month End
description: Close month books and verify reconciliations
when_to_use:
  - month close
tags:
  - finance
---
Run period checks before closing.
`)
	writeServiceTestSkillFile(t, skillsRoot, "analytics/sql-debug", `---
name: SQL Debugging
description: Recover quickly from SQL failures
when_to_use:
  - sql error
tags:
  - sql
---
Inspect schema and retry safely.
`)

	catalog, err := bichatskills.LoadCatalog(skillsRoot)
	require.NoError(t, err)
	selector := bichatskills.NewSelector(catalog)

	agent := newMockAgent()
	model := newMockModel()
	renderer := &spyRenderer{}
	checkpointer := newMockCheckpointer()
	chatRepo := newMockChatRepository()

	policy := bichatctx.ContextPolicy{
		ContextWindow:     4096,
		CompletionReserve: 1024,
		MaxSensitivity:    bichatctx.SensitivityPublic,
		OverflowStrategy:  bichatctx.OverflowTruncate,
	}

	service := NewAgentService(AgentServiceConfig{
		Agent:          agent,
		Model:          model,
		Policy:         policy,
		Renderer:       renderer,
		Checkpointer:   checkpointer,
		EventBus:       hooks.NewEventBus(),
		ChatRepo:       chatRepo,
		SkillsSelector: selector,
	})

	ctx := context.Background()
	tenantID := uuid.New()
	ctx = composables.WithTenantID(ctx, tenantID)

	sessionID := uuid.New()
	session := domain.NewSession(
		domain.WithID(sessionID),
		domain.WithTenantID(tenantID),
		domain.WithUserID(1),
		domain.WithTitle("test"),
	)
	require.NoError(t, chatRepo.CreateSession(ctx, session))

	gen, err := service.ProcessMessage(ctx, sessionID, "please use @finance/month-end checklist", nil)
	require.NoError(t, err)
	require.NotNil(t, gen)
	defer gen.Close()

	renderer.mu.Lock()
	rendered := append([]bichatctx.ContextBlock(nil), renderer.renderedBlocks...)
	renderer.mu.Unlock()

	var found bool
	for _, block := range rendered {
		if block.Meta.Kind != bichatctx.KindReference {
			continue
		}
		payload, ok := block.Payload.(string)
		if !ok {
			continue
		}
		if strings.Contains(payload, "@finance/month-end") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected mention-selected skill in reference block")
}

func TestProcessMessage_ForwardsSessionPreviousResponseID(t *testing.T) {
	t.Parallel()

	agent := newMockAgent()
	model := newMockModel()
	renderer := &spyRenderer{}
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
		EventBus:     hooks.NewEventBus(),
		ChatRepo:     chatRepo,
	})

	ctx := context.Background()
	tenantID := uuid.New()
	ctx = composables.WithTenantID(ctx, tenantID)

	sessionID := uuid.New()
	session := domain.NewSession(
		domain.WithID(sessionID),
		domain.WithTenantID(tenantID),
		domain.WithUserID(1),
		domain.WithTitle("continuity"),
		domain.WithLLMPreviousResponseID("resp_prev_42"),
	)
	require.NoError(t, chatRepo.CreateSession(ctx, session))

	gen, err := service.ProcessMessage(ctx, sessionID, "continue", nil)
	require.NoError(t, err)
	require.NotNil(t, gen)
	defer gen.Close()

	for {
		_, err := gen.Next(ctx)
		if errors.Is(err, types.ErrGeneratorDone) {
			break
		}
		require.NoError(t, err)
	}

	require.NotEmpty(t, model.requests)
	require.NotNil(t, model.requests[0].PreviousResponseID)
	assert.Equal(t, "resp_prev_42", *model.requests[0].PreviousResponseID)
}

func writeServiceTestSkillFile(t *testing.T, root, relDir, content string) {
	t.Helper()
	dir := filepath.Join(root, relDir)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644))
}

func TestProcessMessage_MissingTenantID(t *testing.T) {
	t.Parallel()

	// Setup
	agent := newMockAgent()
	model := newMockModel()
	renderer := &spyRenderer{}
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
		EventBus:     hooks.NewEventBus(),
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
	require.Error(t, err)
	assert.Nil(t, gen)
}

func TestResumeWithAnswer_Success(t *testing.T) {
	t.Parallel()

	// Setup
	agent := newMockAgent()
	model := newMockModel()
	renderer := &spyRenderer{}
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
		EventBus:     hooks.NewEventBus(),
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
			types.UserMessage("What is 2+2?"),
		},
	)
	checkpointID, err := checkpointer.Save(ctx, checkpoint)
	require.NoError(t, err)

	sessionID := uuid.New()

	// Test with multiple answers to verify map support
	answers := map[string]types.Answer{
		"question_1": types.NewAnswer("Q1 2024"),
		"question_2": types.NewAnswer("revenue"),
		"question_3": types.NewAnswer("increase"),
	}

	// Execute
	gen, err := service.ResumeWithAnswer(ctx, sessionID, checkpointID, answers)
	require.NoError(t, err)
	require.NotNil(t, gen)
	defer gen.Close()

	// Collect all events
	var events []agents.ExecutorEvent
	for {
		event, err := gen.Next(ctx)
		if errors.Is(err, types.ErrGeneratorDone) {
			break
		}
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		events = append(events, event)
	}

	// Verify we got events
	assert.NotEmpty(t, events)

	hasContent := false
	var done *agents.ExecutorEvent
	for _, event := range events {
		if event.Type == agents.EventTypeContent {
			hasContent = true
		}
		if event.Type == agents.EventTypeDone {
			e := event
			done = &e
		}
	}

	assert.True(t, hasContent, "Expected content events")
	require.NotNil(t, done, "Expected done event")
	require.NotNil(t, done.Usage)
	assert.Equal(t, 10, done.Usage.PromptTokens)
	assert.Equal(t, 20, done.Usage.CompletionTokens)
	assert.Equal(t, 30, done.Usage.TotalTokens)
}

func TestResumeWithAnswer_EmptyCheckpointID(t *testing.T) {
	t.Parallel()

	// Setup
	agent := newMockAgent()
	model := newMockModel()
	renderer := &spyRenderer{}
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
		EventBus:     hooks.NewEventBus(),
		ChatRepo:     chatRepo,
	})

	// Create context with tenant ID
	ctx := context.Background()
	tenantID := uuid.New()
	ctx = composables.WithTenantID(ctx, tenantID)

	sessionID := uuid.New()
	answers := map[string]types.Answer{"q1": types.NewAnswer("answer")}

	// Execute with empty checkpoint ID
	gen, err := service.ResumeWithAnswer(ctx, sessionID, "", answers)

	// Should fail with validation error
	require.Error(t, err)
	assert.Nil(t, gen)
}

func TestResumeWithAnswer_MissingTenantID(t *testing.T) {
	t.Parallel()

	// Setup
	agent := newMockAgent()
	model := newMockModel()
	renderer := &spyRenderer{}
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
		EventBus:     hooks.NewEventBus(),
		ChatRepo:     chatRepo,
	})

	// Create context WITHOUT tenant ID
	ctx := context.Background()
	sessionID := uuid.New()
	checkpointID := "test-checkpoint"
	answers := map[string]types.Answer{"q1": types.NewAnswer("answer")}

	// Execute
	gen, err := service.ResumeWithAnswer(ctx, sessionID, checkpointID, answers)

	// Should fail without tenant ID
	require.Error(t, err)
	assert.Nil(t, gen)
}

