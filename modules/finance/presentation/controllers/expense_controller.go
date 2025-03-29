package controllers

import (
	"net/http"

	"github.com/iota-uz/iota-sdk/components/base/pagination"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense"
	category "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense_category"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/mappers"
	expenses2 "github.com/iota-uz/iota-sdk/modules/finance/presentation/templates/pages/expenses"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/shared"

	"github.com/a-h/templ"
	"github.com/go-faster/errors"
	"github.com/gorilla/mux"
)

type ExpenseController struct {
	app                    application.Application
	moneyAccountService    *services.MoneyAccountService
	expenseService         *services.ExpenseService
	expenseCategoryService *services.ExpenseCategoryService
	basePath               string
}

type ExpensePaginationResponse struct {
	Expenses        []*viewmodels.Expense
	PaginationState *pagination.State
}

func NewExpensesController(app application.Application) application.Controller {
	return &ExpenseController{
		app:                    app,
		moneyAccountService:    app.Service(services.MoneyAccountService{}).(*services.MoneyAccountService),
		expenseService:         app.Service(services.ExpenseService{}).(*services.ExpenseService),
		expenseCategoryService: app.Service(services.ExpenseCategoryService{}).(*services.ExpenseCategoryService),
		basePath:               "/finance/expenses",
	}
}

func (c *ExpenseController) Key() string {
	return c.basePath
}

func (c *ExpenseController) Register(r *mux.Router) {
	commonMiddleware := []mux.MiddlewareFunc{
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.Tabs(),
		middleware.WithLocalizer(c.app.Bundle()),
		middleware.NavItems(),
		middleware.WithPageContext(),
	}
	getRouter := r.PathPrefix(c.basePath).Subrouter()
	getRouter.Use(commonMiddleware...)

	getRouter.HandleFunc("", c.List).Methods(http.MethodGet)
	getRouter.HandleFunc("/{id:[0-9]+}", c.GetEdit).Methods(http.MethodGet)
	getRouter.HandleFunc("/new", c.GetNew).Methods(http.MethodGet)

	setRouter := r.PathPrefix(c.basePath).Subrouter()
	setRouter.Use(commonMiddleware...)
	setRouter.Use(middleware.WithTransaction())

	setRouter.HandleFunc("", c.Create).Methods(http.MethodPost)
	setRouter.HandleFunc("/{id:[0-9]+}", c.Update).Methods(http.MethodPost)
	setRouter.HandleFunc("/{id:[0-9]+}", c.Delete).Methods(http.MethodDelete)
}

func (c *ExpenseController) viewModelAccounts(r *http.Request) ([]*viewmodels.MoneyAccount, error) {
	accounts, err := c.moneyAccountService.GetAll(r.Context())
	if err != nil {
		return nil, errors.Wrap(err, "Error retrieving moneyaccounts")
	}
	return mapping.MapViewModels(accounts, mappers.MoneyAccountToViewModel), nil
}

func (c *ExpenseController) viewModelExpenses(r *http.Request) (*ExpensePaginationResponse, error) {
	paginationParams := composables.UsePaginated(r)
	params, err := composables.UseQuery(&expense.FindParams{
		Offset: paginationParams.Offset,
		Limit:  paginationParams.Limit,
		SortBy: []string{"created_at desc"},
	}, r)
	if err != nil {
		return nil, errors.Wrap(err, "Error using query")
	}
	expenseEntities, err := c.expenseService.GetPaginated(r.Context(), params)
	if err != nil {
		return nil, errors.Wrap(err, "Error retrieving expenses")
	}
	viewExpenses := mapping.MapViewModels(expenseEntities, mappers.ExpenseToViewModel)

	total, err := c.expenseService.Count(r.Context())
	if err != nil {
		return nil, errors.Wrap(err, "Error counting expenses")
	}

	return &ExpensePaginationResponse{
		Expenses:        viewExpenses,
		PaginationState: pagination.New(c.basePath, paginationParams.Page, int(total), params.Limit),
	}, nil
}

func (c *ExpenseController) viewModelCategories(r *http.Request) ([]*viewmodels.ExpenseCategory, error) {
	categories, err := c.expenseCategoryService.GetAll(r.Context())
	if err != nil {
		return nil, errors.Wrap(err, "Error retrieving categories")
	}
	return mapping.MapViewModels(categories, mappers.ExpenseCategoryToViewModel), nil
}

func (c *ExpenseController) List(w http.ResponseWriter, r *http.Request) {
	paginated, err := c.viewModelExpenses(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	isHxRequest := len(r.Header.Get("Hx-Request")) > 0
	props := &expenses2.IndexPageProps{
		Expenses:        paginated.Expenses,
		PaginationState: paginated.PaginationState,
	}
	if isHxRequest {
		templ.Handler(expenses2.ExpensesTable(props), templ.WithStreaming()).ServeHTTP(w, r)
	} else {
		templ.Handler(expenses2.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *ExpenseController) GetEdit(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	entity, err := c.expenseService.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "Error retrieving expense", http.StatusInternalServerError)
		return
	}
	accounts, err := c.viewModelAccounts(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	categories, err := c.viewModelCategories(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := &expenses2.EditPageProps{
		Expense:    mappers.ExpenseToViewModel(entity),
		Accounts:   accounts,
		Categories: categories,
		Errors:     map[string]string{},
	}
	templ.Handler(expenses2.Edit(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *ExpenseController) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}

	if _, err := c.expenseService.Delete(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	shared.Redirect(w, r, c.basePath)
}

func (c *ExpenseController) Update(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}
	dto := expense.UpdateDTO{}
	if err := shared.Decoder.Decode(&dto, r.Form); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	uniLocalizer, err := composables.UseUniLocalizer(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if errorsMap, ok := dto.Ok(uniLocalizer); !ok {
		entity, err := c.expenseService.GetByID(r.Context(), id)
		if err != nil {
			http.Error(w, "Error retrieving expense", http.StatusInternalServerError)
			return
		}
		accounts, err := c.viewModelAccounts(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		categories, err := c.viewModelCategories(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		props := &expenses2.EditPageProps{
			Expense:    mappers.ExpenseToViewModel(entity),
			Accounts:   accounts,
			Categories: categories,
			Errors:     errorsMap,
		}
		templ.Handler(expenses2.EditForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}
	if err := c.expenseService.Update(r.Context(), id, &dto); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	shared.Redirect(w, r, c.basePath)
}

func (c *ExpenseController) GetNew(w http.ResponseWriter, r *http.Request) {
	accounts, err := c.viewModelAccounts(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	categories, err := c.viewModelCategories(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := &expenses2.CreatePageProps{
		Accounts:   accounts,
		Categories: categories,
		Errors:     map[string]string{},
		Expense: mappers.ExpenseToViewModel(&expense.Expense{
			Category: category.New(0, "", "", 0, &currency.USD),
		}),
	}
	templ.Handler(expenses2.New(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *ExpenseController) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	dto := expense.CreateDTO{}
	if err := shared.Decoder.Decode(&dto, r.Form); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	uniLocalizer, err := composables.UseUniLocalizer(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if errorsMap, ok := dto.Ok(uniLocalizer); !ok {
		accounts, err := c.viewModelAccounts(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		entity, err := dto.ToEntity()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		categories, err := c.viewModelCategories(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		props := &expenses2.CreatePageProps{
			Accounts:   accounts,
			Errors:     errorsMap,
			Categories: categories,
			Expense:    mappers.ExpenseToViewModel(entity),
		}
		templ.Handler(expenses2.CreateForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	if err := c.expenseService.Create(r.Context(), &dto); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}
