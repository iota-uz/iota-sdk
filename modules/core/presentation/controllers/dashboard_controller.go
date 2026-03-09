// Package controllers provides this package.
package controllers

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"

	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/dashboard"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/lens"
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
	})
	if err != nil {
		log.Printf("Failed to create lens data source for dashboard: %v", err)
		return &DashboardController{app: app}
	}
	return &DashboardController{app: app, ds: ds}
}

type DashboardController struct {
	app application.Application
	ds  *lenspostgres.DataSource
}

func (c *DashboardController) createFinanceDashboard() lens.DashboardSpec {
	return lens.Dashboard("finance-overview", "Finance Overview",
		lens.Row(
			panel.Stat("total-balance", "Total Balance", "total-balance").Span(3).Build(),
			panel.Stat("monthly-expenses", "Monthly Expenses", "monthly-expenses").Span(3).Build(),
			panel.Stat("monthly-income", "Monthly Income", "monthly-income").Span(3).Build(),
			panel.Stat("transaction-count", "Transactions This Month", "transaction-count").Span(3).Build(),
		),
		lens.Row(
			panel.Bar("monthly-expenses-chart", "Monthly Expenses Chart", "monthly-expenses-chart").Span(6).Build(),
			panel.StackedBar("monthly-expenses-by-category", "Monthly Expenses by Category", "monthly-expenses-by-category").Legend().Span(6).Build(),
		),
		lens.Row(
			panel.Bar("account-balances", "Account Balances", "account-balances").Span(12).Build(),
		),
		lens.Row(
			panel.TimeSeries("revenue-trend", "Revenue Trend", "revenue-trend").Span(6).Build(),
			panel.Bar("top-counterparties", "Top Counterparties", "top-counterparties").Span(6).Build(),
		),
		lens.Row(
			panel.Gauge("expense-budget-usage", "Monthly Budget Usage", "expense-budget-usage").Span(4).Build(),
			panel.Table("recent-transactions", "Recent Transactions", "recent-transactions").Span(8).Build(),
		),
	).WithDatasets(
		lens.QueryDataset("total-balance", "primary", `SELECT (SUM(ma.balance) / 100.0)::float8 as value FROM money_accounts ma`),
		lens.QueryDataset("monthly-expenses", "primary", `SELECT (SUM(t.amount) / 100.0)::float8 as value
			FROM transactions t
			JOIN expenses e ON t.id = e.transaction_id
			WHERE t.transaction_date >= DATE_TRUNC('month', NOW())
			AND t.transaction_type = 'expense'`),
		lens.QueryDataset("monthly-income", "primary", `SELECT (SUM(t.amount) / 100.0)::float8 as value
			FROM transactions t
			WHERE t.transaction_date >= DATE_TRUNC('month', NOW())
			AND t.transaction_type = 'income'`),
		lens.QueryDataset("transaction-count", "primary", `SELECT COUNT(*)::float8 as value
			FROM transactions t
			WHERE t.transaction_date >= DATE_TRUNC('month', NOW())`),
		lens.QueryDataset("monthly-expenses-chart", "primary", `SELECT TO_CHAR(t.transaction_date, 'YYYY-MM') as label,
			(SUM(t.amount) / 100.0)::float8 as value
			FROM transactions t
			JOIN expenses e ON t.id = e.transaction_id
			WHERE t.transaction_date >= NOW() - INTERVAL '12 months'
			AND t.transaction_type = 'expense'
			GROUP BY TO_CHAR(t.transaction_date, 'YYYY-MM')
			ORDER BY label`),
		lens.QueryDataset("monthly-expenses-by-category", "primary", `SELECT TO_CHAR(t.transaction_date, 'YYYY-MM') as category,
			ec.name as series,
			(SUM(t.amount) / 100.0)::float8 as value
			FROM transactions t
			JOIN expenses e ON t.id = e.transaction_id
			JOIN expense_categories ec ON e.category_id = ec.id
			WHERE t.transaction_date >= NOW() - INTERVAL '6 months'
			AND t.transaction_type = 'expense'
			GROUP BY TO_CHAR(t.transaction_date, 'YYYY-MM'), ec.name
			ORDER BY category, series`),
		lens.QueryDataset("account-balances", "primary", `SELECT ma.name as label,
			(ma.balance / 100.0)::float8 as value
			FROM money_accounts ma
			ORDER BY ma.balance DESC`),
		lens.QueryDataset("revenue-trend", "primary", `SELECT DATE_TRUNC('day', t.transaction_date)::date as label,
			(SUM(t.amount) / 100.0)::float8 as value
			FROM transactions t
			WHERE t.transaction_date >= NOW() - INTERVAL '30 days'
			AND t.transaction_type = 'income'
			GROUP BY DATE_TRUNC('day', t.transaction_date)
			ORDER BY label`),
		lens.QueryDataset("top-counterparties", "primary", `SELECT c.name as label,
			COUNT(p.id)::float8 as value
			FROM counterparty c
			JOIN payments p ON c.id = p.counterparty_id
			JOIN transactions t ON p.transaction_id = t.id
			WHERE t.transaction_date >= NOW() - INTERVAL '30 days'
			GROUP BY c.name
			ORDER BY value DESC`),
		lens.QueryDataset("expense-budget-usage", "primary", `SELECT CASE
				WHEN budget.monthly_limit > 0 THEN
					((current_expenses.total / budget.monthly_limit) * 100.0)::float8
				ELSE 0.0
			END as value
			FROM (SELECT 50000.0 as monthly_limit) budget
			CROSS JOIN (
				SELECT COALESCE(SUM(t.amount) / 100.0, 0.0) as total
				FROM transactions t
				JOIN expenses e ON t.id = e.transaction_id
				WHERE t.transaction_date >= DATE_TRUNC('month', NOW())
				AND t.transaction_type = 'expense'
			) current_expenses`),
		lens.QueryDataset("recent-transactions", "primary", `SELECT t.transaction_date,
			t.transaction_type,
			(t.amount / 100.0)::float8 as amount,
			COALESCE(c.name, 'Internal') as counterparty,
			t.comment
			FROM transactions t
			LEFT JOIN payments p ON t.id = p.transaction_id
			LEFT JOIN counterparty c ON p.counterparty_id = c.id
			ORDER BY t.transaction_date DESC, t.created_at DESC
			LIMIT 20`),
	)
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
	dash := c.createFinanceDashboard()

	var results *runtime.DashboardResult
	if c.ds != nil {
		err := composables.InTx(r.Context(), func(txCtx context.Context) error {
			ctx, cancel := context.WithTimeout(txCtx, 30*time.Second)
			defer cancel()
			executed, execErr := runtime.Execute(ctx, dash, runtime.Runtime{
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
