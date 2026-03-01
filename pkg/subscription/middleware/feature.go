package middleware

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/subscription"
)

func RequireFeature(svc subscription.EntitlementService, feature string) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID, err := composables.UseTenantID(r.Context())
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			allowed, err := svc.HasFeature(r.Context(), tenantID, feature)
			if err != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			if !allowed {
				if htmx.IsHxRequest(r) {
					detail, _ := json.Marshal(map[string]any{
						"feature": feature,
						"reason":  "feature_blocked",
					})
					htmx.SetTrigger(w, "subscription:upgrade", string(detail))
				}
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
