package controllers

import (
	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-erp/internal/app/services"
	"github.com/iota-agency/iota-erp/internal/domain/entities/expense_category"
	"github.com/iota-agency/iota-erp/internal/presentation/mappers"
	"github.com/iota-agency/iota-erp/internal/presentation/templates/pages/expense_categories"
	"github.com/iota-agency/iota-erp/internal/presentation/types"
	"github.com/iota-agency/iota-erp/internal/presentation/view_models"
	"github.com/iota-agency/iota-erp/pkg/composables"
	"net/http"
)

type ExpenseCategoriesController struct {
	app      *services.Application
	basePath string
}

func NewExpenseCategoriesController(app *services.Application) Controller {
	return &ExpenseCategoriesController{
		app:      app,
		basePath: "/finance/expense-categories",
	}
}

func (c *ExpenseCategoriesController) Register(r *mux.Router) {
	r.HandleFunc(c.basePath, c.List).Methods(http.MethodGet)
	r.HandleFunc(c.basePath, c.Create).Methods(http.MethodPost)
	r.HandleFunc(c.basePath+"/{id:[0-9]+}", c.GetEdit).Methods(http.MethodGet)
	r.HandleFunc(c.basePath+"/{id:[0-9]+}", c.PostEdit).Methods(http.MethodPost)
	r.HandleFunc(c.basePath+"/{id:[0-9]+}", c.Delete).Methods(http.MethodDelete)
	r.HandleFunc(c.basePath+"/new", c.GetNew).Methods(http.MethodGet)
}

func (c *ExpenseCategoriesController) List(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := composables.UsePageCtx(r, &composables.PageData{Title: "ExpenseCategories.Meta.List.Title"})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	params := composables.UsePaginated(r)
	categories, err := c.app.ExpenseCategoryService.GetPaginated(r.Context(), params.Limit, params.Offset, []string{})
	if err != nil {
		http.Error(w, "Error retrieving expense categories", http.StatusInternalServerError)
		return
	}
	viewCategories := make([]*view_models.ExpenseCategory, 0, len(categories))
	for _, category := range categories {
		viewCategories = append(viewCategories, mappers.ExpenseCategoryToViewModel(category))
	}
	isHxRequest := len(r.Header.Get("HX-Request")) > 0
	props := &expense_categories.IndexPageProps{
		PageContext: pageCtx,
		Categories:  viewCategories,
	}
	if isHxRequest {
		templ.Handler(expense_categories.CategoriesTable(props), templ.WithStreaming()).ServeHTTP(w, r)
	} else {
		templ.Handler(expense_categories.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *ExpenseCategoriesController) GetEdit(w http.ResponseWriter, r *http.Request) {
	id, err := parseId(r)
	if err != nil {
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}

	pageCtx, err := composables.UsePageCtx(r, &composables.PageData{Title: "ExpenseCategories.Meta.Edit.Title"})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	entity, err := c.app.ExpenseCategoryService.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "Error retrieving expense category", http.StatusInternalServerError)
		return
	}
	currencies, err := c.app.CurrencyService.GetAll(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := &expense_categories.EditPageProps{
		PageContext: pageCtx,
		Category:    entity,
		Currencies:  currencies,
		Errors:      map[string]string{},
	}
	templ.Handler(expense_categories.Edit(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *ExpenseCategoriesController) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseId(r)
	if err != nil {
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}

	if _, err := c.app.ExpenseCategoryService.Delete(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	redirect(w, r, c.basePath)
}

func (c *ExpenseCategoriesController) PostEdit(w http.ResponseWriter, r *http.Request) {
	id, err := parseId(r)
	if err != nil {
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}
	action := r.FormValue("_action")
	r.Form.Del("_action")

	if action == "save" {
		dto := category.UpdateDTO{}
		var pageCtx *types.PageContext
		pageCtx, err = composables.UsePageCtx(r, &composables.PageData{Title: "ExpenseCategories.Meta.Edit.Title"})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := decoder.Decode(&dto, r.Form); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if errors, ok := dto.Ok(pageCtx.UniTranslator); !ok {
			entity, err := c.app.ExpenseCategoryService.GetByID(r.Context(), id)
			if err != nil {
				http.Error(w, "Error retrieving expense category", http.StatusInternalServerError)
				return
			}
			currencies, err := c.app.CurrencyService.GetAll(r.Context())
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			props := &expense_categories.EditPageProps{
				PageContext: pageCtx,
				Category:    entity,
				Currencies:  currencies,
				Errors:      errors,
			}
			templ.Handler(expense_categories.EditForm(props), templ.WithStreaming()).ServeHTTP(w, r)
			return
		}
		err = c.app.ExpenseCategoryService.Update(r.Context(), id, &dto)
	} else if action == "delete" {
		_, err = c.app.ExpenseCategoryService.Delete(r.Context(), id)
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	redirect(w, r, c.basePath)
}

func (c *ExpenseCategoriesController) GetNew(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := composables.UsePageCtx(r, &composables.PageData{Title: "ExpenseCategories.Meta.New.Title"})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	currencies, err := c.app.CurrencyService.GetAll(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := &expense_categories.CreatePageProps{
		PageContext: pageCtx,
		Currencies:  currencies,
		Errors:      map[string]string{},
		Category:    &category.ExpenseCategory{},
	}
	templ.Handler(expense_categories.New(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *ExpenseCategoriesController) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	dto := category.CreateDTO{}
	if err := decoder.Decode(&dto, r.Form); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pageCtx, err := composables.UsePageCtx(r, &composables.PageData{Title: "ExpenseCategories.Meta.New.Title"})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if errors, ok := dto.Ok(pageCtx.UniTranslator); !ok {
		currencies, err := c.app.CurrencyService.GetAll(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		entity, err := dto.ToEntity()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		props := &expense_categories.CreatePageProps{
			PageContext: pageCtx,
			Currencies:  currencies,
			Errors:      errors,
			Category:    entity,
		}
		templ.Handler(expense_categories.CreateForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	if err := c.app.ExpenseCategoryService.Create(r.Context(), &dto); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	redirect(w, r, c.basePath)
}
