package controllers

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/dashboard"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/builder"
	"github.com/iota-uz/iota-sdk/pkg/lens/datasource/postgres"
	"github.com/iota-uz/iota-sdk/pkg/lens/executor"
	"github.com/iota-uz/iota-sdk/pkg/middleware"

	"github.com/gorilla/mux"
)

func NewDashboardController(app application.Application) application.Controller {
	// Setup PostgreSQL data source for lens
	config := configuration.Use()
	pgConfig := postgres.Config{
		ConnectionString: config.Database.ConnectionString(),
		MaxConnections:   5,
		MinConnections:   1,
		QueryTimeout:     30 * time.Second,
	}

	pgDataSource, err := postgres.NewPostgreSQLDataSource(pgConfig)
	if err != nil {
		log.Printf("Failed to create PostgreSQL data source for dashboard: %v", err)
		// Create controller without executor if data source fails
		return &DashboardController{
			app:      app,
			executor: nil,
		}
	}

	// Create executor and register data source
	exec := executor.NewExecutor(nil, 30*time.Second)
	err = exec.RegisterDataSource("postgres", pgDataSource)
	if err != nil {
		log.Printf("Failed to register data source: %v", err)
		pgDataSource.Close()
		exec = nil
	}

	return &DashboardController{
		app:      app,
		executor: exec,
	}
}

type DashboardController struct {
	app      application.Application
	executor executor.Executor
}

// createFinanceDashboard creates a finance dashboard configuration using lens builders
func (c *DashboardController) createFinanceDashboard() lens.DashboardConfig {
	return builder.NewDashboard().
		ID("finance-dashboard").
		Title("Finance Overview").
		Description("Financial metrics and analytics dashboard").
		Grid(12, 120).
		RefreshRate(30*time.Second).
		Variable("tenant_id", "current_tenant").
		Variable("time_range", "30d").
		Panel(
			builder.BarChart().
				ID("monthly-expenses").
				Title("Monthly Expenses").
				Position(0, 0).
				Size(6, 4).
				DataSource("postgres").
				Query(`
					SELECT 
						TO_CHAR(t.transaction_date, 'YYYY-MM') as timestamp,
						(SUM(t.amount) / 100.0)::float8 as value
					FROM transactions t
					JOIN expenses e ON t.id = e.transaction_id
					WHERE t.transaction_date >= NOW() - INTERVAL '12 months'
						AND t.transaction_type = 'expense'
					GROUP BY TO_CHAR(t.transaction_date, 'YYYY-MM')
					ORDER BY timestamp
				`).
				Option("colors", []string{"#ef4444"}).
				Build(),
		).
		Panel(
			builder.PieChart().
				ID("expenses-by-category").
				Title("Expenses by Category").
				Position(6, 0).
				Size(6, 4).
				DataSource("postgres").
				Query(`
					SELECT timestamp, value FROM (
						SELECT 
							ec.name as timestamp,
							(SUM(t.amount) / 100.0)::float8 as value
						FROM transactions t
						JOIN expenses e ON t.id = e.transaction_id
						JOIN expense_categories ec ON e.category_id = ec.id
						WHERE t.transaction_date >= NOW() - INTERVAL '30 days'
							AND t.transaction_type = 'expense'
						GROUP BY ec.name
					) AS aggregated_data
					ORDER BY value DESC
				`).
				Option("colors", []string{"#3b82f6", "#10b981", "#f59e0b", "#ef4444", "#8b5cf6", "#06b6d4", "#84cc16", "#f97316", "#ec4899", "#6366f1"}).
				Build(),
		).
		Panel(
			builder.LineChart().
				ID("account-balances").
				Title("Account Balances Over Time").
				Position(0, 4).
				Size(12, 4).
				DataSource("postgres").
				Query(`
					SELECT 
						ma.name as timestamp,
						(ma.balance / 100.0)::float8 as value
					FROM money_accounts ma
					ORDER BY ma.balance DESC
				`).
				Option("colors", []string{"#10b981", "#3b82f6", "#f59e0b"}).
				Build(),
		).
		Panel(
			builder.AreaChart().
				ID("revenue-trend").
				Title("Revenue Trend").
				Position(0, 8).
				Size(6, 4).
				DataSource("postgres").
				Query(`
					SELECT 
						DATE_TRUNC('day', t.transaction_date) as timestamp,
						(SUM(t.amount) / 100.0)::float8 as value
					FROM transactions t
					WHERE t.transaction_date >= NOW() - INTERVAL '30 days'
						AND t.transaction_type = 'income'
					GROUP BY DATE_TRUNC('day', t.transaction_date)
					ORDER BY timestamp
				`).
				Option("colors", []string{"#10b981"}).
				Build(),
		).
		Panel(
			builder.BarChart().
				ID("top-counterparties").
				Title("Top Counterparties by Transaction Volume").
				Position(6, 8).
				Size(6, 4).
				DataSource("postgres").
				Query(`
					SELECT 
						c.name as timestamp,
						COUNT(p.id)::float as value
					FROM counterparty c
					JOIN payments p ON c.id = p.counterparty_id
					JOIN transactions t ON p.transaction_id = t.id
					WHERE t.transaction_date >= NOW() - INTERVAL '30 days'
					GROUP BY c.name
					ORDER BY value DESC
				`).
				Option("colors", []string{"#06b6d4"}).
				Build(),
		).
		Panel(
			builder.GaugeChart().
				ID("expense-budget-usage").
				Title("Monthly Budget Usage").
				Position(0, 12).
				Size(4, 3).
				DataSource("postgres").
				Query(`
					SELECT 
						'Budget Usage' as timestamp,
						CASE 
							WHEN budget.monthly_limit > 0 THEN 
								((current_expenses.total / budget.monthly_limit) * 100.0)::float8
							ELSE 0.0
						END as value
					FROM (
						SELECT 50000.0 as monthly_limit
					) budget
					CROSS JOIN (
						SELECT 
							COALESCE(SUM(t.amount) / 100.0, 0.0) as total
						FROM transactions t
						JOIN expenses e ON t.id = e.transaction_id
						WHERE t.transaction_date >= DATE_TRUNC('month', NOW())
							AND t.transaction_type = 'expense'
					) current_expenses
				`).
				Option("colors", []string{"#f59e0b"}).
				Build(),
		).
		Panel(
			builder.TableChart().
				ID("recent-transactions").
				Title("Recent Transactions").
				Position(4, 12).
				Size(8, 6).
				DataSource("postgres").
				Query(`
					SELECT 
						t.transaction_date,
						t.transaction_type,
						(t.amount / 100.0)::float8 as amount,
						COALESCE(c.name, 'Internal') as counterparty,
						t.comment
					FROM transactions t
					LEFT JOIN payments p ON t.id = p.transaction_id
					LEFT JOIN counterparty c ON p.counterparty_id = c.id
					ORDER BY t.transaction_date DESC, t.created_at DESC
				`).
				Build(),
		).
		Build()
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
		middleware.Tabs(),
		middleware.ProvideLocalizer(c.app.Bundle()),
		middleware.NavItems(),
		middleware.WithPageContext(),
	)
	router.HandleFunc("/", c.Get)
}

func (c *DashboardController) Get(w http.ResponseWriter, r *http.Request) {
	// Create finance dashboard configuration
	dashboardConfig := c.createFinanceDashboard()

	// Execute dashboard queries if executor is available
	var dashboardResult *executor.DashboardResult
	if c.executor != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		result, err := c.executor.ExecuteDashboard(ctx, dashboardConfig)
		if err != nil {
			log.Printf("Failed to execute dashboard queries: %v", err)
			// Continue with empty result
			dashboardResult = &executor.DashboardResult{
				PanelResults: make(map[string]*executor.ExecutionResult),
				Duration:     0,
				Errors:       []error{err},
				ExecutedAt:   time.Now(),
			}
		} else {
			dashboardResult = result
		}
	}

	props := &dashboard.IndexPageProps{
		Dashboard:       dashboardConfig,
		DashboardResult: dashboardResult,
	}
	templ.Handler(dashboard.Index(props)).ServeHTTP(w, r)
}
