package middleware

import (
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/subscription"
)

func RequireFeature(svc subscription.Engine, feature subscription.FeatureKey) mux.MiddlewareFunc {
	return subscription.RequireFeature(svc, feature)
}
