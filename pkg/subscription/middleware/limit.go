package middleware

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/subscription"
)

func EnforceLimit(svc subscription.EntitlementService, entityType string) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID, err := composables.UseTenantID(r.Context())
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			result, err := svc.CheckLimit(r.Context(), tenantID, entityType)
			if err != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			if !result.Allowed {
				if htmx.IsHxRequest(r) {
					detail, _ := json.Marshal(map[string]any{
						"entity_type": entityType,
						"current":     result.Current,
						"limit":       result.Limit,
						"reason":      "limit_exceeded",
					})
					htmx.SetTrigger(w, "subscription:limit-exceeded", string(detail))
				}
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
