package learning

import (
	tools "github.com/iota-uz/iota-sdk/pkg/bichat/tools"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/learning"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

// mockLearningStore implements learning.LearningStore for testing.
type mockLearningStore struct {
	searchFn         func(ctx context.Context, query string, opts learning.SearchOpts) ([]learning.Learning, error)
	saveFn           func(ctx context.Context, l learning.Learning) error
	incrementUsageFn func(ctx context.Context, id uuid.UUID) error
	deleteFn         func(ctx context.Context, id uuid.UUID) error
	listByTableFn    func(ctx context.Context, tenantID uuid.UUID, tableName string, limit int) ([]learning.Learning, error)
}

func (m *mockLearningStore) Search(ctx context.Context, query string, opts learning.SearchOpts) ([]learning.Learning, error) {
	if m.searchFn != nil {
		return m.searchFn(ctx, query, opts)
	}
	return nil, nil
}

func (m *mockLearningStore) Save(ctx context.Context, l learning.Learning) error {
	if m.saveFn != nil {
		return m.saveFn(ctx, l)
	}
	return nil
}

func (m *mockLearningStore) IncrementUsage(ctx context.Context, id uuid.UUID) error {
	if m.incrementUsageFn != nil {
		return m.incrementUsageFn(ctx, id)
	}
	return nil
}

func (m *mockLearningStore) Delete(ctx context.Context, id uuid.UUID) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return nil
}

func (m *mockLearningStore) ListByTable(ctx context.Context, tenantID uuid.UUID, tableName string, limit int) ([]learning.Learning, error) {
	if m.listByTableFn != nil {
		return m.listByTableFn(ctx, tenantID, tableName, limit)
	}
	return nil, nil
}

func TestSearchLearningsTool_ValidSearchWithResults(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	ctx := composables.WithTenantID(context.Background(), tenantID)

	store := &mockLearningStore{
		searchFn: func(ctx context.Context, query string, opts learning.SearchOpts) ([]learning.Learning, error) {
			return []learning.Learning{
				{
					ID:        uuid.New(),
					TenantID:  tenantID,
					Category:  learning.CategorySQLError,
					Trigger:   "column not found error",
					Lesson:    "use schema_describe first",
					TableName: "sales",
					UsedCount: 5,
				},
			}, nil
		},
	}

	tool := NewSearchLearningsTool(store)
	input := `{"query": "sales table error"}`
	result, err := tool.Call(ctx, input)

	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if !strings.Contains(result, "learnings") {
		t.Errorf("expected 'learnings' in result, got: %s", result)
	}
	if !strings.Contains(result, "sql_error") {
		t.Errorf("expected category in result, got: %s", result)
	}
	if !strings.Contains(result, "sales") {
		t.Errorf("expected table_name in result, got: %s", result)
	}
}

func TestSearchLearningsTool_NoResults(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	ctx := composables.WithTenantID(context.Background(), tenantID)

	store := &mockLearningStore{
		searchFn: func(ctx context.Context, query string, opts learning.SearchOpts) ([]learning.Learning, error) {
			return []learning.Learning{}, nil
		},
	}

	tool := NewSearchLearningsTool(store)
	input := `{"query": "nonexistent pattern"}`
	result, err := tool.Call(ctx, input)

	if err != nil {
		t.Errorf("expected nil error for no results, got: %v", err)
	}
	if !strings.Contains(result, string(tools.ErrCodeNoData)) {
		t.Errorf("expected tools.ErrCodeNoData in result, got: %s", result)
	}
	if !strings.Contains(result, "error") {
		t.Errorf("expected 'error' key in result, got: %s", result)
	}
}

func TestSearchLearningsTool_MissingQuery(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	ctx := composables.WithTenantID(context.Background(), tenantID)

	store := &mockLearningStore{}
	tool := NewSearchLearningsTool(store)
	input := `{}`
	result, err := tool.Call(ctx, input)

	if err != nil {
		t.Errorf("expected nil error for handled validation, got: %v", err)
	}
	if !strings.Contains(result, string(tools.ErrCodeInvalidRequest)) {
		t.Errorf("expected tools.ErrCodeInvalidRequest in result, got: %s", result)
	}
	if !strings.Contains(result, "query parameter is required") {
		t.Errorf("expected 'query parameter is required' message, got: %s", result)
	}
}

func TestSearchLearningsTool_StoreError(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	ctx := composables.WithTenantID(context.Background(), tenantID)

	store := &mockLearningStore{
		searchFn: func(ctx context.Context, query string, opts learning.SearchOpts) ([]learning.Learning, error) {
			return nil, errors.New("database connection lost")
		},
	}

	tool := NewSearchLearningsTool(store)
	input := `{"query": "sales error"}`
	result, err := tool.Call(ctx, input)

	// StructuredTool returns error in payload, not as Go error
	if err != nil {
		t.Errorf("expected nil error (store failure conveyed in payload), got: %v", err)
	}
	if !strings.Contains(result, string(tools.ErrCodeServiceUnavailable)) {
		t.Errorf("expected tools.ErrCodeServiceUnavailable in result, got: %s", result)
	}
}

func TestSearchLearningsTool_InvalidJSON(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	ctx := composables.WithTenantID(context.Background(), tenantID)

	store := &mockLearningStore{}
	tool := NewSearchLearningsTool(store)
	input := `{invalid json}`
	result, err := tool.Call(ctx, input)

	if err != nil {
		t.Errorf("expected nil error for handled parse failure, got: %v", err)
	}
	if !strings.Contains(result, string(tools.ErrCodeInvalidRequest)) {
		t.Errorf("expected tools.ErrCodeInvalidRequest in result, got: %s", result)
	}
	if !strings.Contains(result, "failed to parse input") {
		t.Errorf("expected parse error message, got: %s", result)
	}
}

func TestSaveLearningTool_ValidSave(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	ctx := composables.WithTenantID(context.Background(), tenantID)

	var savedLearning learning.Learning
	store := &mockLearningStore{
		saveFn: func(ctx context.Context, l learning.Learning) error {
			savedLearning = l
			return nil
		},
	}

	tool := NewSaveLearningTool(store)
	input := `{
		"category": "sql_error",
		"trigger": "column does not exist",
		"lesson": "verify columns with schema_describe"
	}`
	result, err := tool.Call(ctx, input)

	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if !strings.Contains(result, "id") {
		t.Errorf("expected 'id' in result, got: %s", result)
	}
	if !strings.Contains(result, "category") {
		t.Errorf("expected 'category' in result, got: %s", result)
	}
	if !strings.Contains(result, "message") {
		t.Errorf("expected 'message' in result, got: %s", result)
	}
	if savedLearning.Category != learning.CategorySQLError {
		t.Errorf("expected CategorySQLError, got: %s", savedLearning.Category)
	}
}

func TestSaveLearningTool_MissingCategory(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	ctx := composables.WithTenantID(context.Background(), tenantID)

	store := &mockLearningStore{}
	tool := NewSaveLearningTool(store)
	input := `{
		"trigger": "some trigger",
		"lesson": "some lesson"
	}`
	result, err := tool.Call(ctx, input)

	if err != nil {
		t.Errorf("expected nil error for handled validation, got: %v", err)
	}
	if !strings.Contains(result, string(tools.ErrCodeInvalidRequest)) {
		t.Errorf("expected tools.ErrCodeInvalidRequest in result, got: %s", result)
	}
	if !strings.Contains(result, "category parameter is required") {
		t.Errorf("expected category error message, got: %s", result)
	}
}

func TestSaveLearningTool_MissingTrigger(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	ctx := composables.WithTenantID(context.Background(), tenantID)

	store := &mockLearningStore{}
	tool := NewSaveLearningTool(store)
	input := `{
		"category": "sql_error",
		"lesson": "some lesson"
	}`
	result, err := tool.Call(ctx, input)

	if err != nil {
		t.Errorf("expected nil error for handled validation, got: %v", err)
	}
	if !strings.Contains(result, string(tools.ErrCodeInvalidRequest)) {
		t.Errorf("expected tools.ErrCodeInvalidRequest in result, got: %s", result)
	}
	if !strings.Contains(result, "trigger parameter is required") {
		t.Errorf("expected trigger error message, got: %s", result)
	}
}

func TestSaveLearningTool_MissingLesson(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	ctx := composables.WithTenantID(context.Background(), tenantID)

	store := &mockLearningStore{}
	tool := NewSaveLearningTool(store)
	input := `{
		"category": "sql_error",
		"trigger": "some trigger"
	}`
	result, err := tool.Call(ctx, input)

	if err != nil {
		t.Errorf("expected nil error for handled validation, got: %v", err)
	}
	if !strings.Contains(result, string(tools.ErrCodeInvalidRequest)) {
		t.Errorf("expected tools.ErrCodeInvalidRequest in result, got: %s", result)
	}
	if !strings.Contains(result, "lesson parameter is required") {
		t.Errorf("expected lesson error message, got: %s", result)
	}
}

func TestSaveLearningTool_InvalidCategory(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	ctx := composables.WithTenantID(context.Background(), tenantID)

	store := &mockLearningStore{}
	tool := NewSaveLearningTool(store)
	input := `{
		"category": "invalid_category",
		"trigger": "some trigger",
		"lesson": "some lesson"
	}`
	result, err := tool.Call(ctx, input)

	if err != nil {
		t.Errorf("expected nil error for handled validation, got: %v", err)
	}
	if !strings.Contains(result, string(tools.ErrCodeInvalidRequest)) {
		t.Errorf("expected tools.ErrCodeInvalidRequest in result, got: %s", result)
	}
	if !strings.Contains(result, "invalid category") {
		t.Errorf("expected invalid category message, got: %s", result)
	}
}

func TestSaveLearningTool_StoreError(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	ctx := composables.WithTenantID(context.Background(), tenantID)

	store := &mockLearningStore{
		saveFn: func(ctx context.Context, l learning.Learning) error {
			return errors.New("database write failed")
		},
	}

	tool := NewSaveLearningTool(store)
	input := `{
		"category": "sql_error",
		"trigger": "some trigger",
		"lesson": "some lesson"
	}`
	result, err := tool.Call(ctx, input)

	// StructuredTool returns error in payload, not as Go error
	if err != nil {
		t.Errorf("expected nil error (store failure conveyed in payload), got: %v", err)
	}
	if !strings.Contains(result, string(tools.ErrCodeServiceUnavailable)) {
		t.Errorf("expected tools.ErrCodeServiceUnavailable in result, got: %s", result)
	}
}
