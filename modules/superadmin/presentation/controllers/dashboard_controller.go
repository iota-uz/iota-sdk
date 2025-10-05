package controllers

import (
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/superadmin/presentation/templates/pages/dashboard"
	"github.com/iota-uz/iota-sdk/modules/superadmin/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/di"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/sirupsen/logrus"

	superadminMiddleware "github.com/iota-uz/iota-sdk/modules/superadmin/middleware"
)

type DashboardController struct {
	app      application.Application
	basePath string
}

func NewDashboardController(app application.Application) application.Controller {
	return &DashboardController{
		app:      app,
		basePath: "/",
	}
}

func (c *DashboardController) Key() string {
	return c.basePath
}

func (c *DashboardController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		superadminMiddleware.RequireSuperAdmin(),
		middleware.ProvideDynamicLogo(c.app),
		middleware.ProvideLocalizer(c.app.Bundle()),
		middleware.NavItems(),
		middleware.WithPageContext(),
	)
	router.HandleFunc("/", c.Index).Methods(http.MethodGet)
	router.HandleFunc("/metrics", di.H(c.GetMetrics)).Methods(http.MethodGet)
}

// Index renders the dashboard page
func (c *DashboardController) Index(w http.ResponseWriter, r *http.Request) {
	// Render dashboard with initial empty metrics (HTMX will load them)
	props := &dashboard.IndexPageProps{
		Metrics: nil, // Will be loaded by HTMX on page load
	}
	templ.Handler(dashboard.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
}

// GetMetrics returns metrics data for HTMX endpoint with optional date filtering
func (c *DashboardController) GetMetrics(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	analyticsService *services.AnalyticsService,
) {
	ctx := r.Context()

	// Parse optional date query parameters
	var startDate, endDate time.Time
	var err error

	if startDateStr := r.URL.Query().Get("startDate"); startDateStr != "" {
		startDate, err = time.Parse("2006-01-02", startDateStr)
		if err != nil {
			logger.Errorf("Error parsing startDate: %v", err)
			http.Error(w, "Invalid startDate format. Use YYYY-MM-DD format.", http.StatusBadRequest)
			return
		}
	}

	if endDateStr := r.URL.Query().Get("endDate"); endDateStr != "" {
		endDate, err = time.Parse("2006-01-02", endDateStr)
		if err != nil {
			logger.Errorf("Error parsing endDate: %v", err)
			http.Error(w, "Invalid endDate format. Use YYYY-MM-DD format.", http.StatusBadRequest)
			return
		}
	}

	// Get metrics from service
	metrics, err := analyticsService.GetDashboardMetrics(ctx, startDate, endDate)
	if err != nil {
		logger.Errorf("Error retrieving dashboard metrics: %v", err)
		http.Error(w, "Error retrieving dashboard metrics", http.StatusInternalServerError)
		return
	}

	// Create props for template
	props := &dashboard.MetricsProps{
		TenantCount:             metrics.TenantCount,
		UserCount:               metrics.UserCount,
		DAU:                     metrics.DAU,
		WAU:                     metrics.WAU,
		MAU:                     metrics.MAU,
		SessionCount:            metrics.SessionCount,
		UserSignupsTimeSeries:   metrics.UserSignupsTimeSeries,
		TenantSignupsTimeSeries: metrics.TenantSignupsTimeSeries,
	}

	// Render metrics template fragment
	templ.Handler(dashboard.MetricsContent(props), templ.WithStreaming()).ServeHTTP(w, r)
}
