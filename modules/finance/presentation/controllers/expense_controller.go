package controllers

import (
	"errors"
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/components/base/pagination"
	"github.com/iota-uz/iota-sdk/components/export"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/exportconfig"
	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/finance/permissions"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/mappers"
	expensesui "github.com/iota-uz/iota-sdk/modules/finance/presentation/templates/pages/expenses"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/di"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/iota-uz/iota-sdk/pkg/shared"
	"github.com/sirupsen/logrus"
)

type ExpenseController struct {
	app      application.Application
	basePath string
}

type ExpensePaginationResponse struct {
	Expenses        []*viewmodels.Expense
	PaginationState *pagination.State
}

func NewExpensesController(app application.Application) application.Controller {
	basePath := "/finance/expenses"

	controller := &ExpenseController{
		app:      app,
		basePath: basePath,
	}

	return controller
}

func (c *ExpenseController) Key() string {
	return c.basePath
}

func (c *ExpenseController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.Tabs(),
		middleware.ProvideLocalizer(c.app.Bundle()),
		middleware.NavItems(),
		middleware.WithPageContext(),
	)
	router.HandleFunc("", di.H(c.List)).Methods(http.MethodGet)
	router.HandleFunc("/export", di.H(c.Export)).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}", di.H(c.GetEdit)).Methods(http.MethodGet)
	router.HandleFunc("/new", di.H(c.GetNew)).Methods(http.MethodGet)
	router.HandleFunc("", di.H(c.Create)).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}", di.H(c.Update)).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}", di.H(c.Delete)).Methods(http.MethodDelete)
}

func (c *ExpenseController) List(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	expenseService *services.ExpenseService,
) {
	params := composables.UsePaginated(r)
	findParams := &expense.FindParams{
		Offset: params.Offset,
		Limit:  params.Limit,
		SortBy: expense.SortBy{
			Fields: []repo.SortByField[expense.Field]{
				{
					Field:     expense.CreatedAt,
					Ascending: false,
				},
			},
		},
		Search: r.URL.Query().Get("Search"),
	}

	if v := r.URL.Query().Get("CreatedAt.To"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			logger.Errorf("Error parsing CreatedAt.To: %v", err)
			http.Error(w, "Invalid date format", http.StatusBadRequest)
			return
		}
		findParams.Filters = append(findParams.Filters, expense.Filter{
			Column: expense.CreatedAt,
			Filter: repo.Lt(t),
		})
	}

	if v := r.URL.Query().Get("CreatedAt.From"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			logger.Errorf("Error parsing CreatedAt.From: %v", err)
			http.Error(w, "Invalid date format", http.StatusBadRequest)
			return
		}
		findParams.Filters = append(findParams.Filters, expense.Filter{
			Column: expense.CreatedAt,
			Filter: repo.Gt(t),
		})
	}

	expenseEntities, err := expenseService.GetPaginated(r.Context(), findParams)
	if err != nil {
		logger.Errorf("Error retrieving expenses: %v", err)
		http.Error(w, "Error retrieving expenses", http.StatusInternalServerError)
		return
	}

	total, err := expenseService.Count(r.Context(), &expense.FindParams{})
	if err != nil {
		logger.Errorf("Error counting expenses: %v", err)
		http.Error(w, "Error counting expenses", http.StatusInternalServerError)
		return
	}

	props := &expensesui.IndexPageProps{
		Expenses:        mapping.MapViewModels(expenseEntities, mappers.ExpenseToViewModel),
		PaginationState: pagination.New(c.basePath, params.Page, int(total), params.Limit),
	}

	if htmx.IsHxRequest(r) {
		templ.Handler(expensesui.ExpensesTable(props), templ.WithStreaming()).ServeHTTP(w, r)
	} else {
		templ.Handler(expensesui.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *ExpenseController) Export(
	r *http.Request,
	w http.ResponseWriter,
	excelService *coreservices.ExcelExportService,
) {
	if err := composables.CanUser(r.Context(), permissions.ExpenseRead); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	format, ok := export.GetExportFormat(r)
	if !ok {
		http.Error(w, "Invalid export format", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		http.Error(w, "Error getting tenant ID", http.StatusInternalServerError)
		return
	}

	// Export all expenses without any filters
	query := `
		SELECT 
			ec.name as category_name,
			tr.amount,
			tr.accounting_period,
			tr.transaction_date as date,
			tr.comment,
			ex.created_at,
			ex.updated_at
		FROM expenses ex 
		LEFT JOIN transactions tr ON tr.id = ex.transaction_id
		LEFT JOIN expense_categories ec ON ec.id = ex.category_id
		WHERE ex.tenant_id = $1
		ORDER BY ex.created_at DESC`

	args := []interface{}{tenantID}

	// For now, only handle Excel export (as per the example)
	// TODO: Extend the exporter service to support other formats
	switch format {
	case export.ExportFormatExcel:
		queryObj := exportconfig.NewQuery(query, args...)
		config := exportconfig.New(exportconfig.WithFilename("expenses_export"))
		upload, err := excelService.ExportFromQuery(
			ctx,
			queryObj,
			config,
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if htmx.IsHxRequest(r) {
			htmx.Redirect(w, upload.URL().String())
		} else {
			http.Redirect(w, r, upload.URL().String(), http.StatusSeeOther)
		}
	default:
		http.Error(w, "Export format not yet supported", http.StatusNotImplemented)
	}
}

func (c *ExpenseController) GetEdit(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	expenseService *services.ExpenseService,
	moneyAccountService *services.MoneyAccountService,
	expenseCategoryService *services.ExpenseCategoryService,
) {
	id, err := shared.ParseUUID(r)
	if err != nil {
		logger.Errorf("Error parsing expense ID: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	entity, err := expenseService.GetByID(r.Context(), id)
	if err != nil {
		logger.Errorf("Error retrieving expense: %v", err)
		http.Error(w, "Error retrieving expense", http.StatusInternalServerError)
		return
	}

	accounts, err := moneyAccountService.GetAll(r.Context())
	if err != nil {
		logger.Errorf("Error retrieving accounts: %v", err)
		http.Error(w, "Error retrieving accounts", http.StatusInternalServerError)
		return
	}

	categories, err := expenseCategoryService.GetAll(r.Context())
	if err != nil {
		logger.Errorf("Error retrieving categories: %v", err)
		http.Error(w, "Error retrieving categories", http.StatusInternalServerError)
		return
	}

	props := &expensesui.EditPageProps{
		Expense:    mappers.ExpenseToViewModel(entity),
		Accounts:   mapping.MapViewModels(accounts, mappers.MoneyAccountToViewModel),
		Categories: mapping.MapViewModels(categories, mappers.ExpenseCategoryToViewModel),
		Errors:     map[string]string{},
	}
	templ.Handler(expensesui.Edit(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *ExpenseController) Delete(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	expenseService *services.ExpenseService,
) {
	id, err := shared.ParseUUID(r)
	if err != nil {
		logger.Errorf("Error parsing expense ID: %v", err)
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}

	if _, err := expenseService.Delete(r.Context(), id); err != nil {
		logger.Errorf("Error deleting expense: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	shared.Redirect(w, r, c.basePath)
}

func (c *ExpenseController) Update(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	expenseService *services.ExpenseService,
	moneyAccountService *services.MoneyAccountService,
	expenseCategoryService *services.ExpenseCategoryService,
) {
	id, err := shared.ParseUUID(r)
	if err != nil {
		logger.Errorf("Error parsing expense ID: %v", err)
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}

	dto, err := composables.UseForm(&dtos.ExpenseUpdateDTO{}, r)
	if err != nil {
		logger.Errorf("Error parsing form: %v", err)
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	existing, err := expenseService.GetByID(r.Context(), id)
	if errors.Is(err, persistence.ErrExpenseNotFound) {
		logger.Errorf("Expense not found: %v", err)
		http.Error(w, "Expense not found", http.StatusNotFound)
		return
	}
	if err != nil {
		logger.Errorf("Error retrieving expense: %v", err)
		http.Error(w, "Error retrieving expense", http.StatusInternalServerError)
		return
	}

	categoryID, err := uuid.Parse(dto.CategoryID)
	if err != nil {
		logger.Errorf("Invalid category ID: %v", err)
		http.Error(w, "Invalid category ID", http.StatusBadRequest)
		return
	}

	cat, err := expenseCategoryService.GetByID(r.Context(), categoryID)
	if errors.Is(err, persistence.ErrExpenseCategoryNotFound) {
		logger.Errorf("Expense category not found: %v", err)
		http.Error(w, "Expense category not found", http.StatusBadRequest)
		return
	}

	if err != nil {
		logger.Errorf("Error retrieving expense category: %v", err)
		http.Error(w, "Error retrieving expense category", http.StatusInternalServerError)
		return
	}

	if errorsMap, ok := dto.Ok(r.Context()); !ok {
		accounts, err := moneyAccountService.GetAll(r.Context())
		if err != nil {
			logger.Errorf("Error retrieving accounts: %v", err)
			http.Error(w, "Error retrieving accounts", http.StatusInternalServerError)
			return
		}

		categories, err := expenseCategoryService.GetAll(r.Context())
		if err != nil {
			logger.Errorf("Error retrieving categories: %v", err)
			http.Error(w, "Error retrieving categories", http.StatusInternalServerError)
			return
		}

		props := &expensesui.EditPageProps{
			Expense:    mappers.ExpenseToViewModel(existing),
			Accounts:   mapping.MapViewModels(accounts, mappers.MoneyAccountToViewModel),
			Categories: mapping.MapViewModels(categories, mappers.ExpenseCategoryToViewModel),
			Errors:     errorsMap,
		}
		templ.Handler(expensesui.EditForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	entity, err := dto.Apply(existing, cat)
	if err != nil {
		logger.Errorf("Error converting DTO to entity: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err := expenseService.Update(r.Context(), entity); err != nil {
		logger.Errorf("Error updating expense: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}

func (c *ExpenseController) GetNew(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	moneyAccountService *services.MoneyAccountService,
	expenseCategoryService *services.ExpenseCategoryService,
) {
	accounts, err := moneyAccountService.GetAll(r.Context())
	if err != nil {
		logger.Errorf("Error retrieving accounts: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	categories, err := expenseCategoryService.GetAll(r.Context())
	if err != nil {
		logger.Errorf("Error retrieving categories: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	props := &expensesui.CreatePageProps{
		Accounts:   mapping.MapViewModels(accounts, mappers.MoneyAccountToViewModel),
		Categories: mapping.MapViewModels(categories, mappers.ExpenseCategoryToViewModel),
		Errors:     map[string]string{},
		Expense:    &viewmodels.Expense{},
	}
	templ.Handler(expensesui.New(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *ExpenseController) Create(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	expenseService *services.ExpenseService,
	moneyAccountService *services.MoneyAccountService,
	expenseCategoryService *services.ExpenseCategoryService,
) {
	dto, err := composables.UseForm(&dtos.ExpenseCreateDTO{}, r)
	if err != nil {
		logger.Errorf("Error parsing form: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	tenantID, err := composables.UseTenantID(r.Context())
	if err != nil {
		logger.Errorf("Error getting tenant ID: %v", err)
		http.Error(w, "Error getting tenant ID", http.StatusInternalServerError)
		return
	}

	categoryID, err := uuid.Parse(dto.CategoryID)
	if err != nil {
		logger.Errorf("Invalid category ID: %v", err)
		http.Error(w, "Invalid category ID", http.StatusBadRequest)
		return
	}

	accountID, err := uuid.Parse(dto.AccountID)
	if err != nil {
		logger.Errorf("Invalid account ID: %v", err)
		http.Error(w, "Invalid account ID", http.StatusBadRequest)
		return
	}

	cat, err := expenseCategoryService.GetByID(r.Context(), categoryID)
	if errors.Is(err, persistence.ErrExpenseCategoryNotFound) {
		logger.Errorf("Expense category not found: %v", err)
		http.Error(w, "Expense category not found", http.StatusBadRequest)
		return
	}
	if err != nil {
		logger.Errorf("Error retrieving expense category: %v", err)
		http.Error(w, "Error retrieving expense category", http.StatusInternalServerError)
		return
	}

	account, err := moneyAccountService.GetByID(r.Context(), accountID)
	if err != nil {
		logger.Errorf("Error retrieving account: %v", err)
		http.Error(w, "Error retrieving account", http.StatusInternalServerError)
		return
	}

	if errorsMap, ok := dto.Ok(r.Context()); !ok {
		accounts, err := moneyAccountService.GetAll(r.Context())
		if err != nil {
			logger.Errorf("Error retrieving accounts: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		entity, err := dto.ToEntityWithReferences(tenantID, account, cat)
		if err != nil {
			logger.Errorf("Error converting DTO to entity: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		categories, err := expenseCategoryService.GetAll(r.Context())
		if err != nil {
			logger.Errorf("Error retrieving categories: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		props := &expensesui.CreatePageProps{
			Accounts:   mapping.MapViewModels(accounts, mappers.MoneyAccountToViewModel),
			Errors:     errorsMap,
			Categories: mapping.MapViewModels(categories, mappers.ExpenseCategoryToViewModel),
			Expense:    mappers.ExpenseToViewModel(entity),
		}
		templ.Handler(expensesui.CreateForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	entity, err := dto.ToEntityWithReferences(tenantID, account, cat)
	if err != nil {
		logger.Errorf("Error converting DTO to entity: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err := expenseService.Create(r.Context(), entity); err != nil {
		logger.Errorf("Error creating expense: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}
