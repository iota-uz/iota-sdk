package controllers

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/go-faster/errors"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/mappers"
	payment_categories2 "github.com/iota-uz/iota-sdk/modules/finance/presentation/templates/pages/payment_categories"
	viewmodels2 "github.com/iota-uz/iota-sdk/modules/finance/presentation/viewmodels"

	"github.com/iota-uz/iota-sdk/components/base/pagination"
	paymentcategory "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment_category"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/iota-uz/iota-sdk/pkg/shared"
)

type PaymentCategoriesController struct {
	app                    application.Application
	paymentCategoryService *services.PaymentCategoryService
	basePath               string
}

type PaymentCategoryPaginatedResponse struct {
	Categories      []*viewmodels2.PaymentCategory
	PaginationState *pagination.State
}

func NewPaymentCategoriesController(app application.Application) application.Controller {
	return &PaymentCategoriesController{
		app:                    app,
		paymentCategoryService: app.Service(services.PaymentCategoryService{}).(*services.PaymentCategoryService),
		basePath:               "/finance/payment-categories",
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
		middleware.Tabs(),
		middleware.ProvideLocalizer(c.app.Bundle()),
		middleware.NavItems(),
		middleware.WithPageContext(),
	}

	getRouter := r.PathPrefix(c.basePath).Subrouter()
	getRouter.Use(commonMiddleware...)
	getRouter.HandleFunc("", c.List).Methods(http.MethodGet)
	getRouter.HandleFunc("/{id:[0-9a-fA-F-]+}", c.GetEdit).Methods(http.MethodGet)
	getRouter.HandleFunc("/new", c.GetNew).Methods(http.MethodGet)

	setRouter := r.PathPrefix(c.basePath).Subrouter()
	setRouter.Use(commonMiddleware...)
	setRouter.Use(middleware.WithTransaction())
	setRouter.HandleFunc("", c.Create).Methods(http.MethodPost)
	setRouter.HandleFunc("/{id:[0-9a-fA-F-]+}", c.Update).Methods(http.MethodPost)
	setRouter.HandleFunc("/{id:[0-9a-fA-F-]+}", c.Delete).Methods(http.MethodDelete)
}

func (c *PaymentCategoriesController) viewModelPaymentCategories(r *http.Request) (*PaymentCategoryPaginatedResponse, error) {
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

	// Use query parameters for additional filtering
	queryParams := r.URL.Query()
	if search := queryParams.Get("search"); search != "" {
		params.Search = search
	}

	paymentEntities, err := c.paymentCategoryService.GetPaginated(r.Context(), params)
	if err != nil {
		return nil, errors.Wrap(err, "Error retrieving payment categories")
	}
	viewCategories := mapping.MapViewModels(paymentEntities, mappers.PaymentCategoryToViewModel)

	total, err := c.paymentCategoryService.Count(r.Context(), params)
	if err != nil {
		return nil, errors.Wrap(err, "Error counting payment categories")
	}

	return &PaymentCategoryPaginatedResponse{
		Categories:      viewCategories,
		PaginationState: pagination.New(c.basePath, paginationParams.Page, int(total), params.Limit),
	}, nil
}

func (c *PaymentCategoriesController) List(w http.ResponseWriter, r *http.Request) {
	paginated, err := c.viewModelPaymentCategories(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	isHxRequest := len(r.Header.Get("Hx-Request")) > 0
	props := &payment_categories2.IndexPageProps{
		Categories:      paginated.Categories,
		PaginationState: paginated.PaginationState,
	}
	if isHxRequest {
		templ.Handler(payment_categories2.CategoriesTable(props), templ.WithStreaming()).ServeHTTP(w, r)
	} else {
		templ.Handler(payment_categories2.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *PaymentCategoriesController) GetEdit(w http.ResponseWriter, r *http.Request) {
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
	props := &payment_categories2.EditPageProps{
		Category: mappers.PaymentCategoryToViewModel(entity),
		Errors:   map[string]string{},
	}
	templ.Handler(payment_categories2.Edit(props), templ.WithStreaming()).ServeHTTP(w, r)
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
		if err := c.paymentCategoryService.Update(r.Context(), entity); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		entity, err := c.paymentCategoryService.GetByID(r.Context(), id)
		if err != nil {
			http.Error(w, "Error retrieving payment category", http.StatusInternalServerError)
			return
		}
		props := &payment_categories2.EditPageProps{
			Category: mappers.PaymentCategoryToViewModel(entity),
			Errors:   errorsMap,
		}
		templ.Handler(payment_categories2.EditForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}
	shared.Redirect(w, r, c.basePath)
}

func (c *PaymentCategoriesController) GetNew(w http.ResponseWriter, r *http.Request) {
	props := &payment_categories2.CreatePageProps{
		Errors:   map[string]string{},
		Category: dtos.PaymentCategoryCreateDTO{},
		PostPath: c.basePath,
	}
	templ.Handler(payment_categories2.New(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *PaymentCategoriesController) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	dto := dtos.PaymentCategoryCreateDTO{}
	if err := shared.Decoder.Decode(&dto, r.Form); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if errorsMap, ok := dto.Ok(r.Context()); !ok {
		props := &payment_categories2.CreatePageProps{
			Errors:   errorsMap,
			Category: dto,
			PostPath: c.basePath,
		}
		templ.Handler(payment_categories2.CreateForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	entity, err := dto.ToEntity()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := c.paymentCategoryService.Create(r.Context(), entity); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}
