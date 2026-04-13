// Package controllers provides this package.
package controllers

import (
	"net/http"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/tenant"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/phone"
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

type SettingsHubController struct {
	basePath string
}

func NewSettingsHubController() application.Controller {
	return &SettingsHubController{basePath: "/settings"}
}

func (c *SettingsHubController) Key() string {
	return c.basePath
}

func (c *SettingsHubController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.ProvideDynamicLogo(),
		middleware.NavItems(),
		middleware.WithPageContext(),
	)
	router.HandleFunc("", c.GetHub).Methods(http.MethodGet)
}

func (c *SettingsHubController) GetHub(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, c.basePath+"/logo", http.StatusFound)
}

type SettingsLogoController struct {
	tenantService *services.TenantService
	uploadService *services.UploadService
	basePath      string
}

func NewSettingsLogoController(
	tenantService *services.TenantService,
	uploadService *services.UploadService,
) application.Controller {
	return &SettingsLogoController{
		tenantService: tenantService,
		uploadService: uploadService,
		basePath:      "/settings/logo",
	}
}

func (c *SettingsLogoController) Key() string {
	return c.basePath
}

func (c *SettingsLogoController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.ProvideDynamicLogo(),
		middleware.NavItems(),
		middleware.WithPageContext(),
	)
	router.HandleFunc("", c.GetLogo).Methods(http.MethodGet)
	router.HandleFunc("", c.PostLogo).Methods(http.MethodPost)
}

func (c *SettingsLogoController) GetLogo(w http.ResponseWriter, r *http.Request) {
	props, err := c.logoProps(r, nil, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	templ.Handler(settings.Logo(props)).ServeHTTP(w, r)
}

func (c *SettingsLogoController) PostLogo(w http.ResponseWriter, r *http.Request) {
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
		tenant = tenant.SetLogoID(&logoID)
	}
	if dto.LogoCompactID > 0 {
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
		tenant = tenant.SetLogoCompactID(&logoCompactID)
	}

	if dto.Phone != "" {
		parsedPhone, err := phone.NewFromE164(dto.Phone)
		if err != nil {
			logger.WithError(err).Error("invalid phone number")
			props, propErr := c.logoProps(r, map[string]string{"Phone": "Invalid phone number format"}, tenant)
			if propErr != nil {
				logger.WithError(propErr).Error("failed to get logo props")
				http.Error(w, propErr.Error(), http.StatusInternalServerError)
				return
			}
			templ.Handler(settings.LogoForm(props)).ServeHTTP(w, r)
			return
		}
		tenant = tenant.SetPhone(parsedPhone)
	} else {
		tenant = tenant.SetPhone(nil)
	}

	if dto.Email != "" {
		parsedEmail, err := internet.NewEmail(dto.Email)
		if err != nil {
			logger.WithError(err).Error("invalid email")
			props, propErr := c.logoProps(r, map[string]string{"Email": "Invalid email format"}, tenant)
			if propErr != nil {
				logger.WithError(propErr).Error("failed to get logo props")
				http.Error(w, propErr.Error(), http.StatusInternalServerError)
				return
			}
			templ.Handler(settings.LogoForm(props)).ServeHTTP(w, r)
			return
		}
		tenant = tenant.SetEmail(parsedEmail)
	} else {
		tenant = tenant.SetEmail(nil)
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

func (c *SettingsLogoController) logoProps(r *http.Request, errors map[string]string, tenant tenant.Tenant) (*settings.LogoPageProps, error) {
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
		if err == nil && c.uploadService.IsAccessible(r.Context(), upload) {
			logoUpload = mappers.UploadToViewModel(upload)
		}
	}

	if tenant.LogoCompactID() != nil {
		upload, err := c.uploadService.GetByID(r.Context(), uint(*tenant.LogoCompactID()))
		if err == nil && c.uploadService.IsAccessible(r.Context(), upload) {
			logoCompactUpload = mappers.UploadToViewModel(upload)
		}
	}

	phoneValue := ""
	if tenant.Phone() != nil {
		phoneValue = tenant.Phone().E164()
	}

	emailValue := ""
	if tenant.Email() != nil {
		emailValue = tenant.Email().Value()
	}

	props := &settings.LogoPageProps{
		PostPath:          c.basePath,
		LogoUpload:        logoUpload,
		LogoCompactUpload: logoCompactUpload,
		Phone:             phoneValue,
		Email:             emailValue,
		Errors:            nonNilErrors,
	}

	return props, nil
}
