package analytics

import (
	"strings"
	"testing"
)

func TestDefaultTenantViews(t *testing.T) {
	t.Parallel()

	views := DefaultTenantViews()

	expectedTables := []string{
		"clients",
		"counterparty",
		"employees",
		"users",
		"warehouse_units",
		"warehouse_positions",
		"warehouse_products",
		"warehouse_orders",
		"inventory",
		"inventory_checks",
		"inventory_check_results",
		"transactions",
		"payments",
		"expenses",
		"expense_categories",
		"payment_categories",
		"money_accounts",
		"debts",
		"projects",
		"billing_transactions",
		"uploads",
		"sessions",
		"chats",
		"message_templates",
		"action_logs",
		"authentication_logs",
		"positions",
		"roles",
		"passports",
		"user_groups",
		"companies",
		"chat_members",
		"ai_chat_configs",
		"permissions",
	}

	if len(views) != len(expectedTables) {
		t.Errorf("expected %d views, got %d", len(expectedTables), len(views))
	}

	// Build a map for easier lookup
	viewMap := make(map[string]View)
	for _, v := range views {
		viewMap[v.Name] = v
	}

	// Verify each expected table has a corresponding view
	for _, tableName := range expectedTables {
		view, exists := viewMap[tableName]
		if !exists {
			t.Errorf("expected view for table %q not found", tableName)
			continue
		}

		// Verify the view has correct properties
		if view.Schema != "analytics" {
			t.Errorf("view %q: expected Schema=analytics, got %q", tableName, view.Schema)
		}

		expectedSQL := "SELECT * FROM public." + tableName + " WHERE tenant_id = current_setting('app.tenant_id', true)::uuid"
		if view.SQL != expectedSQL {
			t.Errorf("view %q: incorrect SQL:\nexpected: %s\ngot:      %s", tableName, expectedSQL, view.SQL)
		}

		// Verify permissions are empty (public by default)
		if len(view.Required) != 0 {
			t.Errorf("view %q: expected no permissions (public), got %d permissions", tableName, len(view.Required))
		}
	}
}

func TestDefaultTenantViews_TenantIsolation(t *testing.T) {
	t.Parallel()

	views := DefaultTenantViews()

	// Every view should include tenant isolation
	for _, view := range views {
		if !strings.Contains(view.SQL, "WHERE tenant_id = current_setting('app.tenant_id', true)::uuid") {
			t.Errorf("view %q missing tenant isolation WHERE clause", view.Name)
		}
	}
}

func TestDefaultTenantViews_PublicSchema(t *testing.T) {
	t.Parallel()

	views := DefaultTenantViews()

	// Every view should select from public schema
	for _, view := range views {
		if !strings.Contains(view.SQL, "FROM public."+view.Name) {
			t.Errorf("view %q does not select from public.%s", view.Name, view.Name)
		}
	}
}

func TestDefaultTenantViews_NoPermissions(t *testing.T) {
	t.Parallel()

	views := DefaultTenantViews()

	// All default views should be public (no permissions)
	for _, view := range views {
		if view.Required != nil && len(view.Required) > 0 {
			t.Errorf("view %q should be public (no permissions), but has %d permissions", view.Name, len(view.Required))
		}
	}
}
