package controllers

import (
	"fmt"
	"net/http"

	"github.com/a-h/templ"
	"github.com/go-faster/errors"
	"github.com/gorilla/mux"

	"github.com/iota-uz/iota-sdk/components/base/pagination"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/position"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/entities/inventory"
	"github.com/iota-uz/iota-sdk/modules/warehouse/presentation/mappers"
	inventory2 "github.com/iota-uz/iota-sdk/modules/warehouse/presentation/templates/pages/inventory"
	"github.com/iota-uz/iota-sdk/modules/warehouse/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/modules/warehouse/services"
	"github.com/iota-uz/iota-sdk/modules/warehouse/services/positionservice"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/shared"
)

type InventoryController struct {
	app              application.Application
	inventoryService *services.InventoryService
	positionService  *positionservice.PositionService
	basePath         string
}

type InventoryCheckPaginatedResponse struct {
	Checks          []*viewmodels.Check
	PaginationState *pagination.State
}

func NewInventoryController(app application.Application) application.Controller {
	return &InventoryController{
		app:              app,
		basePath:         "/warehouse/inventory",
		inventoryService: app.Service(services.InventoryService{}).(*services.InventoryService),
		positionService:  app.Service(positionservice.PositionService{}).(*positionservice.PositionService),
	}
}

func (c *InventoryController) Key() string {
	return c.basePath
}

func (c *InventoryController) Register(r *mux.Router) {
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
	getRouter.HandleFunc("/positions/search", c.SearchPositions).Methods(http.MethodGet)
	getRouter.HandleFunc("/{id:[0-9]+}", c.GetEdit).Methods(http.MethodGet)
	getRouter.HandleFunc("/{id:[0-9]+}/difference", c.GetEditDifference).Methods(http.MethodGet)

	setRouter := r.PathPrefix(c.basePath).Subrouter()
	setRouter.Use(commonMiddleware...)
	setRouter.Use(middleware.WithTransaction())
	setRouter.HandleFunc("", c.Create).Methods(http.MethodPost)
	setRouter.HandleFunc("/{id:[0-9]+}", c.Update).Methods(http.MethodPost)
	setRouter.HandleFunc("/{id:[0-9]+}", c.Delete).Methods(http.MethodDelete)
}

func (c *InventoryController) viewModelChecks(r *http.Request) (*InventoryCheckPaginatedResponse, error) {
	paginationParams := composables.UsePaginated(r)
	params, err := composables.UseQuery(&inventory.FindParams{
		Limit:  paginationParams.Limit,
		Offset: paginationParams.Offset,
		SortBy: []string{"created_at desc"},
	}, r)
	if err != nil {
		return nil, errors.Wrap(err, "Error retrieving query")
	}
	entities, err := c.inventoryService.GetPaginated(r.Context(), params)
	if err != nil {
		return nil, errors.Wrap(err, "Error retrieving inventory checks")
	}
	viewChecks := mapping.MapViewModels(entities, mappers.CheckToViewModel)
	total, err := c.inventoryService.Count(r.Context())
	if err != nil {
		return nil, errors.Wrap(err, "Error counting inventory checks")
	}
	return &InventoryCheckPaginatedResponse{
		PaginationState: pagination.New(c.basePath, paginationParams.Page, int(total), params.Limit),
		Checks:          viewChecks,
	}, nil
}

func (c *InventoryController) List(w http.ResponseWriter, r *http.Request) {
	paginated, err := c.viewModelChecks(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	isHxRequest := len(r.Header.Get("Hx-Request")) > 0
	props := &inventory2.IndexPageProps{
		Checks:          paginated.Checks,
		PaginationState: paginated.PaginationState,
	}
	if isHxRequest {
		templ.Handler(inventory2.InventoryTable(props), templ.WithStreaming()).ServeHTTP(w, r)
	} else {
		templ.Handler(inventory2.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *InventoryController) GetNew(w http.ResponseWriter, r *http.Request) {
	props := &inventory2.CreatePageProps{
		Errors:  map[string]string{},
		Check:   mappers.CheckToViewModel(&inventory.Check{}),
		SaveURL: c.basePath,
	}
	templ.Handler(inventory2.New(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *InventoryController) viewModelPositions(r *http.Request) (*PositionPaginatedResponse, error) {
	paginationParams := composables.UsePaginated(r)
	params, err := composables.UseQuery(&position.FindParams{
		Limit:  paginationParams.Limit,
		Offset: paginationParams.Offset,
		SortBy: []string{"created_at desc"},
	}, r)
	if err != nil {
		return nil, errors.Wrap(err, "Error retrieving query")
	}
	entities, err := c.positionService.GetPaginated(r.Context(), params)
	if err != nil {
		return nil, errors.Wrap(err, "Error retrieving positions")
	}
	total, err := c.positionService.Count(r.Context())
	if err != nil {
		return nil, errors.Wrap(err, "Error counting positions")
	}
	return &PositionPaginatedResponse{
		PaginationState: pagination.New(c.basePath, paginationParams.Page, int(total), params.Limit),
		Positions:       mapping.MapViewModels(entities, mappers.PositionToViewModel),
	}, nil
}

func (c *InventoryController) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	dto := inventory.CreateCheckDTO{}
	if err := shared.Decoder.Decode(&dto, r.Form); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	u, err := composables.UseUser(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	uniLocalizer, err := intl.UseUniLocalizer(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if errorsMap, ok := dto.Ok(uniLocalizer); !ok {
		entity, err := dto.ToEntity(u)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		props := &inventory2.CreatePageProps{
			Errors: errorsMap,
			Check:  mappers.CheckToViewModel(entity),
		}
		templ.Handler(inventory2.CreateForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	if _, err := c.inventoryService.Create(r.Context(), &dto); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}

func (c *InventoryController) GetEdit(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	entity, err := c.inventoryService.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "Error retrieving inventory check", http.StatusInternalServerError)
		return
	}
	props := &inventory2.EditPageProps{
		Check:     mappers.CheckToViewModel(entity),
		Errors:    map[string]string{},
		DeleteURL: fmt.Sprintf("%s/%d", c.basePath, entity.ID),
		SaveURL:   fmt.Sprintf("%s/%d", c.basePath, entity.ID),
	}
	templ.Handler(inventory2.Edit(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *InventoryController) GetEditDifference(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	entity, err := c.inventoryService.GetByIDWithDifference(r.Context(), id)
	if err != nil {
		http.Error(w, "Error retrieving inventory check", http.StatusInternalServerError)
		return
	}
	props := &inventory2.EditPageProps{
		Check:     mappers.CheckToViewModel(entity),
		Errors:    map[string]string{},
		DeleteURL: fmt.Sprintf("%s/%d", c.basePath, entity.ID),
		SaveURL:   fmt.Sprintf("%s/%d", c.basePath, entity.ID),
	}
	templ.Handler(inventory2.Edit(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *InventoryController) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}

	if _, err := c.inventoryService.Delete(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	shared.Redirect(w, r, c.basePath)
}

func (c *InventoryController) Update(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}
	dto := inventory.UpdateCheckDTO{}
	if err := shared.Decoder.Decode(&dto, r.Form); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	uniLocalizer, err := intl.UseUniLocalizer(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if errorsMap, ok := dto.Ok(uniLocalizer); !ok {
		entity, err := c.inventoryService.GetByID(r.Context(), id)
		if err != nil {
			http.Error(w, "Error retrieving unit", http.StatusInternalServerError)
			return
		}
		props := &inventory2.EditPageProps{
			Check:     mappers.CheckToViewModel(entity),
			Errors:    errorsMap,
			DeleteURL: fmt.Sprintf("%s/%d", c.basePath, id),
		}
		templ.Handler(inventory2.EditForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}
	if err := c.inventoryService.Update(r.Context(), id, &dto); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	shared.Redirect(w, r, c.basePath)
}

func (c *InventoryController) SearchPositions(w http.ResponseWriter, r *http.Request) {
	paginated, err := c.viewModelPositions(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := &inventory2.CreatePageProps{
		Errors:          map[string]string{},
		Positions:       paginated.Positions,
		PaginationState: paginated.PaginationState,
		SaveURL:         c.basePath,
	}
	templ.Handler(inventory2.AllPositionsTable(props), templ.WithStreaming()).ServeHTTP(w, r)
}
