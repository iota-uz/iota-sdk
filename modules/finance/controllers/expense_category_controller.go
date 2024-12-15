package controllers

import (
	"net/http"

	"github.com/iota-agency/iota-sdk/pkg/middleware"

	"github.com/a-h/templ"
	"github.com/go-faster/errors"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-sdk/components/base/pagination"
	category "github.com/iota-agency/iota-sdk/modules/finance/domain/aggregates/expense_category"
	"github.com/iota-agency/iota-sdk/modules/finance/services"
	"github.com/iota-agency/iota-sdk/modules/finance/templates/pages/expense_categories"
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/mapping"
	coremappers "github.com/iota-agency/iota-sdk/pkg/presentation/mappers"
	coreservices "github.com/iota-agency/iota-sdk/pkg/services"
	"github.com/iota-agency/iota-sdk/pkg/shared"
	"github.com/iota-agency/iota-sdk/pkg/types"

	"github.com/iota-agency/iota-sdk/modules/finance/mappers"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/presentation/viewmodels"
)

type ExpenseCategoriesController struct {
	app                    application.Application
	currencyService        *coreservices.CurrencyService
	expenseCategoryService *services.ExpenseCategoryService
	basePath               string
}

type ExpenseCategoryPaginatedResponse struct {
	Categories      []*viewmodels.ExpenseCategory
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

func (c *ExpenseCategoriesController) Register(r *mux.Router) {
	commonMiddleware := []mux.MiddlewareFunc{
		middleware.Authorize(),
		middleware.RequireAuthorization(),
		middleware.ProvideUser(),
		middleware.Tabs(),
		middleware.WithLocalizer(c.app.Bundle()),
		middleware.NavItems(c.app),
	}

	getRouter := r.PathPrefix(c.basePath).Subrouter()
	getRouter.Use(commonMiddleware...)
	getRouter.HandleFunc("", c.List).Methods(http.MethodGet)
	getRouter.HandleFunc("/{id:[0-9]+}", c.GetEdit).Methods(http.MethodGet)
	getRouter.HandleFunc("/new", c.GetNew).Methods(http.MethodGet)

	setRouter := r.PathPrefix(c.basePath).Subrouter()
	setRouter.Use(commonMiddleware...)
	setRouter.Use(middleware.WithTransaction())
	getRouter.HandleFunc("", c.Create).Methods(http.MethodPost)
	getRouter.HandleFunc("/{id:[0-9]+}", c.PostEdit).Methods(http.MethodPost)
	getRouter.HandleFunc("/{id:[0-9]+}", c.Delete).Methods(http.MethodDelete)
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
	params, err := composables.UseQuery(&category.FindParams{
		Limit:  paginationParams.Limit,
		Offset: paginationParams.Offset,
		SortBy: []string{"created_at desc"},
	}, r)
	if err != nil {
		return nil, errors.Wrap(err, "Error using query")
	}

	expenseEntities, err := c.expenseCategoryService.GetPaginated(r.Context(), params)
	if err != nil {
		return nil, errors.Wrap(err, "Error retrieving expenses")
	}
	viewCategories := mapping.MapViewModels(expenseEntities, mappers.ExpenseCategoryToViewModel)

	total, err := c.expenseCategoryService.Count(r.Context())
	if err != nil {
		return nil, errors.Wrap(err, "Error counting expenses")
	}

	return &ExpenseCategoryPaginatedResponse{
		Categories:      viewCategories,
		PaginationState: pagination.New(c.basePath, paginationParams.Page, int(total), params.Limit),
	}, nil
}

func (c *ExpenseCategoriesController) List(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := composables.UsePageCtx(
		r,
		types.NewPageData("ExpenseCategories.Meta.List.Title", ""),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	paginated, err := c.viewModelExpenseCategories(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	isHxRequest := len(r.Header.Get("Hx-Request")) > 0
	props := &expense_categories.IndexPageProps{
		PageContext:     pageCtx,
		Categories:      paginated.Categories,
		PaginationState: paginated.PaginationState,
	}
	if isHxRequest {
		templ.Handler(expense_categories.CategoriesTable(props), templ.WithStreaming()).ServeHTTP(w, r)
	} else {
		templ.Handler(expense_categories.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *ExpenseCategoriesController) GetEdit(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	pageCtx, err := composables.UsePageCtx(
		r,
		types.NewPageData("ExpenseCategories.Meta.Edit.Title", ""),
	)
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
	props := &expense_categories.EditPageProps{
		PageContext: pageCtx,
		Category:    mappers.ExpenseCategoryToViewModel(entity),
		Currencies:  currencies,
		Errors:      map[string]string{},
	}
	templ.Handler(expense_categories.Edit(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *ExpenseCategoriesController) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
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

func (c *ExpenseCategoriesController) PostEdit(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}
	action := shared.FormAction(r.FormValue("_action"))
	if !action.IsValid() {
		http.Error(w, "Invalid action", http.StatusBadRequest)
		return
	}
	r.Form.Del("_action")

	switch action {
	case shared.FormActionDelete:
		if _, err := c.expenseCategoryService.Delete(r.Context(), id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case shared.FormActionSave:
		dto := category.UpdateDTO{} //nolint:exhaustruct
		var pageCtx *types.PageContext
		pageCtx, err = composables.UsePageCtx(
			r,
			types.NewPageData("ExpenseCategories.Meta.Edit.Title", ""),
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := shared.Decoder.Decode(&dto, r.Form); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		errorsMap, ok := dto.Ok(pageCtx.UniTranslator)
		if ok {
			if err := c.expenseCategoryService.Update(r.Context(), id, &dto); err != nil {
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
			props := &expense_categories.EditPageProps{
				PageContext: pageCtx,
				Category:    mappers.ExpenseCategoryToViewModel(entity),
				Currencies:  currencies,
				Errors:      errorsMap,
			}
			templ.Handler(expense_categories.EditForm(props), templ.WithStreaming()).ServeHTTP(w, r)
			return
		}
	}
	shared.Redirect(w, r, c.basePath)
}

func (c *ExpenseCategoriesController) GetNew(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := composables.UsePageCtx(
		r,
		types.NewPageData("ExpenseCategories.Meta.New.Title", ""),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	currencies, err := c.viewModelCurrencies(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := &expense_categories.CreatePageProps{
		PageContext: pageCtx,
		Currencies:  currencies,
		Errors:      map[string]string{},
		Category:    category.CreateDTO{}, //nolint:exhaustruct
		PostPath:    c.basePath,
	}
	templ.Handler(expense_categories.New(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *ExpenseCategoriesController) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	dto := category.CreateDTO{} //nolint:exhaustruct
	if err := shared.Decoder.Decode(&dto, r.Form); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pageCtx, err := composables.UsePageCtx(
		r, types.NewPageData("ExpenseCategories.Meta.New.Title", ""),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if errorsMap, ok := dto.Ok(pageCtx.UniTranslator); !ok {
		currencies, err := c.viewModelCurrencies(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		props := &expense_categories.CreatePageProps{
			PageContext: pageCtx,
			Currencies:  currencies,
			Errors:      errorsMap,
			Category:    dto,
			PostPath:    c.basePath,
		}
		templ.Handler(expense_categories.CreateForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	if err := c.expenseCategoryService.Create(r.Context(), &dto); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}
