package controllers

import (
	"github.com/a-h/templ"
	"github.com/go-faster/errors"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-erp/internal/application"
	"github.com/iota-agency/iota-erp/internal/modules/shared"
	"github.com/iota-agency/iota-erp/internal/modules/shared/middleware"
	"github.com/iota-agency/iota-erp/internal/modules/warehouse/domain/aggregates/product"
	"github.com/iota-agency/iota-erp/internal/modules/warehouse/mappers"
	"github.com/iota-agency/iota-erp/internal/modules/warehouse/services"
	"github.com/iota-agency/iota-erp/internal/modules/warehouse/viewmodels"
	"github.com/iota-agency/iota-erp/internal/types"
	"github.com/iota-agency/iota-erp/pkg/composables"
	"net/http"

	"github.com/iota-agency/iota-erp/internal/modules/warehouse/templates/pages/products"
)

type ProductsController struct {
	app             *application.Application
	productService  *services.ProductService
	positionService *services.PositionService
	basePath        string
}

func NewProductsController(app *application.Application) shared.Controller {
	return &ProductsController{
		app:             app,
		productService:  app.Service(services.ProductService{}).(*services.ProductService),
		positionService: app.Service(services.PositionService{}).(*services.PositionService),
		basePath:        "/warehouse/products",
	}
}

func (c *ProductsController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(middleware.RequireAuthorization())
	router.HandleFunc("", c.List).Methods(http.MethodGet)
	router.HandleFunc("", c.Create).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9]+}", c.GetEdit).Methods(http.MethodGet)
	router.HandleFunc("/{id:[0-9]+}", c.PostEdit).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9]+}", c.Delete).Methods(http.MethodDelete)
	router.HandleFunc("/new", c.GetNew).Methods(http.MethodGet)
}

func (c *ProductsController) viewModelProducts(r *http.Request) ([]*viewmodels.Product, error) {
	params := composables.UsePaginated(r)
	productEntities, err := c.productService.GetPaginated(r.Context(), params.Limit, params.Offset, []string{})
	if err != nil {
		return nil, errors.Wrap(err, "Error retrieving products")
	}
	viewProducts := make([]*viewmodels.Product, len(productEntities))
	for i, entity := range productEntities {
		viewProducts[i] = mappers.ProductToViewModel(entity)
	}
	return viewProducts, nil
}

func (c *ProductsController) viewModelPositions(r *http.Request) ([]*viewmodels.Position, error) {
	positions, err := c.positionService.GetAll(r.Context())
	if err != nil {
		return nil, errors.Wrap(err, "Error retrieving positions")
	}
	viewPositions := make([]*viewmodels.Position, len(positions))
	for i, position := range positions {
		viewPositions[i] = mappers.PositionToViewModel(position)
	}
	return viewPositions, nil
}

func (c *ProductsController) List(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := composables.UsePageCtx(
		r,
		types.NewPageData("Products.List.Meta.Title", ""),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	viewProducts, err := c.viewModelProducts(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	isHxRequest := len(r.Header.Get("Hx-Request")) > 0
	props := &products.IndexPageProps{
		PageContext: pageCtx,
		Products:    viewProducts,
	}
	if isHxRequest {
		templ.Handler(products.ProductsTable(props), templ.WithStreaming()).ServeHTTP(w, r)
	} else {
		templ.Handler(products.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *ProductsController) GetEdit(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	pageCtx, err := composables.UsePageCtx(
		r,
		types.NewPageData("Products.Edit.Meta.Title", ""),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	entity, err := c.productService.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "Error retrieving product", http.StatusInternalServerError)
		return
	}
	positions, err := c.viewModelPositions(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := &products.EditPageProps{
		PageContext: pageCtx,
		Product:     mappers.ProductToViewModel(entity),
		Positions:   positions,
		Errors:      map[string]string{},
	}
	templ.Handler(products.Edit(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *ProductsController) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}

	if _, err := c.productService.Delete(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	shared.Redirect(w, r, c.basePath)
}

func (c *ProductsController) PostEdit(w http.ResponseWriter, r *http.Request) {
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
		if _, err := c.productService.Delete(r.Context(), id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case shared.FormActionSave:
		dto := product.UpdateDTO{} //nolint:exhaustruct
		var pageCtx *types.PageContext
		pageCtx, err = composables.UsePageCtx(r, types.NewPageData("Products.Edit.Meta.Title", ""))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := shared.Decoder.Decode(&dto, r.Form); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if errorsMap, ok := dto.Ok(pageCtx.UniTranslator); !ok {
			entity, err := c.productService.GetByID(r.Context(), id)
			if err != nil {
				http.Error(w, "Error retrieving product", http.StatusInternalServerError)
				return
			}
			positions, err := c.viewModelPositions(r)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			props := &products.EditPageProps{
				PageContext: pageCtx,
				Product:     mappers.ProductToViewModel(entity),
				Positions:   positions,
				Errors:      errorsMap,
			}
			templ.Handler(products.EditForm(props), templ.WithStreaming()).ServeHTTP(w, r)
			return
		}
		if err := c.productService.Update(r.Context(), id, &dto); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	shared.Redirect(w, r, c.basePath)
}

func (c *ProductsController) GetNew(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := composables.UsePageCtx(r, types.NewPageData("Products.New.Meta.Title", ""))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	positions, err := c.viewModelPositions(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := &products.CreatePageProps{
		PageContext: pageCtx,
		Positions:   positions,
		Errors:      map[string]string{},
		Product:     mappers.ProductToViewModel(&product.Product{}), //nolint:exhaustruct
		SaveURL:     c.basePath,
	}
	templ.Handler(products.New(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *ProductsController) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	dto := product.CreateDTO{} //nolint:exhaustruct
	if err := shared.Decoder.Decode(&dto, r.Form); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pageCtx, err := composables.UsePageCtx(r, types.NewPageData("Products.New.Meta.Title", ""))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if errorsMap, ok := dto.Ok(pageCtx.UniTranslator); !ok {
		positions, err := c.viewModelPositions(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		entity, err := dto.ToEntity()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		props := &products.CreatePageProps{
			PageContext: pageCtx,
			Positions:   positions,
			Errors:      errorsMap,
			Product:     mappers.ProductToViewModel(entity),
			SaveURL:     c.basePath,
		}
		templ.Handler(products.CreateForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	if err := c.productService.Create(r.Context(), &dto); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}
