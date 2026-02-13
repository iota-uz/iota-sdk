package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/gorilla/mux"

	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/intl"
)

func getToken(r *http.Request) (string, error) {
	conf := configuration.Use()
	token, err := r.Cookie(conf.SidCookieKey)
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
				token, err := getToken(r)
				if err != nil {
					next.ServeHTTP(w, r)
					return
				}
				ctx := r.Context()
				app, err := application.UseApp(ctx)
				if err != nil {
					panic(err)
				}
				authService := app.Service(services.AuthService{}).(*services.AuthService)
				sess, err := authService.Authorize(ctx, token)
				if err != nil {
					next.ServeHTTP(w, r)
					return
				}

				// Security: Validate session is active (not pending 2FA or expired)
				if !sess.IsActive() {
					// Inactive sessions (pending 2FA, expired, etc.) should not be authenticated
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
				app, err := application.UseApp(ctx)
				if err != nil {
					panic(err)
				}
				userService := app.Service(services.UserService{}).(*services.UserService)
				u, err := userService.GetByID(ctx, sess.UserID())
				if err != nil {
					next.ServeHTTP(w, r)
					return
				}

				// Check if user is blocked
				if u.IsBlocked() {
					// Clear session cookie
					conf := configuration.Use()
					http.SetCookie(w, &http.Cookie{
						Name:   conf.SidCookieKey,
						Value:  "",
						Path:   "/",
						MaxAge: -1,
					})
					// Redirect to login with localized error
					errorMsg := intl.MustT(r.Context(), "Login.Errors.AccountBlocked")
					escapedError := url.QueryEscape(errorMsg)
					redirectURL := fmt.Sprintf("/login?error=%s", escapedError)
					http.Redirect(w, r, redirectURL, http.StatusFound)
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
