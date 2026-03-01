// Package controllers provides this package.
package controllers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/health"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

type SystemHealthControllerOptions struct {
	BasePath  string
	CanAccess func(ctx context.Context) error
	Service   health.DetailedHealthService
}

type SystemHealthController struct {
	options *SystemHealthControllerOptions
}

func NewSystemHealthController(_ application.Application, options *SystemHealthControllerOptions) application.Controller {
	if options == nil {
		options = &SystemHealthControllerOptions{}
	}
	if options.BasePath == "" {
		options.BasePath = "/system/health"
	}

	return &SystemHealthController{
		options: options,
	}
}

func (c *SystemHealthController) Key() string {
	return "system-health"
}

func (c *SystemHealthController) Register(r *mux.Router) {
	subRouter := r.PathPrefix(c.options.BasePath).Subrouter()
	subRouter.Use(
		middleware.Authorize(),
		middleware.RequireAuthorization(),
		middleware.ProvideUser(),
	)

	subRouter.HandleFunc("/", c.Get).Methods(http.MethodGet)
}

func (c *SystemHealthController) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := composables.UseLogger(ctx)

	if _, err := composables.UseUser(ctx); err != nil {
		writeSystemHealthError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if c.options.CanAccess != nil {
		if err := c.options.CanAccess(ctx); err != nil {
			writeSystemHealthError(w, http.StatusForbidden, "forbidden")
			return
		}
	}

	if c.options.Service == nil {
		writeSystemHealthError(w, http.StatusInternalServerError, "health service not configured")
		return
	}

	response := c.options.Service.GetDetailedHealth(ctx)
	if response == nil {
		logger.Error("system health service returned nil response")
		writeSystemHealthError(w, http.StatusInternalServerError, "health response unavailable")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	statusCode := http.StatusOK
	if response.Status == health.StatusDown {
		statusCode = http.StatusServiceUnavailable
	}
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.WithError(err).Error("failed to encode system health response")
		http.Error(w, "failed to encode health response", http.StatusInternalServerError)
	}
}

func writeSystemHealthError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": message}); err != nil {
		return
	}
}
