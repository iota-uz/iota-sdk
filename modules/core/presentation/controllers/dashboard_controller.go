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
	lenspostgres "github.com/iota-uz/iota-sdk/pkg/lens/postgres"
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
	ds  lens.DataSource
}

func (c *DashboardController) createFinanceDashboard() lens.Dashboard {
	return lens.NewDashboard("Finance Overview",
		lens.NewRow(
			lens.Metric("total-balance", "Total Balance").
				Query(`SELECT (SUM(ma.balance) / 100.0)::float8 as value FROM money_accounts ma`).
				Unit("USD").Prefix("$").Color("#10b981").Span(3).Build(),
			lens.Metric("monthly-expenses", "Monthly Expenses").
				Query(`SELECT (SUM(t.amount) / 100.0)::float8 as value
					FROM transactions t
					JOIN expenses e ON t.id = e.transaction_id
					WHERE t.transaction_date >= DATE_TRUNC('month', NOW())
					AND t.transaction_type = 'expense'`).
				Unit("USD").Prefix("$").Color("#ef4444").Span(3).Build(),
			lens.Metric("monthly-income", "Monthly Income").
				Query(`SELECT (SUM(t.amount) / 100.0)::float8 as value
					FROM transactions t
					WHERE t.transaction_date >= DATE_TRUNC('month', NOW())
					AND t.transaction_type = 'income'`).
				Unit("USD").Prefix("$").Color("#059669").Span(3).Build(),
			lens.Metric("transaction-count", "Transactions This Month").
				Query(`SELECT COUNT(*)::float8 as value
					FROM transactions t
					WHERE t.transaction_date >= DATE_TRUNC('month', NOW())`).
				Color("#3b82f6").Span(3).Build(),
		),
		lens.NewRow(
			lens.Bar("monthly-expenses-chart", "Monthly Expenses Chart").
				Query(`SELECT TO_CHAR(t.transaction_date, 'YYYY-MM') as label,
					(SUM(t.amount) / 100.0)::float8 as value
					FROM transactions t
					JOIN expenses e ON t.id = e.transaction_id
					WHERE t.transaction_date >= NOW() - INTERVAL '12 months'
					AND t.transaction_type = 'expense'
					GROUP BY TO_CHAR(t.transaction_date, 'YYYY-MM')
					ORDER BY label`).
				Colors("#ef4444").Span(6).Build(),
			lens.StackedBar("monthly-expenses-by-category", "Monthly Expenses by Category").
				Query(`SELECT TO_CHAR(t.transaction_date, 'YYYY-MM') as category,
					ec.name as series,
					(SUM(t.amount) / 100.0)::float8 as value
					FROM transactions t
					JOIN expenses e ON t.id = e.transaction_id
					JOIN expense_categories ec ON e.category_id = ec.id
					WHERE t.transaction_date >= NOW() - INTERVAL '6 months'
					AND t.transaction_type = 'expense'
					GROUP BY TO_CHAR(t.transaction_date, 'YYYY-MM'), ec.name
					ORDER BY category, series`).
				Legend().Span(6).Build(),
		),
		lens.NewRow(
			lens.Line("account-balances", "Account Balances").
				Query(`SELECT ma.name as label,
					(ma.balance / 100.0)::float8 as value
					FROM money_accounts ma
					ORDER BY ma.balance DESC`).
				Colors("#10b981", "#3b82f6", "#f59e0b").Span(12).Build(),
		),
		lens.NewRow(
			lens.Area("revenue-trend", "Revenue Trend").
				Query(`SELECT DATE_TRUNC('day', t.transaction_date)::date as label,
					(SUM(t.amount) / 100.0)::float8 as value
					FROM transactions t
					WHERE t.transaction_date >= NOW() - INTERVAL '30 days'
					AND t.transaction_type = 'income'
					GROUP BY DATE_TRUNC('day', t.transaction_date)
					ORDER BY label`).
				Colors("#10b981").Span(6).Build(),
			lens.Bar("top-counterparties", "Top Counterparties").
				Query(`SELECT c.name as label,
					COUNT(p.id)::float as value
					FROM counterparty c
					JOIN payments p ON c.id = p.counterparty_id
					JOIN transactions t ON p.transaction_id = t.id
					WHERE t.transaction_date >= NOW() - INTERVAL '30 days'
					GROUP BY c.name
					ORDER BY value DESC`).
				Colors("#06b6d4").Span(6).Build(),
		),
		lens.NewRow(
			lens.Gauge("expense-budget-usage", "Monthly Budget Usage").
				Query(`SELECT CASE
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
					) current_expenses`).
				Colors("#f59e0b").Span(4).Build(),
			lens.Table("recent-transactions", "Recent Transactions").
				Query(`SELECT t.transaction_date, t.transaction_type,
					(t.amount / 100.0)::float8 as amount,
					COALESCE(c.name, 'Internal') as counterparty,
					t.comment
					FROM transactions t
					LEFT JOIN payments p ON t.id = p.transaction_id
					LEFT JOIN counterparty c ON p.counterparty_id = c.id
					ORDER BY t.transaction_date DESC, t.created_at DESC
					LIMIT 20`).
				Span(8).Build(),
		),
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

	var results *lens.Results
	if c.ds != nil {
		err := composables.InTx(r.Context(), func(txCtx context.Context) error {
			ctx, cancel := context.WithTimeout(txCtx, 30*time.Second)
			defer cancel()
			results = lens.Execute(ctx, c.ds, dash)
			return nil
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
