package applet

import (
	"net/http"
	"strings"
)

// AppletRouter is an interface for parsing URL paths into RouteContext.
// Applets can provide custom implementations to extract route parameters.
type AppletRouter interface {
	// ParseRoute extracts route parameters from the URL path.
	// The path parameter is the full path after the applet's BasePath.
	// Example: for BasePath="/bichat" and URL="/bichat/sessions/123",
	// path="/sessions/123"
	ParseRoute(r *http.Request, basePath string) RouteContext
}

// DefaultRouter is the default implementation of AppletRouter.
// It returns the full path after BasePath with no parameter extraction.
type DefaultRouter struct{}

// ParseRoute implements AppletRouter for DefaultRouter.
// Returns RouteContext with:
// - Path: full path after BasePath
// - Params: empty map (no extraction)
// - Query: parsed URL query parameters
func (d *DefaultRouter) ParseRoute(r *http.Request, basePath string) RouteContext {
	// Remove basePath prefix from URL path
	fullPath := r.URL.Path
	routePath := strings.TrimPrefix(fullPath, basePath)
	if routePath == "" {
		routePath = "/"
	}

	// Parse query parameters
	queryParams := make(map[string]string)
	for key, values := range r.URL.Query() {
		if len(values) > 0 {
			queryParams[key] = values[0] // Take first value only
		}
	}

	return RouteContext{
		Path:   routePath,
		Params: make(map[string]string), // Empty - no parameter extraction
		Query:  queryParams,
	}
}

// NewDefaultRouter creates a new DefaultRouter instance
func NewDefaultRouter() *DefaultRouter {
	return &DefaultRouter{}
}
