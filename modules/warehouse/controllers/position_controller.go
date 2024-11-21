package controllers

import (
	"fmt"
	"github.com/a-h/templ"
	"github.com/go-faster/errors"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-sdk/components/base/selects"
	position2 "github.com/iota-agency/iota-sdk/modules/warehouse/domain/entities/position"
	"github.com/iota-agency/iota-sdk/modules/warehouse/mappers"
	services2 "github.com/iota-agency/iota-sdk/modules/warehouse/services"
	positions2 "github.com/iota-agency/iota-sdk/modules/warehouse/templates/pages/positions"
	viewmodels2 "github.com/iota-agency/iota-sdk/modules/warehouse/viewmodels"
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/mapping"
	"github.com/iota-agency/iota-sdk/pkg/shared"
	"github.com/iota-agency/iota-sdk/pkg/shared/middleware"
	"github.com/iota-agency/iota-sdk/pkg/types"
	"net/http"
)

type PositionsController struct {
	app             *application.Application
	positionService *services2.PositionService
	unitService     *services2.UnitService
	basePath        string
}

func NewPositionsController(app *application.Application) shared.Controller {
	return &PositionsController{
		app:             app,
		positionService: app.Service(services2.PositionService{}).(*services2.PositionService),
		unitService:     app.Service(services2.UnitService{}).(*services2.UnitService),
		basePath:        "/warehouse/positions",
	}
}

func (c *PositionsController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(middleware.RequireAuthorization())
	router.HandleFunc("", c.List).Methods(http.MethodGet)
	router.HandleFunc("", c.Create).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9]+}", c.GetEdit).Methods(http.MethodGet)
	router.HandleFunc("/{id:[0-9]+}", c.PostEdit).Methods(http.MethodPost)
	router.HandleFunc("/search", c.Search).Methods(http.MethodGet)
	router.HandleFunc("/new", c.GetNew).Methods(http.MethodGet)
}

func (c *PositionsController) viewModelPositions(r *http.Request) ([]*viewmodels2.Position, error) {
	params := composables.UsePaginated(r)
	entities, err := c.positionService.GetPaginated(r.Context(), &position2.FindParams{
		Limit:  params.Limit,
		Offset: params.Offset,
		SortBy: []string{"created_at desc"},
	})
	if err != nil {
		return nil, errors.Wrap(err, "Error retrieving positions")
	}
	return mapping.MapViewModels(entities, mappers.PositionToViewModel), nil
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

	viewPositions, err := c.viewModelPositions(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	isHxRequest := len(r.Header.Get("Hx-Request")) > 0
	props := &positions2.IndexPageProps{
		PageContext: pageCtx,
		Positions:   viewPositions,
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
	// query params
	search := r.URL.Query().Get("q")
	if search == "" {
		http.Error(w, "Search term is required", http.StatusBadRequest)
		return
	}
	entities, err := c.positionService.GetPaginated(r.Context(), &position2.FindParams{
		Search: search,
		Limit:  10,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	pageCtx, err := composables.UsePageCtx(r, types.NewPageData("WarehousePositions.List.Meta.Title", ""))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := &selects.SearchOptionsProps{
		PageContext: pageCtx,
		Options: mapping.MapViewModels(entities, func(pos *position2.Position) *selects.Value {
			return &selects.Value{
				Value: fmt.Sprintf("%d", pos.ID),
				Label: pos.Title,
			}
		}),
		NothingFoundText: pageCtx.T("WarehousePositions.Single.NothingFound"),
	}
	templ.Handler(selects.SearchOptions(props), templ.WithStreaming()).ServeHTTP(w, r)
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
		dto := position2.UpdateDTO{} //nolint:exhaustruct
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
		Position:    mappers.PositionToViewModel(&position2.Position{}), //nolint:exhaustruct
		SaveURL:     c.basePath,
		Units:       unitViewModels,
	}
	templ.Handler(positions2.New(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *PositionsController) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	dto := position2.CreateDTO{} //nolint:exhaustruct
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

	if err := c.positionService.Create(r.Context(), &dto); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}
