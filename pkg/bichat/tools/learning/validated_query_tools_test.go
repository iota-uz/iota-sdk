package learning

import (
	"context"
	"errors"
	tools "github.com/iota-uz/iota-sdk/pkg/bichat/tools"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/learning"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

// mockValidatedQueryStore implements learning.ValidatedQueryStore for testing.
type mockValidatedQueryStore struct {
	searchFn         func(ctx context.Context, question string, opts learning.ValidatedQuerySearchOpts) ([]learning.ValidatedQuery, error)
	saveFn           func(ctx context.Context, query learning.ValidatedQuery) error
	incrementUsageFn func(ctx context.Context, id uuid.UUID) error
	deleteFn         func(ctx context.Context, id uuid.UUID) error
}

func (m *mockValidatedQueryStore) Search(ctx context.Context, question string, opts learning.ValidatedQuerySearchOpts) ([]learning.ValidatedQuery, error) {
	if m.searchFn != nil {
		return m.searchFn(ctx, question, opts)
	}
	return nil, nil
}

func (m *mockValidatedQueryStore) Save(ctx context.Context, query learning.ValidatedQuery) error {
	if m.saveFn != nil {
		return m.saveFn(ctx, query)
	}
	return nil
}

func (m *mockValidatedQueryStore) IncrementUsage(ctx context.Context, id uuid.UUID) error {
	if m.incrementUsageFn != nil {
		return m.incrementUsageFn(ctx, id)
	}
	return nil
}

func (m *mockValidatedQueryStore) Delete(ctx context.Context, id uuid.UUID) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return nil
}

func TestSearchValidatedQueriesTool_ValidSearchWithResults(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	ctx := composables.WithTenantID(context.Background(), tenantID)

	store := &mockValidatedQueryStore{
		searchFn: func(ctx context.Context, question string, opts learning.ValidatedQuerySearchOpts) ([]learning.ValidatedQuery, error) {
			return []learning.ValidatedQuery{
				{
					ID:         uuid.New(),
					TenantID:   tenantID,
					Question:   "What are total sales by customer?",
					SQL:        "SELECT customer_id, SUM(amount) FROM sales GROUP BY customer_id",
					Summary:    "Aggregates sales by customer",
					TablesUsed: []string{"sales"},
					UsedCount:  10,
				},
			}, nil
		},
	}

	tool := NewSearchValidatedQueriesTool(store)
	input := `{"question": "sales by customer"}`
	result, err := tool.Call(ctx, input)

	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if !strings.Contains(result, "queries") {
		t.Errorf("expected 'queries' in result, got: %s", result)
	}
	if !strings.Contains(result, "customer_id") {
		t.Errorf("expected SQL in result, got: %s", result)
	}
	if !strings.Contains(result, "sales") {
		t.Errorf("expected tables_used in result, got: %s", result)
	}
}

func TestSearchValidatedQueriesTool_NoResults(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	ctx := composables.WithTenantID(context.Background(), tenantID)

	store := &mockValidatedQueryStore{
		searchFn: func(ctx context.Context, question string, opts learning.ValidatedQuerySearchOpts) ([]learning.ValidatedQuery, error) {
			return []learning.ValidatedQuery{}, nil
		},
	}

	tool := NewSearchValidatedQueriesTool(store)
	input := `{"question": "nonexistent question"}`
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

func TestSearchValidatedQueriesTool_MissingQuestion(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	ctx := composables.WithTenantID(context.Background(), tenantID)

	store := &mockValidatedQueryStore{}
	tool := NewSearchValidatedQueriesTool(store)
	input := `{}`
	result, err := tool.Call(ctx, input)

	if err != nil {
		t.Errorf("expected nil error for handled validation, got: %v", err)
	}
	if !strings.Contains(result, string(tools.ErrCodeInvalidRequest)) {
		t.Errorf("expected tools.ErrCodeInvalidRequest in result, got: %s", result)
	}
	if !strings.Contains(result, "question parameter is required") {
		t.Errorf("expected 'question parameter is required' message, got: %s", result)
	}
}

func TestSearchValidatedQueriesTool_StoreError(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	ctx := composables.WithTenantID(context.Background(), tenantID)

	store := &mockValidatedQueryStore{
		searchFn: func(ctx context.Context, question string, opts learning.ValidatedQuerySearchOpts) ([]learning.ValidatedQuery, error) {
			return nil, errors.New("database connection lost")
		},
	}

	tool := NewSearchValidatedQueriesTool(store)
	input := `{"question": "sales by customer"}`
	result, err := tool.Call(ctx, input)

	if err == nil {
		t.Error("expected non-nil error for unrecoverable store failure")
	}
	if !strings.Contains(result, string(tools.ErrCodeServiceUnavailable)) {
		t.Errorf("expected tools.ErrCodeServiceUnavailable in result, got: %s", result)
	}
}

func TestSaveValidatedQueryTool_ValidSave(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	ctx := composables.WithTenantID(context.Background(), tenantID)

	var savedQuery learning.ValidatedQuery
	store := &mockValidatedQueryStore{
		saveFn: func(ctx context.Context, query learning.ValidatedQuery) error {
			savedQuery = query
			return nil
		},
	}

	tool := NewSaveValidatedQueryTool(store)
	input := `{
		"question": "What are total sales?",
		"sql": "SELECT SUM(amount) FROM sales",
		"summary": "Calculates total sales",
		"tables_used": ["sales"]
	}`
	result, err := tool.Call(ctx, input)

	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if !strings.Contains(result, "id") {
		t.Errorf("expected 'id' in result, got: %s", result)
	}
	if !strings.Contains(result, "question") {
		t.Errorf("expected 'question' in result, got: %s", result)
	}
	if savedQuery.Question != "What are total sales?" {
		t.Errorf("expected question to be saved, got: %s", savedQuery.Question)
	}
	if len(savedQuery.TablesUsed) != 1 || savedQuery.TablesUsed[0] != "sales" {
		t.Errorf("expected tables_used to be saved, got: %v", savedQuery.TablesUsed)
	}
}

func TestSaveValidatedQueryTool_MissingQuestion(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	ctx := composables.WithTenantID(context.Background(), tenantID)

	store := &mockValidatedQueryStore{}
	tool := NewSaveValidatedQueryTool(store)
	input := `{
		"sql": "SELECT * FROM sales",
		"summary": "Some summary",
		"tables_used": ["sales"]
	}`
	result, err := tool.Call(ctx, input)

	if err != nil {
		t.Errorf("expected nil error for handled validation, got: %v", err)
	}
	if !strings.Contains(result, string(tools.ErrCodeInvalidRequest)) {
		t.Errorf("expected tools.ErrCodeInvalidRequest in result, got: %s", result)
	}
	if !strings.Contains(result, "question parameter is required") {
		t.Errorf("expected question error message, got: %s", result)
	}
}

func TestSaveValidatedQueryTool_MissingSQL(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	ctx := composables.WithTenantID(context.Background(), tenantID)

	store := &mockValidatedQueryStore{}
	tool := NewSaveValidatedQueryTool(store)
	input := `{
		"question": "What are sales?",
		"summary": "Some summary",
		"tables_used": ["sales"]
	}`
	result, err := tool.Call(ctx, input)

	if err != nil {
		t.Errorf("expected nil error for handled validation, got: %v", err)
	}
	if !strings.Contains(result, string(tools.ErrCodeInvalidRequest)) {
		t.Errorf("expected tools.ErrCodeInvalidRequest in result, got: %s", result)
	}
	if !strings.Contains(result, "sql parameter is required") {
		t.Errorf("expected sql error message, got: %s", result)
	}
}

func TestSaveValidatedQueryTool_NonSelectSQL(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	ctx := composables.WithTenantID(context.Background(), tenantID)

	store := &mockValidatedQueryStore{}
	tool := NewSaveValidatedQueryTool(store)
	input := `{
		"question": "Delete all users",
		"sql": "DELETE FROM users",
		"summary": "Deletes users",
		"tables_used": ["users"]
	}`
	result, err := tool.Call(ctx, input)

	if err != nil {
		t.Errorf("expected nil error for handled validation, got: %v", err)
	}
	if !strings.Contains(result, string(tools.ErrCodeInvalidRequest)) {
		t.Errorf("expected tools.ErrCodeInvalidRequest in result, got: %s", result)
	}
	if !strings.Contains(result, "only SELECT and WITH queries can be saved") {
		t.Errorf("expected SELECT/WITH only message, got: %s", result)
	}
}

func TestSaveValidatedQueryTool_MissingTablesUsed(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	ctx := composables.WithTenantID(context.Background(), tenantID)

	store := &mockValidatedQueryStore{}
	tool := NewSaveValidatedQueryTool(store)
	input := `{
		"question": "What are sales?",
		"sql": "SELECT * FROM sales",
		"summary": "Some summary"
	}`
	result, err := tool.Call(ctx, input)

	if err != nil {
		t.Errorf("expected nil error for handled validation, got: %v", err)
	}
	if !strings.Contains(result, string(tools.ErrCodeInvalidRequest)) {
		t.Errorf("expected tools.ErrCodeInvalidRequest in result, got: %s", result)
	}
	if !strings.Contains(result, "tables_used parameter is required") {
		t.Errorf("expected tables_used error message, got: %s", result)
	}
}

func TestSaveValidatedQueryTool_StoreError(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	ctx := composables.WithTenantID(context.Background(), tenantID)

	store := &mockValidatedQueryStore{
		saveFn: func(ctx context.Context, query learning.ValidatedQuery) error {
			return errors.New("database write failed")
		},
	}

	tool := NewSaveValidatedQueryTool(store)
	input := `{
		"question": "What are sales?",
		"sql": "SELECT * FROM sales",
		"summary": "Some summary",
		"tables_used": ["sales"]
	}`
	result, err := tool.Call(ctx, input)

	if err == nil {
		t.Error("expected non-nil error for unrecoverable store failure")
	}
	if !strings.Contains(result, string(tools.ErrCodeServiceUnavailable)) {
		t.Errorf("expected tools.ErrCodeServiceUnavailable in result, got: %s", result)
	}
}
