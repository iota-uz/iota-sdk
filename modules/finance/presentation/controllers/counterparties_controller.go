package controllers

import (
	"errors"
	"net/http"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/components/base"
	"github.com/iota-uz/iota-sdk/components/base/pagination"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/mappers"
	counterpartiesui "github.com/iota-uz/iota-sdk/modules/finance/presentation/templates/pages/counterparties"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/iota-uz/iota-sdk/pkg/shared"
	"github.com/sirupsen/logrus"

	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

type CounterpartiesController struct {
	app                   application.Application
	counterpartiesService *services.CounterpartyService
	basePath              string
}

func NewCounterpartiesController(app application.Application) application.Controller {
	return &CounterpartiesController{
		app:                   app,
		counterpartiesService: app.Service(services.CounterpartyService{}).(*services.CounterpartyService),
		basePath:              "/finance/counterparties",
	}
}

func (c *CounterpartiesController) Key() string {
	return c.basePath
}

func (c *CounterpartiesController) Register(r *mux.Router) {
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
	router.HandleFunc("/search", c.Search).Methods(http.MethodGet)
	router.HandleFunc("", c.Create).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}", c.Update).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}", c.Delete).Methods(http.MethodDelete)
}

func (c *CounterpartiesController) List(w http.ResponseWriter, r *http.Request) {
	params := composables.UsePaginated(r)
	findParams := &counterparty.FindParams{
		Offset: params.Offset,
		Limit:  params.Limit,
		SortBy: counterparty.SortBy{
			Fields: []repo.SortByField[counterparty.Field]{
				{Field: counterparty.CreatedAtField, Ascending: false},
			},
		},
		Search: r.URL.Query().Get("Search"),
	}

	entities, err := c.counterpartiesService.GetPaginated(r.Context(), findParams)
	if err != nil {
		logrus.WithError(err).Error("Error retrieving counterparties")
		http.Error(w, "Error retrieving counterparties", http.StatusInternalServerError)
		return
	}

	total, err := c.counterpartiesService.Count(r.Context(), findParams)
	if err != nil {
		logrus.WithError(err).Error("Error counting counterparties")
		http.Error(w, "Error counting counterparties", http.StatusInternalServerError)
		return
	}

	props := &counterpartiesui.IndexPageProps{
		Counterparties:  mapping.MapViewModels(entities, mappers.CounterpartyToViewModel),
		PaginationState: pagination.New(c.basePath, params.Page, int(total), params.Limit),
	}

	if htmx.IsHxRequest(r) {
		templ.Handler(counterpartiesui.CounterpartiesTable(props), templ.WithStreaming()).ServeHTTP(w, r)
	} else {
		templ.Handler(counterpartiesui.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *CounterpartiesController) GetNew(w http.ResponseWriter, r *http.Request) {
	props := &counterpartiesui.CreatePageProps{
		Counterparty: &viewmodels.Counterparty{},
		Errors:       map[string]string{},
		PostPath:     c.basePath,
	}
	templ.Handler(counterpartiesui.New(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *CounterpartiesController) Create(w http.ResponseWriter, r *http.Request) {
	dto, err := composables.UseForm(&dtos.CounterpartyCreateDTO{}, r)
	if err != nil {
		logrus.WithError(err).Error("Error parsing form")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if errorsMap, ok := dto.Ok(r.Context()); !ok {
		tenantID, err := composables.UseTenantID(r.Context())
		if err != nil {
			logrus.WithError(err).Error("Error getting tenant ID")
			http.Error(w, "Error getting tenant ID", http.StatusInternalServerError)
			return
		}

		entity, err := dto.ToEntity(tenantID)
		if err != nil {
			logrus.WithError(err).Error("Error converting DTO to entity")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		props := &counterpartiesui.CreatePageProps{
			Counterparty: mappers.CounterpartyToViewModel(entity),
			Errors:       errorsMap,
			PostPath:     c.basePath,
		}
		templ.Handler(counterpartiesui.CreateForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	tenantID, err := composables.UseTenantID(r.Context())
	if err != nil {
		logrus.WithError(err).Error("Error getting tenant ID")
		http.Error(w, "Error getting tenant ID", http.StatusInternalServerError)
		return
	}

	entity, err := dto.ToEntity(tenantID)
	if err != nil {
		logrus.WithError(err).Error("Error converting DTO to entity")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err := c.counterpartiesService.Create(r.Context(), entity); err != nil {
		logrus.WithError(err).Error("Error creating counterparty")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}

func (c *CounterpartiesController) GetEdit(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseUUID(r)
	if err != nil {
		logrus.WithError(err).Error("Error parsing counterparty ID")
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}

	entity, err := c.counterpartiesService.GetByID(r.Context(), id)
	if err != nil {
		logrus.WithError(err).Error("Error retrieving counterparty")
		http.Error(w, "Error retrieving counterparty", http.StatusInternalServerError)
		return
	}

	props := &counterpartiesui.EditPageProps{
		Counterparty: mappers.CounterpartyToViewModel(entity),
		Errors:       map[string]string{},
		PostPath:     c.basePath + "/" + id.String(),
		DeletePath:   c.basePath + "/" + id.String(),
	}
	templ.Handler(counterpartiesui.Edit(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *CounterpartiesController) Update(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseUUID(r)
	if err != nil {
		logrus.WithError(err).Error("Error parsing counterparty ID")
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}

	dto, err := composables.UseForm(&dtos.CounterpartyUpdateDTO{}, r)
	if err != nil {
		logrus.WithError(err).Error("Error parsing form")
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	existing, err := c.counterpartiesService.GetByID(r.Context(), id)
	if errors.Is(err, persistence.ErrCounterpartyNotFound) {
		logrus.WithError(err).Error("Counterparty not found")
		http.Error(w, "Counterparty not found", http.StatusNotFound)
		return
	}
	if err != nil {
		logrus.WithError(err).Error("Error retrieving counterparty")
		http.Error(w, "Error retrieving counterparty", http.StatusInternalServerError)
		return
	}

	if errorsMap, ok := dto.Ok(r.Context()); !ok {
		props := &counterpartiesui.EditPageProps{
			Counterparty: mappers.CounterpartyToViewModel(existing),
			Errors:       errorsMap,
			PostPath:     c.basePath + "/" + id.String(),
			DeletePath:   c.basePath + "/" + id.String(),
		}
		templ.Handler(counterpartiesui.EditForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	entity, err := dto.Apply(existing)
	if err != nil {
		logrus.WithError(err).Error("Error applying DTO to entity")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err := c.counterpartiesService.Update(r.Context(), entity); err != nil {
		logrus.WithError(err).Error("Error updating counterparty")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}

func (c *CounterpartiesController) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseUUID(r)
	if err != nil {
		logrus.WithError(err).Error("Error parsing counterparty ID")
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}

	if _, err := c.counterpartiesService.Delete(r.Context(), id); err != nil {
		logrus.WithError(err).Error("Error deleting counterparty")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	shared.Redirect(w, r, c.basePath)
}

func (c *CounterpartiesController) Search(w http.ResponseWriter, r *http.Request) {
	search := r.URL.Query().Get("q")
	entities, err := c.counterpartiesService.GetPaginated(r.Context(), &counterparty.FindParams{
		Search: search,
		Limit:  10,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := mapping.MapViewModels(entities, func(e counterparty.Counterparty) *base.ComboboxOption {
		return &base.ComboboxOption{
			Value: e.ID().String(),
			Label: e.Name(),
		}
	})
	templ.Handler(base.ComboboxOptions(props), templ.WithStreaming()).ServeHTTP(w, r)
}
