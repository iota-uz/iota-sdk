// Package controllers provides this package.
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

// HealthUIController renders the system information UI endpoints.
type HealthUIController struct {
	app     application.Application
	options *HealthUIControllerOptions
}

// NewHealthUIController builds a system info controller from a dependency map.
// Required dependency: "app" (application.Application). Optional: "options" (*HealthUIControllerOptions).
func NewHealthUIController(deps map[string]any) application.Controller {
	rawApp, ok := deps["app"]
	if !ok {
		panic("health ui controller requires dependency \"app\" (application.Application)")
	}
	app, ok := rawApp.(application.Application)
	if !ok {
		panic("health ui controller dependency \"app\" has invalid type")
	}

	options, _ := deps["options"].(*HealthUIControllerOptions)
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

// Key identifies this controller for registration and lookups.
func (c *HealthUIController) Key() string {
	return "health-ui"
}

// Register wires system info and metrics routes with shared middleware.
func (c *HealthUIController) Register(r *mux.Router) {
	subRouter := r.PathPrefix(c.options.BasePath).Subrouter()
	subRouter.Use(
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.ProvideLocalizer(c.app),
		middleware.NavItems(),
		middleware.WithPageContext(),
		middleware.ProvideDynamicLogo(),
	)

	subRouter.HandleFunc("", di.H(c.Index)).Methods(http.MethodGet)
	subRouter.HandleFunc("/", di.H(c.Index)).Methods(http.MethodGet)
	subRouter.HandleFunc("/metrics", di.H(c.MetricsPartial)).Methods(http.MethodGet)
	subRouter.HandleFunc("/metrics/", di.H(c.MetricsPartial)).Methods(http.MethodGet)
}

// Index renders the full system info page.
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
	if vm.Metrics == nil {
		logger.Error("System info metrics are nil")
		http.Error(w, "Unable to build system information", http.StatusInternalServerError)
		return
	}

	templ.Handler(system_info.Index(vm, c.metricsEndpoint()), templ.WithStreaming()).ServeHTTP(w, r)
}

// MetricsPartial renders the system metrics partial and requires HTMX.
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

	if !htmx.IsHxRequest(r) {
		http.Error(w, "Expected HTMX request", http.StatusBadRequest)
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
	if vm.Metrics == nil {
		logger.Error("System info metrics are nil")
		http.Error(w, "Unable to build system information", http.StatusInternalServerError)
		return
	}

	templ.Handler(system_info.MetricsPartial(vm), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *HealthUIController) metricsEndpoint() string {
	basePath := strings.TrimRight(c.options.BasePath, "/")
	return basePath + "/metrics"
}
