package middleware

import (
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/subscription"
)

func RequirePlan(svc subscription.Engine, allowedPlans ...string) mux.MiddlewareFunc {
	return subscription.RequirePlan(svc, allowedPlans...)
}
