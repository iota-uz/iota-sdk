package applet

import (
	"context"
	"embed"

	"github.com/gorilla/mux"
)

// Applet represents a React/Next.js application that integrates with the iota-sdk runtime.
// Applets are self-contained web applications that receive server-side context injection,
// authentication, and routing support from the SDK.
type Applet interface {
	// Name returns the unique identifier for this applet (e.g., "bichat", "analytics")
	Name() string

	// BasePath returns the URL path where this applet is mounted (e.g., "/bichat", "/analytics")
	BasePath() string

	// Config returns the applet's configuration
	Config() Config
}

// Config contains all configuration needed to integrate an applet with the SDK runtime.
type Config struct {
	// WindowGlobal is the JavaScript global variable name for context injection
	// Example: "__BICHAT_CONTEXT__" creates window.__BICHAT_CONTEXT__ = {...}
	WindowGlobal string

	// Endpoints contains URL paths for applet API endpoints
	Endpoints EndpointConfig

	// Assets contains embedded files and serving configuration
	Assets AssetConfig

	// Router is an optional custom router for parsing URL paths into RouteContext.
	// If nil, uses default implementation (path = full path after BasePath, params = empty)
	Router AppletRouter

	// CustomContext is an optional function to add custom fields to InitialContext.Custom.
	// Example: add tenant branding, feature flags, or analytics config.
	// If nil, InitialContext.Custom will be nil.
	CustomContext ContextExtender

	// Middleware is an optional list of custom middleware functions to apply to applet routes.
	// These are applied AFTER standard SDK middleware (Auth, User, Localizer, PageContext).
	// If nil, no custom middleware is applied.
	Middleware []mux.MiddlewareFunc
}

// EndpointConfig contains URL paths for applet API endpoints
type EndpointConfig struct {
	GraphQL string // Optional: GraphQL endpoint path, e.g., "/bichat/graphql"
	Stream  string // Optional: SSE streaming endpoint path, e.g., "/bichat/stream"
	REST    string // Optional: REST API base path, e.g., "/bichat/api"
}

// AssetConfig contains configuration for serving applet static assets (JS, CSS, images)
type AssetConfig struct {
	// FS is the embedded filesystem containing applet assets
	// Example: //go:embed dist/* from React build output
	FS *embed.FS

	// BasePath is the URL path prefix for serving assets
	// Example: "/bichat/assets" serves files from FS at /bichat/assets/*
	BasePath string

	// CSSPath is the URL path to the compiled CSS file (optional)
	// Example: "/bichat/assets/styles.css"
	CSSPath string
}

// ContextExtender is a function that adds custom fields to InitialContext.Custom.
// It receives the request context and returns a map of custom fields.
// Return nil or empty map if no custom fields are needed.
//
// Example:
//
//	func(ctx context.Context) (map[string]interface{}, error) {
//	    tenantID := composables.UseTenantID(ctx)
//	    branding := fetchTenantBranding(ctx, tenantID)
//	    return map[string]interface{}{
//	        "branding": branding,
//	        "features": getFeatureFlags(ctx),
//	    }, nil
//	}
type ContextExtender func(ctx context.Context) (map[string]interface{}, error)
