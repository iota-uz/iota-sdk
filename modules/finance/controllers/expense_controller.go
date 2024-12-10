package controllers

import (
	"github.com/iota-agency/iota-sdk/pkg/middleware"
	coreservices "github.com/iota-agency/iota-sdk/pkg/services"
	"net/http"

	"github.com/a-h/templ"
	"github.com/go-faster/errors"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-sdk/components/base/pagination"
	"github.com/iota-agency/iota-sdk/modules/finance/domain/aggregates/expense"
	"github.com/iota-agency/iota-sdk/modules/finance/services"
	"github.com/iota-agency/iota-sdk/modules/finance/templates/pages/expenses"
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/mapping"
	"github.com/iota-agency/iota-sdk/pkg/shared"
	"github.com/iota-agency/iota-sdk/pkg/types"

	"github.com/iota-agency/iota-sdk/modules/finance/mappers"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/presentation/viewmodels"
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

func (c *ExpenseController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(
		middleware.WithTransaction(),
		middleware.Authorize(c.app.Service(coreservices.AuthService{}).(*coreservices.AuthService)),
		middleware.RequireAuthorization(),
		middleware.ProvideUser(c.app.Service(coreservices.UserService{}).(*coreservices.UserService)),
		middleware.Tabs(c.app.Service(coreservices.TabService{}).(*coreservices.TabService)),
		middleware.WithLocalizer(c.app.Bundle()),
		middleware.NavItems(c.app),
	)
	router.HandleFunc("", c.List).Methods(http.MethodGet)
	router.HandleFunc("", c.Create).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9]+}", c.GetEdit).Methods(http.MethodGet)
	router.HandleFunc("/{id:[0-9]+}", c.PostEdit).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9]+}", c.Delete).Methods(http.MethodDelete)
	router.HandleFunc("/new", c.GetNew).Methods(http.MethodGet)
}

func (c *ExpenseController) viewModelAccounts(r *http.Request) ([]*viewmodels.MoneyAccount, error) {
	accounts, err := c.moneyAccountService.GetAll(r.Context())
	if err != nil {
		return nil, errors.Wrap(err, "Error retrieving moneyaccounts")
	}
	return mapping.MapViewModels(accounts, mappers.MoneyAccountToViewModel), nil
}

func (c *ExpenseController) viewModelExpenses(r *http.Request) (*ExpensePaginationResponse, error) {
	params := composables.UsePaginated(r)
	expenseEntities, err := c.expenseService.GetPaginated(r.Context(), params.Limit, params.Offset, []string{})
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
		PaginationState: pagination.New(c.basePath, params.Page, int(total), params.Limit),
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
	pageCtx, err := composables.UsePageCtx(
		r,
		types.NewPageData("Expenses.Meta.List.Title", ""),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	paginated, err := c.viewModelExpenses(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	isHxRequest := len(r.Header.Get("Hx-Request")) > 0
	props := &expenses.IndexPageProps{
		PageContext:     pageCtx,
		Expenses:        paginated.Expenses,
		PaginationState: paginated.PaginationState,
	}
	if isHxRequest {
		templ.Handler(expenses.ExpensesTable(props), templ.WithStreaming()).ServeHTTP(w, r)
	} else {
		templ.Handler(expenses.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *ExpenseController) GetEdit(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	pageCtx, err := composables.UsePageCtx(
		r,
		types.NewPageData("Expenses.Meta.Edit.Title", ""),
	)
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
	props := &expenses.EditPageProps{
		PageContext: pageCtx,
		Expense:     mappers.ExpenseToViewModel(entity),
		Accounts:    accounts,
		Categories:  categories,
		Errors:      map[string]string{},
	}
	templ.Handler(expenses.Edit(props), templ.WithStreaming()).ServeHTTP(w, r)
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

func (c *ExpenseController) PostEdit(w http.ResponseWriter, r *http.Request) {
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
		if _, err := c.expenseService.Delete(r.Context(), id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case shared.FormActionSave:
		dto := expense.UpdateDTO{} //nolint:exhaustruct
		var pageCtx *types.PageContext
		pageCtx, err = composables.UsePageCtx(r, types.NewPageData("Expenses.Meta.Edit.Title", ""))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := shared.Decoder.Decode(&dto, r.Form); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if errorsMap, ok := dto.Ok(pageCtx.UniTranslator); !ok {
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
			props := &expenses.EditPageProps{
				PageContext: pageCtx,
				Expense:     mappers.ExpenseToViewModel(entity),
				Accounts:    accounts,
				Categories:  categories,
				Errors:      errorsMap,
			}
			templ.Handler(expenses.EditForm(props), templ.WithStreaming()).ServeHTTP(w, r)
			return
		}
		if err := c.expenseService.Update(r.Context(), id, &dto); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	shared.Redirect(w, r, c.basePath)
}

func (c *ExpenseController) GetNew(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := composables.UsePageCtx(r, types.NewPageData("Expenses.Meta.New.Title", ""))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
	props := &expenses.CreatePageProps{
		PageContext: pageCtx,
		Accounts:    accounts,
		Categories:  categories,
		Errors:      map[string]string{},
		Expense:     mappers.ExpenseToViewModel(&expense.Expense{}), //nolint:exhaustruct
	}
	templ.Handler(expenses.New(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *ExpenseController) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	dto := expense.CreateDTO{} //nolint:exhaustruct
	if err := shared.Decoder.Decode(&dto, r.Form); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pageCtx, err := composables.UsePageCtx(r, types.NewPageData("Expenses.Meta.New.Title", ""))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if errorsMap, ok := dto.Ok(pageCtx.UniTranslator); !ok {
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
		props := &expenses.CreatePageProps{
			PageContext: pageCtx,
			Accounts:    accounts,
			Errors:      errorsMap,
			Categories:  categories,
			Expense:     mappers.ExpenseToViewModel(entity),
		}
		templ.Handler(expenses.CreateForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	if err := c.expenseService.Create(r.Context(), &dto); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}
