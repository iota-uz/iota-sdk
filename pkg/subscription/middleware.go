package subscription

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
)

func RequireFeature(svc EntitlementService, feature string) mux.MiddlewareFunc {
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

func RequireTier(svc EntitlementService, allowedTiers ...string) mux.MiddlewareFunc {
	allowed := make(map[string]struct{}, len(allowedTiers))
	for _, tier := range allowedTiers {
		allowed[tier] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID, err := composables.UseTenantID(r.Context())
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			tierInfo, err := svc.GetTier(r.Context(), tenantID)
			if err != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			if _, ok := allowed[tierInfo.Tier]; !ok {
				if htmx.IsHxRequest(r) {
					detail, _ := json.Marshal(map[string]any{
						"tier":          tierInfo.Tier,
						"allowed_tiers": allowedTiers,
						"reason":        "tier_blocked",
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

func EnforceLimit(svc EntitlementService, entityType string) mux.MiddlewareFunc {
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
