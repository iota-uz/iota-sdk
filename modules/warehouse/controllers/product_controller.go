package controllers

import (
	"fmt"
	"github.com/iota-agency/iota-sdk/modules/warehouse/services/position_service"
	"github.com/iota-agency/iota-sdk/modules/warehouse/services/product_service"
	"net/http"

	"github.com/iota-agency/iota-sdk/components/base/pagination"
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/product"
	"github.com/iota-agency/iota-sdk/modules/warehouse/mappers"
	"github.com/iota-agency/iota-sdk/modules/warehouse/templates/pages/products"
	"github.com/iota-agency/iota-sdk/modules/warehouse/viewmodels"
	"github.com/iota-agency/iota-sdk/pkg/mapping"
	"github.com/iota-agency/iota-sdk/pkg/shared"
	"github.com/iota-agency/iota-sdk/pkg/shared/middleware"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"

	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/types"
)

type ProductsController struct {
	app             application.Application
	productService  *product_service.ProductService
	positionService *position_service.PositionService
	basePath        string
}

type PaginatedResponse struct {
	Products        []*viewmodels.Product
	PaginationState *pagination.State
}

func NewProductsController(app application.Application) application.Controller {
	return &ProductsController{
		app:             app,
		productService:  app.Service(product_service.ProductService{}).(*product_service.ProductService),
		positionService: app.Service(position_service.PositionService{}).(*position_service.PositionService),
		basePath:        "/warehouse/products",
	}
}

func (c *ProductsController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(middleware.RequireAuthorization())

	routes := []struct {
		Path    string
		Method  string
		Handler func(http.ResponseWriter, *http.Request)
	}{
		{"", http.MethodGet, c.List},
		{"", http.MethodPost, c.Create},
		{"/{id:[0-9]+}", http.MethodGet, c.GetEdit},
		{"/{id:[0-9]+}", http.MethodPost, c.PostEdit},
		{"/new", http.MethodGet, c.GetNew},
	}

	for _, route := range routes {
		router.HandleFunc(route.Path, route.Handler).Methods(route.Method)
	}
}

func (c *ProductsController) handleError(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func (c *ProductsController) preparePageContext(r *http.Request, titleKey string) (*types.PageContext, error) {
	return composables.UsePageCtx(r, types.NewPageData(titleKey, ""))
}

func (c *ProductsController) getViewModelProducts(r *http.Request) (*PaginatedResponse, error) {
	params := composables.UsePaginated(r)

	productEntities, err := c.productService.GetPaginated(r.Context(), params.Limit, params.Offset, []string{})
	if err != nil {
		return nil, fmt.Errorf("error retrieving products: %w", err)
	}

	viewProducts := mapping.MapViewModels(productEntities, mappers.ProductToViewModel)

	total, err := c.productService.Count(r.Context())
	if err != nil {
		return nil, fmt.Errorf("error counting products: %w", err)
	}

	return &PaginatedResponse{
		Products:        viewProducts,
		PaginationState: pagination.New(c.basePath, params.Page, int(total), params.Limit),
	}, nil
}

func (c *ProductsController) getViewModelPositions(r *http.Request) ([]*viewmodels.Position, error) {
	positions, err := c.positionService.GetAll(r.Context())
	if err != nil {
		return nil, fmt.Errorf("error retrieving positions: %w", err)
	}

	return mapping.MapViewModels(positions, mappers.PositionToViewModel), nil
}

func (c *ProductsController) renderTemplate(w http.ResponseWriter, r *http.Request, template templ.Component) {
	templ.Handler(template, templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *ProductsController) List(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := c.preparePageContext(r, "Products.List.Meta.Title")
	if err != nil {
		c.handleError(w, err)
		return
	}

	paginated, err := c.getViewModelProducts(r)
	if err != nil {
		c.handleError(w, err)
		return
	}

	isHxRequest := len(r.Header.Get("Hx-Request")) > 0
	props := &products.IndexPageProps{
		PageContext:     pageCtx,
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

	pageCtx, err := c.preparePageContext(r, "Products.Edit.Meta.Title")
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
		PageContext: pageCtx,
		Product:     mappers.ProductToViewModel(entity),
		Errors:      map[string]string{},
	}
	c.renderTemplate(w, r, products.Edit(props))
}

func (c *ProductsController) PostEdit(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
	if err != nil {
		c.handleError(w, err)
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
		if _, err := c.productService.Delete(r.Context(), id); err != nil {
			c.handleError(w, err)
			return
		}
	case shared.FormActionSave:
		dto := product.UpdateDTO{}
		pageCtx, err := c.preparePageContext(r, "Products.Edit.Meta.Title")
		if err != nil {
			c.handleError(w, err)
			return
		}

		if err := shared.Decoder.Decode(&dto, r.Form); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if errorsMap, ok := dto.Ok(pageCtx.UniTranslator); !ok {
			entity, err := c.productService.GetByID(r.Context(), id)
			if err != nil {
				c.handleError(w, fmt.Errorf("error retrieving product: %w", err))
				return
			}

			props := &products.EditPageProps{
				PageContext: pageCtx,
				Product:     mappers.ProductToViewModel(entity),
				Errors:      errorsMap,
			}
			c.renderTemplate(w, r, products.EditForm(props))
			return
		}

		if err := c.productService.Update(r.Context(), id, &dto); err != nil {
			c.handleError(w, err)
			return
		}
	}
	shared.Redirect(w, r, c.basePath)
}

func (c *ProductsController) GetNew(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := c.preparePageContext(r, "Products.New.Meta.Title")
	if err != nil {
		c.handleError(w, err)
		return
	}

	props := &products.CreatePageProps{
		PageContext: pageCtx,
		Errors:      map[string]string{},
		Product:     mappers.ProductToViewModel(&product.Product{}),
		SaveURL:     c.basePath,
	}
	c.renderTemplate(w, r, products.New(props))
}

func (c *ProductsController) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	dto := product.CreateDTO{}
	if err := shared.Decoder.Decode(&dto, r.Form); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pageCtx, err := c.preparePageContext(r, "Products.New.Meta.Title")
	if err != nil {
		c.handleError(w, err)
		return
	}

	if errorsMap, ok := dto.Ok(pageCtx.UniTranslator); !ok {
		entity, err := dto.ToEntity()
		if err != nil {
			c.handleError(w, err)
			return
		}

		props := &products.CreatePageProps{
			PageContext: pageCtx,
			Errors:      errorsMap,
			Product:     mappers.ProductToViewModel(entity),
			SaveURL:     c.basePath,
		}
		c.renderTemplate(w, r, products.CreateForm(props))
		return
	}

	if err := c.productService.Create(r.Context(), &dto); err != nil {
		c.handleError(w, err)
		return
	}

	shared.Redirect(w, r, c.basePath)
}
