package controllers

import (
	"fmt"
	"net/http"

	"github.com/iota-uz/iota-sdk/modules/warehouse/presentation/mappers"
	units2 "github.com/iota-uz/iota-sdk/modules/warehouse/presentation/templates/pages/units"
	"github.com/iota-uz/iota-sdk/modules/warehouse/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/middleware"

	"github.com/a-h/templ"
	"github.com/go-faster/errors"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/components/base/pagination"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/entities/unit"
	"github.com/iota-uz/iota-sdk/modules/warehouse/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/shared"
)

type UnitsController struct {
	app         application.Application
	unitService *services.UnitService
	basePath    string
}

type UnitPaginatedResponse struct {
	Units           []*viewmodels.Unit
	PaginationState *pagination.State
}

func NewUnitsController(app application.Application) application.Controller {
	return &UnitsController{
		app:         app,
		unitService: app.Service(services.UnitService{}).(*services.UnitService),
		basePath:    "/warehouse/units",
	}
}

func (c *UnitsController) Key() string {
	return c.basePath
}

func (c *UnitsController) Register(r *mux.Router) {
	commonMiddleware := []mux.MiddlewareFunc{
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.Tabs(),
		middleware.ProvideLocalizer(c.app.Bundle()),
		middleware.NavItems(),
		middleware.WithPageContext(),
	}

	getRouter := r.PathPrefix(c.basePath).Subrouter()
	getRouter.Use(commonMiddleware...)
	getRouter.HandleFunc("", c.List).Methods(http.MethodGet)
	getRouter.HandleFunc("/new", c.GetNew).Methods(http.MethodGet)
	getRouter.HandleFunc("/{id:[0-9]+}", c.GetEdit).Methods(http.MethodGet)

	setRouter := r.PathPrefix(c.basePath).Subrouter()
	setRouter.Use(commonMiddleware...)
	setRouter.Use(middleware.WithTransaction())
	setRouter.HandleFunc("", c.Create).Methods(http.MethodPost)
	setRouter.HandleFunc("/{id:[0-9]+}", c.Update).Methods(http.MethodPost)
	setRouter.HandleFunc("/{id:[0-9]+}", c.Delete).Methods(http.MethodDelete)
}

func (c *UnitsController) viewModelUnits(r *http.Request) (*UnitPaginatedResponse, error) {
	paginationParams := composables.UsePaginated(r)
	params, err := composables.UseQuery(&unit.FindParams{
		Limit:  paginationParams.Limit,
		Offset: paginationParams.Offset,
	}, r)
	if err != nil {
		return nil, errors.Wrap(err, "Error retrieving query")
	}
	entities, err := c.unitService.GetPaginated(r.Context(), params)
	if err != nil {
		return nil, errors.Wrap(err, "Error retrieving units")
	}
	viewUnits := mapping.MapViewModels(entities, mappers.UnitToViewModel)
	total, err := c.unitService.Count(r.Context())
	if err != nil {
		return nil, errors.Wrap(err, "Error counting units")
	}
	return &UnitPaginatedResponse{
		PaginationState: pagination.New(c.basePath, paginationParams.Page, int(total), params.Limit),
		Units:           viewUnits,
	}, nil
}

func (c *UnitsController) List(w http.ResponseWriter, r *http.Request) {
	paginated, err := c.viewModelUnits(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	isHxRequest := len(r.Header.Get("Hx-Request")) > 0
	props := &units2.IndexPageProps{
		Units:           paginated.Units,
		PaginationState: paginated.PaginationState,
	}
	if isHxRequest {
		templ.Handler(units2.UnitsTable(props), templ.WithStreaming()).ServeHTTP(w, r)
	} else {
		templ.Handler(units2.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *UnitsController) GetEdit(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	entity, err := c.unitService.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "Error retrieving unit", http.StatusInternalServerError)
		return
	}
	props := &units2.EditPageProps{
		Unit:   mappers.UnitToViewModel(entity),
		Errors: map[string]string{},
	}
	templ.Handler(units2.Edit(props), templ.WithStreaming()).ServeHTTP(w, r)
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

func (c *UnitsController) Update(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}
	dto := unit.UpdateDTO{}
	if err := shared.Decoder.Decode(&dto, r.Form); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	uniTranslator, err := intl.UseUniLocalizer(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if errorsMap, ok := dto.Ok(uniTranslator); !ok {
		entity, err := c.unitService.GetByID(r.Context(), id)
		if err != nil {
			http.Error(w, "Error retrieving unit", http.StatusInternalServerError)
			return
		}
		props := &units2.EditPageProps{
			Unit:      mappers.UnitToViewModel(entity),
			Errors:    errorsMap,
			DeleteURL: fmt.Sprintf("%s/%d", c.basePath, id),
		}
		templ.Handler(units2.EditForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}
	if err := c.unitService.Update(r.Context(), id, &dto); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	shared.Redirect(w, r, c.basePath)
}

func (c *UnitsController) GetNew(w http.ResponseWriter, r *http.Request) {
	props := &units2.CreatePageProps{
		Errors:  map[string]string{},
		Unit:    mappers.UnitToViewModel(&unit.Unit{}),
		SaveURL: c.basePath,
	}
	templ.Handler(units2.New(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *UnitsController) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	dto := unit.CreateDTO{}
	if err := shared.Decoder.Decode(&dto, r.Form); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	uniTranslator, err := intl.UseUniLocalizer(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if errorsMap, ok := dto.Ok(uniTranslator); !ok {
		entity, err := dto.ToEntity()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		props := &units2.CreatePageProps{
			Errors: errorsMap,
			Unit:   mappers.UnitToViewModel(entity),
		}
		templ.Handler(units2.CreateForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	if _, err := c.unitService.Create(r.Context(), &dto); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}
