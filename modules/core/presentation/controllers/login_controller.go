package controllers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/modules/core/services/twofactor"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/shared"
	pkgtwofactor "github.com/iota-uz/iota-sdk/pkg/twofactor"

	"github.com/go-playground/validator/v10"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/login"
)

type LoginDTO struct {
	Email    string `validate:"required"`
	Password string `validate:"required"`
}

func (e *LoginDTO) Ok(ctx context.Context) (map[string]string, bool) {
	errorMessages := map[string]string{}
	errs := constants.Validate.Struct(e)
	if errs == nil {
		return errorMessages, true
	}

	l, ok := intl.UseLocalizer(ctx)
	if !ok {
		panic(intl.ErrNoLocalizer)
	}

	for _, err := range errs.(validator.ValidationErrors) {
		translatedFieldName := l.MustLocalize(&i18n.LocalizeConfig{
			MessageID: fmt.Sprintf("Login.%s", err.Field()),
		})
		errorMessages[err.Field()] = l.MustLocalize(&i18n.LocalizeConfig{
			MessageID: fmt.Sprintf("ValidationErrors.%s", err.Tag()),
			TemplateData: map[string]string{
				"Field": translatedFieldName,
			},
		})
	}

	return errorMessages, len(errorMessages) == 0
}

func NewLoginController(app application.Application) application.Controller {
	return &LoginController{
		app:            app,
		authService:    app.Service(services.AuthService{}).(*services.AuthService),
		sessionService: app.Service(services.SessionService{}).(*services.SessionService),
		userService:    app.Service(services.UserService{}).(*services.UserService),
	}
}

// SetTwoFactorPolicy sets the 2FA policy for the controller.
func (c *LoginController) SetTwoFactorPolicy(policy pkgtwofactor.TwoFactorPolicy) {
	c.twoFactorPolicy = policy
}

// SetTwoFactorService sets the 2FA service for the controller.
func (c *LoginController) SetTwoFactorService(service *twofactor.TwoFactorService) {
	c.twoFactorService = service
}

type LoginController struct {
	app                 application.Application
	authService         *services.AuthService
	twoFactorPolicy     pkgtwofactor.TwoFactorPolicy
	twoFactorService    *twofactor.TwoFactorService
	sessionService      *services.SessionService
	userService         *services.UserService
}

func (c *LoginController) Key() string {
	return "/login"
}

func (c *LoginController) Register(r *mux.Router) {
	getRouter := r.PathPrefix("/").Subrouter()
	getRouter.Use(
		middleware.ProvideLocalizer(c.app),
		middleware.WithPageContext(),
	)
	getRouter.HandleFunc("/login", c.Get).Methods(http.MethodGet)
	getRouter.HandleFunc("/oauth/google/callback", c.GoogleCallback)

	setRouter := r.PathPrefix("/login").Subrouter()
	setRouter.Use(
		middleware.ProvideLocalizer(c.app),
		middleware.WithTransaction(),
		middleware.IPRateLimitPeriod(10, time.Minute), // 10 login attempts per minute per IP
	)
	setRouter.HandleFunc("", c.Post).Methods(http.MethodPost)
}

func (c *LoginController) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	queryParams := url.Values{
		"next": []string{r.URL.Query().Get("next")},
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		queryParams.Set("error", intl.MustT(r.Context(), "Login.Errors.OauthCodeNotFound"))
		http.Redirect(w, r, fmt.Sprintf("/login?%s", queryParams.Encode()), http.StatusFound)
		return
	}
	state := r.URL.Query().Get("state")
	if state == "" {
		queryParams.Set("error", intl.MustT(r.Context(), "Login.Errors.OauthStateNotFound"))
		http.Redirect(w, r, fmt.Sprintf("/login?%s", queryParams.Encode()), http.StatusFound)
		return
	}
	conf := configuration.Use()
	oauthCookie, err := r.Cookie(conf.OauthStateCookieKey)
	if err != nil {
		queryParams.Set("error", intl.MustT(r.Context(), "Login.Errors.OauthStateNotFound"))
		http.Redirect(w, r, fmt.Sprintf("/login?%s", queryParams.Encode()), http.StatusFound)
		return
	}
	if oauthCookie.Value != state {
		queryParams.Set("error", intl.MustT(r.Context(), "Login.Errors.OauthStateInvalid"))
		http.Redirect(w, r, fmt.Sprintf("/login?%s", queryParams.Encode()), http.StatusFound)
		return
	}

	// Authenticate with Google OAuth (bypasses 2FA for OAuth method)
	u, sess, err := c.authService.AuthenticateGoogle(r.Context(), code)
	if err != nil {
		if errors.Is(err, persistence.ErrUserNotFound) {
			queryParams.Set("error", intl.MustT(r.Context(), "Login.Errors.UserNotFound"))
		} else {
			queryParams.Set("error", intl.MustT(r.Context(), "Errors.Internal"))
		}
		http.Redirect(w, r, fmt.Sprintf("/login?%s", queryParams.Encode()), http.StatusFound)
		return
	}

	// Evaluate 2FA policy even for OAuth (should return false, but makes flow explicit)
	if c.twoFactorPolicy != nil {
		logger := composables.UseLogger(r.Context())

		// Build auth attempt for 2FA policy evaluation with OAuth method
		ip, _ := composables.UseIP(r.Context())
		userAgent, _ := composables.UseUserAgent(r.Context())

		// Convert user ID to UUID using tenant as namespace
		// This creates a deterministic UUID from tenant ID + user ID
		userIDData := make([]byte, 8)
		for i := 0; i < 8; i++ {
			userIDData[i] = byte((u.ID() >> (i * 8)) & 0xFF)
		}
		userUUID := uuid.NewSHA1(u.TenantID(), userIDData)

		attempt := pkgtwofactor.AuthAttempt{
			UserID:    userUUID,
			Method:    pkgtwofactor.AuthMethodOAuth,
			IPAddress: ip,
			UserAgent: userAgent,
			Timestamp: time.Now(),
		}

		// Check if 2FA is required for OAuth (should always be false for now)
		requires2FA, err := c.twoFactorPolicy.Requires(r.Context(), attempt)
		if err != nil {
			// Log but don't block OAuth flow
			logger.Error("Failed to evaluate 2FA policy for OAuth", "error", err)
		}

		// OAuth should bypass 2FA even if policy says otherwise (for now)
		if requires2FA {
			// Future: Could require 2FA even for OAuth if policy changes
			// For now, just log the unexpected result
			logger.Warn("Unexpected: 2FA policy requires 2FA for OAuth method", "method", pkgtwofactor.AuthMethodOAuth)
		}
	}

	// For OAuth, we bypass 2FA and create an active session directly
	sessionCookie := &http.Cookie{
		Name:     conf.SidCookieKey,
		Value:    sess.Token(),
		Expires:  sess.ExpiresAt(),
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
		Secure:   conf.GoAppEnvironment == configuration.Production,
		Domain:   conf.Domain,
	}

	http.SetCookie(w, sessionCookie)

	redirectURL := r.URL.Query().Get("next")
	if redirectURL == "" {
		redirectURL = "/"
	}
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func (c *LoginController) Get(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	errorsMap, err := composables.UseFlashMap[string, string](w, r, "errorsMap")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	errorMessage, err := composables.UseFlash(w, r, "error")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	codeURL, err := c.authService.GoogleAuthenticate(w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := login.Index(&login.LoginProps{
		ErrorsMap:          errorsMap,
		Email:              email,
		ErrorMessage:       string(errorMessage),
		GoogleOAuthCodeURL: codeURL,
	}).Render(r.Context(), w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (c *LoginController) Post(w http.ResponseWriter, r *http.Request) {
	logger := composables.UseLogger(r.Context())
	dto, err := composables.UseForm(&LoginDTO{}, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if errorsMap, ok := dto.Ok(r.Context()); !ok {
		shared.SetFlashMap(w, "errorsMap", errorsMap)
		http.Redirect(w, r, fmt.Sprintf("/login?email=%s&next=%s", dto.Email, r.URL.Query().Get("next")), http.StatusFound)
		return
	}

	// Authenticate user
	u, sess, err := c.authService.Authenticate(r.Context(), dto.Email, dto.Password)
	if err != nil {
		logger.Error("Failed to authenticate user", "error", err)
		if errors.Is(err, composables.ErrInvalidPassword) {
			shared.SetFlash(w, "error", []byte(intl.MustT(r.Context(), "Login.Errors.PasswordInvalid")))
		} else if errors.Is(err, persistence.ErrUserNotFound) {
			shared.SetFlash(w, "error", []byte(intl.MustT(r.Context(), "Login.Errors.PasswordInvalid")))
		} else {
			shared.SetFlash(w, "error", []byte(intl.MustT(r.Context(), "Errors.Internal")))
		}
		http.Redirect(w, r, fmt.Sprintf("/login?email=%s&next=%s", dto.Email, r.URL.Query().Get("next")), http.StatusFound)
		return
	}

	// Check if 2FA policy requires 2FA for this attempt
	if c.twoFactorPolicy != nil && c.twoFactorService != nil {
		// Build auth attempt for 2FA policy evaluation
		ip, _ := composables.UseIP(r.Context())
		userAgent, _ := composables.UseUserAgent(r.Context())

		// Convert user ID to UUID using tenant as namespace
		// This creates a deterministic UUID from tenant ID + user ID
		userIDData := make([]byte, 8)
		for i := 0; i < 8; i++ {
			userIDData[i] = byte((u.ID() >> (i * 8)) & 0xFF)
		}
		userUUID := uuid.NewSHA1(u.TenantID(), userIDData)

		attempt := pkgtwofactor.AuthAttempt{
			UserID:    userUUID,
			Method:    pkgtwofactor.AuthMethodPassword,
			IPAddress: ip,
			UserAgent: userAgent,
			Timestamp: time.Now(),
		}

		// Check if 2FA is required for this authentication attempt
		requires2FA, err := c.twoFactorPolicy.Requires(r.Context(), attempt)
		if err != nil {
			logger.Error("Failed to evaluate 2FA policy", "error", err)
			shared.SetFlash(w, "error", []byte(intl.MustT(r.Context(), "Errors.Internal")))
			http.Redirect(w, r, fmt.Sprintf("/login?email=%s&next=%s", dto.Email, r.URL.Query().Get("next")), http.StatusFound)
			return
		}

		if requires2FA {
			// Create a pending 2FA session
			conf := configuration.Use()
			sessionCookie := &http.Cookie{
				Name:     conf.SidCookieKey,
				Value:    sess.Token(),
				Expires:  sess.ExpiresAt(),
				HttpOnly: false,
				SameSite: http.SameSiteLaxMode,
				Secure:   conf.GoAppEnvironment == configuration.Production,
				Domain:   conf.Domain,
			}

			// Update session status to pending 2FA
			pendingSession := session.New(
				sess.Token(),
				sess.UserID(),
				sess.TenantID(),
				sess.IP(),
				sess.UserAgent(),
				session.WithStatus(session.StatusPending2FA),
				session.WithAudience(sess.Audience()),
				session.WithExpiresAt(sess.ExpiresAt()),
				session.WithCreatedAt(sess.CreatedAt()),
			)

			// Update the session in the database
			if err := c.sessionService.Update(r.Context(), pendingSession); err != nil {
				logger.Error("Failed to update session to pending 2FA", "error", err)
				shared.SetFlash(w, "error", []byte(intl.MustT(r.Context(), "Errors.Internal")))
				http.Redirect(w, r, fmt.Sprintf("/login?email=%s&next=%s", dto.Email, r.URL.Query().Get("next")), http.StatusFound)
				return
			}

			// Set the session cookie
			http.SetCookie(w, sessionCookie)

			// Redirect to 2FA verification or setup based on user's 2FA status
			nextURL := r.URL.Query().Get("next")
			if u.Has2FAEnabled() {
				// User has 2FA enabled, redirect to verification
				http.Redirect(w, r, fmt.Sprintf("/login/2fa/verify?next=%s", url.QueryEscape(nextURL)), http.StatusFound)
			} else {
				// User hasn't set up 2FA, redirect to setup
				http.Redirect(w, r, fmt.Sprintf("/login/2fa/setup?next=%s", url.QueryEscape(nextURL)), http.StatusFound)
			}
			return
		}
	}

	// No 2FA required or policy not configured, create active session
	conf := configuration.Use()
	cookie := &http.Cookie{
		Name:     conf.SidCookieKey,
		Value:    sess.Token(),
		Expires:  sess.ExpiresAt(),
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
		Secure:   conf.GoAppEnvironment == configuration.Production,
		Domain:   conf.Domain,
	}

	redirectURL := r.URL.Query().Get("next")
	if redirectURL == "" {
		redirectURL = "/"
	}
	http.SetCookie(w, cookie)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}
