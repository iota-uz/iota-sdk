package middleware

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/gorilla/mux"

	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

// Require2FAVerification returns a middleware that enforces 2FA verification.
// It checks if a session exists and is in StatusPending2FA:
//   - If pending and user has 2FA enabled → redirects to verifyPath
//   - If pending and user doesn't have 2FA → redirects to setupPath
//   - If session is active or doesn't exist → allows request through
//
// Parameters:
//   - setupPath: path to the 2FA setup flow (e.g., "/login/2fa/setup")
//   - verifyPath: path to the 2FA verify flow (e.g., "/login/2fa/verify")
//
// The middleware preserves the next URL parameter during redirects and avoids
// redirect loops by allowing 2FA endpoints themselves to pass through.
func Require2FAVerification(setupPath, verifyPath string) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				ctx := r.Context()

				// Extract session from context
				sess, err := composables.UseSession(ctx)
				if err != nil {
					// No session found, allow request through
					next.ServeHTTP(w, r)
					return
				}

				// If session is not pending 2FA, allow request through
				if !sess.IsPending() {
					next.ServeHTTP(w, r)
					return
				}

				// Session is pending 2FA verification, get app and user service
				app, err := application.UseApp(ctx)
				if err != nil {
					// If we can't get the app, deny the request
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}

				userService := app.Service(services.UserService{}).(*services.UserService)
				u, err := userService.GetByID(ctx, sess.UserID())
				if err != nil {
					// If user not found or error, deny the request
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}

				// Avoid redirect loops: if the current request is already to a 2FA endpoint, allow it
				currentPath := r.URL.Path
				if currentPath == setupPath || currentPath == verifyPath {
					next.ServeHTTP(w, r)
					return
				}

				// Determine redirect path based on user's 2FA setup status
				redirectPath := setupPath
				if u.Has2FAEnabled() {
					redirectPath = verifyPath
				}

				// Preserve the next URL parameter in the redirect
				nextURL := r.URL.String()
				q := url.Values{}
				q.Set("next", nextURL)
				redirectURL := fmt.Sprintf("%s?%s", redirectPath, q.Encode())

				http.Redirect(w, r, redirectURL, http.StatusFound)
			},
		)
	}
}
