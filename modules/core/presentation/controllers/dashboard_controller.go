// Package controllers provides this package.
package controllers

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/dashboard"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/lens"
	lensbuild "github.com/iota-uz/iota-sdk/pkg/lens/build"
	"github.com/iota-uz/iota-sdk/pkg/lens/datasource"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	lenspostgres "github.com/iota-uz/iota-sdk/pkg/lens/postgres"
	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

func NewDashboardController(app application.Application) application.Controller {
	config := configuration.Use()
	ds, err := lenspostgres.New(lenspostgres.Config{
		ConnectionString: config.Database.ConnectionString(),
		MaxConnections:   5,
		MinConnections:   1,
		QueryTimeout:     30 * time.Second,
		RequiredParams:   []string{"tenant_id"},
	})
	if err != nil {
		log.Printf("Failed to create lens data source for dashboard: %v", err)
		return &DashboardController{app: app}
	}
	return &DashboardController{app: app, ds: ds}
}

type DashboardController struct {
	app application.Application
	ds  datasource.DataSource
}

func (c *DashboardController) createFinanceDashboard(tenantID uuid.UUID) lens.DashboardSpec {
	return lensbuild.Dashboard("finance-overview", "Finance Overview",
		lensbuild.Row(
			panel.Stat("total-balance", "Total Balance", "total-balance").Span(3).Build(),
			panel.Stat("monthly-expenses", "Monthly Expenses", "monthly-expenses").Span(3).Build(),
			panel.Stat("monthly-income", "Monthly Income", "monthly-income").Span(3).Build(),
			panel.Stat("transaction-count", "Transactions This Month", "transaction-count").Span(3).Build(),
		),
		lensbuild.Row(
			panel.Bar("monthly-expenses-chart", "Monthly Expenses Chart", "monthly-expenses-chart").Span(6).Build(),
			panel.StackedBar("monthly-expenses-by-category", "Monthly Expenses by Category", "monthly-expenses-by-category").Legend().Span(6).Build(),
		),
		lensbuild.Row(
			panel.Bar("account-balances", "Account Balances", "account-balances").Span(12).Build(),
		),
		lensbuild.Row(
			panel.TimeSeries("revenue-trend", "Revenue Trend", "revenue-trend").Span(6).Build(),
			panel.Bar("top-counterparties", "Top Counterparties", "top-counterparties").Span(6).Build(),
		),
		lensbuild.Row(
			panel.Gauge("expense-budget-usage", "Monthly Budget Usage", "expense-budget-usage").Span(4).Build(),
			panel.Table("recent-transactions", "Recent Transactions", "recent-transactions").Span(8).Build(),
		),
	).Datasets(
		queryDataset("total-balance", `SELECT COALESCE(SUM(ma.balance), 0)::float8 / 100.0 as value
			FROM money_accounts ma
			WHERE ma.tenant_id = @tenant_id`, tenantID),
		queryDataset("monthly-expenses", `SELECT COALESCE(SUM(t.amount), 0)::float8 / 100.0 as value
			FROM transactions t
			JOIN expenses e ON t.id = e.transaction_id
			WHERE t.tenant_id = @tenant_id
			AND e.tenant_id = @tenant_id
			AND t.transaction_date >= DATE_TRUNC('month', NOW())
			AND t.transaction_type = 'expense'`, tenantID),
		queryDataset("monthly-income", `SELECT COALESCE(SUM(t.amount), 0)::float8 / 100.0 as value
			FROM transactions t
			WHERE t.tenant_id = @tenant_id
			AND t.transaction_date >= DATE_TRUNC('month', NOW())
			AND t.transaction_type = 'income'`, tenantID),
		queryDataset("transaction-count", `SELECT COUNT(*)::float8 as value
			FROM transactions t
			WHERE t.tenant_id = @tenant_id
			AND t.transaction_date >= DATE_TRUNC('month', NOW())`, tenantID),
		queryDataset("monthly-expenses-chart", `SELECT TO_CHAR(t.transaction_date, 'YYYY-MM') as label,
			COALESCE(SUM(t.amount), 0)::float8 / 100.0 as value
			FROM transactions t
			JOIN expenses e ON t.id = e.transaction_id
			WHERE t.tenant_id = @tenant_id
			AND e.tenant_id = @tenant_id
			AND t.transaction_date >= NOW() - INTERVAL '12 months'
			AND t.transaction_type = 'expense'
			GROUP BY TO_CHAR(t.transaction_date, 'YYYY-MM')
			ORDER BY label`, tenantID),
		queryDataset("monthly-expenses-by-category", `SELECT TO_CHAR(t.transaction_date, 'YYYY-MM') as category,
			ec.name as series,
			COALESCE(SUM(t.amount), 0)::float8 / 100.0 as value
			FROM transactions t
			JOIN expenses e ON t.id = e.transaction_id
			JOIN expense_categories ec ON e.category_id = ec.id
			WHERE t.tenant_id = @tenant_id
			AND e.tenant_id = @tenant_id
			AND ec.tenant_id = @tenant_id
			AND t.transaction_date >= NOW() - INTERVAL '6 months'
			AND t.transaction_type = 'expense'
			GROUP BY TO_CHAR(t.transaction_date, 'YYYY-MM'), ec.name
			ORDER BY category, series`, tenantID),
		queryDataset("account-balances", `SELECT ma.name as label,
			(ma.balance / 100.0)::float8 as value
			FROM money_accounts ma
			WHERE ma.tenant_id = @tenant_id
			ORDER BY ma.balance DESC`, tenantID),
		queryDataset("revenue-trend", `SELECT DATE_TRUNC('day', t.transaction_date)::date as label,
			COALESCE(SUM(t.amount), 0)::float8 / 100.0 as value
			FROM transactions t
			WHERE t.tenant_id = @tenant_id
			AND t.transaction_date >= NOW() - INTERVAL '30 days'
			AND t.transaction_type = 'income'
			GROUP BY DATE_TRUNC('day', t.transaction_date)
			ORDER BY label`, tenantID),
		queryDataset("top-counterparties", `SELECT c.name as label,
			COUNT(p.id)::float8 as value
			FROM counterparty c
			JOIN payments p ON c.id = p.counterparty_id
			JOIN transactions t ON p.transaction_id = t.id
			WHERE c.tenant_id = @tenant_id
			AND p.tenant_id = @tenant_id
			AND t.tenant_id = @tenant_id
			AND t.transaction_date >= NOW() - INTERVAL '30 days'
			GROUP BY c.name
			ORDER BY value DESC`, tenantID),
		queryDataset("expense-budget-usage", `SELECT CASE
				WHEN budget.monthly_limit > 0 THEN
					((current_expenses.total / budget.monthly_limit) * 100.0)::float8
				ELSE 0.0
			END as value
			FROM (SELECT 50000.0 as monthly_limit) budget
			CROSS JOIN (
				SELECT COALESCE(SUM(t.amount) / 100.0, 0.0) as total
				FROM transactions t
				JOIN expenses e ON t.id = e.transaction_id
				WHERE t.tenant_id = @tenant_id
				AND e.tenant_id = @tenant_id
				AND t.transaction_date >= DATE_TRUNC('month', NOW())
				AND t.transaction_type = 'expense'
			) current_expenses`, tenantID),
		queryDataset("recent-transactions", `SELECT t.transaction_date,
			t.transaction_type,
			(t.amount / 100.0)::float8 as amount,
			COALESCE(c.name, 'Internal') as counterparty,
			t.comment
			FROM transactions t
			LEFT JOIN payments p ON t.id = p.transaction_id
			LEFT JOIN counterparty c ON p.counterparty_id = c.id
			WHERE t.tenant_id = @tenant_id
			AND (p.id IS NULL OR p.tenant_id = @tenant_id)
			AND (c.id IS NULL OR c.tenant_id = @tenant_id)
			ORDER BY t.transaction_date DESC, t.created_at DESC
			LIMIT 20`, tenantID),
	).Build()
}

func (c *DashboardController) Key() string {
	return "/"
}

func (c *DashboardController) Register(r *mux.Router) {
	router := r.Methods(http.MethodGet).Subrouter()
	router.Use(
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.ProvideDynamicLogo(c.app),
		middleware.ProvideLocalizer(c.app),
		middleware.NavItems(),
		middleware.WithPageContext(),
	)
	router.HandleFunc("/", c.Get)
}

func (c *DashboardController) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, err := composables.UseTenantID(r.Context())
	if err != nil {
		log.Printf("Dashboard tenant lookup failed: %v", err)
		http.Error(w, "tenant not found", http.StatusBadRequest)
		return
	}
	dash := c.createFinanceDashboard(tenantID)

	var results *runtime.Result
	if c.ds != nil {
		err = composables.InTx(r.Context(), func(txCtx context.Context) error {
			ctx, cancel := context.WithTimeout(txCtx, 30*time.Second)
			defer cancel()
			executed, execErr := runtime.Run(ctx, dash, runtime.Request{
				DataSources: map[string]datasource.DataSource{
					"primary": c.ds,
				},
			})
			results = executed
			return execErr
		})
		if err != nil {
			log.Printf("Dashboard transaction failed: %v", err)
		}
	}

	props := &dashboard.IndexPageProps{
		Dashboard: dash,
		Results:   results,
	}
	templ.Handler(dashboard.Index(props)).ServeHTTP(w, r)
}

func queryDataset(name, text string, tenantID uuid.UUID) lens.DatasetSpec {
	spec := lensbuild.QueryDataset(name, "primary", text)
	if spec.Query != nil {
		spec.Query.Params = map[string]lens.ParamValue{
			"tenant_id": {Literal: tenantID},
		}
	}
	return spec
}
