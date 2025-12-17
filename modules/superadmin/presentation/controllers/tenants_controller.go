package controllers

import (
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/google/uuid"
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
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/sirupsen/logrus"

	"github.com/iota-uz/iota-sdk/components/scaffold/table"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/exportconfig"
	"github.com/iota-uz/iota-sdk/modules/superadmin/domain"

	superadminMiddleware "github.com/iota-uz/iota-sdk/modules/superadmin/middleware"
)

const (
	minPasswordLength = 8
	maxPasswordLength = 128
	userNotFoundMsg   = "User not found"
)

type TenantsController struct {
	app         application.Application
	userService *coreservices.UserService
	basePath    string
}

func NewTenantsController(app application.Application, userService *coreservices.UserService) application.Controller {
	return &TenantsController{
		app:         app,
		userService: userService,
		basePath:    "/superadmin/tenants",
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
		superadminMiddleware.RequireSuperAdmin(),
		middleware.ProvideDynamicLogo(c.app),
		middleware.ProvideLocalizer(c.app),
		middleware.NavItems(),
		middleware.WithPageContext(),
	)
	router.HandleFunc("", di.H(c.Index)).Methods(http.MethodGet)
	router.HandleFunc("/export", di.H(c.Export)).Methods(http.MethodPost)
	router.HandleFunc("/{id}/users", di.H(c.TenantUsers)).Methods(http.MethodGet)
	router.HandleFunc("/{id}/users/{userId}/reset-password", di.H(c.ResetUserPassword)).Methods(http.MethodPost)
}

// Index renders the tenants table page and handles HTMX filtering requests
func (c *TenantsController) Index(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	tenantService *services.TenantService,
) {
	ctx := r.Context()

	// Get pagination parameters
	params := composables.UsePaginated(r)

	// Handle sorting parameters
	sortField := table.UseSortQuery(r)
	sortOrder := table.UseOrderQuery(r)

	// Convert to repo.SortBy format
	var sortBy domain.TenantSortBy
	if sortField != "" {
		sortBy = domain.TenantSortBy{
			Fields: []repo.SortByField[string]{
				{Field: sortField, Ascending: sortOrder == "asc"},
			},
		}
	}
	// If empty, services will use default DESC sort

	// Get search parameter
	search := table.UseSearchQuery(r)

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
		tenantsList, total, err = tenantService.FilterByDateRange(ctx, startDate, endDate, params.Limit, params.Offset, sortBy)
		if err != nil {
			logger.Errorf("Error retrieving tenants by date range: %v", err)
			http.Error(w, "Error retrieving tenants", http.StatusInternalServerError)
			return
		}
	} else {
		// Fetch tenants without date filter (existing behavior)
		tenantsList, total, err = tenantService.FindTenants(ctx, params.Limit, params.Offset, search, sortBy)
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
		hxTarget := r.Header.Get("Hx-Target")
		if hxTarget == "sortable-table-container" {
			// Sorting request - return full table to update headers with new sort direction
			templ.Handler(tenants.Table(props), templ.WithStreaming()).ServeHTTP(w, r)
		} else {
			// Filter/search request - return only rows to update table body
			templ.Handler(tenants.TableRows(props.Tenants), templ.WithStreaming()).ServeHTTP(w, r)
		}
	} else {
		templ.Handler(tenants.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

// Export exports tenants to Excel
func (c *TenantsController) Export(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	tenantService *services.TenantService,
	excelService *coreservices.ExcelExportService,
) {
	ctx := r.Context()

	// Build SQL query for export
	query := `
		SELECT
			t.id::text,
			t.name,
			t.email,
			t.phone,
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

// TenantUsers renders the users list for a specific tenant
func (c *TenantsController) TenantUsers(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	tenantUsersService *services.TenantUsersService,
	tenantService *services.TenantService,
) {
	ctx := r.Context()

	// Parse tenant ID from URL path
	vars := mux.Vars(r)
	tenantIDStr := vars["id"]
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		logger.Errorf("Invalid tenant ID: %v", err)
		http.Error(w, "Invalid tenant ID", http.StatusBadRequest)
		return
	}

	// Get tenant info
	tenant, err := tenantService.GetByID(ctx, tenantID)
	if err != nil {
		logger.Errorf("Error retrieving tenant %s: %v", tenantID, err)
		http.Error(w, "Tenant not found", http.StatusNotFound)
		return
	}

	// Verify tenant exists (nil check)
	if tenant == nil {
		logger.Warnf("Tenant %s not found", tenantID)
		http.Error(w, "Tenant not found", http.StatusNotFound)
		return
	}

	// Get pagination parameters
	params := composables.UsePaginated(r)

	// Handle sorting parameters
	sortField := table.UseSortQuery(r)
	sortOrder := table.UseOrderQuery(r)

	// Convert to user.SortBy format
	var sortBy user.SortBy
	if sortField != "" {
		// Map sortField to user.Field constants
		var field user.Field
		switch sortField {
		case "first_name":
			field = user.FirstNameField
		case "last_name":
			field = user.LastNameField
		case "email":
			field = user.EmailField
		case "phone":
			field = user.PhoneField
		case "created_at":
			field = user.CreatedAtField
		default:
			field = user.CreatedAtField // Default sort field
		}

		sortBy = user.SortBy{
			Fields: []repo.SortByField[user.Field]{
				{Field: field, Ascending: sortOrder == "asc"},
			},
		}
	}

	// Get search parameter
	search := table.UseSearchQuery(r)

	// Get users for tenant
	users, total, err := tenantUsersService.GetUsersByTenantID(ctx, tenantID, params.Limit, params.Offset, search, sortBy)
	if err != nil {
		logger.Errorf("Error retrieving users for tenant %s: %v", tenantID, err)
		http.Error(w, "Error retrieving users", http.StatusInternalServerError)
		return
	}

	// Convert users to template format
	tenantUsers := make([]*tenants.TenantUser, len(users))
	for i, u := range users {
		roleName := ""
		if len(u.Roles()) > 0 {
			roleName = u.Roles()[0].Name()
		}

		phone := ""
		if u.Phone() != nil {
			phone = u.Phone().Value()
		}

		tenantUsers[i] = &tenants.TenantUser{
			ID:        u.ID(),
			FirstName: u.FirstName(),
			LastName:  u.LastName(),
			Email:     u.Email().Value(),
			Phone:     phone,
			RoleName:  roleName,
			LastLogin: u.LastLogin(),
			CreatedAt: u.CreatedAt(),
		}
	}

	// Create props for template
	props := &tenants.UsersPageProps{
		Tenant: tenant,
		Users:  tenantUsers,
		Total:  total,
	}

	// Render template
	if htmx.IsHxRequest(r) {
		hxTarget := r.Header.Get("Hx-Target")
		if hxTarget == "sortable-table-container" {
			// Sorting request - return full table to update headers with new sort direction
			templ.Handler(tenants.UsersTable(props), templ.WithStreaming()).ServeHTTP(w, r)
		} else {
			// Filter/search request - return only rows to update table body
			templ.Handler(tenants.UsersTableRows(props.Users), templ.WithStreaming()).ServeHTTP(w, r)
		}
	} else {
		templ.Handler(tenants.Users(props), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

// ResetUserPassword resets a user's password for a specific tenant
// POST /superadmin/tenants/{id}/users/{userId}/reset-password
func (c *TenantsController) ResetUserPassword(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
) {
	ctx := r.Context()

	// Get current admin user for audit logging
	currentAdmin, err := composables.UseUser(ctx)
	if err != nil {
		logger.Errorf("Failed to get current user: %v", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)

	// Parse tenant ID from URL
	tenantIDStr := vars["id"]
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		logger.Errorf("Invalid tenant ID: %v", err)
		http.Error(w, "Invalid tenant ID", http.StatusBadRequest)
		return
	}

	// Parse user ID from URL
	userIDStr := vars["userId"]
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		logger.Errorf("Invalid user ID: %v", err)
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Validate Content-Type header
	contentType := r.Header.Get("Content-Type")
	mediaType, _, parseErr := mime.ParseMediaType(contentType)
	if parseErr != nil || mediaType != "application/json" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnsupportedMediaType)
		if encodeErr := json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    "invalid_content_type",
			"message": "Content-Type must be application/json",
		}); encodeErr != nil {
			logrus.Errorf("Error encoding JSON response: %v", encodeErr)
		}
		return
	}

	// Parse JSON request body
	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Errorf("Error parsing request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Trim whitespace from password
	password := strings.TrimSpace(req.Password)

	// Validate password is not empty
	if password == "" {
		http.Error(w, "Password cannot be empty", http.StatusBadRequest)
		return
	}

	// Validate minimum length
	if len(password) < minPasswordLength {
		http.Error(w, fmt.Sprintf("Password must be at least %d characters", minPasswordLength), http.StatusBadRequest)
		return
	}

	// Validate maximum length (prevent DoS)
	if len(password) > maxPasswordLength {
		http.Error(w, fmt.Sprintf("Password cannot exceed %d characters", maxPasswordLength), http.StatusBadRequest)
		return
	}

	// Fetch user (bypassing normal permission checks for cross-tenant access)
	existingUser, err := c.userService.GetByID(ctx, uint(userID))
	if err != nil {
		logger.Errorf("Error fetching user %d: %v", userID, err)
		http.Error(w, userNotFoundMsg, http.StatusNotFound)
		return
	}

	// Validate user belongs to the specified tenant (cross-tenant protection)
	if existingUser.TenantID() != tenantID {
		logger.Warnf("Cross-tenant access attempt: user %d requested for tenant %s, actual tenant %s",
			userID, tenantID, existingUser.TenantID())
		http.Error(w, userNotFoundMsg, http.StatusNotFound)
		return
	}

	// Set new password (uses bcrypt internally)
	updatedUser, err := existingUser.SetPassword(password)
	if err != nil {
		logger.Errorf("Error hashing password: %v", err)
		http.Error(w, "Error updating password", http.StatusInternalServerError)
		return
	}

	// Update user in database
	_, err = c.userService.Update(ctx, updatedUser)
	if err != nil {
		logger.Errorf("Error updating user %d: %v", userID, err)
		http.Error(w, "Error updating password", http.StatusInternalServerError)
		return
	}

	// Audit log
	logger.WithFields(logrus.Fields{
		"admin_id":    currentAdmin.ID(),
		"admin_email": currentAdmin.Email().Value(),
		"tenant_id":   tenantID.String(),
		"user_id":     userID,
		"user_email":  existingUser.Email().Value(),
		"action":      "password_reset",
		"ip_address":  r.RemoteAddr,
	}).Info("SuperAdmin password reset successful")

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Password reset successfully",
	}); err != nil {
		logrus.Errorf("Error encoding JSON response: %v", err)
	}
}
