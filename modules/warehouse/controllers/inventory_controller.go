package controllers

import (
	"fmt"
	"net/http"

	"github.com/a-h/templ"
	"github.com/go-faster/errors"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-sdk/components/base/pagination"
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/position"
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/entities/inventory"
	"github.com/iota-agency/iota-sdk/modules/warehouse/mappers"
	"github.com/iota-agency/iota-sdk/modules/warehouse/services"
	"github.com/iota-agency/iota-sdk/modules/warehouse/services/position_service"
	inventorytemplate "github.com/iota-agency/iota-sdk/modules/warehouse/templates/pages/inventory"
	"github.com/iota-agency/iota-sdk/modules/warehouse/viewmodels"
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/mapping"
	"github.com/iota-agency/iota-sdk/pkg/middleware"
	"github.com/iota-agency/iota-sdk/pkg/shared"
	"github.com/iota-agency/iota-sdk/pkg/types"
)

type InventoryController struct {
	app              application.Application
	inventoryService *services.InventoryService
	positionService  *position_service.PositionService
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
		positionService:  app.Service(position_service.PositionService{}).(*position_service.PositionService),
	}
}

func (c *InventoryController) Register(r *mux.Router) {
	commonMiddleware := []mux.MiddlewareFunc{
		middleware.Authorize(),
		middleware.RequireAuthorization(),
		middleware.ProvideUser(),
		middleware.Tabs(),
		middleware.WithLocalizer(c.app.Bundle()),
		middleware.NavItems(c.app),
	}
	getRouter := r.PathPrefix(c.basePath).Subrouter()
	getRouter.Use(commonMiddleware...)
	getRouter.HandleFunc("", c.List).Methods(http.MethodGet)
	getRouter.HandleFunc("/new", c.GetNew).Methods(http.MethodGet)
	getRouter.HandleFunc("/new/partial", c.GetNewPartial).Methods(http.MethodGet)
	getRouter.HandleFunc("/positions/search", c.SearchPositions).Methods(http.MethodGet)
	getRouter.HandleFunc("/{id:[0-9]+}", c.GetEdit).Methods(http.MethodGet)
	getRouter.HandleFunc("/{id:[0-9]+}/difference", c.GetEditDifference).Methods(http.MethodGet)

	setRouter := r.PathPrefix(c.basePath).Subrouter()
	setRouter.Use(commonMiddleware...)
	setRouter.Use(middleware.WithTransaction())
	setRouter.HandleFunc("", c.Create).Methods(http.MethodPost)
	setRouter.HandleFunc("/partial", c.CreatePartial).Methods(http.MethodPost)
	setRouter.HandleFunc("/{id:[0-9]+}", c.PostEdit).Methods(http.MethodPost)
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
	pageCtx, err := composables.UsePageCtx(
		r,
		types.NewPageData("WarehouseInventory.List.Meta.Title", ""),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	paginated, err := c.viewModelChecks(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	isHxRequest := len(r.Header.Get("Hx-Request")) > 0
	props := &inventorytemplate.IndexPageProps{
		PageContext:     pageCtx,
		Checks:          paginated.Checks,
		PaginationState: paginated.PaginationState,
	}
	if isHxRequest {
		templ.Handler(inventorytemplate.InventoryTable(props), templ.WithStreaming()).ServeHTTP(w, r)
	} else {
		templ.Handler(inventorytemplate.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *InventoryController) GetNew(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := composables.UsePageCtx(r, types.NewPageData("WarehouseInventory.New.Meta.Title", ""))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := &inventorytemplate.CreatePageProps{
		PageContext: pageCtx,
		Errors:      map[string]string{},
		Check:       mappers.CheckToViewModel(&inventory.Check{}), //nolint:exhaustruct
		SaveURL:     c.basePath,
	}
	templ.Handler(inventorytemplate.New(props), templ.WithStreaming()).ServeHTTP(w, r)
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

func (c *InventoryController) GetNewPartial(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := composables.UsePageCtx(r, types.NewPageData("WarehouseInventory.New.Meta.Title", ""))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	check, err := composables.UseQuery(&inventory.Check{}, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	paginated, err := c.viewModelPositions(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := &inventorytemplate.CreatePageProps{
		PageContext:     pageCtx,
		Errors:          map[string]string{},
		Check:           mappers.CheckToViewModel(check), //nolint:exhaustruct
		Positions:       paginated.Positions,
		PaginationState: paginated.PaginationState,
		SaveURL:         c.basePath + "/partial",
	}
	templ.Handler(inventorytemplate.NewPartial(props), templ.WithStreaming()).ServeHTTP(w, r)
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

	pageCtx, err := composables.UsePageCtx(r, types.NewPageData("WarehouseUnits.New.Meta.Title", ""))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	u, err := composables.UseUser(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if errorsMap, ok := dto.Ok(pageCtx.UniTranslator); !ok {
		entity, err := dto.ToEntity(u.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		props := &inventorytemplate.CreatePageProps{
			PageContext: pageCtx,
			Errors:      errorsMap,
			Check:       mappers.CheckToViewModel(entity),
		}
		templ.Handler(inventorytemplate.CreateForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	if dto.Type == string(inventory.Partial) {
		values, err := shared.Encoder.Encode(dto)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		shared.Redirect(w, r, fmt.Sprintf("%s/new/partial?%s", c.basePath, values.Encode()))
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

	pageCtx, err := composables.UsePageCtx(
		r,
		types.NewPageData("WarehouseInventory.Edit.Meta.Title", ""),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	entity, err := c.inventoryService.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "Error retrieving inventory check", http.StatusInternalServerError)
		return
	}
	props := &inventorytemplate.EditPageProps{
		PageContext: pageCtx,
		Check:       mappers.CheckToViewModel(entity),
		Errors:      map[string]string{},
		DeleteURL:   fmt.Sprintf("%s/%d", c.basePath, entity.ID),
		SaveURL:     fmt.Sprintf("%s/%d", c.basePath, entity.ID),
	}
	templ.Handler(inventorytemplate.Edit(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *InventoryController) GetEditDifference(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	pageCtx, err := composables.UsePageCtx(
		r,
		types.NewPageData("WarehouseInventory.Edit.Meta.Title", ""),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	entity, err := c.inventoryService.GetByIDWithDifference(r.Context(), id)
	if err != nil {
		fmt.Println("DIFFERENCE: ", err)
		http.Error(w, "Error retrieving inventory check", http.StatusInternalServerError)
		return
	}
	props := &inventorytemplate.EditPageProps{
		PageContext: pageCtx,
		Check:       mappers.CheckToViewModel(entity),
		Errors:      map[string]string{},
		DeleteURL:   fmt.Sprintf("%s/%d", c.basePath, entity.ID),
		SaveURL:     fmt.Sprintf("%s/%d", c.basePath, entity.ID),
	}
	templ.Handler(inventorytemplate.Edit(props), templ.WithStreaming()).ServeHTTP(w, r)
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

func (c *InventoryController) PostEdit(w http.ResponseWriter, r *http.Request) {
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
		if _, err := c.inventoryService.Delete(r.Context(), id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case shared.FormActionSave:
		dto := inventory.UpdateCheckDTO{} //nolint:exhaustruct
		var pageCtx *types.PageContext
		pageCtx, err = composables.UsePageCtx(r, types.NewPageData("WarehousePositions.Edit.Meta.Title", ""))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := shared.Decoder.Decode(&dto, r.Form); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if errorsMap, ok := dto.Ok(pageCtx.UniTranslator); !ok {
			entity, err := c.inventoryService.GetByID(r.Context(), id)
			if err != nil {
				http.Error(w, "Error retrieving unit", http.StatusInternalServerError)
				return
			}
			props := &inventorytemplate.EditPageProps{
				PageContext: pageCtx,
				Check:       mappers.CheckToViewModel(entity),
				Errors:      errorsMap,
				DeleteURL:   fmt.Sprintf("%s/%d", c.basePath, id),
			}
			templ.Handler(inventorytemplate.EditForm(props), templ.WithStreaming()).ServeHTTP(w, r)
			return
		}
		if err := c.inventoryService.Update(r.Context(), id, &dto); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	shared.Redirect(w, r, c.basePath)
}

func (c *InventoryController) SearchPositions(w http.ResponseWriter, r *http.Request) {
	paginated, err := c.viewModelPositions(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	pageCtx, err := composables.UsePageCtx(r, types.NewPageData("WarehouseUnits.New.Meta.Title", ""))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := &inventorytemplate.CreatePageProps{
		PageContext:     pageCtx,
		Errors:          map[string]string{},
		Positions:       paginated.Positions,
		PaginationState: paginated.PaginationState,
		SaveURL:         c.basePath,
	}
	templ.Handler(inventorytemplate.AllPositionsTable(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *InventoryController) CreatePartial(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	dto := inventory.CreateCheckDTO{}
	if err := shared.Decoder.Decode(&dto, r.Form); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pageCtx, err := composables.UsePageCtx(r, types.NewPageData("WarehouseUnits.New.Meta.Title", ""))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	u, err := composables.UseUser(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if errorsMap, ok := dto.Ok(pageCtx.UniTranslator); !ok {
		entity, err := dto.ToEntity(u.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		props := &inventorytemplate.CreatePageProps{
			PageContext: pageCtx,
			Errors:      errorsMap,
			Check:       mappers.CheckToViewModel(entity),
		}
		templ.Handler(inventorytemplate.CreateForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	if _, err := c.inventoryService.Create(r.Context(), &dto); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}
