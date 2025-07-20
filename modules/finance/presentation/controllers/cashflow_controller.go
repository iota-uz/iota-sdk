package controllers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	moneyaccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/query"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/mappers"
	reports "github.com/iota-uz/iota-sdk/modules/finance/presentation/templates/pages/reports"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

type CashflowController struct {
	app                    application.Application
	financialReportService *services.FinancialReportService
	moneyAccountService    *services.MoneyAccountService
	queryRepo              query.FinancialReportsQueryRepository
	basePath               string
}

func NewCashflowController(app application.Application) application.Controller {
	basePath := "/finance/reports"

	return &CashflowController{
		app:                    app,
		financialReportService: app.Service(services.FinancialReportService{}).(*services.FinancialReportService),
		moneyAccountService:    app.Service(services.MoneyAccountService{}).(*services.MoneyAccountService),
		queryRepo:              query.NewPgFinancialReportsQueryRepository(),
		basePath:               basePath,
	}
}

func (c *CashflowController) Key() string {
	return c.basePath + "/cashflow"
}

func (c *CashflowController) Register(r *mux.Router) {
	commonMiddleware := []mux.MiddlewareFunc{
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.ProvideDynamicLogo(c.app),
		middleware.Tabs(),
		middleware.ProvideLocalizer(c.app.Bundle()),
		middleware.NavItems(),
		middleware.WithPageContext(),
	}

	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(commonMiddleware...)

	// Cashflow statement routes
	router.HandleFunc("/cashflow", c.GetCashflowStatementPage).Methods(http.MethodGet)
	router.HandleFunc("/cashflow/generate", c.GenerateCashflowStatement).Methods(http.MethodPost)
	router.HandleFunc("/cashflow/data", c.GetCashflowStatementData).Methods(http.MethodGet)
}

// GetCashflowStatementPage renders the cashflow statement page
func (c *CashflowController) GetCashflowStatementPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get all money accounts for the dropdown
	accounts, err := c.moneyAccountService.GetAll(ctx)
	if err != nil {
		http.Error(w, "Failed to get accounts", http.StatusInternalServerError)
		return
	}

	// Set default date range (fiscal year)
	now := time.Now()
	// Assuming fiscal year starts January 1st
	startOfYear := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
	endOfYear := time.Date(now.Year(), 12, 31, 23, 59, 59, 999999999, now.Location())

	// Set default form values
	formData := map[string]interface{}{
		"accounts":  accounts,
		"startDate": startOfYear.Format("2006-01-02"),
		"endDate":   endOfYear.Format("2006-01-02"),
		"accountId": "all", // Default to all accounts
	}

	// Get account ID from query params if provided
	accountIDStr := r.URL.Query().Get("account_id")
	if accountIDStr != "" {
		formData["accountId"] = accountIDStr
	}

	// Always generate and show the report
	if err := c.generateAndRenderReport(w, r, ctx, formData, startOfYear, endOfYear); err != nil {
		// If there's an error, show the page without report
		component := reports.CashflowStatementPage(formData)
		templ.Handler(component, templ.WithStreaming()).ServeHTTP(w, r)
	}
}

// generateAndRenderReport generates cashflow report and renders it
func (c *CashflowController) generateAndRenderReport(w http.ResponseWriter, r *http.Request, ctx context.Context, formData map[string]interface{}, startDate, endDate time.Time) error {
	accountIDStr, _ := formData["accountId"].(string)

	// Handle "all accounts" case
	if accountIDStr == "all" || accountIDStr == "" {
		// For now, we'll use the first account if available
		// TODO: Implement proper all-accounts aggregation
		if accountsList, ok := formData["accounts"].([]interface{}); ok && len(accountsList) > 0 {
			// Get the first account
			for _, acc := range accountsList {
				if account, ok := acc.(interface{ ID() uuid.UUID }); ok {
					accountIDStr = account.ID().String()
					break
				}
			}
		} else if accountsList, ok := formData["accounts"].([]moneyaccount.Account); ok && len(accountsList) > 0 {
			// Handle typed slice case
			accountIDStr = accountsList[0].ID().String()
		} else {
			return nil // No accounts, show empty form
		}
	}

	// Parse account ID
	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		return err
	}

	// Generate cashflow statement
	cashflowStatement, err := c.financialReportService.GenerateCashflowStatement(ctx, accountID, startDate, endDate)
	if err != nil {
		return err
	}

	// Get monthly data for breakdown
	monthlyInflows, monthlyOutflows, err := c.queryRepo.GetMonthlyCashflowByCategory(ctx, accountID, startDate, endDate)
	if err != nil {
		// Fall back to basic view
		viewModel := mappers.ToCashflowStatementViewModel(cashflowStatement, nil)
		component := reports.CashflowStatementPageWithReport(formData, viewModel)
		templ.Handler(component, templ.WithStreaming()).ServeHTTP(w, r)
		return err
	}

	// Get account name
	account, err := c.moneyAccountService.GetByID(ctx, accountID)
	accountName := "All Accounts"
	if err == nil && account != nil {
		accountName = account.Name()
	}

	// Override name if showing all accounts
	if formData["accountId"] == "all" {
		accountName = "All Accounts"
	}

	// Convert to view model with monthly breakdown
	viewModel := mappers.ToCashflowStatementViewModelWithMonthlyData(
		cashflowStatement,
		accountName,
		monthlyInflows,
		monthlyOutflows,
	)
	component := reports.CashflowStatementPageWithReport(formData, viewModel)
	templ.Handler(component, templ.WithStreaming()).ServeHTTP(w, r)
	return nil
}

// GenerateCashflowStatement handles form submission for cashflow statement generation
func (c *CashflowController) GenerateCashflowStatement(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse form data
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	// Get dates
	startDate, err := time.Parse("2006-01-02", r.FormValue("start_date"))
	if err != nil {
		http.Error(w, "Invalid start date format", http.StatusBadRequest)
		return
	}

	endDate, err := time.Parse("2006-01-02", r.FormValue("end_date"))
	if err != nil {
		http.Error(w, "Invalid end date format", http.StatusBadRequest)
		return
	}

	// Get all accounts for the dropdown
	accounts, err := c.moneyAccountService.GetAll(ctx)
	if err != nil {
		http.Error(w, "Failed to get accounts", http.StatusInternalServerError)
		return
	}

	// Set form data
	formData := map[string]interface{}{
		"accounts":  accounts,
		"startDate": startDate.Format("2006-01-02"),
		"endDate":   endDate.Format("2006-01-02"),
		"accountId": r.FormValue("account_id"),
	}

	// Generate and render the report
	if err := c.generateAndRenderReport(w, r, ctx, formData, startDate, endDate); err != nil {
		// Return just the error message in a simple div
		errorComponent := `<div class="text-red-600 p-4">Error generating report: ` + err.Error() + `</div>`
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(errorComponent))
		return
	}
}

// GetCashflowStatementData returns cashflow statement data as JSON
func (c *CashflowController) GetCashflowStatementData(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters
	accountIDStr := r.URL.Query().Get("account_id")
	if accountIDStr == "" || accountIDStr == "all" {
		http.Error(w, "Account ID is required", http.StatusBadRequest)
		return
	}

	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		http.Error(w, "Invalid account ID", http.StatusBadRequest)
		return
	}

	// Parse dates
	startDateStr := r.URL.Query().Get("start_date")
	endDateStr := r.URL.Query().Get("end_date")

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		http.Error(w, "Invalid start date format", http.StatusBadRequest)
		return
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		http.Error(w, "Invalid end date format", http.StatusBadRequest)
		return
	}

	// Generate cashflow statement
	cashflowStatement, err := c.financialReportService.GenerateCashflowStatement(
		ctx,
		accountID,
		startDate,
		endDate,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get account name
	account, err := c.moneyAccountService.GetByID(ctx, accountID)
	accountName := "Unknown Account"
	if err == nil && account != nil {
		accountName = account.Name()
	}

	// Convert to response DTO
	responseDTO := mappers.ToCashflowStatementResponseDTO(cashflowStatement, accountName)

	// Return as JSON
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(responseDTO); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}
