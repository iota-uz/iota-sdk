package agents

import (
	"context"
	"errors"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/google/uuid"
	coreagents "github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockSQLExecutor struct {
	executeQueryFn func(ctx context.Context, sql string, params []any, timeout time.Duration) (*bichatsql.QueryResult, error)
}

func (m *mockSQLExecutor) ExecuteQuery(ctx context.Context, sql string, params []any, timeout time.Duration) (*bichatsql.QueryResult, error) {
	if m.executeQueryFn != nil {
		return m.executeQueryFn(ctx, sql, params, timeout)
	}
	return &bichatsql.QueryResult{Columns: []string{}, Rows: [][]any{}, RowCount: 0}, nil
}

type mockChatRepoForDefinitions struct{}

func (m *mockChatRepoForDefinitions) CreateSession(ctx context.Context, session domain.Session) error {
	return nil
}

func (m *mockChatRepoForDefinitions) GetSession(ctx context.Context, id uuid.UUID) (domain.Session, error) {
	return nil, errors.New("not found")
}

func (m *mockChatRepoForDefinitions) UpdateSession(ctx context.Context, session domain.Session) error {
	return nil
}

func (m *mockChatRepoForDefinitions) UpdateSessionTitle(ctx context.Context, id uuid.UUID, title string) error {
	return nil
}

func (m *mockChatRepoForDefinitions) UpdateSessionTitleIfEmpty(ctx context.Context, id uuid.UUID, title string) (bool, error) {
	return true, nil
}

func (m *mockChatRepoForDefinitions) ListUserSessions(ctx context.Context, userID int64, opts domain.ListOptions) ([]domain.Session, error) {
	return nil, nil
}

func (m *mockChatRepoForDefinitions) CountUserSessions(ctx context.Context, userID int64, opts domain.ListOptions) (int, error) {
	return 0, nil
}

func (m *mockChatRepoForDefinitions) DeleteSession(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *mockChatRepoForDefinitions) SaveMessage(ctx context.Context, msg types.Message) error {
	return nil
}

func (m *mockChatRepoForDefinitions) GetMessage(ctx context.Context, id uuid.UUID) (types.Message, error) {
	return nil, errors.New("not found")
}

func (m *mockChatRepoForDefinitions) GetSessionMessages(ctx context.Context, sessionID uuid.UUID, opts domain.ListOptions) ([]types.Message, error) {
	return nil, nil
}

func (m *mockChatRepoForDefinitions) TruncateMessagesFrom(ctx context.Context, sessionID uuid.UUID, from time.Time) (int64, error) {
	return 0, nil
}

func (m *mockChatRepoForDefinitions) UpdateMessageQuestionData(ctx context.Context, msgID uuid.UUID, qd *types.QuestionData) error {
	return nil
}

func (m *mockChatRepoForDefinitions) GetPendingQuestionMessage(ctx context.Context, sessionID uuid.UUID) (types.Message, error) {
	return nil, errors.New("no pending question")
}

func (m *mockChatRepoForDefinitions) SaveAttachment(ctx context.Context, attachment domain.Attachment) error {
	return nil
}

func (m *mockChatRepoForDefinitions) GetAttachment(ctx context.Context, id uuid.UUID) (domain.Attachment, error) {
	return nil, errors.New("not found")
}

func (m *mockChatRepoForDefinitions) GetMessageAttachments(ctx context.Context, messageID uuid.UUID) ([]domain.Attachment, error) {
	return nil, nil
}

func (m *mockChatRepoForDefinitions) DeleteAttachment(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *mockChatRepoForDefinitions) SaveArtifact(ctx context.Context, artifact domain.Artifact) error {
	return nil
}

func (m *mockChatRepoForDefinitions) GetArtifact(ctx context.Context, id uuid.UUID) (domain.Artifact, error) {
	return nil, errors.New("not found")
}

func (m *mockChatRepoForDefinitions) GetSessionArtifacts(ctx context.Context, sessionID uuid.UUID, opts domain.ListOptions) ([]domain.Artifact, error) {
	return nil, nil
}

func (m *mockChatRepoForDefinitions) DeleteSessionArtifacts(ctx context.Context, sessionID uuid.UUID) (int64, error) {
	return 0, nil
}

func (m *mockChatRepoForDefinitions) DeleteArtifact(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *mockChatRepoForDefinitions) UpdateArtifact(ctx context.Context, id uuid.UUID, name, description string) error {
	return nil
}

func findDefinitionByName(t *testing.T, defs []SubAgentDefinition, name string) SubAgentDefinition {
	t.Helper()
	for _, def := range defs {
		if def.Name == name {
			return def
		}
	}
	t.Fatalf("definition %q not found", name)
	return SubAgentDefinition{}
}

func TestParseSubAgentDefinition_Valid(t *testing.T) {
	t.Parallel()

	content := `---
name: sql-analyst
description: SQL specialist
model: gpt-5
tools:
  - schema_list
  - sql_execute
---
System prompt body`

	def, err := ParseSubAgentDefinition(content, "sql.md")
	require.NoError(t, err)
	assert.Equal(t, "sql-analyst", def.Name)
	assert.Equal(t, "SQL specialist", def.Description)
	assert.Equal(t, "gpt-5", def.Model)
	assert.Equal(t, []string{"schema_list", "sql_execute"}, def.Tools)
	assert.Equal(t, "System prompt body", def.SystemPrompt)
}

func TestParseSubAgentDefinition_StrictCanonicalFields(t *testing.T) {
	t.Parallel()

	content := `---
name: sql-analyst
description: SQL specialist
model: gpt-5
when_to_use: should-fail
tools:
  - schema_list
---
System prompt body`

	_, err := ParseSubAgentDefinition(content, "sql.md")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "field when_to_use not found")
}

func TestLoadSubAgentDefinitions_DefaultEmbedded(t *testing.T) {
	t.Parallel()

	defs, err := LoadSubAgentDefinitions(DefaultSubAgentDefinitionsFS(), DefaultSubAgentDefinitionsBasePath)
	require.NoError(t, err)
	require.Len(t, defs, 2)

	sqlDef := findDefinitionByName(t, defs, "sql-analyst")
	excelDef := findDefinitionByName(t, defs, "excel-analyst")

	assert.Equal(t, "Specialized agent for SQL query generation and database analysis", sqlDef.Description)
	assert.Equal(t, []string{"schema_list", "schema_describe", "sql_execute"}, sqlDef.Tools)
	assert.NotEmpty(t, sqlDef.SystemPrompt)

	assert.Equal(t, "Specialized agent for spreadsheet attachments and large attachment-driven analysis", excelDef.Description)
	assert.Equal(t, []string{"artifact_reader", "ask_user_question"}, excelDef.Tools)
	assert.NotEmpty(t, excelDef.SystemPrompt)
}

func TestLoadSubAgentDefinitions_EmptyDirectoryFails(t *testing.T) {
	t.Parallel()

	_, err := LoadSubAgentDefinitions(fstest.MapFS{}, ".")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no sub-agent definition markdown files found")
}

func TestLoadSubAgentDefinitions_DuplicateNamesFail(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"defs/a.md": &fstest.MapFile{Data: []byte(`---
name: duplicate
description: one
model: gpt
tools: [ask_user_question]
---
a`)},
		"defs/b.md": &fstest.MapFile{Data: []byte(`---
name: duplicate
description: two
model: gpt
tools: [ask_user_question]
---
b`)},
	}

	_, err := LoadSubAgentDefinitions(fsys, "defs")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate sub-agent name")
}

func TestBuildSubAgent_SQLDefinition(t *testing.T) {
	t.Parallel()

	executor := &mockSQLExecutor{}
	defs, err := LoadSubAgentDefinitions(DefaultSubAgentDefinitionsFS(), DefaultSubAgentDefinitionsBasePath)
	require.NoError(t, err)
	sqlDef := findDefinitionByName(t, defs, "sql-analyst")

	agent, err := BuildSubAgent(sqlDef, SubAgentDependencies{QueryExecutor: executor})
	require.NoError(t, err)

	metadata := agent.Metadata()
	assert.Equal(t, "sql-analyst", metadata.Name)
	assert.Equal(t, sqlDef.Description, metadata.Description)
	assert.Equal(t, sqlDef.Description, metadata.WhenToUse)
	assert.Equal(t, "gpt-5.2", metadata.Model)
	assert.Equal(t, []string{coreagents.ToolFinalAnswer}, metadata.TerminationTools)

	toolNames := make(map[string]bool)
	for _, tool := range agent.Tools() {
		toolNames[tool.Name()] = true
	}
	assert.True(t, toolNames["schema_list"])
	assert.True(t, toolNames["schema_describe"])
	assert.True(t, toolNames["sql_execute"])
	assert.False(t, toolNames["ask_user_question"])
}

func TestBuildSubAgent_SQLDefinitionWithModelOverride(t *testing.T) {
	t.Parallel()

	executor := &mockSQLExecutor{}
	defs, err := LoadSubAgentDefinitions(DefaultSubAgentDefinitionsFS(), DefaultSubAgentDefinitionsBasePath)
	require.NoError(t, err)
	sqlDef := findDefinitionByName(t, defs, "sql-analyst")

	agent, err := BuildSubAgent(sqlDef, SubAgentDependencies{QueryExecutor: executor}, WithSubAgentModel("gpt-5.2"))
	require.NoError(t, err)
	assert.Equal(t, "gpt-5.2", agent.Metadata().Model)
}

func TestBuildSubAgent_ExcelDefinition(t *testing.T) {
	t.Parallel()

	defs, err := LoadSubAgentDefinitions(DefaultSubAgentDefinitionsFS(), DefaultSubAgentDefinitionsBasePath)
	require.NoError(t, err)
	excelDef := findDefinitionByName(t, defs, "excel-analyst")

	agent, err := BuildSubAgent(excelDef, SubAgentDependencies{
		ChatRepository: &mockChatRepoForDefinitions{},
		FileStorage:    &mockFileStorage{},
	})
	require.NoError(t, err)

	toolNames := make(map[string]bool)
	for _, tool := range agent.Tools() {
		toolNames[tool.Name()] = true
	}
	assert.True(t, toolNames["artifact_reader"])
	assert.True(t, toolNames["ask_user_question"])
	assert.False(t, toolNames["schema_list"])
}

func TestBuildSubAgent_UnknownToolFails(t *testing.T) {
	t.Parallel()

	def := SubAgentDefinition{
		Name:         "x",
		Description:  "x",
		Model:        "gpt-5",
		Tools:        []string{"unknown_tool"},
		SystemPrompt: "prompt",
	}

	_, err := BuildSubAgent(def, SubAgentDependencies{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown tool "unknown_tool"`)
}

func TestBuildSubAgent_MissingDependenciesFail(t *testing.T) {
	t.Parallel()

	defs, err := LoadSubAgentDefinitions(DefaultSubAgentDefinitionsFS(), DefaultSubAgentDefinitionsBasePath)
	require.NoError(t, err)

	sqlDef := findDefinitionByName(t, defs, "sql-analyst")
	_, err = BuildSubAgent(sqlDef, SubAgentDependencies{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `tool "schema_list" requires query executor`)

	excelDef := findDefinitionByName(t, defs, "excel-analyst")
	_, err = BuildSubAgent(excelDef, SubAgentDependencies{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `tool "artifact_reader" requires chat repository`)
}

func TestBuildSubAgent_SQLToolRouting(t *testing.T) {
	t.Parallel()

	executor := &mockSQLExecutor{
		executeQueryFn: func(ctx context.Context, sql string, params []any, timeout time.Duration) (*bichatsql.QueryResult, error) {
			if strings.Contains(sql, "FROM pg_class") || strings.Contains(sql, "pg_namespace") {
				return &bichatsql.QueryResult{
					Columns:  []string{"schema", "name", "approximate_row_count"},
					Rows:     [][]any{{"analytics", "test_table", int64(123)}},
					RowCount: 1,
				}, nil
			}
			if strings.Contains(sql, "pg_catalog.pg_views") {
				return &bichatsql.QueryResult{
					Columns:  []string{"schema", "name", "type"},
					Rows:     [][]any{{"analytics", "test_view", "view"}},
					RowCount: 1,
				}, nil
			}
			if strings.Contains(sql, "information_schema.columns") {
				return &bichatsql.QueryResult{
					Columns:  []string{"column_name", "data_type", "is_nullable", "column_default", "character_maximum_length", "numeric_precision", "numeric_scale"},
					Rows:     [][]any{{"id", "integer", "NO", nil, nil, nil, nil}},
					RowCount: 1,
				}, nil
			}
			return &bichatsql.QueryResult{
				Columns:  []string{"id", "name"},
				Rows:     [][]any{{1, "test"}},
				RowCount: 1,
			}, nil
		},
	}

	defs, err := LoadSubAgentDefinitions(DefaultSubAgentDefinitionsFS(), DefaultSubAgentDefinitionsBasePath)
	require.NoError(t, err)
	sqlDef := findDefinitionByName(t, defs, "sql-analyst")

	agent, err := BuildSubAgent(sqlDef, SubAgentDependencies{QueryExecutor: executor})
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("schema_list", func(t *testing.T) {
		result, err := agent.OnToolCall(ctx, "schema_list", `{}`)
		require.NoError(t, err)
		assert.NotEmpty(t, result)
	})

	t.Run("sql_execute", func(t *testing.T) {
		result, err := agent.OnToolCall(ctx, "sql_execute", `{"query":"SELECT * FROM users LIMIT 10"}`)
		require.NoError(t, err)
		assert.NotEmpty(t, result)
	})

	t.Run("unknown tool", func(t *testing.T) {
		_, err := agent.OnToolCall(ctx, "unknown_tool", `{}`)
		require.Error(t, err)
	})
}

func TestDefinitionsBuildAndRegisterInRegistry(t *testing.T) {
	t.Parallel()

	defs, err := LoadSubAgentDefinitions(DefaultSubAgentDefinitionsFS(), DefaultSubAgentDefinitionsBasePath)
	require.NoError(t, err)

	registry := coreagents.NewAgentRegistry()
	for _, def := range defs {
		deps := SubAgentDependencies{}
		if def.Name == "sql-analyst" {
			deps.QueryExecutor = &mockSQLExecutor{}
		}
		if def.Name == "excel-analyst" {
			deps.ChatRepository = &mockChatRepoForDefinitions{}
			deps.FileStorage = &mockFileStorage{}
		}
		agent, err := BuildSubAgent(def, deps)
		require.NoError(t, err)
		require.NoError(t, registry.Register(agent))
	}

	description := registry.Describe()
	assert.Contains(t, description, "## sql-analyst")
	assert.Contains(t, description, "## excel-analyst")
}
