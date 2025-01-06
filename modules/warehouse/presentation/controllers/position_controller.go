package controllers

import (
	"fmt"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/entities/unit"
	"github.com/iota-uz/iota-sdk/modules/warehouse/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/warehouse/presentation/mappers"
	positions2 "github.com/iota-uz/iota-sdk/modules/warehouse/presentation/templates/pages/positions"
	viewmodels2 "github.com/iota-uz/iota-sdk/modules/warehouse/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/modules/warehouse/services/positionservice"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"net/http"
	"strconv"

	"github.com/a-h/templ"
	"github.com/go-faster/errors"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/components/base"
	"github.com/iota-uz/iota-sdk/components/base/pagination"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/position"
	"github.com/iota-uz/iota-sdk/modules/warehouse/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/shared"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

type PositionsController struct {
	app             application.Application
	positionService *positionservice.PositionService
	unitService     *services.UnitService
	basePath        string
}

type PositionPaginatedResponse struct {
	Positions       []*viewmodels2.Position
	PaginationState *pagination.State
}

func NewPositionsController(app application.Application) application.Controller {
	return &PositionsController{
		app:             app,
		positionService: app.Service(positionservice.PositionService{}).(*positionservice.PositionService),
		unitService:     app.Service(services.UnitService{}).(*services.UnitService),
		basePath:        "/warehouse/positions",
	}
}

func (c *PositionsController) Key() string {
	return c.basePath
}

func (c *PositionsController) Register(r *mux.Router) {
	commonMiddleware := []mux.MiddlewareFunc{
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.Tabs(),
		middleware.WithLocalizer(c.app.Bundle()),
		middleware.NavItems(),
	}
	getRouter := r.PathPrefix(c.basePath).Subrouter()
	getRouter.Use(commonMiddleware...)

	getRouter.HandleFunc("", c.List).Methods(http.MethodGet)
	getRouter.HandleFunc("/{id:[0-9]+}", c.GetEdit).Methods(http.MethodGet)
	getRouter.HandleFunc("/search", c.Search).Methods(http.MethodGet)
	getRouter.HandleFunc("/new", c.GetNew).Methods(http.MethodGet)
	getRouter.HandleFunc("/upload", c.GetUpload).Methods(http.MethodGet)

	setRouter := r.PathPrefix(c.basePath).Subrouter()
	setRouter.Use(commonMiddleware...)
	setRouter.Use(middleware.WithTransaction())

	setRouter.HandleFunc("", c.Create).Methods(http.MethodPost)
	setRouter.HandleFunc("/{id:[0-9]+}", c.PostEdit).Methods(http.MethodPost)
	setRouter.HandleFunc("/{id:[0-9]+}", c.Delete).Methods(http.MethodDelete)
	setRouter.HandleFunc("/upload", c.HandleUpload).Methods(http.MethodPost)
}

func (c *PositionsController) GetUpload(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := composables.UsePageCtx(r, types.NewPageData("WarehousePositions.Upload.Meta.Title", ""))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := &positions2.UploadPageProps{
		PageContext: pageCtx,
		SaveURL:     c.basePath + "/upload",
		Errors:      map[string]string{},
	}
	templ.Handler(positions2.Upload(props), templ.WithStreaming()).ServeHTTP(w, r)
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
		props := &positions2.UploadPageProps{
			PageContext: pageCtx,
			SaveURL:     c.basePath + "/upload",
			Errors:      errorsMap,
		}
		templ.Handler(positions2.UploadForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	if err := c.positionService.UpdateWithFile(r.Context(), dto.FileID); err != nil {
		var vErr serrors.Base
		if errors.As(err, &vErr) {
			props := &positions2.UploadPageProps{
				PageContext: pageCtx,
				SaveURL:     c.basePath + "/upload",
				Errors: map[string]string{
					"FileID": vErr.Localize(pageCtx.Localizer),
				},
			}
			templ.Handler(positions2.UploadForm(props), templ.WithStreaming()).ServeHTTP(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}

func (c *PositionsController) viewModelPositions(r *http.Request) (*PositionPaginatedResponse, error) {
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

func (c *PositionsController) viewModelUnits(r *http.Request) ([]*viewmodels2.Unit, error) {
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

	unitViewModels, err := c.viewModelUnits(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	isHxRequest := len(r.Header.Get("Hx-Request")) > 0
	props := &positions2.IndexPageProps{
		PageContext:     pageCtx,
		Positions:       paginated.Positions,
		Units:           unitViewModels,
		PaginationState: paginated.PaginationState,
	}
	if isHxRequest {
		templ.Handler(positions2.PositionsTable(props), templ.WithStreaming()).ServeHTTP(w, r)
	} else {
		templ.Handler(positions2.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
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
	props := &positions2.EditPageProps{
		PageContext: pageCtx,
		Position:    mappers.PositionToViewModel(entity),
		Units:       unitViewModels,
		Errors:      map[string]string{},
		SaveURL:     fmt.Sprintf("%s/%d", c.basePath, id),
		DeleteURL:   fmt.Sprintf("%s/%d", c.basePath, id),
	}
	templ.Handler(positions2.Edit(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *PositionsController) Search(w http.ResponseWriter, r *http.Request) {
	search := r.URL.Query().Get("q")
	entities, err := c.positionService.GetPaginated(r.Context(), &position.FindParams{
		Query: search,
		Field: "title",
		Limit: 10,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := mapping.MapViewModels(entities, func(pos *position.Position) *base.ComboboxOption {
		return &base.ComboboxOption{
			Value: strconv.FormatUint(uint64(pos.ID), 10),
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
		dto := position.UpdateDTO{}
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
			props := &positions2.EditPageProps{
				PageContext: pageCtx,
				Position:    mappers.PositionToViewModel(entity),
				Units:       unitViewModels,
				Errors:      errorsMap,
				SaveURL:     fmt.Sprintf("%s/%d", c.basePath, id),
				DeleteURL:   fmt.Sprintf("%s/%d", c.basePath, id),
			}
			templ.Handler(positions2.EditForm(props), templ.WithStreaming()).ServeHTTP(w, r)
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
	props := &positions2.CreatePageProps{
		PageContext: pageCtx,
		Errors:      map[string]string{},
		Position: mappers.PositionToViewModel(&position.Position{
			Unit: &unit.Unit{},
		}),
		SaveURL: c.basePath,
		Units:   unitViewModels,
	}
	templ.Handler(positions2.New(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *PositionsController) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	dto := position.CreateDTO{}
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
		props := &positions2.CreatePageProps{
			PageContext: pageCtx,
			Errors:      errorsMap,
			Position:    mappers.PositionToViewModel(entity),
		}
		templ.Handler(positions2.CreateForm(props), templ.WithStreaming()).ServeHTTP(w, r)
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
