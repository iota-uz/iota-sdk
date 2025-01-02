package controllers

import (
	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/components/base"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"net/http"
	"strconv"

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
	getRouter.HandleFunc("/search", c.Search).Methods(http.MethodGet)
}

func (c *CounterpartiesController) Search(w http.ResponseWriter, r *http.Request) {
	search := r.URL.Query().Get("q")
	entities, err := c.counterpartiesService.GetPaginated(r.Context(), &counterparty.FindParams{
		Query: search,
		Field: "name",
		Limit: 10,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := mapping.MapViewModels(entities, func(e counterparty.Counterparty) *base.ComboboxOption {
		return &base.ComboboxOption{
			Value: strconv.FormatUint(uint64(e.ID()), 10),
			Label: e.Name(),
		}
	})
	templ.Handler(base.ComboboxOptions(props), templ.WithStreaming()).ServeHTTP(w, r)
}
