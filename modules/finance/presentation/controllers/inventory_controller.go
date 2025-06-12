package controllers

import (
	"fmt"
	"net/http"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/components/base/pagination"
	coremappers "github.com/iota-uz/iota-sdk/modules/core/presentation/mappers"
	coreviewmodels "github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/inventory"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/mappers"
	inventoryTemplates "github.com/iota-uz/iota-sdk/modules/finance/presentation/templates/pages/inventory"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/shared"
)

type InventoryController struct {
	app              application.Application
	inventoryService *services.InventoryService
	currencyService  *coreservices.CurrencyService
	basePath         string
}

type InventoryPaginatedResponse struct {
	Items           []*viewmodels.Inventory
	PaginationState *pagination.State
}

func NewInventoryController(app application.Application) application.Controller {
	return &InventoryController{
		app:              app,
		inventoryService: app.Service(services.InventoryService{}).(*services.InventoryService),
		currencyService:  app.Service(coreservices.CurrencyService{}).(*coreservices.CurrencyService),
		basePath:         "/finance/inventory",
	}
}

func (c *InventoryController) Key() string {
	return c.basePath
}

func (c *InventoryController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.Tabs(),
		middleware.ProvideLocalizer(c.app.Bundle()),
		middleware.NavItems(),
		middleware.WithPageContext(),
	)
	router.HandleFunc("", c.List).Methods(http.MethodGet)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}", c.GetEdit).Methods(http.MethodGet)
	router.HandleFunc("/new", c.GetNew).Methods(http.MethodGet)

	router.HandleFunc("", c.Create).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}", c.Update).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}", c.Delete).Methods(http.MethodDelete)
}

func (c *InventoryController) viewModelCurrencies(r *http.Request) ([]*coreviewmodels.Currency, error) {
	currencies, err := c.currencyService.GetAll(r.Context())
	if err != nil {
		return nil, err
	}
	return mapping.MapViewModels(currencies, coremappers.CurrencyToViewModel), nil
}

func (c *InventoryController) viewModelInventory(r *http.Request) (*InventoryPaginatedResponse, error) {
	paginationParams := composables.UsePaginated(r)
	params := &inventory.FindParams{
		Limit:  paginationParams.Limit,
		Offset: paginationParams.Offset,
	}

	if query := r.URL.Query().Get("query"); query != "" {
		params.Query = query
		params.Field = "name"
	}

	items, err := c.inventoryService.GetPaginated(r.Context(), params)
	if err != nil {
		return nil, err
	}

	total, err := c.inventoryService.Count(r.Context())
	if err != nil {
		return nil, err
	}

	viewItems := mapping.MapViewModels(items, mappers.InventoryToViewModel)

	return &InventoryPaginatedResponse{
		Items:           viewItems,
		PaginationState: pagination.New(c.basePath, paginationParams.Page, int(total), params.Limit),
	}, nil
}

func (c *InventoryController) List(w http.ResponseWriter, r *http.Request) {
	paginated, err := c.viewModelInventory(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	isHxRequest := len(r.Header.Get("Hx-Request")) > 0
	props := &inventoryTemplates.IndexPageProps{
		Inventory:       paginated.Items,
		PaginationState: paginated.PaginationState,
	}
	if isHxRequest {
		templ.Handler(inventoryTemplates.InventoryTable(props), templ.WithStreaming()).ServeHTTP(w, r)
	} else {
		templ.Handler(inventoryTemplates.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *InventoryController) GetEdit(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseUUID(r)
	if err != nil {
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}

	entity, err := c.inventoryService.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "Error retrieving inventory item", http.StatusInternalServerError)
		return
	}
	currencies, err := c.viewModelCurrencies(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := &inventoryTemplates.EditPageProps{
		Inventory:  mappers.InventoryToViewModel(entity),
		Currencies: currencies,
		Errors:     map[string]string{},
		PostPath:   fmt.Sprintf("%s/%s", c.basePath, id.String()),
		DeletePath: fmt.Sprintf("%s/%s", c.basePath, id.String()),
	}
	templ.Handler(inventoryTemplates.Edit(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *InventoryController) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseUUID(r)
	if err != nil {
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}

	if err := c.inventoryService.Delete(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	shared.Redirect(w, r, c.basePath)
}

func (c *InventoryController) Update(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseUUID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	dto := dtos.InventoryUpdateDTO{}
	if err := shared.Decoder.Decode(&dto, r.Form); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if errorsMap, ok := dto.Ok(r.Context()); !ok {
		entity, err := c.inventoryService.GetByID(r.Context(), id)
		if err != nil {
			http.Error(w, "Error retrieving inventory item", http.StatusInternalServerError)
			return
		}
		currencies, err := c.viewModelCurrencies(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		props := &inventoryTemplates.EditPageProps{
			Inventory:  mappers.InventoryToViewModel(entity),
			Currencies: currencies,
			Errors:     errorsMap,
			PostPath:   fmt.Sprintf("%s/%s", c.basePath, id.String()),
			DeletePath: fmt.Sprintf("%s/%s", c.basePath, id.String()),
		}
		templ.Handler(inventoryTemplates.EditForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	existing, err := c.inventoryService.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "Error retrieving inventory item", http.StatusInternalServerError)
		return
	}

	entity, err := dto.Apply(existing)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := c.inventoryService.Update(r.Context(), entity); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}

func (c *InventoryController) GetNew(w http.ResponseWriter, r *http.Request) {
	currencies, err := c.viewModelCurrencies(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := &inventoryTemplates.CreatePageProps{
		Currencies: currencies,
		Errors:     map[string]string{},
		Inventory:  &viewmodels.Inventory{},
		PostPath:   c.basePath,
	}
	templ.Handler(inventoryTemplates.New(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *InventoryController) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	dto := dtos.InventoryCreateDTO{}
	if err := shared.Decoder.Decode(&dto, r.Form); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if errorsMap, ok := dto.Ok(r.Context()); !ok {
		currencies, err := c.viewModelCurrencies(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		entity, err := dto.ToEntity()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		props := &inventoryTemplates.CreatePageProps{
			Currencies: currencies,
			Errors:     errorsMap,
			Inventory:  mappers.InventoryToViewModel(entity),
			PostPath:   c.basePath,
		}
		templ.Handler(inventoryTemplates.CreateForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	entity, err := dto.ToEntity()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err := c.inventoryService.Create(r.Context(), entity); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}
