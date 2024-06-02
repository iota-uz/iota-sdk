// Package composables provides a set of composable functions that can be used to access request parameters in a type-safe way.
// This package is inspired by the React hooks API and aims to provide a similar experience for server side code.
package composables

import (
	"context"
	"github.com/iota-agency/iota-erp/sdk/middleware"
	"gorm.io/gorm"
	"net/http"
)

// UseParams returns the request parameters from the context.
// If the parameters are not found, the second return value will be false.
func UseParams[U any, S any](ctx context.Context) (*middleware.RequestParams[U, S], bool) {
	params, ok := ctx.Value("params").(*middleware.RequestParams[U, S])
	return params, ok
}

// UseTx returns the database transaction from the context.
// If the transaction is not found, the second return value will be false.
func UseTx(ctx context.Context) (*gorm.DB, bool) {
	params, ok := UseParams[any, any](ctx)
	if !ok {
		return nil, false
	}
	return params.Tx, true
}

// UseUser returns the user from the context.
// If the user is not found, the second return value will be false.
func UseUser[U any](ctx context.Context) (*U, bool) {
	params, ok := UseParams[*U, any](ctx)
	if !ok {
		return nil, false
	}
	return params.User, true
}

// UseSession returns the session from the context.
// If the session is not found, the second return value will be false.
func UseSession[S any](ctx context.Context) (*S, bool) {
	params, ok := UseParams[any, *S](ctx)
	if !ok {
		return nil, false
	}
	return params.Session, true
}

// UseAuthenticated returns whether the user is authenticated and the second return value is true.
// If the user is not authenticated, the second return value is false.
func UseAuthenticated(ctx context.Context) (bool, bool) {
	params, ok := UseParams[any, any](ctx)
	if !ok {
		return false, false
	}
	return params.Authenticated, true
}

// UseIp returns the IP address from the context.
// If the IP address is not found, the second return value will be false.
func UseIp(ctx context.Context) (string, bool) {
	params, ok := UseParams[any, any](ctx)
	if !ok {
		return "", false
	}
	return params.Ip, true
}

// UseUserAgent returns the user agent from the context.
// If the user agent is not found, the second return value will be false.
func UseUserAgent(ctx context.Context) (string, bool) {
	params, ok := UseParams[any, any](ctx)
	if !ok {
		return "", false
	}
	return params.UserAgent, true
}

// UseWriter returns the response writer from the context.
// If the response writer is not found, the second return value will be false.
func UseWriter(ctx context.Context) (http.ResponseWriter, bool) {
	params, ok := UseParams[any, any](ctx)
	if !ok {
		return nil, false
	}
	return params.Writer, true
}
