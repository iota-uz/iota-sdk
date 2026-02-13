package controllers

import (
	"fmt"
	"net/http"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"

	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/mappers"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/account"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

type AccountController struct {
	app            application.Application
	userService    *services.UserService
	tenantService  *services.TenantService
	uploadService  *services.UploadService
	sessionService *services.SessionService
	basePath       string
}

func NewAccountController(app application.Application) application.Controller {
	return &AccountController{
		app:            app,
		userService:    app.Service(services.UserService{}).(*services.UserService),
		tenantService:  app.Service(services.TenantService{}).(*services.TenantService),
		uploadService:  app.Service(services.UploadService{}).(*services.UploadService),
		sessionService: app.Service(services.SessionService{}).(*services.SessionService),
		basePath:       "/account",
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
	getRouter.HandleFunc("/sessions", c.GetSessions).Methods(http.MethodGet)

	setRouter := r.PathPrefix(c.basePath).Subrouter()
	setRouter.Use(commonMiddleware...)
	setRouter.HandleFunc("", c.Update).Methods(http.MethodPost)

	deleteRouter := r.PathPrefix(c.basePath).Subrouter()
	deleteRouter.Use(commonMiddleware...)
	// Register specific routes before parameterized ones to avoid route conflicts
	deleteRouter.HandleFunc("/sessions/others", c.RevokeOtherSessions).Methods(http.MethodDelete)
	deleteRouter.HandleFunc("/sessions/{token}", c.RevokeSession).Methods(http.MethodDelete)
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

	if cannot := composables.CanUser(r.Context(), permissions.UserUpdate); cannot != nil {
		if u.Email() != nil && u.Email().Value() != "" && u.Email().Value() != entity.Email().Value() {
			logger.Error("normal user can not change non empty email")
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}

		if u.Phone() != nil && u.Phone().Value() != "" && u.Phone().Value() != entity.Phone().Value() {
			logger.Error("normal user can not change non empty phone")
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}
	}

	if _, err := c.userService.UpdateSelf(r.Context(), entity); err != nil {
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

func (c *AccountController) GetSessions(w http.ResponseWriter, r *http.Request) {
	logger := composables.UseLogger(r.Context())

	// Get current user
	user, err := composables.UseUser(r.Context())
	if err != nil {
		logger.WithError(err).Error("failed to get user from context")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get current session token from cookie
	config := configuration.Use()
	cookie, err := r.Cookie(config.SidCookieKey)
	if err != nil {
		logger.WithError(err).Error("failed to get session cookie")
		http.Error(w, "Session not found", http.StatusUnauthorized)
		return
	}
	currentToken := cookie.Value

	// Fetch all sessions for the user
	sessions, err := c.sessionService.GetByUserID(r.Context(), user.ID())
	if err != nil {
		logger.WithError(err).Error("failed to fetch user sessions")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Map sessions to ViewModels
	sessionVMs := make([]*viewmodels.Session, 0, len(sessions))
	for _, s := range sessions {
		sessionVMs = append(sessionVMs, viewmodels.SessionToViewModel(s, currentToken))
	}

	// Render sessions page
	props := &account.SessionsPageProps{
		Sessions:     sessionVMs,
		CurrentToken: currentToken,
		Errors:       make(map[string]string),
	}

	templ.Handler(account.SessionsPage(props)).ServeHTTP(w, r)
}

func (c *AccountController) RevokeSession(w http.ResponseWriter, r *http.Request) {
	logger := composables.UseLogger(r.Context())
	pageCtx := composables.UsePageCtx(r.Context())

	// Get token from URL path
	vars := mux.Vars(r)
	tokenHash := vars["token"]

	// Get current session token from cookie
	config := configuration.Use()
	cookie, err := r.Cookie(config.SidCookieKey)
	if err != nil {
		logger.WithError(err).Error("failed to get session cookie")
		http.Error(w, "Session not found", http.StatusUnauthorized)
		return
	}
	currentToken := cookie.Value

	// Get current user
	user, err := composables.UseUser(r.Context())
	if err != nil {
		logger.WithError(err).Error("failed to get user from context")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Fetch all sessions to find the one to revoke
	sessions, err := c.sessionService.GetByUserID(r.Context(), user.ID())
	if err != nil {
		logger.WithError(err).Error("failed to fetch user sessions")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Find the session to revoke by comparing token hash (TokenID) from URL
	var sessionToRevoke string
	for _, s := range sessions {
		sessionVM := viewmodels.SessionToViewModel(s, currentToken)
		if sessionVM.TokenID == tokenHash {
			sessionToRevoke = s.Token()
			break
		}
	}

	if sessionToRevoke == "" {
		logger.Error("session not found")
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	// Prevent revoking current session
	if sessionToRevoke == currentToken {
		logger.Error("cannot revoke current session")
		htmx.SetTrigger(w, "error", fmt.Sprintf(`{"message": "%s"}`, pageCtx.T("Account.Sessions.CannotRevokeCurrent")))
		http.Error(w, pageCtx.T("Account.Sessions.CannotRevokeCurrent"), http.StatusForbidden)
		return
	}

	// Revoke the session
	if err := c.sessionService.TerminateSession(r.Context(), sessionToRevoke); err != nil {
		logger.WithError(err).Error("failed to terminate session")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return success with HTMX trigger
	htmx.SetTrigger(w, "success", fmt.Sprintf(`{"message": "%s"}`, pageCtx.T("Account.Sessions.RevokeSuccess")))
	w.WriteHeader(http.StatusOK)
}

func (c *AccountController) RevokeOtherSessions(w http.ResponseWriter, r *http.Request) {
	logger := composables.UseLogger(r.Context())
	pageCtx := composables.UsePageCtx(r.Context())

	// Get current user
	user, err := composables.UseUser(r.Context())
	if err != nil {
		logger.WithError(err).Error("failed to get user from context")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get current session token from cookie
	config := configuration.Use()
	cookie, err := r.Cookie(config.SidCookieKey)
	if err != nil {
		logger.WithError(err).Error("failed to get session cookie")
		http.Error(w, "Session not found", http.StatusUnauthorized)
		return
	}
	currentToken := cookie.Value

	// Terminate all other sessions
	count, err := c.sessionService.TerminateOtherSessions(r.Context(), user.ID(), currentToken)
	if err != nil {
		logger.WithError(err).Error("failed to terminate other sessions")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return success with count and refresh page
	successMsg := fmt.Sprintf(pageCtx.T("Account.Sessions.RevokeAllSuccess"), count)
	successMsg = fmt.Sprintf(`{"message": "%s"}`, successMsg)
	htmx.SetTrigger(w, "success", successMsg)
	htmx.Refresh(w)
}
