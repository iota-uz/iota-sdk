// Package controllers provides this package.
package controllers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/go-i18n/v2/i18n"
	coreuser "github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
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
	"github.com/iota-uz/iota-sdk/pkg/security"
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

type MiddlewareChainCustomizer func(defaults []mux.MiddlewareFunc) []mux.MiddlewareFunc

type LoginControllerOptions struct {
	// LoginAccessCheck runs after successful authentication and before session creation.
	LoginAccessCheck func(ctx context.Context, u coreuser.User) error
	// CustomizeGetMiddlewares receives default middleware for /login GET and OAuth callback routes
	// and returns the final chain.
	CustomizeGetMiddlewares MiddlewareChainCustomizer
	// CustomizePostMiddlewares receives default middleware for /login POST route
	// and returns the final chain.
	CustomizePostMiddlewares MiddlewareChainCustomizer
	// Renderer allows replacing the default SDK login page rendering while reusing controller logic.
	Renderer LoginPageRenderer
	// MethodProviders allows extending login with custom methods and routes.
	MethodProviders []LoginMethodProvider
	// IncludePasswordMethod controls whether the default password method is shown (defaults to true).
	IncludePasswordMethod *bool
	// IncludeGoogleMethod controls whether the default Google OAuth method is shown (defaults to true).
	IncludeGoogleMethod *bool
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

func NewLoginController(app application.Application, opts ...*LoginControllerOptions) application.Controller {
	options := &LoginControllerOptions{}
	if len(opts) > 0 && opts[0] != nil {
		options = opts[0]
	}
	return &LoginController{
		app:            app,
		authService:    app.Service(services.AuthService{}).(*services.AuthService),
		sessionService: app.Service(services.SessionService{}).(*services.SessionService),
		userService:    app.Service(services.UserService{}).(*services.UserService),
		options:        options,
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
	app              application.Application
	authService      *services.AuthService
	twoFactorPolicy  pkgtwofactor.TwoFactorPolicy
	twoFactorService *twofactor.TwoFactorService
	sessionService   *services.SessionService
	userService      *services.UserService
	options          *LoginControllerOptions
}

func (c *LoginController) Key() string {
	return "/login"
}

func (c *LoginController) Register(r *mux.Router) {
	getRouter := r.PathPrefix("/").Subrouter()
	getRouter.Use(c.GetMiddlewares()...)
	getRouter.HandleFunc("/login", c.Get).Methods(http.MethodGet)
	getRouter.HandleFunc("/oauth/google/callback", c.GoogleCallback)

	postRouter := r.PathPrefix("/login").Subrouter()
	postRouter.Use(c.PostMiddlewares()...)
	postRouter.HandleFunc("", c.Post).Methods(http.MethodPost)

	for _, provider := range c.optionsOrDefault().MethodProviders {
		if provider == nil {
			continue
		}
		provider.RegisterRoutes(r, c)
	}
}

// GetMiddlewares returns middleware used for login GET routes.
func (c *LoginController) GetMiddlewares() []mux.MiddlewareFunc {
	defaults := []mux.MiddlewareFunc{
		middleware.ProvideLocalizer(c.app),
		middleware.WithPageContext(),
	}
	if c.optionsOrDefault().CustomizeGetMiddlewares != nil {
		return c.optionsOrDefault().CustomizeGetMiddlewares(cloneMiddlewares(defaults))
	}
	return defaults
}

// PostMiddlewares returns middleware used for login POST routes.
func (c *LoginController) PostMiddlewares() []mux.MiddlewareFunc {
	defaults := []mux.MiddlewareFunc{
		middleware.ProvideLocalizer(c.app),
		middleware.IPRateLimitPeriod(10, time.Minute), // 10 login attempts per minute per IP
	}
	if c.optionsOrDefault().CustomizePostMiddlewares != nil {
		return c.optionsOrDefault().CustomizePostMiddlewares(cloneMiddlewares(defaults))
	}
	return defaults
}

func cloneMiddlewares(mws []mux.MiddlewareFunc) []mux.MiddlewareFunc {
	return append([]mux.MiddlewareFunc(nil), mws...)
}

func (c *LoginController) optionsOrDefault() *LoginControllerOptions {
	if c.options == nil {
		return &LoginControllerOptions{}
	}
	return c.options
}

func (c *LoginController) runLoginAccessCheck(ctx context.Context, u coreuser.User) error {
	if c.options == nil || c.options.LoginAccessCheck == nil {
		return nil
	}
	return c.options.LoginAccessCheck(ctx, u)
}

func (c *LoginController) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	// Validate and sanitize the redirect URL to prevent open redirect attacks
	nextURL := security.GetValidatedRedirect(r.URL.Query().Get("next"))
	queryParams := url.Values{
		"next": []string{nextURL},
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

	if err := c.runLoginAccessCheck(r.Context(), u); err != nil {
		shared.SetFlash(w, "error", []byte(err.Error()))
		queryParams.Set("error", err.Error())
		http.Redirect(w, r, fmt.Sprintf("/login?%s", queryParams.Encode()), http.StatusFound)
		return
	}

	c.finalizeAuthenticatedSession(
		w,
		r,
		u,
		sess,
		pkgtwofactor.AuthMethodOAuth,
		nextURL,
		fmt.Sprintf("/login?%s", queryParams.Encode()),
	)
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

	methods, err := c.buildLoginMethods(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	viewModel := LoginPageViewModel{
		ErrorsMap:    errorsMap,
		Email:        email,
		ErrorMessage: string(errorMessage),
		Methods:      methods,
	}

	if renderer := c.optionsOrDefault().Renderer; renderer != nil {
		if err := renderer(r.Context(), viewModel).Render(r.Context(), w); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	if err := c.renderDefaultLogin(w, r, viewModel); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (c *LoginController) renderDefaultLogin(w http.ResponseWriter, r *http.Request, vm LoginPageViewModel) error {
	logoComponent, _ := composables.UseLogo(r.Context())
	return login.Index(&login.LoginProps{
		ErrorsMap:    vm.ErrorsMap,
		Email:        vm.Email,
		ErrorMessage: vm.ErrorMessage,
		Methods:      toTemplateLoginMethods(vm.Methods),
		Logo:         logoComponent,
	}).Render(r.Context(), w)
}

func toTemplateLoginMethods(methods []LoginMethod) []login.LoginMethod {
	mapped := make([]login.LoginMethod, 0, len(methods))
	for _, method := range methods {
		mapped = append(mapped, login.LoginMethod{
			ID:         method.ID,
			Label:      method.Label,
			Href:       method.Href,
			Variant:    method.Variant,
			Icon:       method.Icon,
			Attributes: method.Attributes,
		})
	}
	return mapped
}

func (c *LoginController) buildLoginMethods(w http.ResponseWriter, r *http.Request) ([]LoginMethod, error) {
	methods := make([]LoginMethod, 0, len(c.optionsOrDefault().MethodProviders)+2)
	seen := make(map[string]struct{}, len(c.optionsOrDefault().MethodProviders)+2)

	if c.includePasswordMethod() {
		method := LoginMethod{
			ID:      "password",
			Label:   intl.MustT(r.Context(), "Login.Login"),
			Variant: "password",
		}
		methods = append(methods, method)
		seen[method.ID] = struct{}{}
	}

	if c.includeGoogleMethod() && configuration.Use().Google.IsConfigured() {
		codeURL, err := c.authService.GoogleAuthenticate(w)
		if err != nil {
			return nil, err
		}
		method := LoginMethod{
			ID:      "google",
			Label:   intl.MustT(r.Context(), "Login.LoginWithGoogle"),
			Href:    codeURL,
			Variant: "oauth-google",
			Icon:    login.GoogleIcon(),
		}
		methods = append(methods, method)
		seen[method.ID] = struct{}{}
	}

	for _, provider := range c.optionsOrDefault().MethodProviders {
		if provider == nil {
			continue
		}

		method, err := provider.BuildMethod(r.Context(), r)
		if err != nil {
			composables.UseLogger(r.Context()).Error("failed to build login method", "provider", provider.ID(), "error", err)
			continue
		}
		if method == nil {
			continue
		}

		method.ID = strings.TrimSpace(method.ID)
		if method.ID == "" {
			composables.UseLogger(r.Context()).Warn("login method provider returned empty method id", "provider", provider.ID())
			continue
		}
		if _, ok := seen[method.ID]; ok {
			composables.UseLogger(r.Context()).Warn("login method provider returned duplicate method id", "provider", provider.ID(), "method", method.ID)
			continue
		}

		methods = append(methods, *method)
		seen[method.ID] = struct{}{}
	}

	return methods, nil
}

func (c *LoginController) includePasswordMethod() bool {
	if c.optionsOrDefault().IncludePasswordMethod == nil {
		return true
	}
	return *c.optionsOrDefault().IncludePasswordMethod
}

func (c *LoginController) includeGoogleMethod() bool {
	if c.optionsOrDefault().IncludeGoogleMethod == nil {
		return true
	}
	return *c.optionsOrDefault().IncludeGoogleMethod
}

func (c *LoginController) Post(w http.ResponseWriter, r *http.Request) {
	logger := composables.UseLogger(r.Context())

	// Validate the redirect URL early to prevent open redirect attacks
	nextURL := security.GetValidatedRedirect(r.URL.Query().Get("next"))
	authRequestID := r.URL.Query().Get("auth_request")
	if authRequestID == "" {
		// Backward compatibility for older OIDC login links.
		authRequestID = r.URL.Query().Get("oidc_auth_id")
	}

	postLoginRedirectURL := nextURL
	if authRequestID != "" {
		postLoginRedirectURL = fmt.Sprintf("/oidc/authorize/callback?id=%s", url.QueryEscape(authRequestID))
	}

	dto, err := composables.UseForm(&LoginDTO{}, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if errorsMap, ok := dto.Ok(r.Context()); !ok {
		shared.SetFlashMap(w, "errorsMap", errorsMap)
		http.Redirect(w, r, fmt.Sprintf("/login?email=%s&next=%s", url.QueryEscape(dto.Email), url.QueryEscape(nextURL)), http.StatusFound)
		return
	}

	u, sess, err := c.authService.Authenticate(r.Context(), dto.Email, dto.Password)
	if err != nil {
		logger.Error("POST /login: InTx failed", "error", err)
		if errors.Is(err, composables.ErrInvalidPassword) {
			shared.SetFlash(w, "error", []byte(intl.MustT(r.Context(), "Login.Errors.PasswordInvalid")))
		} else if errors.Is(err, persistence.ErrUserNotFound) {
			shared.SetFlash(w, "error", []byte(intl.MustT(r.Context(), "Login.Errors.PasswordInvalid")))
		} else {
			shared.SetFlash(w, "error", []byte(intl.MustT(r.Context(), "Errors.Internal")))
		}
		http.Redirect(w, r, fmt.Sprintf("/login?email=%s&next=%s", url.QueryEscape(dto.Email), url.QueryEscape(nextURL)), http.StatusFound)
		return
	}

	if err := c.runLoginAccessCheck(r.Context(), u); err != nil {
		shared.SetFlash(w, "error", []byte(err.Error()))
		http.Redirect(w, r, fmt.Sprintf("/login?email=%s&next=%s", url.QueryEscape(dto.Email), url.QueryEscape(nextURL)), http.StatusFound)
		return
	}

	c.finalizeAuthenticatedSession(
		w,
		r,
		u,
		sess,
		pkgtwofactor.AuthMethodPassword,
		postLoginRedirectURL,
		fmt.Sprintf("/login?email=%s&next=%s", url.QueryEscape(dto.Email), url.QueryEscape(nextURL)),
	)
}

// FinalizeAuthenticatedUser creates a session for an already-authenticated user and applies
// login access checks, 2FA policy, pending-2FA handling, and cookie issuance.
func (c *LoginController) FinalizeAuthenticatedUser(
	w http.ResponseWriter,
	r *http.Request,
	u coreuser.User,
	method pkgtwofactor.AuthMethod,
	nextURL string,
) {
	validatedNextURL := security.GetValidatedRedirect(nextURL)
	loginRedirectURL := fmt.Sprintf("/login?next=%s", url.QueryEscape(validatedNextURL))

	if err := c.runLoginAccessCheck(r.Context(), u); err != nil {
		shared.SetFlash(w, "error", []byte(err.Error()))
		http.Redirect(w, r, loginRedirectURL, http.StatusFound)
		return
	}

	sess, err := c.createSessionForUser(r.Context(), u)
	if err != nil {
		composables.UseLogger(r.Context()).Error("failed to create session during finalize login", "error", err)
		shared.SetFlash(w, "error", []byte(intl.MustT(r.Context(), "Errors.Internal")))
		http.Redirect(w, r, loginRedirectURL, http.StatusFound)
		return
	}

	c.finalizeAuthenticatedSession(w, r, u, sess, method, validatedNextURL, loginRedirectURL)
}

func (c *LoginController) finalizeAuthenticatedSession(
	w http.ResponseWriter,
	r *http.Request,
	u coreuser.User,
	sess session.Session,
	method pkgtwofactor.AuthMethod,
	nextURL string,
	loginErrorRedirectURL string,
) {
	validatedNextURL := security.GetValidatedRedirect(nextURL)
	if strings.TrimSpace(loginErrorRedirectURL) == "" {
		loginErrorRedirectURL = fmt.Sprintf("/login?next=%s", url.QueryEscape(validatedNextURL))
	}

	requires2FA, err := c.requiresTwoFactor(r.Context(), u, method)
	if err != nil {
		composables.UseLogger(r.Context()).Error("failed to evaluate 2FA policy", "method", method, "error", err)
		shared.SetFlash(w, "error", []byte(intl.MustT(r.Context(), "Errors.Internal")))
		http.Redirect(w, r, loginErrorRedirectURL, http.StatusFound)
		return
	}

	if requires2FA {
		pendingSession := session.New(
			sess.Token(),
			sess.UserID(),
			sess.TenantID(),
			sess.IP(),
			sess.UserAgent(),
			session.WithStatus(session.StatusPending2FA),
			session.WithAudience(sess.Audience()),
			session.WithExpiresAt(time.Now().Add(10*time.Minute)),
			session.WithCreatedAt(sess.CreatedAt()),
		)
		if err := c.sessionService.Update(r.Context(), pendingSession); err != nil {
			composables.UseLogger(r.Context()).Error("failed to update session to pending 2FA", "method", method, "error", err)
			shared.SetFlash(w, "error", []byte(intl.MustT(r.Context(), "Errors.Internal")))
			http.Redirect(w, r, loginErrorRedirectURL, http.StatusFound)
			return
		}

		http.SetCookie(w, c.sessionCookie(pendingSession.Token(), pendingSession.ExpiresAt()))
		if u.Has2FAEnabled() {
			http.Redirect(w, r, fmt.Sprintf("/login/2fa/verify?next=%s", url.QueryEscape(validatedNextURL)), http.StatusFound)
			return
		}

		http.Redirect(w, r, fmt.Sprintf("/login/2fa/setup?next=%s", url.QueryEscape(validatedNextURL)), http.StatusFound)
		return
	}

	http.SetCookie(w, c.sessionCookie(sess.Token(), sess.ExpiresAt()))
	http.Redirect(w, r, validatedNextURL, http.StatusFound)
}

func (c *LoginController) requiresTwoFactor(
	ctx context.Context,
	u coreuser.User,
	method pkgtwofactor.AuthMethod,
) (bool, error) {
	requires2FA := u.Has2FAEnabled()
	if c.twoFactorPolicy == nil {
		return requires2FA, nil
	}

	ip, _ := composables.UseIP(ctx)
	userAgent, _ := composables.UseUserAgent(ctx)

	attempt := pkgtwofactor.AuthAttempt{
		UserID:    userIDToNamespacedUUID(u.TenantID(), u.ID()),
		Method:    method,
		IPAddress: ip,
		UserAgent: userAgent,
		Timestamp: time.Now(),
	}
	return c.twoFactorPolicy.Requires(ctx, attempt)
}

func (c *LoginController) sessionCookie(token string, expiresAt time.Time) *http.Cookie {
	conf := configuration.Use()
	return &http.Cookie{
		Name:     conf.SidCookieKey,
		Value:    token,
		Expires:  expiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   conf.GoAppEnvironment == configuration.Production,
		Domain:   conf.Domain,
		Path:     "/",
	}
}

func (c *LoginController) createSessionForUser(ctx context.Context, u coreuser.User) (session.Session, error) {
	ctx = composables.WithTenantID(ctx, u.TenantID())

	ip, ok := composables.UseIP(ctx)
	if !ok {
		ip = "0.0.0.0"
	}

	userAgent, ok := composables.UseUserAgent(ctx)
	if !ok {
		userAgent = "Unknown"
	}

	token, err := newSessionToken()
	if err != nil {
		return nil, err
	}

	if err := c.userService.UpdateLastLogin(ctx, u.ID()); err != nil {
		return nil, err
	}
	if err := c.userService.UpdateLastAction(ctx, u.ID()); err != nil {
		return nil, err
	}

	dto := &session.CreateDTO{
		Token:     token,
		UserID:    u.ID(),
		TenantID:  u.TenantID(),
		IP:        ip,
		UserAgent: userAgent,
	}
	if err := c.sessionService.Create(ctx, dto); err != nil {
		return nil, err
	}

	return dto.ToEntity(), nil
}

func newSessionToken() (string, error) {
	bytes := make([]byte, 24)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func userIDToNamespacedUUID(tenantID uuid.UUID, userID uint) uuid.UUID {
	var userIDData [8]byte
	binary.LittleEndian.PutUint64(userIDData[:], uint64(userID))
	return uuid.NewSHA1(tenantID, userIDData[:])
}
