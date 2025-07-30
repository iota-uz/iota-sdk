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
	paymentcategories "github.com/iota-uz/iota-sdk/modules/finance/presentation/templates/pages/payment_categories"

	paymentcategory "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment_category"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/iota-uz/iota-sdk/pkg/shared"
)

type PaymentCategoriesController struct {
	app                    application.Application
	paymentCategoryService *services.PaymentCategoryService
	basePath               string
	tableDefinition        table.TableDefinition
}

func NewPaymentCategoriesController(app application.Application) application.Controller {
	basePath := "/finance/payment-categories"

	// Create table definition with columns for HTMX requests
	tableDefinition := table.NewTableDefinition("", basePath).
		WithColumns(
			table.Column("name", "Name"),
			table.Column("description", "Description"),
			table.Column("created_at", "Created At"),
		).
		WithInfiniteScroll(true).
		Build()

	return &PaymentCategoriesController{
		app:                    app,
		paymentCategoryService: app.Service(services.PaymentCategoryService{}).(*services.PaymentCategoryService),
		basePath:               basePath,
		tableDefinition:        tableDefinition,
	}
}

func (c *PaymentCategoriesController) Key() string {
	return c.basePath
}

func (c *PaymentCategoriesController) Register(r *mux.Router) {
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

func (c *PaymentCategoriesController) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pageCtx := composables.UsePageCtx(ctx)

	paginationParams := composables.UsePaginated(r)
	params := &paymentcategory.FindParams{
		Limit:  paginationParams.Limit,
		Offset: paginationParams.Offset,
		SortBy: paymentcategory.SortBy{
			Fields: []repo.SortByField[paymentcategory.Field]{
				{
					Field:     paymentcategory.CreatedAt,
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
			params.Filters = append(params.Filters, paymentcategory.Filter{
				Column: paymentcategory.CreatedAt,
				Filter: repo.Between(fromTime, toTime),
			})
		}
	}

	paymentEntities, err := c.paymentCategoryService.GetPaginated(ctx, params)
	if err != nil {
		http.Error(w, "Error retrieving payment categories", http.StatusInternalServerError)
		return
	}

	total, err := c.paymentCategoryService.Count(ctx, params)
	if err != nil {
		http.Error(w, "Error counting payment categories", http.StatusInternalServerError)
		return
	}

	// Create table definition with localized values (only for full page render)
	var definition table.TableDefinition
	if !htmx.IsHxRequest(r) {
		// Create action for drawer
		createAction := actions.CreateAction(
			pageCtx.T("PaymentCategories.List.New"),
			"",
		)
		createAction.Attrs = templ.Attributes{
			"hx-get":    c.basePath + "/new/drawer",
			"hx-target": "#view-drawer",
			"hx-swap":   "innerHTML",
		}

		definition = table.NewTableDefinition(
			pageCtx.T("PaymentCategories.Meta.List.Title"),
			c.basePath,
		).
			WithColumns(
				table.Column("name", pageCtx.T("PaymentCategories.List.Name")),
				table.Column("description", pageCtx.T("PaymentCategories.Single._Description")),
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
	viewCategories := mapping.MapViewModels(paymentEntities, mappers.PaymentCategoryToViewModel)
	rows := make([]table.TableRow, 0, len(viewCategories))

	for _, cat := range viewCategories {
		createdAt, err := time.Parse("2006-01-02T15:04:05Z07:00", cat.CreatedAt)
		if err != nil {
			createdAt, err = time.Parse("2006-01-02 15:04:05", cat.CreatedAt)
			if err != nil {
				createdAt = time.Now()
			}
		}

		cells := []table.TableCell{
			table.Cell(templ.Raw(cat.Name), cat.Name),
			table.Cell(templ.Raw(cat.Description), cat.Description),
			table.Cell(table.DateTime(createdAt), cat.CreatedAt),
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

func (c *PaymentCategoriesController) GetEditDrawer(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseUUID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	entity, err := c.paymentCategoryService.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "Error retrieving payment category", http.StatusInternalServerError)
		return
	}
	props := &paymentcategories.DrawerEditProps{
		Category: mappers.PaymentCategoryToViewModel(entity),
		Errors:   map[string]string{},
	}
	templ.Handler(paymentcategories.EditDrawer(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *PaymentCategoriesController) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseUUID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err := c.paymentCategoryService.Delete(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}

func (c *PaymentCategoriesController) Update(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseUUID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	dto, err := composables.UseForm(&dtos.PaymentCategoryUpdateDTO{}, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	isDrawer := htmx.Target(r) != "" && htmx.Target(r) != "edit-content"

	if errorsMap, ok := dto.Ok(r.Context()); ok {
		existing, err := c.paymentCategoryService.GetByID(r.Context(), id)
		if err != nil {
			http.Error(w, "Error retrieving payment category", http.StatusInternalServerError)
			return
		}

		entity, err := dto.Apply(existing)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if _, err := c.paymentCategoryService.Update(r.Context(), entity); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Always redirect to refresh the table
		shared.Redirect(w, r, c.basePath)
	} else {
		entity, err := c.paymentCategoryService.GetByID(r.Context(), id)
		if err != nil {
			http.Error(w, "Error retrieving payment category", http.StatusInternalServerError)
			return
		}

		if isDrawer {
			props := &paymentcategories.DrawerEditProps{
				Category: mappers.PaymentCategoryToViewModel(entity),
				Errors:   errorsMap,
			}
			templ.Handler(paymentcategories.EditDrawer(props), templ.WithStreaming()).ServeHTTP(w, r)
		} else {
			http.Error(w, "Edit form not supported - use drawer", http.StatusBadRequest)
		}
	}
}

func (c *PaymentCategoriesController) GetNewDrawer(w http.ResponseWriter, r *http.Request) {
	props := &paymentcategories.DrawerCreateProps{
		Errors:   map[string]string{},
		Category: dtos.PaymentCategoryCreateDTO{},
	}
	templ.Handler(paymentcategories.CreateDrawer(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *PaymentCategoriesController) Create(w http.ResponseWriter, r *http.Request) {
	dto, err := composables.UseForm(&dtos.PaymentCategoryCreateDTO{}, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	isDrawer := htmx.IsHxRequest(r) && htmx.Target(r) == "payment-category-create-drawer"

	if errorsMap, ok := dto.Ok(r.Context()); !ok {
		if isDrawer {
			props := &paymentcategories.DrawerCreateProps{
				Errors:   errorsMap,
				Category: *dto,
			}
			templ.Handler(paymentcategories.CreateDrawer(props), templ.WithStreaming()).ServeHTTP(w, r)
		} else {
			http.Error(w, "Create form not supported - use drawer", http.StatusBadRequest)
		}
		return
	}

	entity, err := dto.ToEntity()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err := c.paymentCategoryService.Create(r.Context(), entity); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}
