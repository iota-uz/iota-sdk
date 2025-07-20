package controllers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/query"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/mappers"
	reports "github.com/iota-uz/iota-sdk/modules/finance/presentation/templates/pages/reports"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/shared"
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

	// Set default date range (current month)
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	endOfMonth := startOfMonth.AddDate(0, 1, -1)

	// Set default form values
	formData := map[string]interface{}{
		"accounts":  accounts,
		"startDate": startOfMonth.Format("2006-01-02"),
		"endDate":   endOfMonth.Format("2006-01-02"),
	}

	// Get account ID from query params if provided
	accountIDStr := r.URL.Query().Get("account_id")
	if accountIDStr != "" {
		accountID, err := uuid.Parse(accountIDStr)
		if err == nil {
			formData["accountId"] = accountID.String()

			// Generate cashflow statement for the selected account
			cashflowStatement, err := c.financialReportService.GenerateCashflowStatement(
				ctx,
				accountID,
				startOfMonth,
				endOfMonth,
			)

			if err == nil {
				// Get monthly data for breakdown
				monthlyInflows, monthlyOutflows, err := c.queryRepo.GetMonthlyCashflowByCategory(ctx, accountID, startOfMonth, endOfMonth)
				if err != nil {
					// Fall back to basic view
					viewModel := mappers.ToCashflowStatementViewModel(cashflowStatement, nil)
					component := reports.CashflowStatementPageWithReport(formData, viewModel)
					templ.Handler(component, templ.WithStreaming()).ServeHTTP(w, r)
					return
				}

				// Get account name
				account, err := c.moneyAccountService.GetByID(ctx, accountID)
				accountName := "Unknown Account"
				if err == nil && account != nil {
					accountName = account.Name()
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
				return
			}
		}
	}

	// Show form without report
	component := reports.CashflowStatementPage(formData)
	templ.Handler(component, templ.WithStreaming()).ServeHTTP(w, r)
}

// GenerateCashflowStatement handles form submission for cashflow statement generation
func (c *CashflowController) GenerateCashflowStatement(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse form data
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	// Create and validate DTO
	dto := &dtos.CashflowStatementRequestDTO{
		AccountID: uuid.MustParse(r.FormValue("account_id")),
		StartDate: shared.DateOnly(time.Time{}), // Will be parsed below
		EndDate:   shared.DateOnly(time.Time{}), // Will be parsed below
	}

	// Parse dates
	startDate, err := time.Parse("2006-01-02", r.FormValue("start_date"))
	if err != nil {
		http.Error(w, "Invalid start date format", http.StatusBadRequest)
		return
	}
	dto.StartDate = shared.DateOnly(startDate)

	endDate, err := time.Parse("2006-01-02", r.FormValue("end_date"))
	if err != nil {
		http.Error(w, "Invalid end date format", http.StatusBadRequest)
		return
	}
	dto.EndDate = shared.DateOnly(endDate)

	// Validate DTO
	if errors, ok := dto.Ok(ctx); !ok {
		// Convert validation errors to JSON and send back
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(errors); err != nil {
			http.Error(w, "Failed to encode validation errors", http.StatusInternalServerError)
		}
		return
	}

	// Generate cashflow statement
	cashflowStatement, err := c.financialReportService.GenerateCashflowStatement(
		ctx,
		dto.AccountID,
		time.Time(dto.StartDate),
		time.Time(dto.EndDate),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get monthly data for breakdown
	monthlyInflows, monthlyOutflows, err := c.queryRepo.GetMonthlyCashflowByCategory(
		ctx,
		dto.AccountID,
		time.Time(dto.StartDate),
		time.Time(dto.EndDate),
	)
	if err != nil {
		// Fall back to basic view
		viewModel := mappers.ToCashflowStatementViewModel(cashflowStatement, nil)
		component := reports.CashflowStatementReport(viewModel)
		templ.Handler(component, templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	// Get account name
	account, err := c.moneyAccountService.GetByID(ctx, dto.AccountID)
	accountName := "Unknown Account"
	if err == nil && account != nil {
		accountName = account.Name()
	}

	// Convert to view model with monthly breakdown
	viewModel := mappers.ToCashflowStatementViewModelWithMonthlyData(
		cashflowStatement,
		accountName,
		monthlyInflows,
		monthlyOutflows,
	)
	component := reports.CashflowStatementReport(viewModel)
	templ.Handler(component, templ.WithStreaming()).ServeHTTP(w, r)
}

// GetCashflowStatementData returns cashflow statement data as JSON for AJAX requests
func (c *CashflowController) GetCashflowStatementData(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters
	accountIDStr := r.URL.Query().Get("account_id")
	startDateStr := r.URL.Query().Get("start_date")
	endDateStr := r.URL.Query().Get("end_date")

	// Parse account ID
	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		http.Error(w, "Invalid account ID", http.StatusBadRequest)
		return
	}

	// Parse dates
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
