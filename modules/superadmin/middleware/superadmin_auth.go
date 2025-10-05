package middleware

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
)

// RequireSuperAdmin returns a middleware that ensures only superadmin users can access the route.
// If the user is not a superadmin, returns 403 Forbidden.
//
// Example usage in a controller's Register method:
//
//	func (c *SuperAdminController) Register(router *mux.Router) {
//		// Create a subrouter for superadmin routes
//		s := router.PathPrefix("/superadmin").Subrouter()
//
//		// Apply the superadmin middleware to all routes
//		s.Use(middleware.Authorize())      // Ensure user is authenticated
//		s.Use(middleware.ProvideUser())    // Load user into context
//		s.Use(RequireSuperAdmin())         // Enforce superadmin access
//
//		// Register protected routes
//		s.HandleFunc("/dashboard", c.Dashboard).Methods(http.MethodGet)
//		s.HandleFunc("/tenants", c.ListTenants).Methods(http.MethodGet)
//		s.HandleFunc("/system/settings", c.SystemSettings).Methods(http.MethodGet)
//	}
//
// The middleware chain ensures:
//  1. User is authenticated (has valid session)
//  2. User object is loaded into context
//  3. User type is TypeSuperAdmin (otherwise returns 403 Forbidden)
//
// All handlers registered under this router will only be accessible to superadmin users.
func RequireSuperAdmin() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Get logger from context
			logger, ok := ctx.Value(constants.LoggerKey).(*logrus.Entry)
			if !ok {
				logger = logrus.WithField("middleware", "superadmin_auth")
			}

			// Extract user from context
			usr, err := composables.UseUser(ctx)
			if err != nil {
				logger.WithError(err).Warn("No user found in context")
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			// Check if user is nil (shouldn't happen if UseUser succeeds, but safety check)
			if usr == nil {
				logger.Warn("User is nil despite no error from UseUser")
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			// Check if user type is superadmin
			if usr.Type() != user.TypeSuperAdmin {
				logger.WithFields(logrus.Fields{
					"user_id":   usr.ID(),
					"user_type": usr.Type(),
				}).Warn("Non-superadmin user attempted to access superadmin route")
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			// User is superadmin, allow request to proceed
			logger.WithField("user_id", usr.ID()).Debug("Superadmin user authorized")
			next.ServeHTTP(w, r)
		})
	}
}
