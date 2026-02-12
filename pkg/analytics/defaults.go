package analytics

// DefaultTenantViews returns the default set of tenant-isolated analytics views
// that match the tables created by the iota-sdk migration (changes-1769853356.sql).
//
// Each view is defined as:
//
//	SELECT * FROM public.{table} WHERE tenant_id = current_setting('app.tenant_id', true)::uuid
//
// All views are public (no permission requirements) by default.
// Consumers should register additional views with permissions as needed.
func DefaultTenantViews() []View {
	tables := []string{
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

	views := make([]View, 0, len(tables))
	for _, t := range tables {
		views = append(views, TenantView(t))
	}
	return views
}
