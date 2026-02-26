package controllers

import (
	"context"
	"net/http"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/system_info"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

type SystemInfoUIControllerOptions struct {
	BasePath       string
	CanAccess      func(ctx context.Context) error
	BuildViewModel func(ctx context.Context, r *http.Request) (*viewmodels.SystemInfoViewModel, error)
}

type HealthUIControllerOptions = SystemInfoUIControllerOptions

type SystemInfoUIController struct {
	app     application.Application
	options *SystemInfoUIControllerOptions
}

func NewSystemInfoUIController(app application.Application, options *SystemInfoUIControllerOptions) application.Controller {
	if options == nil {
		options = &SystemInfoUIControllerOptions{}
	}
	if options.BasePath == "" {
		options.BasePath = "/system/info"
	}

	return &SystemInfoUIController{
		app:     app,
		options: options,
	}
}

func NewHealthUIController(app application.Application, options *HealthUIControllerOptions) application.Controller {
	return NewSystemInfoUIController(app, options)
}

func (c *SystemInfoUIController) Key() string {
	return "system-info-ui"
}

func (c *SystemInfoUIController) Register(r *mux.Router) {
	subRouter := r.PathPrefix(c.options.BasePath).Subrouter()
	subRouter.Use(
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.ProvideLocalizer(c.app),
		middleware.NavItems(),
		middleware.WithPageContext(),
		middleware.ProvideDynamicLogo(c.app),
	)

	subRouter.HandleFunc("", c.Index).Methods(http.MethodGet)
	subRouter.HandleFunc("/", c.Index).Methods(http.MethodGet)
	subRouter.HandleFunc("/metrics", c.MetricsPartial).Methods(http.MethodGet)
}

func (c *SystemInfoUIController) Index(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if c.options.CanAccess != nil {
		if err := c.options.CanAccess(ctx); err != nil {
			RenderForbidden(w, r)
			return
		}
	}

	if c.options.BuildViewModel == nil {
		http.Error(w, "system info view model builder is not configured", http.StatusInternalServerError)
		return
	}

	vm, err := c.options.BuildViewModel(ctx, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	templ.Handler(system_info.Index(vm), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *SystemInfoUIController) MetricsPartial(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if c.options.CanAccess != nil {
		if err := c.options.CanAccess(ctx); err != nil {
			RenderForbidden(w, r)
			return
		}
	}

	if c.options.BuildViewModel == nil {
		http.Error(w, "system info view model builder is not configured", http.StatusInternalServerError)
		return
	}

	vm, err := c.options.BuildViewModel(ctx, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	templ.Handler(system_info.MetricsPartial(vm), templ.WithStreaming()).ServeHTTP(w, r)
}
