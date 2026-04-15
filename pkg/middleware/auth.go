// Package middleware provides this package.
package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gorilla/mux"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"golang.org/x/text/language"

	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/intl"
)

// resolveSIDKey returns the session-ID cookie key from the typed httpconfig.Config
// registered in the composition container, falling back to "sid" if unavailable.
func resolveSIDKey(ctx context.Context) string {
	container, err := composition.UseContainer(ctx)
	if err != nil {
		return "sid"
	}
	cfg, err := composition.Resolve[*httpconfig.Config](container)
	if err != nil || cfg == nil {
		return "sid"
	}
	if cfg.Cookies.SID == "" {
		return "sid"
	}
	return cfg.Cookies.SID
}

func getToken(r *http.Request, sidKey string) (string, error) {
	token, err := r.Cookie(sidKey)
	if errors.Is(err, http.ErrNoCookie) {
		v := r.Header.Get("Authorization")
		if v == "" {
			return "", errors.New("no token found")
		}
		return v, nil
	}
	if err != nil {
		return "", err
	}
	return token.Value, nil
}

func Authorize() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				ctx := r.Context()
				token, err := getToken(r, resolveSIDKey(ctx))
				if err != nil {
					next.ServeHTTP(w, r)
					return
				}
				container, err := composition.UseContainer(ctx)
				if err != nil {
					composables.UseLogger(ctx).WithError(err).Error("Authorize: composition container not found in context")
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}
				authService, err := composition.Resolve[*services.AuthService](container)
				if err != nil {
					composables.UseLogger(ctx).WithError(err).Error("Authorize: failed to resolve AuthService")
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}
				sess, err := authService.Authorize(ctx, token)
				if err != nil {
					next.ServeHTTP(w, r)
					return
				}

				// Security: pending 2FA sessions are only allowed on 2FA routes.
				// Other inactive sessions (expired, invalid status) are not authenticated.
				if !sess.IsActive() {
					if !sess.IsPending() || !strings.HasPrefix(r.URL.Path, "/login/2fa/") {
						next.ServeHTTP(w, r)
						return
					}
				}

				if _, err := composables.UseTenantID(ctx); err != nil {
					ctx = composables.WithTenantID(ctx, sess.TenantID())
				}

				params, ok := composables.UseParams(ctx)
				if !ok {
					panic("params not found. Add RequestParams middleware up the chain")
				}
				params.Authenticated = true
				next.ServeHTTP(w, r.WithContext(context.WithValue(ctx, constants.SessionKey, sess)))
			},
		)
	}
}

// AuthorizeAnySession attaches the session context when the token is valid,
// regardless of the session status (active, pending 2FA, etc.).
func AuthorizeAnySession() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				ctx := r.Context()
				token, err := getToken(r, resolveSIDKey(ctx))
				if err != nil {
					next.ServeHTTP(w, r)
					return
				}
				container, err := composition.UseContainer(ctx)
				if err != nil {
					composables.UseLogger(ctx).WithError(err).Error("AuthorizeAnySession: composition container not found in context")
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}
				authService, err := composition.Resolve[*services.AuthService](container)
				if err != nil {
					composables.UseLogger(ctx).WithError(err).Error("AuthorizeAnySession: failed to resolve AuthService")
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}
				sess, err := authService.Authorize(ctx, token)
				if err != nil {
					next.ServeHTTP(w, r)
					return
				}

				if _, err := composables.UseTenantID(ctx); err != nil {
					ctx = composables.WithTenantID(ctx, sess.TenantID())
				}

				params, ok := composables.UseParams(ctx)
				if !ok {
					panic("params not found. Add RequestParams middleware up the chain")
				}
				params.Authenticated = true
				next.ServeHTTP(w, r.WithContext(context.WithValue(ctx, constants.SessionKey, sess)))
			},
		)
	}
}

func ProvideUser() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				ctx := r.Context()
				sess, err := composables.UseSession(ctx)
				if err != nil {
					next.ServeHTTP(w, r)
					return
				}
				container, err := composition.UseContainer(ctx)
				if err != nil {
					composables.UseLogger(ctx).WithError(err).Error("ProvideUser: composition container not found in context")
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}
				userService, err := composition.Resolve[*services.UserService](container)
				if err != nil {
					composables.UseLogger(ctx).WithError(err).Error("ProvideUser: failed to resolve UserService")
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}
				u, err := userService.GetByID(ctx, sess.UserID())
				if err != nil {
					next.ServeHTTP(w, r)
					return
				}

				// Refresh the localizer early so that the user's saved UI
				// language takes precedence over the header-based locale
				// that the global ProvideLocalizer middleware installed
				// earlier in the chain. This must run before the
				// blocked-user check so the error message is rendered in
				// the user's preferred language.
				ctx = refreshLocalizerForUser(ctx, container, string(u.UILanguage()))

				// Check if user is blocked
				if u.IsBlocked() {
					// Clear session cookie
					http.SetCookie(w, &http.Cookie{
						Name:   resolveSIDKey(ctx),
						Value:  "",
						Path:   "/",
						MaxAge: -1,
					})
					// Redirect to login with localized error
					errorMsg := intl.MustT(ctx, "Login.Errors.AccountBlocked")
					escapedError := url.QueryEscape(errorMsg)
					redirectURL := fmt.Sprintf("/login?error=%s", escapedError)
					http.Redirect(w, r.WithContext(ctx), redirectURL, http.StatusFound)
					return
				}

				// Set the user in context
				ctx = context.WithValue(ctx, constants.UserKey, u)

				// Check if we already have a tenant in context
				_, tenantErr := composables.UseTenantID(ctx)
				if tenantErr != nil {
					ctx = context.WithValue(ctx, constants.TenantIDKey, u.TenantID())
				}

				next.ServeHTTP(w, r.WithContext(ctx))
			},
		)
	}
}

// refreshLocalizerForUser rebuilds the localizer on the context using the
// user's saved UI language as the primary source, falling back to whatever
// was already in context if the user's preference cannot be parsed or the
// bundle cannot be resolved. Called from ProvideUser after the user has
// been loaded, so the authenticated request rendering uses the user's
// preferred language — not the Accept-Language header the global
// ProvideLocalizer middleware selected pre-auth.
//
// Resolves the bundle lazily through the composition container rather
// than capturing it at install time because ProvideUser has no access to
// a captured bundle.
func refreshLocalizerForUser(ctx context.Context, container *composition.Container, uiLanguage string) context.Context {
	code := strings.TrimSpace(uiLanguage)
	if code == "" {
		return ctx
	}
	tag, err := language.Parse(code)
	if err != nil {
		return ctx
	}
	bundle, err := composition.Resolve[*i18n.Bundle](container)
	if err != nil || bundle == nil {
		return ctx
	}
	ctx = intl.WithLocalizer(ctx, i18n.NewLocalizer(bundle, tag.String()))
	ctx = intl.WithLocale(ctx, tag)
	return ctx
}

func RedirectNotAuthenticated() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				params, ok := composables.UseParams(r.Context())
				if !ok {
					panic("params not found. Add RequestParams middleware up the chain")
				}
				if !params.Authenticated {
					http.Redirect(w, r, fmt.Sprintf("/login?next=%s", r.URL), http.StatusFound)
					return
				}
				next.ServeHTTP(w, r)
			},
		)
	}
}

func RequireAuthorization() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				params, ok := composables.UseParams(r.Context())
				if !ok {
					panic("params not found. Add RequestParams middleware up the chain")
				}
				if !params.Authenticated {
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}
				next.ServeHTTP(w, r)
			},
		)
	}
}
