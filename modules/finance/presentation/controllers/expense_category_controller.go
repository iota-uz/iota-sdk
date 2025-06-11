package controllers

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/go-faster/errors"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/mappers"
	expense_categories2 "github.com/iota-uz/iota-sdk/modules/finance/presentation/templates/pages/expense_categories"
	viewmodels2 "github.com/iota-uz/iota-sdk/modules/finance/presentation/viewmodels"

	"github.com/iota-uz/iota-sdk/components/base/pagination"
	coremappers "github.com/iota-uz/iota-sdk/modules/core/presentation/mappers"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	category "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense_category"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/iota-uz/iota-sdk/pkg/shared"
)

type ExpenseCategoriesController struct {
	app                    application.Application
	currencyService        *coreservices.CurrencyService
	expenseCategoryService *services.ExpenseCategoryService
	basePath               string
}

type ExpenseCategoryPaginatedResponse struct {
	Categories      []*viewmodels2.ExpenseCategory
	PaginationState *pagination.State
}

func NewExpenseCategoriesController(app application.Application) application.Controller {
	return &ExpenseCategoriesController{
		app:                    app,
		currencyService:        app.Service(coreservices.CurrencyService{}).(*coreservices.CurrencyService),
		expenseCategoryService: app.Service(services.ExpenseCategoryService{}).(*services.ExpenseCategoryService),
		basePath:               "/finance/expense-categories",
	}
}

func (c *ExpenseCategoriesController) Key() string {
	return c.basePath
}

func (c *ExpenseCategoriesController) Register(r *mux.Router) {
	commonMiddleware := []mux.MiddlewareFunc{
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.Tabs(),
		middleware.ProvideLocalizer(c.app.Bundle()),
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

func (c *ExpenseCategoriesController) viewModelCurrencies(r *http.Request) ([]*viewmodels.Currency, error) {
	currencies, err := c.currencyService.GetAll(r.Context())
	if err != nil {
		return nil, err
	}
	return mapping.MapViewModels(currencies, coremappers.CurrencyToViewModel), nil
}

func (c *ExpenseCategoriesController) viewModelExpenseCategories(r *http.Request) (*ExpenseCategoryPaginatedResponse, error) {
	paginationParams := composables.UsePaginated(r)
	params := &category.FindParams{
		Limit:  paginationParams.Limit,
		Offset: paginationParams.Offset,
		SortBy: category.SortBy{
			Fields: []repo.SortByField[category.Field]{
				{
					Field:     category.CreatedAt,
					Ascending: false,
				},
			},
		},
	}

	// Use query parameters for additional filtering
	queryParams := r.URL.Query()
	if search := queryParams.Get("search"); search != "" {
		params.Search = search
	}

	expenseEntities, err := c.expenseCategoryService.GetPaginated(r.Context(), params)
	if err != nil {
		return nil, errors.Wrap(err, "Error retrieving expenses")
	}
	viewCategories := mapping.MapViewModels(expenseEntities, mappers.ExpenseCategoryToViewModel)

	total, err := c.expenseCategoryService.Count(r.Context(), params)
	if err != nil {
		return nil, errors.Wrap(err, "Error counting expenses")
	}

	return &ExpenseCategoryPaginatedResponse{
		Categories:      viewCategories,
		PaginationState: pagination.New(c.basePath, paginationParams.Page, int(total), params.Limit),
	}, nil
}

func (c *ExpenseCategoriesController) List(w http.ResponseWriter, r *http.Request) {
	paginated, err := c.viewModelExpenseCategories(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	isHxRequest := len(r.Header.Get("Hx-Request")) > 0
	props := &expense_categories2.IndexPageProps{
		Categories:      paginated.Categories,
		PaginationState: paginated.PaginationState,
	}
	if isHxRequest {
		templ.Handler(expense_categories2.CategoriesTable(props), templ.WithStreaming()).ServeHTTP(w, r)
	} else {
		templ.Handler(expense_categories2.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *ExpenseCategoriesController) GetEdit(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseUUID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	entity, err := c.expenseCategoryService.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "Error retrieving expense category", http.StatusInternalServerError)
		return
	}
	currencies, err := c.viewModelCurrencies(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := &expense_categories2.EditPageProps{
		Category:   mappers.ExpenseCategoryToViewModel(entity),
		Currencies: currencies,
		Errors:     map[string]string{},
	}
	templ.Handler(expense_categories2.Edit(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *ExpenseCategoriesController) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseUUID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err := c.expenseCategoryService.Delete(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	shared.Redirect(w, r, c.basePath)
}

func (c *ExpenseCategoriesController) Update(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseUUID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	dto, err := composables.UseForm(&dtos.ExpenseCategoryUpdateDTO{}, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if errorsMap, ok := dto.Ok(r.Context()); ok {
		entity, err := dto.ToEntity(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := c.expenseCategoryService.Update(r.Context(), entity); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		entity, err := c.expenseCategoryService.GetByID(r.Context(), id)
		if err != nil {
			http.Error(w, "Error retrieving expense category", http.StatusInternalServerError)
			return
		}
		currencies, err := c.viewModelCurrencies(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		props := &expense_categories2.EditPageProps{
			Category:   mappers.ExpenseCategoryToViewModel(entity),
			Currencies: currencies,
			Errors:     errorsMap,
		}
		templ.Handler(expense_categories2.EditForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}
	shared.Redirect(w, r, c.basePath)
}

func (c *ExpenseCategoriesController) GetNew(w http.ResponseWriter, r *http.Request) {
	currencies, err := c.viewModelCurrencies(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := &expense_categories2.CreatePageProps{
		Currencies: currencies,
		Errors:     map[string]string{},
		Category:   dtos.ExpenseCategoryCreateDTO{},
		PostPath:   c.basePath,
	}
	templ.Handler(expense_categories2.New(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *ExpenseCategoriesController) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	dto := dtos.ExpenseCategoryCreateDTO{}
	if err := shared.Decoder.Decode(&dto, r.Form); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if errorsMap, ok := dto.Ok(r.Context()); !ok {
		currencies, err := c.viewModelCurrencies(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		props := &expense_categories2.CreatePageProps{
			Currencies: currencies,
			Errors:     errorsMap,
			Category:   dto,
			PostPath:   c.basePath,
		}
		templ.Handler(expense_categories2.CreateForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	entity, err := dto.ToEntity()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := c.expenseCategoryService.Create(r.Context(), entity); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}
