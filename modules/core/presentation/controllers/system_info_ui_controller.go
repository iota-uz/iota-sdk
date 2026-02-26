package controllers

import (
	"context"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/system_info"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/di"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

type HealthUIControllerOptions struct {
	BasePath       string
	CanAccess      func(ctx context.Context) error
	BuildViewModel func(ctx context.Context, r *http.Request) (*viewmodels.SystemInfoViewModel, error)
}

type HealthUIController struct {
	app     application.Application
	options *HealthUIControllerOptions
}

func NewHealthUIController(app application.Application, options *HealthUIControllerOptions) application.Controller {
	if options == nil {
		options = &HealthUIControllerOptions{}
	}
	if options.BasePath == "" {
		options.BasePath = "/system/info"
	}

	return &HealthUIController{
		app:     app,
		options: options,
	}
}

func (c *HealthUIController) Key() string {
	return "health-ui"
}

func (c *HealthUIController) Register(r *mux.Router) {
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

	subRouter.HandleFunc("", di.H(c.Index)).Methods(http.MethodGet)
	subRouter.HandleFunc("/", di.H(c.Index)).Methods(http.MethodGet)
	subRouter.HandleFunc("/metrics", di.H(c.MetricsPartial)).Methods(http.MethodGet)
}

func (c *HealthUIController) Index(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := composables.UseLogger(ctx)

	if c.options.CanAccess != nil {
		if err := c.options.CanAccess(ctx); err != nil {
			RenderForbidden(w, r)
			return
		}
	}

	if c.options.BuildViewModel == nil {
		http.Error(w, "system info is temporarily unavailable", http.StatusInternalServerError)
		return
	}

	vm, err := c.options.BuildViewModel(ctx, r)
	if err != nil {
		logger.Errorf("Failed to build system info view model: %v", err)
		http.Error(w, "Unable to build system information", http.StatusInternalServerError)
		return
	}
	if vm == nil {
		logger.Error("System info view model is nil")
		http.Error(w, "Unable to build system information", http.StatusInternalServerError)
		return
	}

	templ.Handler(system_info.Index(vm, c.metricsEndpoint()), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *HealthUIController) MetricsPartial(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := composables.UseLogger(ctx)

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
		logger.Errorf("Failed to build system info view model: %v", err)
		http.Error(w, "Unable to build system information", http.StatusInternalServerError)
		return
	}
	if vm == nil {
		logger.Error("System info view model is nil")
		http.Error(w, "Unable to build system information", http.StatusInternalServerError)
		return
	}
	if !htmx.IsHxRequest(r) {
		http.Error(w, "Expected HTMX request", http.StatusBadRequest)
		return
	}

	templ.Handler(system_info.MetricsPartial(vm), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *HealthUIController) metricsEndpoint() string {
	basePath := strings.TrimRight(c.options.BasePath, "/")
	return basePath + "/metrics"
}
