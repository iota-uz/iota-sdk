package applet

import (
	"context"
	"io/fs"

	"github.com/a-h/templ"
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

	// Layout, when set, renders the applet inside an existing templ layout.
	// The controller will pass the applet shell as `children` to the returned component.
	// When nil, the controller renders a standalone HTML document.
	Layout LayoutFactory

	// Title is used as the page title when Layout is set.
	// When empty, applet.Name() is used.
	Title string

	// Mount controls what root element is rendered into the applet HTML shell.
	// Default is <div id="root"></div>.
	Mount MountConfig
}

// LayoutFactory produces a layout component for an applet request.
// The returned component is expected to render `{ children... }` somewhere.
type LayoutFactory func(title string) templ.Component

// MountConfig describes the DOM element that the frontend app mounts into.
// This prevents "blank screen" failures when different applets expect different roots
// (e.g., a custom element vs a #root div).
type MountConfig struct {
	// Tag is the HTML tag name to render.
	// Examples: "div" (default), "bi-chat-root".
	Tag string

	// ID is rendered as the element id attribute when non-empty.
	// Default: "root" (only when Tag is empty or "div").
	ID string

	// Attributes are rendered as HTML attributes on the mount element.
	// Example: {"base-path": "/bi-chat"}.
	Attributes map[string]string
}

// EndpointConfig contains URL paths for applet API endpoints
type EndpointConfig struct {
	GraphQL string // Optional: GraphQL endpoint path, e.g., "/bichat/graphql"
	Stream  string // Optional: SSE streaming endpoint path, e.g., "/bichat/stream"
	REST    string // Optional: REST API base path, e.g., "/bichat/api"
}

// AssetConfig contains configuration for serving applet static assets (JS, CSS, images)
type AssetConfig struct {
	// FS is the filesystem containing applet assets.
	// Can be an embedded FS (*embed.FS) or a sub-filesystem (fs.Sub result).
	// Example: fs.Sub(embedFS, "dist") to serve files from dist/ subdirectory
	FS fs.FS

	// BasePath is the URL path prefix for serving assets
	// Example: "/bichat/assets" serves files from FS at /bichat/assets/*
	BasePath string

	// ManifestPath is the path to the Vite manifest.json file within FS (optional)
	// If provided, assets will be resolved from the manifest instead of using fixed names.
	// Example: ".vite/manifest.json" or "dist/.vite/manifest.json"
	// When set, Entrypoint must also be set to specify the entry file name.
	ManifestPath string

	// Entrypoint is the entry file name used to look up assets in the manifest (optional)
	// Example: "src/main.tsx" or "index.html"
	// This should match the entry point configured in vite.config.ts
	// Required if ManifestPath is set.
	Entrypoint string
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
