package controllers

import (
	"fmt"
	"github.com/a-h/templ"
	"github.com/go-faster/errors"
	"github.com/gorilla/mux"
	"net/http"

	"github.com/iota-agency/iota-erp/internal/app/services"
	"github.com/iota-agency/iota-erp/internal/domain/aggregates/expense"
	"github.com/iota-agency/iota-erp/internal/presentation/mappers"
	"github.com/iota-agency/iota-erp/internal/presentation/templates/pages/expenses"
	"github.com/iota-agency/iota-erp/internal/presentation/viewmodels"
	"github.com/iota-agency/iota-erp/pkg/composables"
	"github.com/iota-agency/iota-erp/pkg/middleware"
)

type ExpenseController struct {
	app      *services.Application
	basePath string
}

func NewExpensesController(app *services.Application) Controller {
	return &ExpenseController{
		app:      app,
		basePath: "/finance/expenses",
	}
}

func (c *ExpenseController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(middleware.RequireAuthorization())
	router.HandleFunc("", c.List).Methods(http.MethodGet)
	router.HandleFunc("", c.Create).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9]+}", c.GetEdit).Methods(http.MethodGet)
	router.HandleFunc("/{id:[0-9]+}", c.PostEdit).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9]+}", c.Delete).Methods(http.MethodDelete)
	router.HandleFunc("/new", c.GetNew).Methods(http.MethodGet)
}

func (c *ExpenseController) viewModelAccounts(r *http.Request) ([]*viewmodels.MoneyAccount, error) {
	accounts, err := c.app.MoneyAccountService.GetAll(r.Context())
	if err != nil {
		return nil, errors.Wrap(err, "Error retrieving moneyaccounts")
	}
	viewAccounts := make([]*viewmodels.MoneyAccount, len(accounts))
	for i, account := range accounts {
		viewAccounts[i] = mappers.MoneyAccountToViewModel(
			account, fmt.Sprintf("%s/%d", c.basePath, account.ID),
		)
	}
	return viewAccounts, nil
}

func (c *ExpenseController) viewModelExpenses(r *http.Request) ([]*viewmodels.Expense, error) {
	params := composables.UsePaginated(r)
	expenseEntities, err := c.app.ExpenseService.GetPaginated(r.Context(), params.Limit, params.Offset, []string{})
	if err != nil {
		return nil, errors.Wrap(err, "Error retrieving expenses")
	}
	viewExpenses := make([]*viewmodels.Expense, len(expenseEntities))
	for i, entity := range expenseEntities {
		viewExpenses[i] = mappers.ExpenseToViewModel(entity)
	}
	return viewExpenses, nil
}

func (c *ExpenseController) viewModelCategories(r *http.Request) ([]*viewmodels.ExpenseCategory, error) {
	categories, err := c.app.ExpenseCategoryService.GetAll(r.Context())
	if err != nil {
		return nil, errors.Wrap(err, "Error retrieving categories")
	}
	viewCategories := make([]*viewmodels.ExpenseCategory, len(categories))
	for i, category := range categories {
		viewCategories[i] = mappers.ExpenseCategoryToViewModel(category)
	}
	return viewCategories, nil
}

func (c *ExpenseController) List(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := composables.UsePageCtx(
		r,
		composables.NewPageData("Expenses.Meta.List.Title", ""),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	viewExpenses, err := c.viewModelExpenses(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	isHxRequest := len(r.Header.Get("Hx-Request")) > 0
	props := &expenses.IndexPageProps{
		PageContext: pageCtx,
		Expenses:    viewExpenses,
	}
	if isHxRequest {
		templ.Handler(expenses.ExpensesTable(props), templ.WithStreaming()).ServeHTTP(w, r)
	} else {
		templ.Handler(expenses.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *ExpenseController) GetEdit(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	pageCtx, err := composables.UsePageCtx(
		r,
		composables.NewPageData("Expenses.Meta.Edit.Title", ""),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	entity, err := c.app.ExpenseService.GetByID(r.Context(), id)
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
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}

	if _, err := c.app.ExpenseService.Delete(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	redirect(w, r, c.basePath)
}

func (c *ExpenseController) PostEdit(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}
	action := FormAction(r.FormValue("_action"))
	if !action.IsValid() {
		http.Error(w, "Invalid action", http.StatusBadRequest)
		return
	}
	r.Form.Del("_action")

	switch action {
	case FormActionDelete:
		if _, err := c.app.ExpenseService.Delete(r.Context(), id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case FormActionSave:
		dto := expense.UpdateDTO{} //nolint:exhaustruct
		var pageCtx *composables.PageContext
		pageCtx, err = composables.UsePageCtx(r, composables.NewPageData("Expenses.Meta.Edit.Title", ""))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := decoder.Decode(&dto, r.Form); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if errorsMap, ok := dto.Ok(pageCtx.UniTranslator); !ok {
			entity, err := c.app.ExpenseService.GetByID(r.Context(), id)
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
		if err := c.app.ExpenseService.Update(r.Context(), id, &dto); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	redirect(w, r, c.basePath)
}

func (c *ExpenseController) GetNew(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := composables.UsePageCtx(r, composables.NewPageData("Expenses.Meta.New.Title", ""))
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
	if err := decoder.Decode(&dto, r.Form); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pageCtx, err := composables.UsePageCtx(r, composables.NewPageData("Expenses.Meta.New.Title", ""))
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

	if err := c.app.ExpenseService.Create(r.Context(), &dto); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	redirect(w, r, c.basePath)
}
