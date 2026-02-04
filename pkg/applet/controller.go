package applet

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/sirupsen/logrus"
)

var htmlNameRe = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

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

// Register implements the application.Controller interface.
// This is the standard method for registering controllers in the application.
func (c *AppletController) Register(router *mux.Router) {
	c.RegisterRoutes(router)
}

// Key implements the application.Controller interface.
// Returns a unique key for this applet controller.
func (c *AppletController) Key() string {
	return "applet_" + c.applet.Name()
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
	appletRouter.PathPrefix("/").HandlerFunc(c.RenderApp).Methods("GET", "HEAD") // Catch-all for sub-paths

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
	ctx := r.Context()

	if c.logger != nil {
		c.logger.Infof("AppletController.RenderApp called for path: %s", r.URL.Path)
	}

	// Build context
	initialContext, err := c.builder.Build(ctx, r, c.applet.BasePath())
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
	c.render(ctx, w, r, contextJSON)
}

// render renders either a standalone HTML page or an applet wrapped in a templ layout.
// The context is injected as: window[Config.WindowGlobal] = {...}
func (c *AppletController) render(ctx context.Context, w http.ResponseWriter, r *http.Request, contextJSON []byte) {
	config := c.applet.Config()

	// Compute full URL paths by combining applet base path with relative asset paths.
	assetsPath := config.Assets.BasePath
	if assetsPath == "" {
		assetsPath = "/assets"
	}
	assetsBasePath := c.applet.BasePath() + assetsPath

	// Build script tag for context injection
	// Use safe JSON injection to prevent script tag breakouts
	contextScript, err := c.buildSafeContextScript(config.WindowGlobal, contextJSON)
	if err != nil {
		if c.logger != nil {
			c.logger.WithError(err).Error("Failed to build context script")
		}
		http.Error(w, "Failed to inject context", http.StatusInternalServerError)
		return
	}

	// Resolve assets from manifest (required)
	var cssLinks, jsScripts string
	if config.Assets.ManifestPath == "" || config.Assets.Entrypoint == "" {
		if c.logger != nil {
			c.logger.Error("Applet asset configuration missing ManifestPath or Entrypoint - manifest-based resolution is required")
		}
		http.Error(w, "Applet asset configuration invalid", http.StatusInternalServerError)
		return
	}

	resolved, err := c.resolveManifestAssets(config, assetsBasePath)
	if err != nil {
		if c.logger != nil {
			c.logger.Errorf("Failed to resolve manifest assets: %v", err)
		}
		http.Error(w, "Failed to resolve applet assets", http.StatusInternalServerError)
		return
	}

	cssLinks = c.buildCSSLinks(resolved.CSSFiles)
	jsScripts = c.buildJSScripts(resolved.JSFiles)

	// Build mount element
	mountHTML := c.buildMountElement(config)

	// Prefer rendering inside an existing layout when configured.
	if config.Layout != nil {
		title := strings.TrimSpace(config.Title)
		if title == "" {
			title = c.applet.Name()
		}

		// Merge applet CSS into the existing head component (if present).
		if cssLinks != "" {
			if existingHead, ok := ctx.Value(constants.HeadKey).(templ.Component); ok && existingHead != nil {
				mergedHead := templ.ComponentFunc(func(headCtx context.Context, wr io.Writer) error {
					if err := existingHead.Render(headCtx, wr); err != nil {
						return err
					}
					return templ.Raw(cssLinks).Render(headCtx, wr)
				})
				ctx = context.WithValue(ctx, constants.HeadKey, mergedHead)
			}
		}

		shell := templ.ComponentFunc(func(shellCtx context.Context, wr io.Writer) error {
			if _, err := io.WriteString(wr, mountHTML); err != nil {
				return err
			}
			if _, err := io.WriteString(wr, contextScript); err != nil {
				return err
			}
			if _, err := io.WriteString(wr, jsScripts); err != nil {
				return err
			}
			return nil
		})

		ctx = templ.WithChildren(ctx, shell)
		layout := config.Layout(title)
		templ.Handler(layout, templ.WithStreaming()).ServeHTTP(w, r.WithContext(ctx))
		return
	}

	// Standalone HTML page (no iota layout).
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s</title>
    %s
</head>
<body>
    %s
    %s
    %s
</body>
</html>`,
		c.applet.Name(),
		cssLinks,
		mountHTML,
		contextScript,
		jsScripts,
	)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(html))
}

func (c *AppletController) buildMountElement(config Config) string {
	tag := strings.TrimSpace(config.Mount.Tag)
	id := strings.TrimSpace(config.Mount.ID)
	attrs := config.Mount.Attributes

	// Defaults
	if tag == "" {
		tag = "div"
	}
	if !htmlNameRe.MatchString(tag) {
		tag = "div"
		id = "root"
		attrs = nil
	}
	if id == "" && tag == "div" {
		id = "root"
	}

	var b strings.Builder
	b.WriteString("<")
	b.WriteString(template.HTMLEscapeString(tag))

	if id != "" {
		b.WriteString(` id="`)
		b.WriteString(template.HTMLEscapeString(id))
		b.WriteString(`"`)
	}

	for k, v := range attrs {
		k = strings.TrimSpace(k)
		if k == "" || !htmlNameRe.MatchString(k) {
			continue
		}
		b.WriteString(" ")
		b.WriteString(template.HTMLEscapeString(k))
		b.WriteString(`="`)
		b.WriteString(template.HTMLEscapeString(v))
		b.WriteString(`"`)
	}

	// BiChat and similar applets commonly rely on flex layout.
	// Keep this minimal; applets can override via attributes/styles if needed.
	// No default styling when tag != "div".

	b.WriteString("></")
	b.WriteString(template.HTMLEscapeString(tag))
	b.WriteString(">")
	return b.String()
}

// resolveManifestAssets resolves assets from a Vite manifest
func (c *AppletController) resolveManifestAssets(config Config, assetsBasePath string) (*ResolvedAssets, error) {
	manifest, err := loadManifest(config.Assets.FS, config.Assets.ManifestPath)
	if err != nil {
		return nil, err
	}

	return resolveAssetsFromManifest(manifest, config.Assets.Entrypoint, assetsBasePath)
}

// buildCSSLinks builds HTML link tags for CSS files
func (c *AppletController) buildCSSLinks(cssFiles []string) string {
	if len(cssFiles) == 0 {
		return ""
	}

	var links string
	for _, cssFile := range cssFiles {
		links += fmt.Sprintf(`<link rel="stylesheet" href="%s">`, cssFile)
	}
	return links
}

// buildJSScripts builds HTML script tags for JS files
func (c *AppletController) buildJSScripts(jsFiles []string) string {
	if len(jsFiles) == 0 {
		return ""
	}

	var scripts string
	for _, jsFile := range jsFiles {
		scripts += fmt.Sprintf(`<script type="module" src="%s"></script>`, jsFile)
	}
	return scripts
}

// buildSafeContextScript builds a safe script tag for context injection.
// Escapes sequences that could break out of the script tag (e.g., </script>).
// Uses JSON encoding with HTML-safe escaping.
func (c *AppletController) buildSafeContextScript(windowGlobal string, contextJSON []byte) (string, error) {
	// Use bracket notation to avoid requiring WindowGlobal to be a JS identifier.
	// Also escape any </script>-like sequences in both key and value.
	keyJSON, err := json.Marshal(windowGlobal)
	if err != nil {
		return "", fmt.Errorf("failed to marshal window global key: %w", err)
	}
	safeKey := escapeJSONForScriptTag(keyJSON)
	safeValue := escapeJSONForScriptTag(contextJSON)

	return fmt.Sprintf(`<script>window[%s] = %s;</script>`, safeKey, safeValue), nil
}

// escapeJSONForScriptTag escapes JSON to prevent script tag breakouts.
// This is a defense-in-depth measure - JSON should already be safe, but we
// escape potentially dangerous sequences anyway.
// Only escapes </script> sequences (case-insensitive) to prevent script tag termination.
// We don't escape all `<` characters as that would break valid JSON.
func escapeJSONForScriptTag(jsonBytes []byte) string {
	jsonStr := string(jsonBytes)
	// Replace </script> with <\/script> to prevent script tag termination
	// This is safe because \/ is valid in JSON (escaped forward slash)
	// We handle case-insensitive matching for defense-in-depth
	jsonStr = strings.ReplaceAll(jsonStr, "</script>", "<\\/script>")
	jsonStr = strings.ReplaceAll(jsonStr, "</SCRIPT>", "<\\/SCRIPT>")
	jsonStr = strings.ReplaceAll(jsonStr, "</Script>", "<\\/Script>")
	jsonStr = strings.ReplaceAll(jsonStr, "</sCrIpT>", "<\\/sCrIpT>")
	return jsonStr
}
