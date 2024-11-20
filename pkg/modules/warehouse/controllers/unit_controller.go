package controllers

import (
	"fmt"
	"github.com/iota-agency/iota-sdk/pkg/modules/warehouse/domain/entities/unit"
	"net/http"

	"github.com/a-h/templ"
	"github.com/go-faster/errors"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/modules/shared"
	"github.com/iota-agency/iota-sdk/pkg/modules/shared/middleware"
	"github.com/iota-agency/iota-sdk/pkg/modules/warehouse/mappers"
	"github.com/iota-agency/iota-sdk/pkg/modules/warehouse/services"
	"github.com/iota-agency/iota-sdk/pkg/modules/warehouse/templates/pages/units"
	"github.com/iota-agency/iota-sdk/pkg/modules/warehouse/viewmodels"
	"github.com/iota-agency/iota-sdk/pkg/types"
)

type UnitsController struct {
	app         *application.Application
	unitService *services.UnitService
	basePath    string
}

func NewUnitsController(app *application.Application) shared.Controller {
	return &UnitsController{
		app:         app,
		unitService: app.Service(services.UnitService{}).(*services.UnitService),
		basePath:    "/warehouse/units",
	}
}

func (c *UnitsController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(middleware.RequireAuthorization())
	router.HandleFunc("", c.List).Methods(http.MethodGet)
	router.HandleFunc("", c.Create).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9]+}", c.GetEdit).Methods(http.MethodGet)
	router.HandleFunc("/{id:[0-9]+}", c.PostEdit).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9]+}", c.Delete).Methods(http.MethodDelete)
	router.HandleFunc("/new", c.GetNew).Methods(http.MethodGet)
}

func (c *UnitsController) viewModelUnits(r *http.Request) ([]*viewmodels.Unit, error) {
	entities, err := c.unitService.GetAll(r.Context())
	if err != nil {
		return nil, errors.Wrap(err, "Error retrieving units")
	}
	viewUnits := make([]*viewmodels.Unit, len(entities))
	for i, u := range entities {
		viewUnits[i] = mappers.UnitToViewModel(u)
	}
	return viewUnits, nil
}

func (c *UnitsController) List(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := composables.UsePageCtx(
		r,
		types.NewPageData("WarehouseUnits.List.Meta.Title", ""),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	viewUnits, err := c.viewModelUnits(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	isHxRequest := len(r.Header.Get("Hx-Request")) > 0
	props := &units.IndexPageProps{
		PageContext: pageCtx,
		Units:       viewUnits,
	}
	if isHxRequest {
		templ.Handler(units.UnitsTable(props), templ.WithStreaming()).ServeHTTP(w, r)
	} else {
		templ.Handler(units.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *UnitsController) GetEdit(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	pageCtx, err := composables.UsePageCtx(
		r,
		types.NewPageData("WarehouseUnits.Edit.Meta.Title", ""),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	entity, err := c.unitService.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "Error retrieving unit", http.StatusInternalServerError)
		return
	}
	props := &units.EditPageProps{
		PageContext: pageCtx,
		Unit:        mappers.UnitToViewModel(entity),
		Errors:      map[string]string{},
	}
	templ.Handler(units.Edit(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *UnitsController) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}

	if _, err := c.unitService.Delete(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	shared.Redirect(w, r, c.basePath)
}

func (c *UnitsController) PostEdit(w http.ResponseWriter, r *http.Request) {
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
		if _, err := c.unitService.Delete(r.Context(), id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case shared.FormActionSave:
		dto := unit.UpdateDTO{} //nolint:exhaustruct
		var pageCtx *types.PageContext
		pageCtx, err = composables.UsePageCtx(r, types.NewPageData("WarehouseUnits.Edit.Meta.Title", ""))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := shared.Decoder.Decode(&dto, r.Form); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if errorsMap, ok := dto.Ok(pageCtx.UniTranslator); !ok {
			entity, err := c.unitService.GetByID(r.Context(), id)
			if err != nil {
				http.Error(w, "Error retrieving unit", http.StatusInternalServerError)
				return
			}
			props := &units.EditPageProps{
				PageContext: pageCtx,
				Unit:        mappers.UnitToViewModel(entity),
				Errors:      errorsMap,
				DeleteURL:   fmt.Sprintf("%s/%d", c.basePath, id),
			}
			templ.Handler(units.EditForm(props), templ.WithStreaming()).ServeHTTP(w, r)
			return
		}
		if err := c.unitService.Update(r.Context(), id, &dto); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	shared.Redirect(w, r, c.basePath)
}

func (c *UnitsController) GetNew(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := composables.UsePageCtx(r, types.NewPageData("WarehouseUnits.New.Meta.Title", ""))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := &units.CreatePageProps{
		PageContext: pageCtx,
		Errors:      map[string]string{},
		Unit:        mappers.UnitToViewModel(&unit.Unit{}), //nolint:exhaustruct
		SaveURL:     c.basePath,
	}
	templ.Handler(units.New(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *UnitsController) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	dto := unit.CreateDTO{} //nolint:exhaustruct
	if err := shared.Decoder.Decode(&dto, r.Form); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pageCtx, err := composables.UsePageCtx(r, types.NewPageData("WarehouseUnits.New.Meta.Title", ""))
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
		props := &units.CreatePageProps{
			PageContext: pageCtx,
			Errors:      errorsMap,
			Unit:        mappers.UnitToViewModel(entity),
		}
		templ.Handler(units.CreateForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	if err := c.unitService.Create(r.Context(), &dto); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}