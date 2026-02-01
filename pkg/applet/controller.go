package applet

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"

	"github.com/gorilla/mux"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/sirupsen/logrus"
)

var mimeTypes = map[string]string{
	".js":    "application/javascript; charset=utf-8",
	".css":   "text/css; charset=utf-8",
	".json":  "application/json; charset=utf-8",
	".svg":   "image/svg+xml",
	".png":   "image/png",
	".jpg":   "image/jpeg",
	".jpeg":  "image/jpeg",
	".gif":   "image/gif",
	".woff":  "font/woff",
	".woff2": "font/woff2",
}

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

	if c.logger != nil {
		c.logger.Infof("AppletController.RegisterRoutes: Registering routes for applet at BasePath: %s", c.applet.BasePath())
	}

	// Serve static assets on parent router (without middleware) so they can be
	// loaded without authentication. This must be done BEFORE the subrouter
	// is created, otherwise the subrouter's middleware would apply.
	if config.Assets.FS != nil {
		c.registerAssetRoutes(router)
	}

	// Create subrouter for applet (main app routes with middleware)
	appletRouter := router.PathPrefix(c.applet.BasePath()).Subrouter()

	// Apply custom middleware if provided
	if config.Middleware != nil {
		for _, middleware := range config.Middleware {
			appletRouter.Use(middleware)
		}
	}

	// Main applet routes
	// Register both root path variants to handle with/without trailing slash
	appletRouter.HandleFunc("", c.RenderApp).Methods("GET")
	appletRouter.HandleFunc("/", c.RenderApp).Methods("GET")
	appletRouter.PathPrefix("/").HandlerFunc(c.RenderApp) // Catch-all for sub-paths

	if c.logger != nil {
		c.logger.Infof("AppletController.RegisterRoutes: Successfully registered routes for %s", c.applet.BasePath())
	}
}

// registerAssetRoutes registers routes for serving static assets (JS, CSS, images)
// Assets are served from the parent router (without middleware) so they load without auth.
func (c *AppletController) registerAssetRoutes(router *mux.Router) {
	config := c.applet.Config()

	// Serve assets from embedded FS
	assetsBasePath := config.Assets.BasePath
	if assetsBasePath == "" {
		assetsBasePath = "/assets"
	}

	// Full path includes applet base path (e.g., /bi-chat/assets)
	fullAssetsPath := c.applet.BasePath() + assetsBasePath

	fileServer := http.FileServer(http.FS(config.Assets.FS))

	// Wrap with explicit MIME type handling
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set Content-Type based on file extension
		if mimeType, ok := mimeTypes[filepath.Ext(r.URL.Path)]; ok {
			w.Header().Set("Content-Type", mimeType)
		}
		fileServer.ServeHTTP(w, r)
	})

	router.PathPrefix(fullAssetsPath).Handler(
		http.StripPrefix(fullAssetsPath, handler),
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

	if c.logger != nil {
		c.logger.Infof("AppletController.RenderApp called for path: %s", r.URL.Path)
	}

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

	// Compute full URL paths by combining applet base path with relative asset paths.
	// Config paths are relative to the applet (e.g., "/assets"), but HTML needs
	// absolute paths (e.g., "/bi-chat/assets").
	assetsURL := c.applet.BasePath() + config.Assets.BasePath

	// Build script tag for context injection
	contextScript := fmt.Sprintf(
		`<script>window.%s = %s;</script>`,
		template.JSEscapeString(config.WindowGlobal),
		contextJSON, // Already valid JSON
	)

	// Build CSS link tag if CSSPath is provided
	cssLink := ""
	if config.Assets.CSSPath != "" {
		cssURL := c.applet.BasePath() + config.Assets.CSSPath
		cssLink = fmt.Sprintf(`<link rel="stylesheet" href="%s">`, cssURL)
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
    <script type="module" src="%s/main.js"></script>
</body>
</html>`,
		c.applet.Name(),
		cssLink,
		contextScript,
		assetsURL,
	)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}
