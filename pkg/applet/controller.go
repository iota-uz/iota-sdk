package applet

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/sirupsen/logrus"
)

// AppletController is a generic controller for rendering React/Next.js applets.
// It handles:
// - Standard middleware application (Auth, User, Localizer, PageContext)
// - Custom middleware from applet config
// - Context building and injection
// - Static asset serving
type AppletController struct {
	applet  Applet
	builder *ContextBuilder
	logger  *logrus.Logger
}

// NewAppletController creates a new AppletController for the given applet.
//
// Required dependencies:
//   - applet: The applet to render
//   - bundle: i18n bundle for translation loading
//   - sessionConfig: Session expiry and refresh configuration
//   - logger: Structured logger for operations
//   - metrics: Metrics recorder for performance tracking
//
// Optional via BuilderOption:
//   - WithTenantNameResolver: Custom tenant name resolution
//   - WithErrorEnricher: Custom error context enrichment
func NewAppletController(
	applet Applet,
	bundle *i18n.Bundle,
	sessionConfig SessionConfig,
	logger *logrus.Logger,
	metrics MetricsRecorder,
	opts ...BuilderOption,
) *AppletController {
	config := applet.Config()
	builder := NewContextBuilder(config, bundle, sessionConfig, logger, metrics, opts...)

	return &AppletController{
		applet:  applet,
		builder: builder,
		logger:  logger,
	}
}

// RegisterRoutes registers all routes for the applet.
// This includes:
// - Main applet route (renders React app with context injection)
// - Asset serving routes (JS, CSS, images)
func (c *AppletController) RegisterRoutes(router *mux.Router) {
	config := c.applet.Config()

	// Create subrouter for applet
	appletRouter := router.PathPrefix(c.applet.BasePath()).Subrouter()

	// Apply custom middleware if provided
	if config.Middleware != nil {
		for _, middleware := range config.Middleware {
			appletRouter.Use(middleware)
		}
	}

	// Serve static assets
	if config.Assets.FS != nil {
		c.registerAssetRoutes(appletRouter)
	}

	// Main applet route (must be last to act as catch-all)
	appletRouter.PathPrefix("/").HandlerFunc(c.RenderApp)
}

// registerAssetRoutes registers routes for serving static assets (JS, CSS, images)
func (c *AppletController) registerAssetRoutes(router *mux.Router) {
	config := c.applet.Config()

	// Serve assets from embedded FS
	assetsPath := config.Assets.BasePath
	if assetsPath == "" {
		assetsPath = "/assets"
	}

	// Create sub-filesystem for assets
	assetsFS, err := fs.Sub(config.Assets.FS, ".")
	if err != nil {
		if c.logger != nil {
			c.logger.Errorf("Failed to create assets sub-filesystem: %v", err)
		}
		return
	}

	// Strip BasePath prefix and serve files
	router.PathPrefix(assetsPath).Handler(
		http.StripPrefix(assetsPath, http.FileServer(http.FS(assetsFS))),
	)
}

// RenderApp renders the React/Next.js app with server-side context injection.
// Steps:
// 1. Build InitialContext from request context
// 2. Serialize context as JSON
// 3. Inject into HTML as window[Config.WindowGlobal]
// 4. Serve HTML with injected script
func (c *AppletController) RenderApp(w http.ResponseWriter, r *http.Request) {
	const op serrors.Op = "AppletController.RenderApp"
	ctx := r.Context()

	// Build context
	initialContext, err := c.builder.Build(ctx, r)
	if err != nil {
		if c.logger != nil {
			c.logger.Errorf("Failed to build context: %v", err)
		}
		http.Error(w, "Failed to build context", http.StatusInternalServerError)
		return
	}

	// Serialize context as JSON
	contextJSON, err := json.Marshal(initialContext)
	if err != nil {
		if c.logger != nil {
			c.logger.Errorf("Failed to marshal context: %v", err)
		}
		http.Error(w, "Failed to serialize context", http.StatusInternalServerError)
		return
	}

	// Render HTML with context injection
	c.renderHTML(w, contextJSON)
}

// renderHTML renders the HTML page with injected context.
// The context is injected as: window[Config.WindowGlobal] = {...}
func (c *AppletController) renderHTML(w http.ResponseWriter, contextJSON []byte) {
	config := c.applet.Config()

	// Build script tag for context injection
	contextScript := fmt.Sprintf(
		`<script>window.%s = %s;</script>`,
		template.JSEscapeString(config.WindowGlobal),
		contextJSON, // Already valid JSON
	)

	// Build CSS link tag if CSSPath is provided
	cssLink := ""
	if config.Assets.CSSPath != "" {
		cssLink = fmt.Sprintf(`<link rel="stylesheet" href="%s">`, config.Assets.CSSPath)
	}

	// Build HTML page
	// Note: Future enhancement could load this from embedded FS for customization
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s</title>
    %s
</head>
<body>
    <div id="root"></div>
    %s
    <script src="%s/main.js"></script>
</body>
</html>`,
		c.applet.Name(),
		cssLink,
		contextScript,
		config.Assets.BasePath,
	)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}
