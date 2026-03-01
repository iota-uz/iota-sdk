package middleware

import (
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/subscription"
)

func RequireTier(svc subscription.EntitlementService, allowedTiers ...string) mux.MiddlewareFunc {
	return subscription.RequireTier(svc, allowedTiers...)
}
