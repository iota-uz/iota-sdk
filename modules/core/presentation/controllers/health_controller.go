package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

func NewHealthController(app application.Application) application.Controller {
	return &HealthController{
		app: app,
	}
}

type HealthController struct {
	app application.Application
}

func (c *HealthController) Key() string {
	return "/health"
}

func (c *HealthController) Register(r *mux.Router) {
	router := r.Methods(http.MethodGet).Subrouter()
	router.HandleFunc("/health", c.Get)
}

func (c *HealthController) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := composables.UseLogger(ctx)

	w.Header().Set("Content-Type", "application/json")
	status := "healthy"
	httpStatus := http.StatusOK

	if err := c.quickDBCheck(ctx); err != nil {
		status = "unhealthy"
		httpStatus = http.StatusServiceUnavailable
		logger.Warnf("Health check failed: %v", err)
	}

	spotlightCtx, spotlightCancel := context.WithTimeout(ctx, 2*time.Second)
	defer spotlightCancel()
	if err := c.app.Spotlight().Readiness(spotlightCtx); err != nil {
		status = "unhealthy"
		httpStatus = http.StatusServiceUnavailable
		logger.Warnf("Spotlight health check failed: %v", err)
	}

	w.WriteHeader(httpStatus)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": status}); err != nil {
		logger.Errorf("Failed to write health response: %v", err)
	}
}

func (c *HealthController) quickDBCheck(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	db := c.app.DB()
	if db == nil {
		return fmt.Errorf("database connection pool not available")
	}

	var result int
	return db.QueryRow(timeoutCtx, "SELECT 1").Scan(&result)
}
