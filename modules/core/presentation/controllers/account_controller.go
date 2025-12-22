package controllers

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"

	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/mappers"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/account"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

type AccountController struct {
	app           application.Application
	userService   *services.UserService
	tenantService *services.TenantService
	uploadService *services.UploadService
	basePath      string
}

func NewAccountController(app application.Application) application.Controller {
	return &AccountController{
		app:           app,
		userService:   app.Service(services.UserService{}).(*services.UserService),
		tenantService: app.Service(services.TenantService{}).(*services.TenantService),
		uploadService: app.Service(services.UploadService{}).(*services.UploadService),
		basePath:      "/account",
	}
}

func (c *AccountController) Key() string {
	return c.basePath
}

func (c *AccountController) Register(r *mux.Router) {
	commonMiddleware := []mux.MiddlewareFunc{
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.ProvideDynamicLogo(c.app),
		middleware.ProvideLocalizer(c.app),
		middleware.NavItems(),
		middleware.WithPageContext(),
	}
	getRouter := r.PathPrefix(c.basePath).Subrouter()
	getRouter.Use(commonMiddleware...)
	getRouter.HandleFunc("", c.Get).Methods(http.MethodGet)

	setRouter := r.PathPrefix(c.basePath).Subrouter()
	setRouter.Use(commonMiddleware...)
	setRouter.HandleFunc("", c.Update).Methods(http.MethodPost)
}

func (c *AccountController) defaultProps(r *http.Request, errors map[string]string) (*account.ProfilePageProps, error) {
	nonNilErrors := make(map[string]string)
	if errors != nil {
		nonNilErrors = errors
	}
	u, err := composables.UseUser(r.Context())
	if err != nil {
		return nil, err
	}

	// Get supported languages from application config
	supportedLanguages := c.app.GetSupportedLanguages()
	languages := intl.GetSupportedLanguages(supportedLanguages)

	props := &account.ProfilePageProps{
		PostPath:  c.basePath,
		User:      mappers.UserToViewModel(u),
		Errors:    nonNilErrors,
		Languages: languages,
	}
	return props, nil
}

func (c *AccountController) Get(w http.ResponseWriter, r *http.Request) {
	props, err := c.defaultProps(r, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	templ.Handler(account.Index(props)).ServeHTTP(w, r)
}

func (c *AccountController) Update(w http.ResponseWriter, r *http.Request) {
	logger := composables.UseLogger(r.Context())
	dto, err := composables.UseForm(&dtos.SaveAccountDTO{}, r)
	if err != nil {
		logger.WithError(err).Error("failed to parse form")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if errors, ok := dto.Ok(r.Context()); !ok {
		props, err := c.defaultProps(r, errors)
		if err != nil {
			logger.WithError(err).Error("failed to get default props")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		logger.WithField("errors", errors).Debug("validation failed")
		templ.Handler(account.ProfileForm(props)).ServeHTTP(w, r)
		return
	}
	u, err := composables.UseUser(r.Context())
	if err != nil {
		logger.WithError(err).Error("failed to get user from context")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	entity, err := dto.Apply(u)
	if err != nil {
		logger.WithError(err).Error("failed to apply dto")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := c.userService.Update(r.Context(), entity); err != nil {
		logger.WithError(err).Error("failed to update user")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Refresh", "true")

	// Get supported languages for the response
	supportedLanguages := c.app.GetSupportedLanguages()
	languages := intl.GetSupportedLanguages(supportedLanguages)

	templ.Handler(account.ProfileForm(&account.ProfilePageProps{
		User:      mappers.UserToViewModel(entity),
		Errors:    map[string]string{},
		Languages: languages,
	})).ServeHTTP(w, r)
}
