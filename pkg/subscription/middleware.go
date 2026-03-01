package subscription

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/sirupsen/logrus"
)

func RequireFeature(engine Engine, feature FeatureKey) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			subject, err := tenantSubjectFromContext(r)
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			decision, err := engine.EvaluateFeature(r.Context(), subject, feature)
			if err != nil {
				logrus.WithError(err).WithFields(logrus.Fields{
					"subject_scope": subject.Scope,
					"subject_id":    subject.ID.String(),
					"feature":       feature,
				}).Error("Subscription feature evaluation failed")
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			if !decision.Allowed {
				if htmx.IsHxRequest(r) {
					detail, _ := json.Marshal(map[string]any{
						"feature": feature,
						"reason":  decision.Reason,
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

func RequirePlan(engine Engine, allowedPlans ...string) mux.MiddlewareFunc {
	allowed := make(map[string]struct{}, len(allowedPlans))
	for _, planID := range allowedPlans {
		allowed[planID] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			subject, err := tenantSubjectFromContext(r)
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			plan, err := engine.CurrentPlan(r.Context(), subject)
			if err != nil {
				logrus.WithError(err).WithFields(logrus.Fields{
					"subject_scope": subject.Scope,
					"subject_id":    subject.ID.String(),
				}).Error("Subscription plan evaluation failed")
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			if _, ok := allowed[plan.ID]; !ok {
				if htmx.IsHxRequest(r) {
					detail, _ := json.Marshal(map[string]any{
						"plan_id":       plan.ID,
						"allowed_plans": allowedPlans,
						"reason":        "plan_blocked",
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

func EnforceLimit(engine Engine, entityType string) mux.MiddlewareFunc {
	quota := QuotaKey{
		Resource: entityType,
		Window:   WindowNone,
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			subject, err := tenantSubjectFromContext(r)
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			result, err := engine.EvaluateLimit(r.Context(), subject, quota)
			if err != nil {
				logrus.WithError(err).WithFields(logrus.Fields{
					"subject_scope": subject.Scope,
					"subject_id":    subject.ID.String(),
					"quota":         quota.String(),
				}).Error("Subscription limit evaluation failed")
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			if !result.Allowed {
				if htmx.IsHxRequest(r) {
					detail, _ := json.Marshal(map[string]any{
						"entity_type": entityType,
						"current":     result.Current,
						"limit":       result.Limit,
						"reason":      result.Reason,
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

func tenantSubjectFromContext(r *http.Request) (Subject, error) {
	tenantID, err := composables.UseTenantID(r.Context())
	if err != nil {
		return Subject{}, err
	}
	return Subject{
		Scope: ScopeTenant,
		ID:    tenantID,
	}, nil
}
