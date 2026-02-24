package controllers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/a-h/templ"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/dashboard"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/builder"
	"github.com/iota-uz/iota-sdk/pkg/lens/datasource/postgres"
	"github.com/iota-uz/iota-sdk/pkg/lens/executor"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/serrors"

	"github.com/gorilla/mux"
)

func NewDashboardController(app application.Application) application.Controller {
	return &DashboardController{
		app: app,
	}
}

const executorInitCooldown = 30 * time.Second

type DashboardController struct {
	app             application.Application
	executorMu      sync.Mutex
	executor        executor.Executor
	executorInitErr error
	executorInitAt  time.Time
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
			builder.MetricCard().
				ID("total-balance").
				Title("Total Balance").
				Position(0, 0).
				Size(3, 2).
				DataSource("postgres").
				Query(`
					SELECT 
						'Total Balance' as timestamp,
						(SUM(ma.balance) / 100.0)::float8 as value
					FROM money_accounts ma
				`).
				Option("unit", "USD").
				Option("color", "#10b981").
				Option("icon", "💰").
				Build(),
		).
		Panel(
			builder.MetricCard().
				ID("monthly-expenses").
				Title("Monthly Expenses").
				Position(3, 0).
				Size(3, 2).
				DataSource("postgres").
				Query(`
					SELECT 
						'Monthly Expenses' as timestamp,
						(SUM(t.amount) / 100.0)::float8 as value
					FROM transactions t
					JOIN expenses e ON t.id = e.transaction_id
					WHERE t.transaction_date >= DATE_TRUNC('month', NOW())
						AND t.transaction_type = 'expense'
				`).
				Option("unit", "USD").
				Option("color", "#ef4444").
				Option("icon", "📊").
				Build(),
		).
		Panel(
			builder.MetricCard().
				ID("monthly-income").
				Title("Monthly Income").
				Position(6, 0).
				Size(3, 2).
				DataSource("postgres").
				Query(`
					SELECT 
						'Monthly Income' as timestamp,
						(SUM(t.amount) / 100.0)::float8 as value
					FROM transactions t
					WHERE t.transaction_date >= DATE_TRUNC('month', NOW())
						AND t.transaction_type = 'income'
				`).
				Option("unit", "USD").
				Option("color", "#059669").
				Option("icon", "📈").
				Build(),
		).
		Panel(
			builder.MetricCard().
				ID("transaction-count").
				Title("Transactions This Month").
				Position(9, 0).
				Size(3, 2).
				DataSource("postgres").
				Query(`
					SELECT 
						'Transaction Count' as timestamp,
						COUNT(*)::float8 as value
					FROM transactions t
					WHERE t.transaction_date >= DATE_TRUNC('month', NOW())
				`).
				Option("color", "#3b82f6").
				Option("icon", "🔄").
				Build(),
		).
		Panel(
			builder.BarChart().
				ID("monthly-expenses-chart").
				Title("Monthly Expenses Chart").
				Position(0, 2).
				Size(6, 4).
				DataSource("postgres").
				Query(`
					SELECT 
						TO_CHAR(t.transaction_date, 'YYYY-MM') as label,
						(SUM(t.amount) / 100.0)::float8 as value
					FROM transactions t
					JOIN expenses e ON t.id = e.transaction_id
					WHERE t.transaction_date >= NOW() - INTERVAL '12 months'
						AND t.transaction_type = 'expense'
					GROUP BY TO_CHAR(t.transaction_date, 'YYYY-MM')
					ORDER BY label
				`).
				Option("colors", []string{"#ef4444"}).
				OnDataPointClick(lens.ActionConfig{
					Type: lens.ActionTypeNavigation,
					Navigation: &lens.NavigationAction{
						URL:    "/transactions?month={month}&type=expense",
						Target: "_blank",
						Variables: map[string]string{
							"month": "{label}",
						},
					},
				}).
				Build(),
		).
		Panel(
			builder.StackedBarChart().
				ID("monthly-expenses-by-category").
				Title("Monthly Expenses by Category").
				Position(6, 2).
				Size(6, 4).
				DataSource("postgres").
				Query(`
					SELECT 
						TO_CHAR(t.transaction_date, 'YYYY-MM') as category,
						ec.name as series,
						(SUM(t.amount) / 100.0)::float8 as value
					FROM transactions t
					JOIN expenses e ON t.id = e.transaction_id
					JOIN expense_categories ec ON e.category_id = ec.id
					WHERE t.transaction_date >= NOW() - INTERVAL '6 months'
						AND t.transaction_type = 'expense'
					GROUP BY TO_CHAR(t.transaction_date, 'YYYY-MM'), ec.name
					ORDER BY category, series
				`).
				OnDrillDown(map[string]string{
					"month":    "{categoryName}",
					"category": "{seriesName}",
				}, "expense-details").
				Build(),
		).
		Panel(
			builder.LineChart().
				ID("account-balances").
				Title("Account Balances Over Time").
				Position(0, 6).
				Size(12, 4).
				DataSource("postgres").
				Query(`
					SELECT 
						ma.name as label,
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
				Position(0, 10).
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
				Position(6, 10).
				Size(6, 4).
				DataSource("postgres").
				Query(`
					SELECT 
						c.name as label,
						COUNT(p.id)::float as value
					FROM counterparty c
					JOIN payments p ON c.id = p.counterparty_id
					JOIN transactions t ON p.transaction_id = t.id
					WHERE t.transaction_date >= NOW() - INTERVAL '30 days'
					GROUP BY c.name
					ORDER BY value DESC
				`).
				Option("colors", []string{"#06b6d4"}).
				OnModal("Counterparty Details", "Counterparty: {label}<br>Transaction Count: {value}", "/api/counterparty/{label}/details").
				Build(),
		).
		Panel(
			builder.GaugeChart().
				ID("expense-budget-usage").
				Title("Monthly Budget Usage").
				Position(0, 14).
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
				Position(4, 14).
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
				OnCustom("openTransactionDetails", map[string]string{
					"transactionId": "{rowId}",
					"amount":        "{amount}",
				}).
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
		middleware.ProvideDynamicLogo(c.app),
		middleware.ProvideLocalizer(c.app),
		middleware.NavItems(),
		middleware.WithPageContext(),
	)
	router.HandleFunc("/", c.Get)
}

func (c *DashboardController) Get(w http.ResponseWriter, r *http.Request) {
	// Create finance dashboard configuration
	dashboardConfig := c.createFinanceDashboard()
	exec := c.ensureExecutor()

	// Execute dashboard queries if executor is available
	var dashboardResult *executor.DashboardResult
	if exec != nil {
		err := composables.InTx(r.Context(), func(txCtx context.Context) error {
			ctx, cancel := context.WithTimeout(txCtx, 30*time.Second)
			defer cancel()

			result, err := exec.ExecuteDashboard(ctx, dashboardConfig)
			if err != nil {
				log.Printf("Failed to execute dashboard queries: %v", err)
				dashboardResult = &executor.DashboardResult{
					PanelResults: make(map[string]*executor.ExecutionResult),
					Duration:     0,
					Errors:       []error{err},
					ExecutedAt:   time.Now(),
				}
				return nil
			}
			dashboardResult = result
			return nil
		})
		if err != nil {
			log.Printf("Dashboard transaction failed: %v", err)
			dashboardResult = &executor.DashboardResult{
				PanelResults: make(map[string]*executor.ExecutionResult),
				Duration:     0,
				Errors:       []error{err},
				ExecutedAt:   time.Now(),
			}
		}
	}

	props := &dashboard.IndexPageProps{
		Dashboard:       dashboardConfig,
		DashboardResult: dashboardResult,
	}
	templ.Handler(dashboard.Index(props)).ServeHTTP(w, r)
}

func (c *DashboardController) ensureExecutor() executor.Executor {
	const op = serrors.Op("DashboardController.ensureExecutor")
	c.executorMu.Lock()
	defer c.executorMu.Unlock()

	if c.executor != nil {
		return c.executor
	}
	if c.executorInitErr != nil && time.Since(c.executorInitAt) < executorInitCooldown {
		return nil
	}
	c.executorInitErr = nil

	if c.app == nil || c.app.DB() == nil {
		c.executorInitErr = serrors.E(op, errors.New("database pool is not available"))
		c.executorInitAt = time.Now()
		log.Printf("Failed to create dashboard executor: %v", c.executorInitErr)
		return nil
	}

	pgDataSource, err := postgres.NewPostgreSQLDataSourceFromPool(c.app.DB(), postgres.Config{
		QueryTimeout: 30 * time.Second,
	})
	if err != nil {
		c.executorInitErr = serrors.E(op, err)
		c.executorInitAt = time.Now()
		log.Printf("Failed to create PostgreSQL data source for dashboard: %v", c.executorInitErr)
		return nil
	}

	exec := executor.NewExecutor(nil, 30*time.Second)
	if err := exec.RegisterDataSource("postgres", pgDataSource); err != nil {
		c.executorInitErr = serrors.E(op, err)
		c.executorInitAt = time.Now()
		log.Printf("Failed to register dashboard data source: %v", c.executorInitErr)
		if closeErr := pgDataSource.Close(); closeErr != nil {
			log.Printf("Failed to close dashboard data source: %v", serrors.E(op, closeErr))
		}
		return nil
	}

	c.executor = exec
	return c.executor
}

func (c *DashboardController) Close() error {
	const op = serrors.Op("DashboardController.Close")
	c.executorMu.Lock()
	defer c.executorMu.Unlock()

	if c.executor == nil {
		return nil
	}

	err := c.executor.Close()
	c.executor = nil
	c.executorInitErr = nil
	if err != nil {
		return serrors.E(op, err)
	}
	return nil
}
