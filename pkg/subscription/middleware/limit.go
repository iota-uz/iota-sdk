package middleware

import (
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/subscription"
)

func EnforceLimit(svc subscription.Engine, entityType string) mux.MiddlewareFunc {
	return subscription.EnforceLimit(svc, entityType)
}
