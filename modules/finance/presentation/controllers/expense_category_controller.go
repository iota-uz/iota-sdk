package controllers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/components/filters"
	"github.com/iota-uz/iota-sdk/components/scaffold/actions"
	"github.com/iota-uz/iota-sdk/components/scaffold/table"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/mappers"
	expense_categories2 "github.com/iota-uz/iota-sdk/modules/finance/presentation/templates/pages/expense_categories"

	category "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense_category"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/iota-uz/iota-sdk/pkg/shared"
)

type ExpenseCategoriesController struct {
	app                    application.Application
	expenseCategoryService *services.ExpenseCategoryService
	basePath               string
	tableDefinition        table.TableDefinition
}

func NewExpenseCategoriesController(app application.Application) application.Controller {
	basePath := "/finance/expense-categories"

	// Create table definition once at initialization
	// Note: We'll set the actual localized values in the List method since we need context
	tableDefinition := table.NewTableDefinition("", basePath).
		WithInfiniteScroll(true).
		Build()

	return &ExpenseCategoriesController{
		app:                    app,
		expenseCategoryService: app.Service(services.ExpenseCategoryService{}).(*services.ExpenseCategoryService),
		basePath:               basePath,
		tableDefinition:        tableDefinition,
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
		middleware.ProvideDynamicLogo(c.app),
		middleware.Tabs(),
		middleware.ProvideLocalizer(c.app.Bundle()),
		middleware.NavItems(),
		middleware.WithPageContext(),
	}

	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(commonMiddleware...)
	router.HandleFunc("", c.List).Methods(http.MethodGet)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}/drawer", c.GetEditDrawer).Methods(http.MethodGet)
	router.HandleFunc("/new/drawer", c.GetNewDrawer).Methods(http.MethodGet)
	router.HandleFunc("", c.Create).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}", c.Update).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}", c.Delete).Methods(http.MethodDelete)
}

func (c *ExpenseCategoriesController) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pageCtx := composables.UsePageCtx(ctx)

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

	if search := table.UseSearchQuery(r); search != "" {
		params.Search = search
	}

	if from := r.URL.Query().Get("CreatedAt.From"); from != "" {
		if to := r.URL.Query().Get("CreatedAt.To"); to != "" {
			fromTime, err := time.Parse(time.RFC3339, from)
			if err != nil {
				http.Error(w, "Invalid from date format", http.StatusBadRequest)
				return
			}
			toTime, err := time.Parse(time.RFC3339, to)
			if err != nil {
				http.Error(w, "Invalid to date format", http.StatusBadRequest)
				return
			}
			params.Filters = append(params.Filters, category.Filter{
				Column: category.CreatedAt,
				Filter: repo.Between(fromTime, toTime),
			})
		}
	}

	expenseEntities, err := c.expenseCategoryService.GetPaginated(ctx, params)
	if err != nil {
		http.Error(w, "Error retrieving expense categories", http.StatusInternalServerError)
		return
	}

	total, err := c.expenseCategoryService.Count(ctx, params)
	if err != nil {
		http.Error(w, "Error counting expense categories", http.StatusInternalServerError)
		return
	}

	// Create table definition with localized values (only for full page render)
	var definition table.TableDefinition
	if !htmx.IsHxRequest(r) {
		// Create action for drawer
		createAction := actions.CreateAction(
			pageCtx.T("ExpenseCategories.List.New"),
			"",
		)
		createAction.Attrs = templ.Attributes{
			"hx-get":    c.basePath + "/new/drawer",
			"hx-target": "#view-drawer",
			"hx-swap":   "innerHTML",
		}

		definition = table.NewTableDefinition(
			pageCtx.T("ExpenseCategories.Meta.List.Title"),
			c.basePath,
		).
			WithColumns(
				table.Column("name", pageCtx.T("ExpenseCategories.List.Name")),
				table.Column("description", pageCtx.T("ExpenseCategories.Single._Description")),
				table.Column("created_at", pageCtx.T("CreatedAt")),
			).
			WithActions(actions.RenderAction(createAction)).
			WithFilters(filters.CreatedAt()).
			WithInfiniteScroll(true).
			Build()
	} else {
		// For HTMX requests, use minimal definition
		definition = c.tableDefinition
	}

	// Build table rows
	viewCategories := mapping.MapViewModels(expenseEntities, mappers.ExpenseCategoryToViewModel)
	rows := make([]table.TableRow, 0, len(viewCategories))

	for _, cat := range viewCategories {
		createdAt, err := time.Parse("2006-01-02T15:04:05Z07:00", cat.CreatedAt)
		if err != nil {
			createdAt, err = time.Parse("2006-01-02 15:04:05", cat.CreatedAt)
			if err != nil {
				createdAt = time.Now()
			}
		}

		cells := []templ.Component{
			templ.Raw(cat.Name),
			templ.Raw(cat.Description),
			table.DateTime(createdAt),
		}

		row := table.Row(cells...).ApplyOpts(
			table.WithDrawer(fmt.Sprintf("%s/%s/drawer", c.basePath, cat.ID)),
		)
		rows = append(rows, row)
	}

	// Create table data
	tableData := table.NewTableData().
		WithRows(rows...).
		WithPagination(paginationParams.Page, paginationParams.Limit, int64(total)).
		WithQueryParams(r.URL.Query())

	// Create renderer and render appropriate component
	renderer := table.NewTableRenderer(definition, tableData)

	if htmx.IsHxRequest(r) {
		templ.Handler(renderer.RenderRows(), templ.WithStreaming()).ServeHTTP(w, r)
	} else {
		templ.Handler(renderer.RenderFull(), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *ExpenseCategoriesController) GetEditDrawer(w http.ResponseWriter, r *http.Request) {
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
	props := &expense_categories2.DrawerEditProps{
		Category: mappers.ExpenseCategoryToViewModel(entity),
		Errors:   map[string]string{},
	}
	templ.Handler(expense_categories2.EditDrawer(props), templ.WithStreaming()).ServeHTTP(w, r)
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

	isDrawer := r.Header.Get("HX-Target") != "" && r.Header.Get("HX-Target") != "edit-content"

	if errorsMap, ok := dto.Ok(r.Context()); ok {
		existing, err := c.expenseCategoryService.GetByID(r.Context(), id)
		if err != nil {
			http.Error(w, "Error retrieving expense category", http.StatusInternalServerError)
			return
		}

		entity, err := dto.Apply(existing)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if _, err := c.expenseCategoryService.Update(r.Context(), entity); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Always redirect to refresh the table
		shared.Redirect(w, r, c.basePath)
	} else {
		entity, err := c.expenseCategoryService.GetByID(r.Context(), id)
		if err != nil {
			http.Error(w, "Error retrieving expense category", http.StatusInternalServerError)
			return
		}

		if isDrawer {
			props := &expense_categories2.DrawerEditProps{
				Category: mappers.ExpenseCategoryToViewModel(entity),
				Errors:   errorsMap,
			}
			templ.Handler(expense_categories2.EditDrawer(props), templ.WithStreaming()).ServeHTTP(w, r)
		} else {
			http.Error(w, "Edit form not supported - use drawer", http.StatusBadRequest)
		}
	}
}

func (c *ExpenseCategoriesController) GetNewDrawer(w http.ResponseWriter, r *http.Request) {
	props := &expense_categories2.DrawerCreateProps{
		Errors:   map[string]string{},
		Category: dtos.ExpenseCategoryCreateDTO{},
	}
	templ.Handler(expense_categories2.CreateDrawer(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *ExpenseCategoriesController) Create(w http.ResponseWriter, r *http.Request) {
	dto, err := composables.UseForm(&dtos.ExpenseCategoryCreateDTO{}, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	isDrawer := htmx.IsHxRequest(r) && r.Header.Get("HX-Target") == "expense-category-create-drawer"

	if errorsMap, ok := dto.Ok(r.Context()); !ok {
		if isDrawer {
			props := &expense_categories2.DrawerCreateProps{
				Errors:   errorsMap,
				Category: *dto,
			}
			templ.Handler(expense_categories2.CreateDrawer(props), templ.WithStreaming()).ServeHTTP(w, r)
		} else {
			http.Error(w, "Create form not supported - use drawer", http.StatusBadRequest)
		}
		return
	}

	tenantID, err := composables.UseTenantID(r.Context())
	if err != nil {
		http.Error(w, "Error getting tenant ID", http.StatusInternalServerError)
		return
	}

	entity, err := dto.ToEntity(tenantID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err := c.expenseCategoryService.Create(r.Context(), entity); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}
