package controllers

import (
	"fmt"
	"github.com/a-h/templ"
	"github.com/go-faster/errors"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-erp/internal/application"
	"github.com/iota-agency/iota-erp/internal/modules/shared"
	"github.com/iota-agency/iota-erp/internal/modules/shared/middleware"
	"github.com/iota-agency/iota-erp/internal/modules/warehouse/domain/entities/position"
	"github.com/iota-agency/iota-erp/internal/modules/warehouse/mappers"
	"github.com/iota-agency/iota-erp/internal/modules/warehouse/services"
	"github.com/iota-agency/iota-erp/internal/modules/warehouse/viewmodels"
	"github.com/iota-agency/iota-erp/internal/types"
	"github.com/iota-agency/iota-erp/pkg/composables"
	"net/http"

	"github.com/iota-agency/iota-erp/internal/modules/warehouse/templates/pages/positions"
)

type PositionsController struct {
	app             *application.Application
	positionService *services.PositionService
	unitService     *services.UnitService
	basePath        string
}

func NewPositionsController(app *application.Application) shared.Controller {
	return &PositionsController{
		app:             app,
		positionService: app.Service(services.PositionService{}).(*services.PositionService),
		unitService:     app.Service(services.UnitService{}).(*services.UnitService),
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
	router.HandleFunc("/{id:[0-9]+}", c.Delete).Methods(http.MethodDelete)
	router.HandleFunc("/new", c.GetNew).Methods(http.MethodGet)
}

func (c *PositionsController) viewModelPositions(r *http.Request) ([]*viewmodels.Position, error) {
	params := composables.UsePaginated(r)
	entities, err := c.positionService.GetPaginated(r.Context(), params.Limit, params.Offset, []string{})
	if err != nil {
		return nil, errors.Wrap(err, "Error retrieving positions")
	}
	viewPositions := make([]*viewmodels.Position, len(entities))
	for i, p := range entities {
		viewPositions[i] = mappers.PositionToViewModel(p)
	}
	return viewPositions, nil
}

func (c *PositionsController) viewModelUnits(r *http.Request) ([]*viewmodels.Unit, error) {
	entities, err := c.unitService.GetAll(r.Context())
	if err != nil {
		return nil, errors.Wrap(err, "Error retrieving units")
	}
	viewUnits := make([]*viewmodels.Unit, len(entities))
	for i, p := range entities {
		viewUnits[i] = mappers.UnitToViewModel(p)
	}
	return viewUnits, nil
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
	props := &positions.IndexPageProps{
		PageContext: pageCtx,
		Positions:   viewPositions,
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

func (c *PositionsController) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}

	if _, err := c.positionService.Delete(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	shared.Redirect(w, r, c.basePath)
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

	if err := c.positionService.Create(r.Context(), &dto); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}
