package controllers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/query"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/mappers"
	reports "github.com/iota-uz/iota-sdk/modules/finance/presentation/templates/pages/reports"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

type FinancialReportController struct {
	app                    application.Application
	financialReportService *services.FinancialReportService
	queryRepo              query.FinancialReportsQueryRepository
	basePath               string
}

func NewFinancialReportController(app application.Application) application.Controller {
	basePath := "/finance/reports"

	return &FinancialReportController{
		app:                    app,
		financialReportService: app.Service(services.FinancialReportService{}).(*services.FinancialReportService),
		queryRepo:              query.NewPgFinancialReportsQueryRepository(),
		basePath:               basePath,
	}
}

func (c *FinancialReportController) Key() string {
	return c.basePath
}

func (c *FinancialReportController) Register(r *mux.Router) {
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

	// Income statement routes
	router.HandleFunc("/income-statement", c.GetIncomeStatementPage).Methods(http.MethodGet)
	router.HandleFunc("/income-statement/generate", c.GenerateIncomeStatement).Methods(http.MethodPost)
	router.HandleFunc("/income-statement/data", c.GetIncomeStatementData).Methods(http.MethodGet)
}

// GetIncomeStatementPage renders the income statement page
func (c *FinancialReportController) GetIncomeStatementPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Set default date range (current fiscal year - Jan 1 to Dec 31)
	now := time.Now()
	startOfYear := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
	endOfYear := time.Date(now.Year(), 12, 31, 23, 59, 59, 0, now.Location())

	// Set default form values
	formData := map[string]interface{}{
		"startDate": startOfYear.Format("2006-01-02"),
		"endDate":   endOfYear.Format("2006-01-02"),
	}

	// Automatically generate the current fiscal year income statement
	incomeStatement, err := c.financialReportService.GenerateIncomeStatement(
		ctx,
		startOfYear,
		endOfYear,
	)

	var reportComponent templ.Component
	if err != nil {
		// If there's an error, show the form without the report
		reportComponent = reports.IncomeStatementPage(formData)
	} else {
		// Get monthly data for proper breakdown
		monthlyIncomeData, err := c.queryRepo.GetMonthlyIncomeByCategory(ctx, startOfYear, endOfYear)
		if err != nil {
			// If monthly data fails, fall back to basic view
			viewModel := mappers.ToIncomeStatementViewModel(incomeStatement)
			reportComponent = reports.IncomeStatementPageWithReport(formData, viewModel)
		} else {
			monthlyExpenseData, err := c.queryRepo.GetMonthlyExpensesByCategory(ctx, startOfYear, endOfYear)
			if err != nil {
				// If monthly expense data fails, fall back to basic view
				viewModel := mappers.ToIncomeStatementViewModel(incomeStatement)
				reportComponent = reports.IncomeStatementPageWithReport(formData, viewModel)
			} else {
				// Convert to view model with monthly breakdown
				viewModel := mappers.ToIncomeStatementViewModelWithMonthlyData(incomeStatement, monthlyIncomeData, monthlyExpenseData)
				reportComponent = reports.IncomeStatementPageWithReport(formData, viewModel)
			}
		}
	}

	templ.Handler(reportComponent, templ.WithStreaming()).ServeHTTP(w, r)
}

// GenerateIncomeStatement handles form submission for income statement generation
func (c *FinancialReportController) GenerateIncomeStatement(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse form data
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	// Parse start date
	startDateStr := r.FormValue("start_date")
	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		http.Error(w, "Invalid start date format", http.StatusBadRequest)
		return
	}

	// Parse end date
	endDateStr := r.FormValue("end_date")
	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		http.Error(w, "Invalid end date format", http.StatusBadRequest)
		return
	}

	// Validate date range
	if startDate.After(endDate) {
		http.Error(w, "Start date must be before end date", http.StatusBadRequest)
		return
	}

	// Generate income statement
	incomeStatement, err := c.financialReportService.GenerateIncomeStatement(
		ctx,
		startDate,
		endDate,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get monthly data for proper breakdown
	monthlyIncomeData, err := c.queryRepo.GetMonthlyIncomeByCategory(ctx, startDate, endDate)
	if err != nil {
		// If monthly data fails, fall back to basic view
		viewModel := mappers.ToIncomeStatementViewModel(incomeStatement)
		component := reports.IncomeStatementReport(viewModel)
		templ.Handler(component, templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	monthlyExpenseData, err := c.queryRepo.GetMonthlyExpensesByCategory(ctx, startDate, endDate)
	if err != nil {
		// If monthly expense data fails, fall back to basic view
		viewModel := mappers.ToIncomeStatementViewModel(incomeStatement)
		component := reports.IncomeStatementReport(viewModel)
		templ.Handler(component, templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	// Convert to view model with monthly breakdown
	viewModel := mappers.ToIncomeStatementViewModelWithMonthlyData(incomeStatement, monthlyIncomeData, monthlyExpenseData)
	component := reports.IncomeStatementReport(viewModel)
	templ.Handler(component, templ.WithStreaming()).ServeHTTP(w, r)
}

// GetIncomeStatementData returns income statement data as JSON for AJAX requests
func (c *FinancialReportController) GetIncomeStatementData(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters
	startDateStr := r.URL.Query().Get("start_date")
	endDateStr := r.URL.Query().Get("end_date")

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

	// Validate date range
	if startDate.After(endDate) {
		http.Error(w, "Start date must be before end date", http.StatusBadRequest)
		return
	}

	// Generate income statement
	w.Header().Set("Content-Type", "application/json")

	incomeStatement, err := c.financialReportService.GenerateIncomeStatement(
		ctx,
		startDate,
		endDate,
	)
	if err != nil {
		http.Error(w, "Failed to generate income statement", http.StatusInternalServerError)
		return
	}

	// Get monthly data for proper breakdown
	monthlyIncomeData, err := c.queryRepo.GetMonthlyIncomeByCategory(ctx, startDate, endDate)
	if err != nil {
		// If monthly data fails, fall back to basic view
		viewModel := mappers.ToIncomeStatementViewModel(incomeStatement)
		response := mappers.ToIncomeStatementResponseDTO(viewModel)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
		return
	}

	monthlyExpenseData, err := c.queryRepo.GetMonthlyExpensesByCategory(ctx, startDate, endDate)
	if err != nil {
		// If monthly expense data fails, fall back to basic view
		viewModel := mappers.ToIncomeStatementViewModel(incomeStatement)
		response := mappers.ToIncomeStatementResponseDTO(viewModel)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
		return
	}

	// Convert to view model with monthly breakdown
	viewModel := mappers.ToIncomeStatementViewModelWithMonthlyData(incomeStatement, monthlyIncomeData, monthlyExpenseData)
	response := mappers.ToIncomeStatementResponseDTO(viewModel)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
