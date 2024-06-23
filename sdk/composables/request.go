// Package composables provides a set of composable functions that can be used to access request parameters in a type-safe way.
// This package is inspired by the React hooks API and aims to provide a similar experience for server side code.
package composables

import (
	"context"
	"golang.org/x/text/language"
	"gorm.io/gorm"
	"log"
	"net/http"
)

type Params struct {
	Ip            string
	UserAgent     string
	Authenticated bool
	Request       *http.Request
	Writer        http.ResponseWriter
	Meta          map[string]interface{}
}

// UseParams returns the request parameters from the context.
// If the parameters are not found, the second return value will be false.
func UseParams(ctx context.Context) (*Params, bool) {
	params, ok := ctx.Value("params").(*Params)
	return params, ok
}

// WithParams returns a new context with the request parameters.
func WithParams(ctx context.Context, params *Params) context.Context {
	return context.WithValue(ctx, "params", params)
}

// UseRequest returns the request from the context.
// If the request is not found, the second return value will be false.
func UseRequest(ctx context.Context) (*http.Request, bool) {
	params, ok := UseParams(ctx)
	if !ok {
		return nil, false
	}
	return params.Request, true
}

// UseLogger returns the logger from the context.
// If the logger is not found, the second return value will be false.
func UseLogger(ctx context.Context) (*log.Logger, bool) {
	logger, ok := ctx.Value("logger").(*log.Logger)
	if !ok {
		return nil, false
	}
	return logger, true
}

// UseMeta returns the metadata from the context.
// If the metadata is not found, the second return value will be false.
func UseMeta(ctx context.Context) (map[string]interface{}, bool) {
	params, ok := UseParams(ctx)
	if !ok {
		return nil, false
	}
	return params.Meta, true
}

// WithTx returns a new context with the database transaction.
func WithTx(ctx context.Context, tx *gorm.DB) context.Context {
	return context.WithValue(ctx, "tx", tx)
}

// UseTx returns the database transaction from the context.
// If the transaction is not found, the second return value will be false.
func UseTx(ctx context.Context) (*gorm.DB, bool) {
	tx, ok := ctx.Value("tx").(*gorm.DB)
	if !ok {
		return nil, false
	}
	return tx, true
}

// UseAuthenticated returns whether the user is authenticated and the second return value is true.
// If the user is not authenticated, the second return value is false.
func UseAuthenticated(ctx context.Context) bool {
	params, ok := UseParams(ctx)
	if !ok {
		return false
	}
	return params.Authenticated
}

// UseIp returns the IP address from the context.
// If the IP address is not found, the second return value will be false.
func UseIp(ctx context.Context) (string, bool) {
	params, ok := UseParams(ctx)
	if !ok {
		return "", false
	}
	return params.Ip, true
}

// UseUserAgent returns the user agent from the context.
// If the user agent is not found, the second return value will be false.
func UseUserAgent(ctx context.Context) (string, bool) {
	params, ok := UseParams(ctx)
	if !ok {
		return "", false
	}
	return params.UserAgent, true
}

// UseWriter returns the response writer from the context.
// If the response writer is not found, the second return value will be false.
func UseWriter(ctx context.Context) (http.ResponseWriter, bool) {
	params, ok := UseParams(ctx)
	if !ok {
		return nil, false
	}
	return params.Writer, true
}

// UseLocale returns the locale from the context.
// If the locale is not found, the second return value will be false.
func UseLocale(ctx context.Context, defaultLocale language.Tag) language.Tag {
	params, ok := UseParams(ctx)
	if !ok {
		return defaultLocale
	}
	headerValue := params.Request.Header.Get("Accept-Language")
	tags, _, err := language.ParseAcceptLanguage(headerValue)
	if err != nil {
		return defaultLocale
	}
	if len(tags) == 0 {
		return defaultLocale
	}
	return tags[0]
}
