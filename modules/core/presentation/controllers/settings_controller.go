package controllers

import (
	"net/http"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/tenant"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/core/services"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/mappers"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/settings"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

type SettingsController struct {
	app           application.Application
	tenantService *services.TenantService
	uploadService *services.UploadService
	basePath      string
}

func NewSettingsController(app application.Application) application.Controller {
	return &SettingsController{
		app:           app,
		tenantService: app.Service(services.TenantService{}).(*services.TenantService),
		uploadService: app.Service(services.UploadService{}).(*services.UploadService),
		basePath:      "/settings",
	}
}

func (c *SettingsController) Key() string {
	return c.basePath
}

func (c *SettingsController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.ProvideDynamicLogo(c.app),
		middleware.Tabs(),
		middleware.ProvideLocalizer(c.app.Bundle()),
		middleware.NavItems(),
		middleware.WithPageContext(),
	)
	router.HandleFunc("/logo", c.GetLogo).Methods(http.MethodGet)
	router.HandleFunc("/logo", c.PostLogo).Methods(http.MethodPost)
}

func (c *SettingsController) GetLogo(w http.ResponseWriter, r *http.Request) {
	props, err := c.logoProps(r, nil, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	templ.Handler(settings.Logo(props)).ServeHTTP(w, r)
}

func (c *SettingsController) PostLogo(w http.ResponseWriter, r *http.Request) {
	logger := composables.UseLogger(r.Context())

	dto, err := composables.UseForm(&dtos.SaveLogosDTO{}, r)
	if err != nil {
		logger.WithError(err).Error("failed to parse form")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if errors, ok := dto.Ok(r.Context()); !ok {
		props, err := c.logoProps(r, errors, nil)
		if err != nil {
			logger.WithError(err).Error("failed to get logo props")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		templ.Handler(settings.LogoForm(props)).ServeHTTP(w, r)
		return
	}

	u, err := composables.UseUser(r.Context())
	if err != nil {
		logger.WithError(err).Error("failed to get user from context")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tenant, err := c.tenantService.GetByID(r.Context(), u.TenantID())
	if err != nil {
		logger.WithError(err).Error("failed to get tenant")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if dto.LogoID > 0 {
		// Validate that the upload exists before setting it
		exists, err := c.uploadService.Exists(r.Context(), uint(dto.LogoID))
		if err != nil {
			logger.WithError(err).Error("failed to check logo upload existence")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !exists {
			logger.Error("logo upload not found")
			http.Error(w, "Logo upload not found", http.StatusBadRequest)
			return
		}
		logoID := dto.LogoID
		tenant.SetLogoID(&logoID)
	}
	if dto.LogoCompactID > 0 {
		// Validate that the upload exists before setting it
		exists, err := c.uploadService.Exists(r.Context(), uint(dto.LogoCompactID))
		if err != nil {
			logger.WithError(err).Error("failed to check compact logo upload existence")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !exists {
			logger.Error("compact logo upload not found")
			http.Error(w, "Compact logo upload not found", http.StatusBadRequest)
			return
		}
		logoCompactID := dto.LogoCompactID
		tenant.SetLogoCompactID(&logoCompactID)
	}

	if _, err := c.tenantService.Update(r.Context(), tenant); err != nil {
		logger.WithError(err).Error("failed to update tenant")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	props, err := c.logoProps(r, nil, tenant)
	if err != nil {
		logger.WithError(err).Error("failed to get logo props")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	templ.Handler(settings.LogoForm(props)).ServeHTTP(w, r)
}

func (c *SettingsController) logoProps(r *http.Request, errors map[string]string, tenant *tenant.Tenant) (*settings.LogoPageProps, error) {
	nonNilErrors := make(map[string]string)
	if errors != nil {
		nonNilErrors = errors
	}

	if tenant == nil {
		u, err := composables.UseUser(r.Context())
		if err != nil {
			return nil, err
		}

		tenant, err = c.tenantService.GetByID(r.Context(), u.TenantID())
		if err != nil {
			return nil, err
		}
	}

	var logoUpload *viewmodels.Upload
	var logoCompactUpload *viewmodels.Upload

	if tenant.LogoID() != nil {
		upload, err := c.uploadService.GetByID(r.Context(), uint(*tenant.LogoID()))
		if err == nil {
			logoUpload = mappers.UploadToViewModel(upload)
		}
	}

	if tenant.LogoCompactID() != nil {
		upload, err := c.uploadService.GetByID(r.Context(), uint(*tenant.LogoCompactID()))
		if err == nil {
			logoCompactUpload = mappers.UploadToViewModel(upload)
		}
	}

	props := &settings.LogoPageProps{
		PostPath:          c.basePath + "/logo",
		LogoUpload:        logoUpload,
		LogoCompactUpload: logoCompactUpload,
		Errors:            nonNilErrors,
	}

	return props, nil
}
