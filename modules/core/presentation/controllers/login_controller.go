// Package controllers provides this package.
package controllers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/iota-uz/go-i18n/v2/i18n"
	coreuser "github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
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
		app:             app,
		authService:     app.Service(services.AuthService{}).(*services.AuthService),
		authFlowService: app.Service(services.AuthFlowService{}).(*services.AuthFlowService),
		options:         options,
	}
}

// SetTwoFactorPolicy sets the 2FA policy for the controller.
func (c *LoginController) SetTwoFactorPolicy(policy pkgtwofactor.TwoFactorPolicy) {
	c.authFlowService.SetTwoFactorPolicy(policy)
}

// SetTwoFactorService sets the 2FA service for the controller.
func (c *LoginController) SetTwoFactorService(service *twofactor.TwoFactorService) {
	c.twoFactorService = service
}

type LoginController struct {
	app              application.Application
	authService      *services.AuthService
	authFlowService  *services.AuthFlowService
	twoFactorService *twofactor.TwoFactorService
	options          *LoginControllerOptions
}

func (c *LoginController) Key() string {
	return "/login"
}

func (c *LoginController) Register(r *mux.Router) {
	getRouter := r.PathPrefix("/").Subrouter()
	getRouter.Use(c.GetMiddlewares()...)
	getRouter.HandleFunc("/login", c.Get).Methods(http.MethodGet)
	getRouter.HandleFunc("/oauth/google/callback", c.GoogleCallback).Methods(http.MethodGet)

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

	authResult, err := c.authFlowService.AuthenticateGoogle(r.Context(), code)
	if err != nil {
		if errors.Is(err, persistence.ErrUserNotFound) {
			queryParams.Set("error", intl.MustT(r.Context(), "Login.Errors.UserNotFound"))
		} else {
			queryParams.Set("error", intl.MustT(r.Context(), "Errors.Internal"))
		}
		http.Redirect(w, r, fmt.Sprintf("/login?%s", queryParams.Encode()), http.StatusFound)
		return
	}

	loginRedirectURL := fmt.Sprintf("/login?%s", queryParams.Encode())
	finalizeResult, err := c.authFlowService.FinalizeAuthentication(r.Context(), authResult, services.FinalizeAuthenticationOptions{
		NextURL:          nextURL,
		ErrorRedirectURL: loginRedirectURL,
		AccessCheck:      c.runLoginAccessCheck,
	})
	if err != nil {
		c.handleFinalizeError(w, r, loginRedirectURL, err)
		return
	}

	c.applyFinalizeResult(w, r, finalizeResult)
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

	logoComponent, _ := composables.UseLogo(r.Context())

	viewModel := LoginPageViewModel{
		ErrorsMap:    errorsMap,
		Email:        email,
		ErrorMessage: string(errorMessage),
		Methods:      methods,
		Logo:         logoComponent,
	}

	if renderer := c.optionsOrDefault().Renderer; renderer != nil {
		if err := c.renderLoginComponent(w, r, renderer(r.Context(), viewModel)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	if len(methods) == 0 {
		http.Error(w, "no login methods configured", http.StatusInternalServerError)
		return
	}

	if err := c.renderDefaultLogin(w, r, viewModel); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (c *LoginController) renderDefaultLogin(w http.ResponseWriter, r *http.Request, vm LoginPageViewModel) error {
	return c.renderLoginComponent(w, r, login.Index(&login.LoginProps{
		ErrorsMap:    vm.ErrorsMap,
		Email:        vm.Email,
		ErrorMessage: vm.ErrorMessage,
		Methods:      toTemplateLoginMethods(vm.Methods),
		Logo:         vm.Logo,
	}))
}

func (c *LoginController) renderLoginComponent(w http.ResponseWriter, r *http.Request, component interface{ Render(context.Context, io.Writer) error }) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return component.Render(r.Context(), w)
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

		builtMethod := *method
		builtMethod.ID = strings.TrimSpace(builtMethod.ID)
		if builtMethod.ID == "" {
			composables.UseLogger(r.Context()).Warn("login method provider returned empty method id", "provider", provider.ID())
			continue
		}
		if builtMethod.Href != "" && !security.IsValidRedirect(builtMethod.Href) {
			composables.UseLogger(r.Context()).Warn("login method provider returned invalid method href", "provider", provider.ID(), "method", builtMethod.ID, "href", builtMethod.Href)
			continue
		}
		if _, ok := seen[builtMethod.ID]; ok {
			composables.UseLogger(r.Context()).Warn("login method provider returned duplicate method id", "provider", provider.ID(), "method", builtMethod.ID)
			continue
		}

		methods = append(methods, builtMethod)
		seen[builtMethod.ID] = struct{}{}
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

	authResult, err := c.authFlowService.AuthenticatePassword(r.Context(), dto.Email, dto.Password)
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

	loginRedirectURL := fmt.Sprintf("/login?email=%s&next=%s", url.QueryEscape(dto.Email), url.QueryEscape(nextURL))
	finalizeResult, err := c.authFlowService.FinalizeAuthentication(r.Context(), authResult, services.FinalizeAuthenticationOptions{
		NextURL:          postLoginRedirectURL,
		ErrorRedirectURL: loginRedirectURL,
		AccessCheck:      c.runLoginAccessCheck,
	})
	if err != nil {
		c.handleFinalizeError(w, r, loginRedirectURL, err)
		return
	}

	c.applyFinalizeResult(w, r, finalizeResult)
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

	finalizeResult, err := c.authFlowService.FinalizeAuthenticatedUser(r.Context(), u, method, services.FinalizeAuthenticationOptions{
		NextURL:          validatedNextURL,
		ErrorRedirectURL: loginRedirectURL,
		AccessCheck:      c.runLoginAccessCheck,
	})
	if err != nil {
		c.handleFinalizeError(w, r, loginRedirectURL, err)
		return
	}

	c.applyFinalizeResult(w, r, finalizeResult)
}

func (c *LoginController) handleFinalizeError(
	w http.ResponseWriter,
	r *http.Request,
	redirectURL string,
	err error,
) {
	var userVisibleErr *services.UserVisibleError
	if errors.As(err, &userVisibleErr) && strings.TrimSpace(userVisibleErr.Message) != "" {
		shared.SetFlash(w, "error", []byte(userVisibleErr.Message))
		http.Redirect(w, r, redirectURL, http.StatusFound)
		return
	}

	composables.UseLogger(r.Context()).Error("failed to finalize login", "error", err)
	shared.SetFlash(w, "error", []byte(intl.MustT(r.Context(), "Errors.Internal")))
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func (c *LoginController) applyFinalizeResult(
	w http.ResponseWriter,
	r *http.Request,
	result *services.FinalizeAuthenticationResult,
) {
	if result == nil {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	if result.Cookie != nil {
		http.SetCookie(w, result.Cookie)
	}
	http.Redirect(w, r, result.RedirectURL, http.StatusFound)
}
