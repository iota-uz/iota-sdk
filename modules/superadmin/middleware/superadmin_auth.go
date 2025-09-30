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
