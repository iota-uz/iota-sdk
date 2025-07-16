package controllers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/go-faster/errors"
	"github.com/gorilla/mux"
	icons "github.com/iota-uz/icons/phosphor"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/mappers"
	expense_categories2 "github.com/iota-uz/iota-sdk/modules/finance/presentation/templates/pages/expense_categories"
	viewmodels2 "github.com/iota-uz/iota-sdk/modules/finance/presentation/viewmodels"

	"github.com/iota-uz/iota-sdk/components/base/button"
	"github.com/iota-uz/iota-sdk/components/base/pagination"
	"github.com/iota-uz/iota-sdk/components/scaffold/table"
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
}

type ExpenseCategoryPaginatedResponse struct {
	Categories      []*viewmodels2.ExpenseCategory
	PaginationState *pagination.State
}

func NewExpenseCategoriesController(app application.Application) application.Controller {
	return &ExpenseCategoriesController{
		app:                    app,
		expenseCategoryService: app.Service(services.ExpenseCategoryService{}).(*services.ExpenseCategoryService),
		basePath:               "/finance/expense-categories",
	}
}

// formatDateTime converts a string datetime to a formatted string
func formatDateTime(dateStr string) string {
	if dateStr == "" {
		return "-"
	}
	// Try to parse the date string
	t, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		// Try another common format
		t, err = time.Parse("2006-01-02T15:04:05Z", dateStr)
		if err != nil {
			return dateStr // Return as-is if parsing fails
		}
	}
	return t.Format("2006-01-02 15:04:05")
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
	router.HandleFunc("/{id:[0-9a-fA-F-]+}", c.GetEdit).Methods(http.MethodGet)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}/view", c.ViewDrawer).Methods(http.MethodGet)
	router.HandleFunc("/new", c.GetNew).Methods(http.MethodGet)
	router.HandleFunc("", c.Create).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}", c.Update).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}", c.Delete).Methods(http.MethodDelete)
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
	ctx := r.Context()
	pageCtx := composables.UsePageCtx(ctx)

	// Create table configuration - simple data structure
	tableConfig := &table.TableConfig{
		Title:   pageCtx.T("NavigationLinks.ExpenseCategories"),
		DataURL: "/finance/expense-categories",
		Columns: []table.TableColumn{
			table.Column("name", pageCtx.T("ExpenseCategories.List.Name")),
			table.Column("createdAt", pageCtx.T("CreatedAt")),
			table.Column("updatedAt", pageCtx.T("UpdatedAt")),
		},
		Actions: []templ.Component{
			button.Primary(button.Props{
				Size: button.SizeNormal,
				Href: "/finance/expense-categories/new",
				Icon: icons.PlusCircle(icons.Props{Size: "18"}),
			}),
		},
	}

	// Fetch data
	paginated, err := c.viewModelExpenseCategories(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Add rows with hover effect and drawer functionality
	for _, category := range paginated.Categories {
		// Create name with description underneath
		nameWithDescription := fmt.Sprintf(`<div class="flex flex-col">
			<span class="font-medium">%s</span>
			<span class="text-sm text-gray-500">%s</span>
		</div>`, category.Name, category.Description)

		row := table.Row(
			templ.Raw(nameWithDescription),
			templ.Raw(fmt.Sprintf(`<div x-data="relativeformat"><span x-text="format('%s')"></span></div>`, category.CreatedAt)),
			templ.Raw(fmt.Sprintf(`<div x-data="relativeformat"><span x-text="format('%s')"></span></div>`, category.UpdatedAt)),
		).ApplyOpts(table.WithDrawer(fmt.Sprintf("/finance/expense-categories/%s/view", category.ID)))
		tableConfig.Rows = append(tableConfig.Rows, row)
	}

	// Configure infinite scroll if there are more pages
	if paginated.PaginationState != nil && len(paginated.PaginationState.Pages()) > 1 {
		paginationParams := composables.UsePaginated(r)
		tableConfig.Infinite = table.InfiniteScroll{
			HasMore: paginationParams.Page < len(paginated.PaginationState.Pages()),
			Page:    paginationParams.Page,
			PerPage: paginationParams.Limit,
		}
	}

	// Render appropriate component based on request type
	var component templ.Component
	if htmx.IsHxRequest(r) {
		// For HTMX requests, only render the table rows
		component = table.Rows(tableConfig)
	} else {
		// For full page requests, use table.Page which includes all scaffolding
		component = table.Page(tableConfig)
	}

	if err := component.Render(ctx, w); err != nil {
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
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
	props := &expense_categories2.EditPageProps{
		Category: mappers.ExpenseCategoryToViewModel(entity),
		Errors:   map[string]string{},
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
	} else {
		entity, err := c.expenseCategoryService.GetByID(r.Context(), id)
		if err != nil {
			http.Error(w, "Error retrieving expense category", http.StatusInternalServerError)
			return
		}
		props := &expense_categories2.EditPageProps{
			Category: mappers.ExpenseCategoryToViewModel(entity),
			Errors:   errorsMap,
		}
		templ.Handler(expense_categories2.EditForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}
	shared.Redirect(w, r, c.basePath)
}

func (c *ExpenseCategoriesController) GetNew(w http.ResponseWriter, r *http.Request) {
	props := &expense_categories2.CreatePageProps{
		Errors:   map[string]string{},
		Category: dtos.ExpenseCategoryCreateDTO{},
		PostPath: c.basePath,
	}
	templ.Handler(expense_categories2.New(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *ExpenseCategoriesController) Create(w http.ResponseWriter, r *http.Request) {
	dto, err := composables.UseForm(&dtos.ExpenseCategoryCreateDTO{}, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if errorsMap, ok := dto.Ok(r.Context()); !ok {
		props := &expense_categories2.CreatePageProps{
			Errors:   errorsMap,
			Category: *dto,
			PostPath: c.basePath,
		}
		templ.Handler(expense_categories2.CreateForm(props), templ.WithStreaming()).ServeHTTP(w, r)
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

// ViewDrawer renders category details in a drawer component
// ListCompact demonstrates using the table scaffolding for embedded/compact views
func (c *ExpenseCategoriesController) ListCompact(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pageCtx := composables.UsePageCtx(ctx)

	// Create a minimal table configuration
	tableConfig := &table.TableConfig{
		DataURL: "/finance/expense-categories/compact",
		Columns: []table.TableColumn{
			table.Column("name", pageCtx.T("Name")),
			table.Column("actions", "", table.WithClass("w-8")),
		},
	}

	// Fetch data with limited results
	params := &category.FindParams{
		Limit:  5,
		Offset: 0,
		SortBy: category.SortBy{
			Fields: []repo.SortByField[category.Field]{
				{Field: category.UpdatedAt, Ascending: false},
			},
		},
	}

	categories, err := c.expenseCategoryService.GetPaginated(ctx, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Add simplified rows
	for _, entity := range categories {
		category := mappers.ExpenseCategoryToViewModel(entity)
		row := table.Row(
			templ.Raw(category.Name),
			button.Ghost(button.Props{
				Size: button.SizeSM,
				Href: fmt.Sprintf("/finance/expense-categories/%s", category.ID),
				Icon: icons.ArrowRight(icons.Props{Size: "16"}),
			}),
		)
		tableConfig.Rows = append(tableConfig.Rows, row)
	}

	// Render just the table without wrapper
	component := table.Table(tableConfig)
	if err := component.Render(ctx, w); err != nil {
		http.Error(w, "Failed to render compact table", http.StatusInternalServerError)
	}
}

func (c *ExpenseCategoriesController) ViewDrawer(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseUUID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	entity, err := c.expenseCategoryService.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "Category not found", http.StatusNotFound)
		return
	}

	category := mappers.ExpenseCategoryToViewModel(entity)

	// Prepare drawer fields
	ctx := r.Context()
	pageCtx := composables.UsePageCtx(ctx)

	fields := []table.DetailFieldValue{
		{
			Label: pageCtx.T("ExpenseCategories.List.Name"),
			Value: category.Name,
			Type:  table.DetailFieldTypeText,
		},
		{
			Label: pageCtx.T("CreatedAt"),
			Value: formatDateTime(category.CreatedAt),
			Type:  table.DetailFieldTypeDateTime,
		},
		{
			Label: pageCtx.T("UpdatedAt"),
			Value: formatDateTime(category.UpdatedAt),
			Type:  table.DetailFieldTypeDateTime,
		},
	}

	actions := []table.DetailAction{
		{
			Label: pageCtx.T("Edit"),
			URL:   fmt.Sprintf("/finance/expense-categories/%s", category.ID),
			Class: "btn-primary",
		},
		{
			Label:   pageCtx.T("Delete"),
			URL:     fmt.Sprintf("/finance/expense-categories/%s", category.ID),
			Method:  "DELETE",
			Class:   "btn-danger",
			Confirm: pageCtx.T("ExpenseCategories.Delete.Confirm"),
		},
	}

	// Render the drawer content
	component := table.DetailsDrawer(table.DetailsDrawerProps{
		ID:          fmt.Sprintf("category-%s", category.ID),
		Title:       category.Name,
		CallbackURL: "/finance/expense-categories",
		Fields:      fields,
		Actions:     actions,
	})

	if err := component.Render(r.Context(), w); err != nil {
		http.Error(w, "Failed to render drawer", http.StatusInternalServerError)
	}
}
