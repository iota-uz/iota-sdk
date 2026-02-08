package bichat

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/kb"
	"github.com/iota-uz/iota-sdk/pkg/bichat/learning"
	"github.com/iota-uz/iota-sdk/pkg/bichat/prompts"
	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type configTestExecutor struct{}

func (m *configTestExecutor) ExecuteQuery(ctx context.Context, sql string, params []any, timeout time.Duration) (*bichatsql.QueryResult, error) {
	return &bichatsql.QueryResult{
		Columns:  []string{"ok"},
		Rows:     [][]any{{1}},
		RowCount: 1,
	}, nil
}

type configTestModel struct{}

func (m *configTestModel) Generate(ctx context.Context, req agents.Request, opts ...agents.GenerateOption) (*agents.Response, error) {
	return &agents.Response{
		Message: types.AssistantMessage("ok"),
	}, nil
}

func (m *configTestModel) Stream(ctx context.Context, req agents.Request, opts ...agents.GenerateOption) (types.Generator[agents.Chunk], error) {
	return types.NewGenerator(ctx, func(ctx context.Context, yield func(agents.Chunk) bool) error {
		return nil
	}), nil
}

func (m *configTestModel) Info() agents.ModelInfo {
	return agents.ModelInfo{Name: "gpt-test", Provider: "test"}
}

func (m *configTestModel) HasCapability(capability agents.Capability) bool {
	return false
}

func (m *configTestModel) Pricing() agents.ModelPricing {
	return agents.ModelPricing{}
}

type configTestLearningStore struct{}

func (m *configTestLearningStore) Save(ctx context.Context, learning learning.Learning) error {
	return nil
}

func (m *configTestLearningStore) Search(ctx context.Context, query string, opts learning.SearchOpts) ([]learning.Learning, error) {
	return []learning.Learning{}, nil
}

func (m *configTestLearningStore) IncrementUsage(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *configTestLearningStore) Delete(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *configTestLearningStore) ListByTable(ctx context.Context, tenantID uuid.UUID, tableName string, limit int) ([]learning.Learning, error) {
	return []learning.Learning{}, nil
}

type configTestValidatedStore struct{}

func (m *configTestValidatedStore) Save(ctx context.Context, query learning.ValidatedQuery) error {
	return nil
}

func (m *configTestValidatedStore) Search(ctx context.Context, question string, opts learning.ValidatedQuerySearchOpts) ([]learning.ValidatedQuery, error) {
	return []learning.ValidatedQuery{}, nil
}

func (m *configTestValidatedStore) IncrementUsage(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *configTestValidatedStore) Delete(ctx context.Context, id uuid.UUID) error {
	return nil
}

type configTestKBSearcher struct{}

func (m *configTestKBSearcher) Search(ctx context.Context, query string, opts kb.SearchOptions) ([]kb.SearchResult, error) {
	return []kb.SearchResult{}, nil
}

var errDocumentNotFound = errors.New("document not found")

func (m *configTestKBSearcher) GetDocument(ctx context.Context, id string) (*kb.Document, error) {
	return nil, errDocumentNotFound
}

func (m *configTestKBSearcher) IsAvailable() bool {
	return true
}

type configTestChatRepository struct{}

func (m *configTestChatRepository) CreateSession(ctx context.Context, session domain.Session) error {
	return nil
}

func (m *configTestChatRepository) GetSession(ctx context.Context, id uuid.UUID) (domain.Session, error) {
	return nil, errors.New("session not found")
}

func (m *configTestChatRepository) UpdateSession(ctx context.Context, session domain.Session) error {
	return nil
}

func (m *configTestChatRepository) ListUserSessions(ctx context.Context, userID int64, opts domain.ListOptions) ([]domain.Session, error) {
	return []domain.Session{}, nil
}

func (m *configTestChatRepository) CountUserSessions(ctx context.Context, userID int64, opts domain.ListOptions) (int, error) {
	return 0, nil
}

func (m *configTestChatRepository) DeleteSession(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *configTestChatRepository) SaveMessage(ctx context.Context, msg types.Message) error {
	return nil
}

func (m *configTestChatRepository) GetMessage(ctx context.Context, id uuid.UUID) (types.Message, error) {
	return nil, errors.New("message not found")
}

func (m *configTestChatRepository) GetSessionMessages(ctx context.Context, sessionID uuid.UUID, opts domain.ListOptions) ([]types.Message, error) {
	return []types.Message{}, nil
}

func (m *configTestChatRepository) TruncateMessagesFrom(ctx context.Context, sessionID uuid.UUID, from time.Time) (int64, error) {
	return 0, nil
}

func (m *configTestChatRepository) SaveAttachment(ctx context.Context, attachment domain.Attachment) error {
	return nil
}

func (m *configTestChatRepository) GetAttachment(ctx context.Context, id uuid.UUID) (domain.Attachment, error) {
	return nil, errors.New("attachment not found")
}

func (m *configTestChatRepository) GetMessageAttachments(ctx context.Context, messageID uuid.UUID) ([]domain.Attachment, error) {
	return []domain.Attachment{}, nil
}

func (m *configTestChatRepository) DeleteAttachment(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *configTestChatRepository) SaveArtifact(ctx context.Context, artifact domain.Artifact) error {
	return nil
}

func (m *configTestChatRepository) GetArtifact(ctx context.Context, id uuid.UUID) (domain.Artifact, error) {
	return nil, errors.New("artifact not found")
}

func (m *configTestChatRepository) GetSessionArtifacts(ctx context.Context, sessionID uuid.UUID, opts domain.ListOptions) ([]domain.Artifact, error) {
	return []domain.Artifact{}, nil
}

func (m *configTestChatRepository) DeleteSessionArtifacts(ctx context.Context, sessionID uuid.UUID) (int64, error) {
	return 0, nil
}

func (m *configTestChatRepository) DeleteArtifact(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *configTestChatRepository) UpdateArtifact(ctx context.Context, id uuid.UUID, name, description string) error {
	return nil
}

func (m *configTestChatRepository) UpdateMessageQuestionData(ctx context.Context, msgID uuid.UUID, qd *types.QuestionData) error {
	return nil
}

func (m *configTestChatRepository) GetPendingQuestionMessage(ctx context.Context, sessionID uuid.UUID) (types.Message, error) {
	return nil, errors.New("no pending question")
}

func TestModuleConfig_BuildParentAgent_UsesConfiguredKnowledgeTools(t *testing.T) {
	t.Parallel()

	cfg := NewModuleConfig(
		func(ctx context.Context) uuid.UUID { return uuid.New() },
		func(ctx context.Context) int64 { return 42 },
		nil,
		&configTestModel{},
		DefaultContextPolicy(),
		nil,
		WithQueryExecutor(&configTestExecutor{}),
		WithKBSearcher(&configTestKBSearcher{}),
		WithLearningStore(&configTestLearningStore{}),
		WithValidatedQueryStore(&configTestValidatedStore{}),
		WithCodeInterpreter(true),
	)

	err := cfg.BuildParentAgent()
	require.NoError(t, err)
	require.NotNil(t, cfg.ParentAgent)

	toolNames := map[string]bool{}
	for _, tool := range cfg.ParentAgent.Tools() {
		toolNames[tool.Name()] = true
	}

	assert.True(t, toolNames["kb_search"])
	assert.True(t, toolNames["search_learnings"])
	assert.True(t, toolNames["save_learning"])
	assert.True(t, toolNames["search_validated_queries"])
	assert.True(t, toolNames["save_validated_query"])
	assert.True(t, toolNames["code_interpreter"])
	assert.Equal(t, "gpt-test", cfg.ParentAgent.Metadata().Model)
}

func newConfigWithProjectPromptOpts(opts ...ConfigOption) *ModuleConfig {
	baseOpts := []ConfigOption{
		WithQueryExecutor(&configTestExecutor{}),
		WithNoOpAttachmentStorage(),
		WithTitleGenerationDisabled(),
	}
	baseOpts = append(baseOpts, opts...)

	return NewModuleConfig(
		func(ctx context.Context) uuid.UUID { return uuid.New() },
		func(ctx context.Context) int64 { return 42 },
		&configTestChatRepository{},
		&configTestModel{},
		DefaultContextPolicy(),
		nil,
		baseOpts...,
	)
}

func TestModuleConfig_BuildServices_ResolvesStaticProjectPromptExtension(t *testing.T) {
	t.Parallel()

	cfg := newConfigWithProjectPromptOpts(
		WithProjectPromptExtension("  insurance bi domain  "),
	)

	err := cfg.BuildServices()
	require.NoError(t, err)
	assert.Equal(t, "insurance bi domain", cfg.resolvedProjectPromptExtension)
}

func TestModuleConfig_BuildServices_ResolvesProviderProjectPromptExtension(t *testing.T) {
	t.Parallel()

	cfg := newConfigWithProjectPromptOpts(
		WithProjectPromptExtension("fallback"),
		WithProjectPromptExtensionProvider(prompts.ProjectPromptExtensionProviderFunc(func() (string, error) {
			return "  provider extension  ", nil
		})),
	)

	err := cfg.BuildServices()
	require.NoError(t, err)
	assert.Equal(t, "provider extension", cfg.resolvedProjectPromptExtension)
}

func TestModuleConfig_BuildServices_ProjectPromptProviderErrorFails(t *testing.T) {
	t.Parallel()

	cfg := newConfigWithProjectPromptOpts(
		WithProjectPromptExtension("fallback"),
		WithProjectPromptExtensionProvider(prompts.ProjectPromptExtensionProviderFunc(func() (string, error) {
			return "", errors.New("provider failed")
		})),
	)

	err := cfg.BuildServices()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve project prompt extension")
}

func TestModuleConfig_BuildServices_EmptyProviderFallsBackToStaticPrompt(t *testing.T) {
	t.Parallel()

	cfg := newConfigWithProjectPromptOpts(
		WithProjectPromptExtension("  static extension  "),
		WithProjectPromptExtensionProvider(prompts.ProjectPromptExtensionProviderFunc(func() (string, error) {
			return "   ", nil
		})),
	)

	err := cfg.BuildServices()
	require.NoError(t, err)
	assert.Equal(t, "static extension", cfg.resolvedProjectPromptExtension)
}
