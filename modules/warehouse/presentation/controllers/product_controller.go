package controllers

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"

	"github.com/iota-uz/iota-sdk/components/base/pagination"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/product"
	"github.com/iota-uz/iota-sdk/modules/warehouse/presentation/mappers"
	"github.com/iota-uz/iota-sdk/modules/warehouse/presentation/templates/pages/products"
	"github.com/iota-uz/iota-sdk/modules/warehouse/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/modules/warehouse/services/positionservice"
	"github.com/iota-uz/iota-sdk/modules/warehouse/services/productservice"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/iota-uz/iota-sdk/pkg/shared"
)

type ProductsController struct {
	app             application.Application
	productService  *productservice.ProductService
	positionService *positionservice.PositionService
	basePath        string
}

type PaginatedResponse struct {
	Products        []*viewmodels.Product
	PaginationState *pagination.State
}

func NewProductsController(app application.Application) application.Controller {
	return &ProductsController{
		app:             app,
		productService:  app.Service(productservice.ProductService{}).(*productservice.ProductService),
		positionService: app.Service(positionservice.PositionService{}).(*positionservice.PositionService),
		basePath:        "/warehouse/products",
	}
}

func (c *ProductsController) Key() string {
	return c.basePath
}

func (c *ProductsController) Register(r *mux.Router) {
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

	getRouter := r.PathPrefix(c.basePath).Subrouter()
	getRouter.Use(commonMiddleware...)
	getRouter.HandleFunc("", c.List).Methods(http.MethodGet)
	getRouter.HandleFunc("/new", c.GetNew).Methods(http.MethodGet)
	getRouter.HandleFunc("/{id:[0-9]+}", c.GetEdit).Methods(http.MethodGet)

	setRouter := r.PathPrefix(c.basePath).Subrouter()
	setRouter.Use(commonMiddleware...)
	setRouter.Use(middleware.WithTransaction())
	setRouter.HandleFunc("", c.Create).Methods(http.MethodPost)
	setRouter.HandleFunc("/{id:[0-9]+}", c.Update).Methods(http.MethodPost)
}

func (c *ProductsController) handleError(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func (c *ProductsController) getViewModelProducts(r *http.Request) (*PaginatedResponse, error) {
	paginationParams := composables.UsePaginated(r)
	params, err := composables.UseQuery(&product.FindParams{
		Limit:  paginationParams.Limit,
		Offset: paginationParams.Offset,
		SortBy: []string{"created_at desc"},
	}, r)
	if err != nil {
		return nil, fmt.Errorf("error retrieving query: %w", err)
	}
	productEntities, err := c.productService.GetPaginated(r.Context(), params)
	if err != nil {
		return nil, fmt.Errorf("error retrieving products: %w", err)
	}

	viewProducts := mapping.MapViewModels(productEntities, mappers.ProductToViewModel)

	total, err := c.productService.Count(r.Context(), &product.CountParams{})
	if err != nil {
		return nil, fmt.Errorf("error counting products: %w", err)
	}

	return &PaginatedResponse{
		Products:        viewProducts,
		PaginationState: pagination.New(c.basePath, paginationParams.Page, int(total), params.Limit),
	}, nil
}

func (c *ProductsController) renderTemplate(w http.ResponseWriter, r *http.Request, template templ.Component) {
	templ.Handler(template, templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *ProductsController) List(w http.ResponseWriter, r *http.Request) {
	paginated, err := c.getViewModelProducts(r)
	if err != nil {
		c.handleError(w, err)
		return
	}

	isHxRequest := len(r.Header.Get("Hx-Request")) > 0
	props := &products.IndexPageProps{
		Products:        paginated.Products,
		PaginationState: paginated.PaginationState,
	}
	var template templ.Component
	if isHxRequest {
		template = products.ProductsTable(props)
	} else {
		template = products.Index(props)
	}
	c.renderTemplate(w, r, template)
}

func (c *ProductsController) GetEdit(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
	if err != nil {
		c.handleError(w, err)
		return
	}

	entity, err := c.productService.GetByID(r.Context(), id)
	if err != nil {
		c.handleError(w, fmt.Errorf("error retrieving product: %w", err))
		return
	}

	props := &products.EditPageProps{
		Product: mappers.ProductToViewModel(entity),
		Errors:  map[string]string{},
	}
	c.renderTemplate(w, r, products.Edit(props))
}

func (c *ProductsController) Update(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
	if err != nil {
		c.handleError(w, err)
		return
	}

	dto, err := composables.UseForm(&product.UpdateDTO{}, r)
	if err != nil {
		c.handleError(w, fmt.Errorf("error parsing form: %w", err))
		return
	}

	entity, err := c.productService.GetByID(r.Context(), id)
	if err != nil {
		c.handleError(w, fmt.Errorf("error retrieving product: %w", err))
		return
	}
	uniLocalizer, err := intl.UseUniLocalizer(r.Context())
	if err != nil {
		c.handleError(w, fmt.Errorf("error retrieving localizer: %w", err))
		return
	}
	if errorsMap, ok := dto.Ok(uniLocalizer); !ok {
		props := &products.EditPageProps{
			Product: mappers.ProductToViewModel(entity),
			Errors:  errorsMap,
		}
		c.renderTemplate(w, r, products.EditForm(props))
		return
	}

	localizer, ok := intl.UseLocalizer(r.Context())
	if !ok {
		c.handleError(w, fmt.Errorf("error retrieving localizer"))
		return
	}

	if err := c.productService.Update(r.Context(), id, dto); err != nil {
		var vErr serrors.Base
		if errors.As(err, &vErr) {
			// Create a new product with the invalid RFID for display purposes
			entityWithInvalidRfid := product.New(dto.Rfid, entity.Status(),
				product.WithID(entity.ID()),
				product.WithTenantID(entity.TenantID()),
				product.WithPosition(entity.Position()),
				product.WithCreatedAt(entity.CreatedAt()),
				product.WithUpdatedAt(entity.UpdatedAt()))
			props := &products.EditPageProps{
				Errors: map[string]string{
					"Rfid": vErr.Localize(localizer),
				},
				Product: mappers.ProductToViewModel(entityWithInvalidRfid),
			}
			c.renderTemplate(w, r, products.EditForm(props))
			return
		}
		c.handleError(w, err)
		return
	}
	shared.Redirect(w, r, c.basePath)
}

func (c *ProductsController) GetNew(w http.ResponseWriter, r *http.Request) {
	props := &products.CreatePageProps{
		Errors:  map[string]string{},
		Product: mappers.ProductToViewModel(product.New("", product.InStock)),
		SaveURL: c.basePath,
	}
	c.renderTemplate(w, r, products.New(props))
}

func (c *ProductsController) Create(w http.ResponseWriter, r *http.Request) {
	dto, err := composables.UseForm(&product.CreateDTO{}, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	entity, err := dto.ToEntity()
	if err != nil {
		c.handleError(w, err)
		return
	}
	uniLocalizer, err := intl.UseUniLocalizer(r.Context())
	if err != nil {
		c.handleError(w, fmt.Errorf("error retrieving localizer: %w", err))
		return
	}
	if errorsMap, ok := dto.Ok(uniLocalizer); !ok {
		props := &products.CreatePageProps{
			Errors:  errorsMap,
			Product: mappers.ProductToViewModel(entity),
			SaveURL: c.basePath,
		}
		c.renderTemplate(w, r, products.CreateForm(props))
		return
	}

	localizer, ok := intl.UseLocalizer(r.Context())
	if !ok {
		c.handleError(w, fmt.Errorf("error retrieving localizer"))
		return
	}

	if err := c.productService.Create(r.Context(), dto); err != nil {
		var vErr serrors.Base
		if errors.As(err, &vErr) {
			props := &products.CreatePageProps{
				Errors: map[string]string{
					"Rfid": vErr.Localize(localizer),
				},
				Product: mappers.ProductToViewModel(entity),
				SaveURL: c.basePath,
			}
			c.renderTemplate(w, r, products.CreateForm(props))
			return
		}
		c.handleError(w, err)
		return
	}

	shared.Redirect(w, r, c.basePath)
}
