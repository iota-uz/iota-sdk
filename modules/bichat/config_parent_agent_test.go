package bichat

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/google/uuid"
	bichatmoduleagents "github.com/iota-uz/iota-sdk/modules/bichat/agents"
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

func (m *configTestChatRepository) UpdateSessionTitle(ctx context.Context, id uuid.UUID, title string) error {
	return nil
}

func (m *configTestChatRepository) UpdateSessionTitleIfEmpty(ctx context.Context, id uuid.UUID, title string) (bool, error) {
	return true, nil
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
		WithCapabilities(Capabilities{CodeInterpreter: true}),
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
	baseOpts := make([]ConfigOption, 0, 3+len(opts))
	baseOpts = append(baseOpts,
		WithQueryExecutor(&configTestExecutor{}),
		WithAttachmentStorageMode(AttachmentStorageModeNoOp),
	)
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

func TestModuleConfig_Validate_InvalidCodeInterpreterMemoryLimit(t *testing.T) {
	t.Parallel()

	cfg := NewModuleConfig(
		func(ctx context.Context) uuid.UUID { return uuid.New() },
		func(ctx context.Context) int64 { return 42 },
		&configTestChatRepository{},
		&configTestModel{},
		DefaultContextPolicy(),
		nil,
		WithAttachmentStorageMode(AttachmentStorageModeNoOp),
		WithQueryExecutor(&configTestExecutor{}),
		WithCodeInterpreterMemoryLimit("2g"),
	)

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CodeInterpreterMemoryLimit")
}

func TestModuleConfig_BuildServices_LoadsSkillsCatalog(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestSkillFile(t, root, "analytics/sql-debug", `---
name: SQL Debugging
description: Recover from SQL errors
when_to_use:
  - sql error
tags:
  - sql
---
Use schema tools before retrying.
`)

	cfg := newConfigWithProjectPromptOpts(
		WithSkillsDir(root),
	)

	err := cfg.BuildServices()
	require.NoError(t, err)
	require.NotNil(t, cfg.skillsCatalog)
	assert.Len(t, cfg.skillsCatalog.Skills, 1)
	require.NotNil(t, cfg.skillsSelector)
}

func TestModuleConfig_BuildServices_InvalidSkillsCatalogFails(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestSkillFile(t, root, "analytics/sql-debug", `---
name: Broken
description: Missing required fields
---
Body
`)

	cfg := newConfigWithProjectPromptOpts(
		WithSkillsDir(root),
	)

	err := cfg.BuildServices()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load skills catalog")
}

func TestModuleConfig_BuildServices_LoadsMarkdownSubAgents(t *testing.T) {
	t.Parallel()

	definitionFS := fstest.MapFS{
		"defs/sql.md": &fstest.MapFile{Data: []byte(`---
name: sql-analyst
description: SQL specialist
model: gpt-from-file
tools:
  - schema_list
  - schema_describe
  - sql_execute
---
SQL prompt`)},
		"defs/excel.md": &fstest.MapFile{Data: []byte(`---
name: excel-analyst
description: Spreadsheet specialist
model: gpt-from-file
tools:
  - artifact_reader
  - ask_user_question
---
Excel prompt`)},
	}

	cfg := newConfigWithProjectPromptOpts(
		WithCapabilities(Capabilities{MultiAgent: true}),
		WithSubAgentDefinitionsSource(definitionFS, "defs"),
	)

	err := cfg.BuildServices()
	require.NoError(t, err)
	require.NotNil(t, cfg.AgentRegistry)

	sqlAgent, exists := cfg.AgentRegistry.Get("sql-analyst")
	require.True(t, exists)
	assert.Equal(t, "gpt-test", sqlAgent.Metadata().Model, "runtime model should override front matter model")

	excelAgent, exists := cfg.AgentRegistry.Get("excel-analyst")
	require.True(t, exists)
	assert.Equal(t, "gpt-test", excelAgent.Metadata().Model)
}

func TestModuleConfig_BuildServices_InvalidSubAgentDefinitionFails(t *testing.T) {
	t.Parallel()

	definitionFS := fstest.MapFS{
		"defs/sql.md": &fstest.MapFile{Data: []byte(`---
name: sql-analyst
description: SQL specialist
model: gpt-from-file
when_to_use: should-fail
tools:
  - schema_list
---
SQL prompt`)},
	}

	cfg := newConfigWithProjectPromptOpts(
		WithCapabilities(Capabilities{MultiAgent: true}),
		WithSubAgentDefinitionsSource(definitionFS, "defs"),
	)

	err := cfg.BuildServices()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to setup configured sub-agents")
}

func TestModuleConfig_BuildServices_UnresolvedSubAgentToolFails(t *testing.T) {
	t.Parallel()

	definitionFS := fstest.MapFS{
		"defs/sql.md": &fstest.MapFile{Data: []byte(`---
name: sql-analyst
description: SQL specialist
model: gpt-from-file
tools:
  - made_up_tool
---
SQL prompt`)},
	}

	cfg := newConfigWithProjectPromptOpts(
		WithCapabilities(Capabilities{MultiAgent: true}),
		WithSubAgentDefinitionsSource(definitionFS, "defs"),
	)

	err := cfg.BuildServices()
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown tool "made_up_tool"`)
}

func TestModuleConfig_BuildServices_DuplicateCustomSubAgentFails(t *testing.T) {
	t.Parallel()

	definitionFS := fstest.MapFS{
		"defs/sql.md": &fstest.MapFile{Data: []byte(`---
name: sql-analyst
description: SQL specialist
model: gpt-from-file
tools:
  - schema_list
  - schema_describe
  - sql_execute
---
SQL prompt`)},
	}

	custom := agents.NewBaseAgent(
		agents.WithName("sql-analyst"),
		agents.WithDescription("custom"),
		agents.WithWhenToUse("custom"),
		agents.WithModel("gpt-custom"),
		agents.WithSystemPrompt("custom prompt"),
		agents.WithTerminationTools(agents.ToolFinalAnswer),
	)

	cfg := newConfigWithProjectPromptOpts(
		WithCapabilities(Capabilities{MultiAgent: true}),
		WithSubAgentDefinitionsSource(definitionFS, "defs"),
		WithSubAgents(custom),
	)

	err := cfg.BuildServices()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestModuleConfig_WithSubAgentDefinitionsSource_UsesDefaultWhenNotSet(t *testing.T) {
	t.Parallel()

	cfg := newConfigWithProjectPromptOpts(
		WithCapabilities(Capabilities{MultiAgent: true}),
	)
	require.NotNil(t, cfg.SubAgentDefinitionsFS)
	assert.Equal(t, bichatmoduleagents.DefaultSubAgentDefinitionsBasePath, cfg.SubAgentDefinitionsBasePath)
}

func writeTestSkillFile(t *testing.T, root, relDir, content string) {
	t.Helper()
	dir := filepath.Join(root, relDir)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644))
}
