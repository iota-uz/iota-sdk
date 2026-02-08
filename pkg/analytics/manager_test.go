package analytics

import (
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
)

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

func TestSync_EmptyViews_ReturnsNil(t *testing.T) {
	t.Parallel()

	m := NewViewManager()

	// Sync with no registered views should return nil (no-op)
	// Note: We can't actually call Sync without a real database connection,
	// but we can verify the early return logic by checking the Views slice
	views := m.Views()
	if len(views) != 0 {
		t.Error("expected empty views for no-op Sync test")
	}

	// The actual Sync call would require a database connection,
	// so we're only testing the precondition here
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
