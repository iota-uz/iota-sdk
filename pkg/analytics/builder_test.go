package analytics

import (
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
)

func TestTenantView(t *testing.T) {
	t.Parallel()

	view := TenantView("users")

	if view.Name != "users" {
		t.Errorf("expected Name=users, got %q", view.Name)
	}

	expectedSQL := "SELECT * FROM public.users WHERE tenant_id = current_setting('app.tenant_id', true)::uuid"
	if view.SQL != expectedSQL {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expectedSQL, view.SQL)
	}

	if view.Schema != "analytics" {
		t.Errorf("expected Schema=analytics, got %q", view.Schema)
	}
}

func TestTenantViewFrom(t *testing.T) {
	t.Parallel()

	view := TenantViewFrom("users_table", "custom_schema")

	if view.Name != "users_table" {
		t.Errorf("expected Name=users_table, got %q", view.Name)
	}

	expectedSQL := "SELECT * FROM custom_schema.users_table WHERE tenant_id = current_setting('app.tenant_id', true)::uuid"
	if view.SQL != expectedSQL {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expectedSQL, view.SQL)
	}

	if view.Schema != "analytics" {
		t.Errorf("expected Schema=analytics, got %q", view.Schema)
	}
}

func TestCustomView(t *testing.T) {
	t.Parallel()

	customSQL := "SELECT id, name FROM users WHERE active = true"
	view := CustomView("active_users", customSQL)

	if view.Name != "active_users" {
		t.Errorf("expected Name=active_users, got %q", view.Name)
	}

	if view.SQL != customSQL {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", customSQL, view.SQL)
	}

	if view.Schema != "analytics" {
		t.Errorf("expected Schema=analytics, got %q", view.Schema)
	}
}

func TestRequireAny(t *testing.T) {
	t.Parallel()

	perm1 := permission.MustCreate(
		uuid.New(),
		"test.read.all",
		"test",
		permission.ActionRead,
		permission.ModifierAll,
	)
	perm2 := permission.MustCreate(
		uuid.New(),
		"test.update.all",
		"test",
		permission.ActionUpdate,
		permission.ModifierAll,
	)

	view := TenantView("users", RequireAny(perm1, perm2))

	if len(view.Required) != 2 {
		t.Errorf("expected 2 permissions, got %d", len(view.Required))
	}

	if view.Logic != LogicAny {
		t.Errorf("expected Logic=LogicAny, got %v", view.Logic)
	}

	if !view.Required[0].Equals(perm1) {
		t.Errorf("first permission mismatch")
	}

	if !view.Required[1].Equals(perm2) {
		t.Errorf("second permission mismatch")
	}
}

func TestRequireAll(t *testing.T) {
	t.Parallel()

	perm1 := permission.MustCreate(
		uuid.New(),
		"test.read.all",
		"test",
		permission.ActionRead,
		permission.ModifierAll,
	)
	perm2 := permission.MustCreate(
		uuid.New(),
		"test.update.all",
		"test",
		permission.ActionUpdate,
		permission.ModifierAll,
	)

	view := TenantView("users", RequireAll(perm1, perm2))

	if len(view.Required) != 2 {
		t.Errorf("expected 2 permissions, got %d", len(view.Required))
	}

	if view.Logic != LogicAll {
		t.Errorf("expected Logic=LogicAll, got %v", view.Logic)
	}

	if !view.Required[0].Equals(perm1) {
		t.Errorf("first permission mismatch")
	}

	if !view.Required[1].Equals(perm2) {
		t.Errorf("second permission mismatch")
	}
}

func TestWithDescription(t *testing.T) {
	t.Parallel()

	desc := "All active users in the system"
	view := TenantView("users", Desc(desc))

	if view.Description != desc {
		t.Errorf("expected Description=%q, got %q", desc, view.Description)
	}
}

func TestDefaultSchema(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		view View
	}{
		{
			name: "TenantView",
			view: TenantView("users"),
		},
		{
			name: "TenantViewFrom",
			view: TenantViewFrom("users", "public"),
		},
		{
			name: "CustomView",
			view: CustomView("custom", "SELECT 1"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.view.Schema != "analytics" {
				t.Errorf("expected default Schema=analytics, got %q", tt.view.Schema)
			}
		})
	}
}

func TestBuildReturnsProperView(t *testing.T) {
	t.Parallel()

	perm := permission.MustCreate(
		uuid.New(),
		"users.read.all",
		"users",
		permission.ActionRead,
		permission.ModifierAll,
	)

	view := TenantView("users", RequireAny(perm), Desc("User list view"))

	// Verify all fields are properly set
	if view.Name != "users" {
		t.Errorf("Name: expected users, got %q", view.Name)
	}

	if view.Schema != "analytics" {
		t.Errorf("Schema: expected analytics, got %q", view.Schema)
	}

	if view.SQL == "" {
		t.Error("SQL should not be empty")
	}

	if len(view.Required) != 1 {
		t.Errorf("Required: expected 1 permission, got %d", len(view.Required))
	}

	if view.Logic != LogicAny {
		t.Errorf("Logic: expected LogicAny, got %v", view.Logic)
	}

	if view.Description != "User list view" {
		t.Errorf("Description: expected 'User list view', got %q", view.Description)
	}
}

func TestMultipleOptions(t *testing.T) {
	t.Parallel()

	perm1 := permission.MustCreate(
		uuid.New(),
		"test.read.all",
		"test",
		permission.ActionRead,
		permission.ModifierAll,
	)

	view := CustomView(
		"test",
		"SELECT 1",
		RequireAny(perm1),
		Desc("test"),
		InSchema("custom"),
	)

	if view.Name != "test" {
		t.Error("view name should be 'test'")
	}

	if len(view.Required) != 1 {
		t.Error("view should have 1 permission")
	}

	if view.Description != "test" {
		t.Error("view should have description 'test'")
	}

	if view.Schema != "custom" {
		t.Error("view should have schema 'custom'")
	}
}
