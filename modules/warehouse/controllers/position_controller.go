package controllers

import (
	"fmt"
	"github.com/iota-agency/iota-sdk/modules/warehouse/controllers/dtos"
	"github.com/iota-agency/iota-sdk/modules/warehouse/services/position_service"
	"github.com/iota-agency/iota-sdk/pkg/middleware"
	"github.com/iota-agency/iota-sdk/pkg/serrors"
	coreservices "github.com/iota-agency/iota-sdk/pkg/services"
	"net/http"

	"github.com/a-h/templ"
	"github.com/go-faster/errors"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-sdk/components/base"
	"github.com/iota-agency/iota-sdk/components/base/pagination"
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/position"
	"github.com/iota-agency/iota-sdk/modules/warehouse/mappers"
	"github.com/iota-agency/iota-sdk/modules/warehouse/services"
	"github.com/iota-agency/iota-sdk/modules/warehouse/templates/pages/positions"
	"github.com/iota-agency/iota-sdk/modules/warehouse/viewmodels"
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/mapping"
	"github.com/iota-agency/iota-sdk/pkg/shared"
	"github.com/iota-agency/iota-sdk/pkg/types"
)

type PositionsController struct {
	app             application.Application
	positionService *position_service.PositionService
	unitService     *services.UnitService
	basePath        string
}

type PositionPaginatedResponse struct {
	Positions       []*viewmodels.Position
	PaginationState *pagination.State
}

func NewPositionsController(app application.Application) application.Controller {
	return &PositionsController{
		app:             app,
		positionService: app.Service(position_service.PositionService{}).(*position_service.PositionService),
		unitService:     app.Service(services.UnitService{}).(*services.UnitService),
		basePath:        "/warehouse/positions",
	}
}

func (c *PositionsController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(
		middleware.WithTransaction(),
		middleware.Authorize(c.app.Service(coreservices.AuthService{}).(*coreservices.AuthService)),
		middleware.RequireAuthorization(),
		middleware.ProvideUser(c.app.Service(coreservices.UserService{}).(*coreservices.UserService)),
		middleware.Tabs(c.app.Service(coreservices.TabService{}).(*coreservices.TabService)),
		middleware.WithLocalizer(c.app.Bundle()),
		middleware.NavItems(c.app),
	)

	router.HandleFunc("", c.List).Methods(http.MethodGet)
	router.HandleFunc("", c.Create).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9]+}", c.GetEdit).Methods(http.MethodGet)
	router.HandleFunc("/{id:[0-9]+}", c.PostEdit).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9]+}", c.Delete).Methods(http.MethodDelete)
	router.HandleFunc("/search", c.Search).Methods(http.MethodGet)
	router.HandleFunc("/new", c.GetNew).Methods(http.MethodGet)
	router.HandleFunc("/upload", c.GetUpload).Methods(http.MethodGet)
	router.HandleFunc("/upload", c.HandleUpload).Methods(http.MethodPost)
}

func (c *PositionsController) GetUpload(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := composables.UsePageCtx(r, types.NewPageData("WarehousePositions.Upload.Meta.Title", ""))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := &positions.UploadPageProps{
		PageContext: pageCtx,
		SaveURL:     c.basePath + "/upload",
		Errors:      map[string]string{},
	}
	templ.Handler(positions.Upload(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *PositionsController) HandleUpload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	dto := dtos.PositionsUploadDTO{}
	if err := shared.Decoder.Decode(&dto, r.Form); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	pageCtx, err := composables.UsePageCtx(r, types.NewPageData("WarehousePositions.Upload.Meta.Title", ""))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if errorsMap, ok := dto.Ok(pageCtx.UniTranslator); !ok {
		props := &positions.UploadPageProps{
			PageContext: pageCtx,
			SaveURL:     c.basePath + "/upload",
			Errors:      errorsMap,
		}
		templ.Handler(positions.UploadForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	if err := c.positionService.UpdateWithFile(r.Context(), dto.FileID); err != nil {
		var vErr serrors.BaseError
		if errors.As(err, &vErr) {
			props := &positions.UploadPageProps{
				PageContext: pageCtx,
				SaveURL:     c.basePath + "/upload",
				Errors: map[string]string{
					"FileID": vErr.Localize(pageCtx.Localizer),
				},
			}
			templ.Handler(positions.UploadForm(props), templ.WithStreaming()).ServeHTTP(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}

func (c *PositionsController) viewModelPositions(r *http.Request) (*PositionPaginatedResponse, error) {
	params := composables.UsePaginated(r)
	entities, err := c.positionService.GetPaginated(r.Context(), &position.FindParams{
		Limit:  params.Limit,
		Offset: params.Offset,
		SortBy: []string{"created_at desc"},
	})
	if err != nil {
		return nil, errors.Wrap(err, "Error retrieving positions")
	}
	total, err := c.positionService.Count(r.Context())
	if err != nil {
		return nil, errors.Wrap(err, "Error counting positions")
	}
	return &PositionPaginatedResponse{
		PaginationState: pagination.New(c.basePath, params.Page, int(total), params.Limit),
		Positions:       mapping.MapViewModels(entities, mappers.PositionToViewModel),
	}, nil
}

func (c *PositionsController) viewModelUnits(r *http.Request) ([]*viewmodels.Unit, error) {
	entities, err := c.unitService.GetAll(r.Context())
	if err != nil {
		return nil, errors.Wrap(err, "Error retrieving units")
	}
	return mapping.MapViewModels(entities, mappers.UnitToViewModel), nil
}

func (c *PositionsController) List(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := composables.UsePageCtx(
		r,
		types.NewPageData("WarehousePositions.List.Meta.Title", ""),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	paginated, err := c.viewModelPositions(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	isHxRequest := len(r.Header.Get("Hx-Request")) > 0
	props := &positions.IndexPageProps{
		PageContext:     pageCtx,
		Positions:       paginated.Positions,
		PaginationState: paginated.PaginationState,
	}
	if isHxRequest {
		templ.Handler(positions.PositionsTable(props), templ.WithStreaming()).ServeHTTP(w, r)
	} else {
		templ.Handler(positions.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *PositionsController) GetEdit(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	pageCtx, err := composables.UsePageCtx(
		r,
		types.NewPageData("WarehousePositions.Edit.Meta.Title", ""),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	entity, err := c.positionService.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "Error retrieving position", http.StatusInternalServerError)
		return
	}
	unitViewModels, err := c.viewModelUnits(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := &positions.EditPageProps{
		PageContext: pageCtx,
		Position:    mappers.PositionToViewModel(entity),
		Units:       unitViewModels,
		Errors:      map[string]string{},
		SaveURL:     fmt.Sprintf("%s/%d", c.basePath, id),
		DeleteURL:   fmt.Sprintf("%s/%d", c.basePath, id),
	}
	templ.Handler(positions.Edit(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *PositionsController) Search(w http.ResponseWriter, r *http.Request) {
	search := r.URL.Query().Get("q")
	entities, err := c.positionService.GetPaginated(r.Context(), &position.FindParams{
		Search: search,
		Limit:  10,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := mapping.MapViewModels(entities, func(pos *position.Position) *base.ComboboxOption {
		return &base.ComboboxOption{
			Value: fmt.Sprintf("%d", pos.ID),
			Label: pos.Title,
		}
	})
	templ.Handler(base.ComboboxOptions(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *PositionsController) PostEdit(w http.ResponseWriter, r *http.Request) {
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
		if _, err := c.positionService.Delete(r.Context(), id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case shared.FormActionSave:
		dto := position.UpdateDTO{} //nolint:exhaustruct
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
			entity, err := c.positionService.GetByID(r.Context(), id)
			if err != nil {
				http.Error(w, "Error retrieving position", http.StatusInternalServerError)
				return
			}
			unitViewModels, err := c.viewModelUnits(r)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			props := &positions.EditPageProps{
				PageContext: pageCtx,
				Position:    mappers.PositionToViewModel(entity),
				Units:       unitViewModels,
				Errors:      errorsMap,
				SaveURL:     fmt.Sprintf("%s/%d", c.basePath, id),
				DeleteURL:   fmt.Sprintf("%s/%d", c.basePath, id),
			}
			templ.Handler(positions.EditForm(props), templ.WithStreaming()).ServeHTTP(w, r)
			return
		}
		if err := c.positionService.Update(r.Context(), id, &dto); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	shared.Redirect(w, r, c.basePath)
}

func (c *PositionsController) GetNew(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := composables.UsePageCtx(r, types.NewPageData("WarehousePositions.New.Meta.Title", ""))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	unitViewModels, err := c.viewModelUnits(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := &positions.CreatePageProps{
		PageContext: pageCtx,
		Errors:      map[string]string{},
		Position:    mappers.PositionToViewModel(&position.Position{}), //nolint:exhaustruct
		SaveURL:     c.basePath,
		Units:       unitViewModels,
	}
	templ.Handler(positions.New(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *PositionsController) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	dto := position.CreateDTO{} //nolint:exhaustruct
	if err := shared.Decoder.Decode(&dto, r.Form); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pageCtx, err := composables.UsePageCtx(r, types.NewPageData("WarehousePositions.New.Meta.Title", ""))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if errorsMap, ok := dto.Ok(pageCtx.UniTranslator); !ok {
		entity, err := dto.ToEntity()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		props := &positions.CreatePageProps{
			PageContext: pageCtx,
			Errors:      errorsMap,
			Position:    mappers.PositionToViewModel(entity),
		}
		templ.Handler(positions.CreateForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	if _, err := c.positionService.Create(r.Context(), &dto); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}

func (c *PositionsController) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := c.positionService.Delete(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	shared.Redirect(w, r, c.basePath)
}
