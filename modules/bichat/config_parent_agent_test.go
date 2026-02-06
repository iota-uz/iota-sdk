package bichat

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/kb"
	"github.com/iota-uz/iota-sdk/pkg/bichat/learning"
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

func (m *configTestKBSearcher) GetDocument(ctx context.Context, id string) (*kb.Document, error) {
	return nil, nil
}

func (m *configTestKBSearcher) IsAvailable() bool {
	return true
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
