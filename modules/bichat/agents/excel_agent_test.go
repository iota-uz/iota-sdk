package agents

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockChatRepoForExcel struct{}

func (m *mockChatRepoForExcel) CreateSession(ctx context.Context, session domain.Session) error {
	return nil
}

func (m *mockChatRepoForExcel) GetSession(ctx context.Context, id uuid.UUID) (domain.Session, error) {
	return nil, errors.New("not found")
}

func (m *mockChatRepoForExcel) UpdateSession(ctx context.Context, session domain.Session) error {
	return nil
}

func (m *mockChatRepoForExcel) ListUserSessions(ctx context.Context, userID int64, opts domain.ListOptions) ([]domain.Session, error) {
	return nil, nil
}

func (m *mockChatRepoForExcel) CountUserSessions(ctx context.Context, userID int64, opts domain.ListOptions) (int, error) {
	return 0, nil
}

func (m *mockChatRepoForExcel) DeleteSession(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *mockChatRepoForExcel) SaveMessage(ctx context.Context, msg types.Message) error {
	return nil
}

func (m *mockChatRepoForExcel) GetMessage(ctx context.Context, id uuid.UUID) (types.Message, error) {
	return nil, errors.New("not found")
}

func (m *mockChatRepoForExcel) GetSessionMessages(ctx context.Context, sessionID uuid.UUID, opts domain.ListOptions) ([]types.Message, error) {
	return nil, nil
}

func (m *mockChatRepoForExcel) TruncateMessagesFrom(ctx context.Context, sessionID uuid.UUID, from time.Time) (int64, error) {
	return 0, nil
}

func (m *mockChatRepoForExcel) UpdateMessageQuestionData(ctx context.Context, msgID uuid.UUID, qd *types.QuestionData) error {
	return nil
}

func (m *mockChatRepoForExcel) GetPendingQuestionMessage(ctx context.Context, sessionID uuid.UUID) (types.Message, error) {
	return nil, errors.New("no pending question")
}

func (m *mockChatRepoForExcel) SaveAttachment(ctx context.Context, attachment domain.Attachment) error {
	return nil
}

func (m *mockChatRepoForExcel) GetAttachment(ctx context.Context, id uuid.UUID) (domain.Attachment, error) {
	return nil, errors.New("not found")
}

func (m *mockChatRepoForExcel) GetMessageAttachments(ctx context.Context, messageID uuid.UUID) ([]domain.Attachment, error) {
	return nil, nil
}

func (m *mockChatRepoForExcel) DeleteAttachment(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *mockChatRepoForExcel) SaveArtifact(ctx context.Context, artifact domain.Artifact) error {
	return nil
}

func (m *mockChatRepoForExcel) GetArtifact(ctx context.Context, id uuid.UUID) (domain.Artifact, error) {
	return nil, errors.New("not found")
}

func (m *mockChatRepoForExcel) GetSessionArtifacts(ctx context.Context, sessionID uuid.UUID, opts domain.ListOptions) ([]domain.Artifact, error) {
	return nil, nil
}

func (m *mockChatRepoForExcel) DeleteSessionArtifacts(ctx context.Context, sessionID uuid.UUID) (int64, error) {
	return 0, nil
}

func (m *mockChatRepoForExcel) DeleteArtifact(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *mockChatRepoForExcel) UpdateArtifact(ctx context.Context, id uuid.UUID, name, description string) error {
	return nil
}

func TestNewExcelAgent(t *testing.T) {
	t.Parallel()

	agent, err := NewExcelAgent(&mockChatRepoForExcel{}, &mockFileStorage{})
	require.NoError(t, err)
	require.NotNil(t, agent)

	metadata := agent.Metadata()
	assert.Equal(t, "excel-analyst", metadata.Name)
	assert.Equal(t, "Specialized agent for spreadsheet attachments and large attachment-driven analysis", metadata.Description)
	assert.Contains(t, metadata.WhenToUse, "large spreadsheets")
	assert.Equal(t, "gpt-5.2-2025-12-11", metadata.Model)
	assert.Equal(t, []string{agents.ToolFinalAnswer}, metadata.TerminationTools)
}

func TestNewExcelAgent_WithModel(t *testing.T) {
	t.Parallel()

	agent, err := NewExcelAgent(&mockChatRepoForExcel{}, &mockFileStorage{},
		WithExcelAgentModel("gpt-3.5-turbo"),
	)
	require.NoError(t, err)

	assert.Equal(t, "gpt-3.5-turbo", agent.Metadata().Model)
}

func TestNewExcelAgent_NilRepo(t *testing.T) {
	t.Parallel()

	agent, err := NewExcelAgent(nil, &mockFileStorage{})
	require.Error(t, err)
	require.Nil(t, agent)
	assert.Contains(t, err.Error(), "chat repository is required")
}

func TestNewExcelAgent_NilStorage(t *testing.T) {
	t.Parallel()

	agent, err := NewExcelAgent(&mockChatRepoForExcel{}, nil)
	require.Error(t, err)
	require.Nil(t, agent)
	assert.Contains(t, err.Error(), "file storage is required")
}

func TestExcelAgent_ToolSetup(t *testing.T) {
	t.Parallel()

	agent, err := NewExcelAgent(&mockChatRepoForExcel{}, &mockFileStorage{})
	require.NoError(t, err)

	toolNames := make(map[string]bool)
	for _, tool := range agent.Tools() {
		toolNames[tool.Name()] = true
	}

	assert.True(t, toolNames["artifact_reader"])
	assert.True(t, toolNames["ask_user_question"])
	assert.False(t, toolNames["schema_list"])
}

func TestExcelAgent_WithArtifactReaderTool(t *testing.T) {
	t.Parallel()

	customTool := agents.NewTool(
		"artifact_reader",
		"Read artifacts",
		map[string]any{"type": "object"},
		func(ctx context.Context, input string) (string, error) { return "ok", nil },
	)

	agent, err := NewExcelAgent(&mockChatRepoForExcel{}, &mockFileStorage{},
		WithExcelAgentArtifactReaderTool(customTool),
	)
	require.NoError(t, err)

	assert.Contains(t, agent.SystemPrompt(context.Background()), "ATTACHMENT-FIRST GUIDELINES")
	toolNames := make(map[string]bool)
	for _, tool := range agent.Tools() {
		toolNames[tool.Name()] = true
	}
	assert.True(t, toolNames["artifact_reader"])
}

func TestExcelAgent_InterfaceCompliance(t *testing.T) {
	t.Parallel()

	agent, err := NewExcelAgent(&mockChatRepoForExcel{}, &mockFileStorage{})
	require.NoError(t, err)

	var _ agents.ExtendedAgent = agent
	var _ agents.Agent = agent
}
