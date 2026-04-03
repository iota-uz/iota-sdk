package analytics

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/jackc/pgx/v5/pgconn"
)

type fakeBeginner struct {
	beginCalled bool
	beginErr    error
	tx          *fakeTx
}

func (f *fakeBeginner) Begin(ctx context.Context) (dbTx, error) {
	f.beginCalled = true
	if f.beginErr != nil {
		return nil, f.beginErr
	}

	return f.tx, nil
}

type fakeTx struct {
	querySQL  string
	queryArgs []any
	queryErr  error
	rows      dbRows

	executed  []string
	execErr   error
	committed bool
}

func (f *fakeTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	f.executed = append(f.executed, sql)
	if f.execErr != nil {
		return pgconn.NewCommandTag(""), f.execErr
	}

	return pgconn.NewCommandTag("EXEC"), nil
}

func (f *fakeTx) Query(ctx context.Context, sql string, args ...any) (dbRows, error) {
	f.querySQL = sql
	f.queryArgs = args
	if f.queryErr != nil {
		return nil, f.queryErr
	}

	if f.rows == nil {
		return &fakeRows{}, nil
	}

	return f.rows, nil
}

func (f *fakeTx) Commit(ctx context.Context) error {
	f.committed = true
	return nil
}

func (f *fakeTx) Rollback(ctx context.Context) error {
	return nil
}

type fakeRows struct {
	values []viewKey
	index  int
	err    error
}

func (f *fakeRows) Next() bool {
	if f.index >= len(f.values) {
		return false
	}

	f.index++
	return true
}

func (f *fakeRows) Scan(dest ...any) error {
	current := f.values[f.index-1]
	*(dest[0].(*string)) = current.schema
	*(dest[1].(*string)) = current.name
	return nil
}

func (f *fakeRows) Err() error {
	return f.err
}

func (f *fakeRows) Close() {}

func TestNewViewManager_DefaultSchema(t *testing.T) {
	t.Parallel()

	m := NewViewManager()

	if m.Schema() != "analytics" {
		t.Errorf("expected default schema=analytics, got %q", m.Schema())
	}
}

func TestNewViewManager_WithSchema(t *testing.T) {
	t.Parallel()

	m := NewViewManager(WithSchema("custom_analytics"))

	if m.Schema() != "custom_analytics" {
		t.Errorf("expected schema=custom_analytics, got %q", m.Schema())
	}
}

func TestRegister_AddsViews(t *testing.T) {
	t.Parallel()

	m := NewViewManager()

	view1 := TenantView("users")
	view2 := TenantView("products")

	m.Register(view1, view2)

	views := m.Views()
	if len(views) != 2 {
		t.Errorf("expected 2 views, got %d", len(views))
	}

	if views[0].Name != "users" {
		t.Errorf("expected first view name=users, got %q", views[0].Name)
	}

	if views[1].Name != "products" {
		t.Errorf("expected second view name=products, got %q", views[1].Name)
	}
}

func TestRegister_MultipleCallsAccumulate(t *testing.T) {
	t.Parallel()

	m := NewViewManager()

	view1 := TenantView("users")
	m.Register(view1)

	view2 := TenantView("products")
	m.Register(view2)

	views := m.Views()
	if len(views) != 2 {
		t.Errorf("expected 2 views after two Register calls, got %d", len(views))
	}
}

func TestRegister_FillsEmptySchemaWithDefault(t *testing.T) {
	t.Parallel()

	m := NewViewManager(WithSchema("custom_schema"))

	// Create a view with empty schema
	view := View{
		Name:   "test_view",
		SQL:    "SELECT 1",
		Schema: "", // empty
	}

	m.Register(view)

	views := m.Views()
	if len(views) != 1 {
		t.Fatalf("expected 1 view, got %d", len(views))
	}

	if views[0].Schema != "custom_schema" {
		t.Errorf("expected Schema=custom_schema (filled from manager default), got %q", views[0].Schema)
	}
}

func TestRegister_PreservesExistingSchema(t *testing.T) {
	t.Parallel()

	m := NewViewManager(WithSchema("analytics"))

	// Create a view with explicit schema
	view := View{
		Name:   "test_view",
		SQL:    "SELECT 1",
		Schema: "custom_schema",
	}

	m.Register(view)

	views := m.Views()
	if len(views) != 1 {
		t.Fatalf("expected 1 view, got %d", len(views))
	}

	if views[0].Schema != "custom_schema" {
		t.Errorf("expected Schema=custom_schema (preserved), got %q", views[0].Schema)
	}
}

func TestViews_ReturnsCopy(t *testing.T) {
	t.Parallel()

	m := NewViewManager()

	view := TenantView("users")
	m.Register(view)

	// Get views and modify the returned slice
	views1 := m.Views()
	originalLen := len(views1)

	// Append to the returned slice (result not used; we only need to trigger potential mutation)
	_ = append(views1, TenantView("products"))

	// Get views again - should still have original length
	views2 := m.Views()

	if len(views2) != originalLen {
		t.Errorf("modifying returned slice affected manager's internal state: expected %d views, got %d", originalLen, len(views2))
	}
}

func TestViews_EmptyManager(t *testing.T) {
	t.Parallel()

	m := NewViewManager()

	views := m.Views()

	if views == nil {
		t.Error("Views() should return non-nil slice for empty manager")
	}

	if len(views) != 0 {
		t.Errorf("expected 0 views, got %d", len(views))
	}
}

func TestSchema_ReturnsCorrectValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		options        []ManagerOption
		expectedSchema string
	}{
		{
			name:           "default",
			options:        nil,
			expectedSchema: "analytics",
		},
		{
			name:           "custom schema",
			options:        []ManagerOption{WithSchema("reports")},
			expectedSchema: "reports",
		},
		{
			name:           "empty schema override",
			options:        []ManagerOption{WithSchema("")},
			expectedSchema: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewViewManager(tt.options...)
			if m.Schema() != tt.expectedSchema {
				t.Errorf("expected schema=%q, got %q", tt.expectedSchema, m.Schema())
			}
		})
	}
}

func TestSync_EmptyManagerDropsManagedSchemaViews(t *testing.T) {
	t.Parallel()

	m := NewViewManager()

	tx := &fakeTx{
		rows: &fakeRows{
			values: []viewKey{
				{schema: "analytics", name: "old_clients"},
				{schema: "analytics", name: "old_policies"},
			},
		},
	}
	beginner := &fakeBeginner{tx: tx}

	err := m.sync(context.Background(), beginner)
	if err != nil {
		t.Fatalf("expected sync to succeed, got error: %v", err)
	}

	expectedSQL := []string{
		`DROP VIEW IF EXISTS "analytics"."old_clients" CASCADE`,
		`DROP VIEW IF EXISTS "analytics"."old_policies" CASCADE`,
	}
	if !reflect.DeepEqual(tx.executed, expectedSQL) {
		t.Fatalf("unexpected executed SQL:\nexpected: %#v\ngot: %#v", expectedSQL, tx.executed)
	}

	if !tx.committed {
		t.Fatal("expected sync transaction to commit")
	}
}

func TestRegister_WithPermissions(t *testing.T) {
	t.Parallel()

	m := NewViewManager()

	perm := permission.MustCreate(
		uuid.New(),
		"users.read.all",
		"users",
		permission.ActionRead,
		permission.ModifierAll,
	)

	view := TenantView("users", RequireAny(perm), Desc("User list with permissions"))

	m.Register(view)

	views := m.Views()
	if len(views) != 1 {
		t.Fatalf("expected 1 view, got %d", len(views))
	}

	// Verify permissions are preserved
	if len(views[0].Required) != 1 {
		t.Errorf("expected 1 permission, got %d", len(views[0].Required))
	}

	if views[0].Logic != LogicAny {
		t.Errorf("expected Logic=LogicAny, got %v", views[0].Logic)
	}

	if views[0].Description != "User list with permissions" {
		t.Errorf("expected description to be preserved, got %q", views[0].Description)
	}
}

func TestRegister_BulkRegistration(t *testing.T) {
	t.Parallel()

	m := NewViewManager()

	// Register multiple views at once
	views := []View{
		TenantView("users"),
		TenantView("products"),
		TenantView("orders"),
	}

	m.Register(views...)

	registered := m.Views()
	if len(registered) != 3 {
		t.Errorf("expected 3 views, got %d", len(registered))
	}
}

func TestWithSchema_MultipleOptions(t *testing.T) {
	t.Parallel()

	// Last option should win
	m := NewViewManager(
		WithSchema("first"),
		WithSchema("second"),
		WithSchema("third"),
	)

	if m.Schema() != "third" {
		t.Errorf("expected schema=third (last option), got %q", m.Schema())
	}
}

func TestColumnComment_AddsColumnComments(t *testing.T) {
	t.Parallel()

	m := NewViewManager()

	view := TenantView(
		"users",
		ColumnComment("id", "Unique identifier"),
		ColumnComment("email", "User email address"),
		Desc("User table view"),
	)

	m.Register(view)

	views := m.Views()
	if len(views) != 1 {
		t.Fatalf("expected 1 view, got %d", len(views))
	}

	// Verify column comments
	if len(views[0].ColumnComments) != 2 {
		t.Errorf("expected 2 column comments, got %d", len(views[0].ColumnComments))
	}

	if views[0].ColumnComments["id"] != "Unique identifier" {
		t.Errorf("expected column comment for 'id', got %q", views[0].ColumnComments["id"])
	}

	if views[0].ColumnComments["email"] != "User email address" {
		t.Errorf("expected column comment for 'email', got %q", views[0].ColumnComments["email"])
	}

	if views[0].Description != "User table view" {
		t.Errorf("expected view description to be preserved, got %q", views[0].Description)
	}
}

func TestSync_ReconcilesStaleAndDesiredViews(t *testing.T) {
	t.Parallel()

	m := NewViewManager()
	m.Register(View{
		Name:        "kept_view",
		SQL:         "SELECT 1 AS value",
		Description: "View description",
		ColumnComments: map[string]string{
			"zeta":  "Last column",
			"alpha": "First column",
		},
	})

	tx := &fakeTx{
		rows: &fakeRows{
			values: []viewKey{
				{schema: "analytics", name: "kept_view"},
				{schema: "analytics", name: "stale_view"},
			},
		},
	}
	beginner := &fakeBeginner{tx: tx}

	err := m.sync(context.Background(), beginner)
	if err != nil {
		t.Fatalf("expected sync to succeed, got error: %v", err)
	}

	if !strings.Contains(tx.querySQL, "information_schema.views") {
		t.Fatalf("expected existing-view discovery query, got %q", tx.querySQL)
	}

	if len(tx.queryArgs) != 1 {
		t.Fatalf("expected a single query argument, got %d", len(tx.queryArgs))
	}

	gotSchemas, ok := tx.queryArgs[0].([]string)
	if !ok {
		t.Fatalf("expected schemas query arg to be []string, got %T", tx.queryArgs[0])
	}
	if !reflect.DeepEqual(gotSchemas, []string{"analytics"}) {
		t.Fatalf("unexpected managed schemas: %#v", gotSchemas)
	}

	expectedSQL := []string{
		`DROP VIEW IF EXISTS "analytics"."stale_view" CASCADE`,
		`DROP VIEW IF EXISTS "analytics"."kept_view" CASCADE`,
		`CREATE VIEW "analytics"."kept_view" AS SELECT 1 AS value`,
		`COMMENT ON VIEW "analytics"."kept_view" IS 'View description'`,
		`COMMENT ON COLUMN "analytics"."kept_view"."alpha" IS 'First column'`,
		`COMMENT ON COLUMN "analytics"."kept_view"."zeta" IS 'Last column'`,
	}
	if !reflect.DeepEqual(tx.executed, expectedSQL) {
		t.Fatalf("unexpected executed SQL:\nexpected: %#v\ngot: %#v", expectedSQL, tx.executed)
	}

	if !tx.committed {
		t.Fatal("expected sync transaction to commit")
	}
}

func TestSync_DuplicateQualifiedViewReturnsError(t *testing.T) {
	t.Parallel()

	m := NewViewManager()
	m.Register(
		View{Name: "users", SQL: "SELECT 1"},
		View{Name: "users", SQL: "SELECT 2"},
	)

	beginner := &fakeBeginner{tx: &fakeTx{}}

	err := m.sync(context.Background(), beginner)
	if err == nil {
		t.Fatal("expected duplicate view sync to fail")
	}

	if !strings.Contains(err.Error(), `duplicate view definition for analytics.users`) {
		t.Fatalf("unexpected error: %v", err)
	}

	if beginner.beginCalled {
		t.Fatal("expected duplicate definition to fail before opening a transaction")
	}
}
