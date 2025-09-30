package controllers

import (
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/modules/superadmin/domain/entities"
	"github.com/iota-uz/iota-sdk/modules/superadmin/presentation/templates/pages/tenants"
	"github.com/iota-uz/iota-sdk/modules/superadmin/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/di"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/sirupsen/logrus"

	"github.com/iota-uz/iota-sdk/components/scaffold/table"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/exportconfig"
)

type TenantsController struct {
	app      application.Application
	basePath string
}

func NewTenantsController(app application.Application) application.Controller {
	return &TenantsController{
		app:      app,
		basePath: "/superadmin/tenants",
	}
}

func (c *TenantsController) Key() string {
	return c.basePath
}

func (c *TenantsController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.ProvideDynamicLogo(c.app),
		middleware.ProvideLocalizer(c.app.Bundle()),
		middleware.NavItems(),
		middleware.WithPageContext(),
	)
	router.HandleFunc("", di.H(c.Index)).Methods(http.MethodGet)
	router.HandleFunc("/export", di.H(c.Export)).Methods(http.MethodPost)
}

// Index renders the tenants table page and handles HTMX filtering requests
func (c *TenantsController) Index(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	tenantQueryService *services.TenantQueryService,
	tenantService *services.TenantService,
) {
	ctx := r.Context()

	// Get pagination parameters
	params := composables.UsePaginated(r)

	// Handle sorting parameters
	sortField := table.UseSortQuery(r)
	sortOrder := table.UseOrderQuery(r)

	// Get search parameter
	search := r.URL.Query().Get("search")

	// Parse optional date range parameters
	startDateStr := r.URL.Query().Get("start_date")
	endDateStr := r.URL.Query().Get("end_date")

	var tenantsList []*entities.TenantInfo
	var total int
	var err error

	// Use date range filtering if dates are provided
	if startDateStr != "" || endDateStr != "" {
		var startDate, endDate time.Time

		if startDateStr != "" {
			startDate, err = time.Parse(time.RFC3339, startDateStr)
			if err != nil {
				logger.Errorf("Error parsing start_date: %v", err)
				http.Error(w, "Invalid start_date format. Use RFC3339 format.", http.StatusBadRequest)
				return
			}
		}

		if endDateStr != "" {
			endDate, err = time.Parse(time.RFC3339, endDateStr)
			if err != nil {
				logger.Errorf("Error parsing end_date: %v", err)
				http.Error(w, "Invalid end_date format. Use RFC3339 format.", http.StatusBadRequest)
				return
			}
		}

		// Fetch tenants with date range filter
		tenantsList, total, err = tenantService.FilterByDateRange(ctx, startDate, endDate, params.Limit, params.Offset, sortField, sortOrder)
		if err != nil {
			logger.Errorf("Error retrieving tenants by date range: %v", err)
			http.Error(w, "Error retrieving tenants", http.StatusInternalServerError)
			return
		}
	} else {
		// Fetch tenants without date filter (existing behavior)
		tenantsList, total, err = tenantQueryService.FindTenants(ctx, params.Limit, params.Offset, search, sortField, sortOrder)
		if err != nil {
			logger.Errorf("Error retrieving tenants: %v", err)
			http.Error(w, "Error retrieving tenants", http.StatusInternalServerError)
			return
		}
	}

	// Create props for template
	props := &tenants.IndexPageProps{
		Tenants:   tenantsList,
		Total:     total,
		StartDate: startDateStr,
		EndDate:   endDateStr,
	}

	// Check if HTMX request
	if htmx.IsHxRequest(r) {
		templ.Handler(tenants.TableRows(props.Tenants), templ.WithStreaming()).ServeHTTP(w, r)
	} else {
		templ.Handler(tenants.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

// Export exports tenants to Excel
func (c *TenantsController) Export(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	tenantService *services.TenantQueryService,
	excelService *coreservices.ExcelExportService,
) {
	ctx := r.Context()

	// Build SQL query for export
	query := `
		SELECT
			t.id,
			t.name,
			t.domain,
			COALESCE(u.user_count, 0) as user_count,
			t.created_at,
			t.updated_at
		FROM tenants t
		LEFT JOIN (
			SELECT tenant_id, COUNT(*) as user_count
			FROM users
			GROUP BY tenant_id
		) u ON t.id = u.tenant_id
		ORDER BY t.created_at DESC`

	// Create export config
	queryObj := exportconfig.NewQuery(query)
	config := exportconfig.New(exportconfig.WithFilename("tenants_export"))

	// Export to Excel
	upload, err := excelService.ExportFromQuery(ctx, queryObj, config)
	if err != nil {
		logger.Errorf("Error exporting tenants: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Redirect to download URL
	if htmx.IsHxRequest(r) {
		htmx.Redirect(w, upload.URL().String())
	} else {
		http.Redirect(w, r, upload.URL().String(), http.StatusSeeOther)
	}
}
