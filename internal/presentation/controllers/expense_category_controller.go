package controllers

import (
	"github.com/gorilla/schema"
	"github.com/iota-agency/iota-erp/internal/presentation/types"
	"net/http"
	"strconv"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-erp/internal/app/services"
	"github.com/iota-agency/iota-erp/internal/domain/entities/expense_category"
	"github.com/iota-agency/iota-erp/internal/presentation/templates/pages/expense_categories"
	"github.com/iota-agency/iota-erp/pkg/composables"
)

type ExpenseCategoriesController struct {
	app *services.Application
}

func NewExpenseCategoriesController(app *services.Application) Controller {
	return &ExpenseCategoriesController{
		app: app,
	}
}

func (c *ExpenseCategoriesController) Register(r *mux.Router) {
	r.HandleFunc("/finance/expense-categories", c.List).Methods(http.MethodGet)
	r.HandleFunc("/finance/expense-categories", c.Create).Methods(http.MethodPost)
	r.HandleFunc("/finance/expense-categories/{id:[0-9]+}", c.GetEdit).Methods(http.MethodGet)
	r.HandleFunc("/finance/expense-categories/{id:[0-9]+}", c.PostEdit).Methods(http.MethodPost)
	r.HandleFunc("/finance/expense-categories/{id:[0-9]+}", c.Delete).Methods(http.MethodDelete)
	r.HandleFunc("/finance/expense-categories/new", c.GetNew).Methods(http.MethodGet)
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
	isHxRequest := len(r.Header.Get("HX-Request")) > 0
	if isHxRequest {
		templ.Handler(expense_categories.CategoriesTable(pageCtx.Localizer, categories), templ.WithStreaming()).ServeHTTP(w, r)
	} else {
		templ.Handler(expense_categories.Index(pageCtx, categories), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *ExpenseCategoriesController) GetEdit(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}

	pageCtx, err := composables.UsePageCtx(r, &composables.PageData{Title: "ExpenseCategories.Meta.Edit.Title"})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	entity, err := c.app.ExpenseCategoryService.GetByID(r.Context(), uint(id))
	if err != nil {
		http.Error(w, "Error retrieving expense category", http.StatusInternalServerError)
		return
	}
	templ.Handler(expense_categories.Edit(pageCtx, entity, map[string]string{}), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *ExpenseCategoriesController) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}

	if _, err := c.app.ExpenseCategoryService.Delete(r.Context(), uint(id)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	redirect(w, r, "/expense-categories")
}

func (c *ExpenseCategoriesController) PostEdit(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(mux.Vars(r)["id"])
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
		if err := schema.NewDecoder().Decode(&dto, r.Form); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := validate.Struct(&dto); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if errors, ok := dto.Ok(pageCtx.Localizer); !ok {
			entity, err := c.app.ExpenseCategoryService.GetByID(r.Context(), uint(id))
			if err != nil {
				http.Error(w, "Error retrieving expense category", http.StatusInternalServerError)
				return
			}
			templ.Handler(expense_categories.EditForm(pageCtx.Localizer, entity, errors), templ.WithStreaming()).ServeHTTP(w, r)
			return
		}
		err = c.app.ExpenseCategoryService.Update(r.Context(), uint(id), &dto)
	} else if action == "delete" {
		_, err = c.app.ExpenseCategoryService.Delete(r.Context(), uint(id))
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	redirect(w, r, "/finance/expense-categories")
}

func (c *ExpenseCategoriesController) GetNew(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := composables.UsePageCtx(r, &composables.PageData{Title: "ExpenseCategories.Meta.New.Title"})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	templ.Handler(expense_categories.New(pageCtx, map[string]string{}), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *ExpenseCategoriesController) Create(w http.ResponseWriter, r *http.Request) {
	dto := category.CreateDTO{}

	if err := schema.NewDecoder().Decode(&dto, r.Form); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := validate.Struct(&dto); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pageCtx, err := composables.UsePageCtx(r, &composables.PageData{Title: "ExpenseCategories.Meta.New.Title"})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if errors, ok := dto.Ok(pageCtx.Localizer); !ok {
		templ.Handler(expense_categories.CreateForm(pageCtx.Localizer, dto.ToEntity(), errors), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	if err := c.app.ExpenseCategoryService.Create(r.Context(), &dto); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	redirect(w, r, "/finance/expense-categories")
}
