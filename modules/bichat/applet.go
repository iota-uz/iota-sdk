package bichat

import (
	"context"
	"io/fs"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/bichat/presentation/assets"
	"github.com/iota-uz/iota-sdk/pkg/applet"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

// distFS is a sub-filesystem of DistFS rooted at "dist/".
// This allows the AppletController to access files directly (e.g., "main.js" instead of "dist/main.js").
var distFS fs.FS

func init() {
	var err error
	distFS, err = fs.Sub(assets.DistFS, "dist")
	if err != nil {
		panic("failed to create distFS sub-filesystem: " + err.Error())
	}
}

// BiChatApplet implements the applet.Applet interface for BiChat.
// This enables BiChat to integrate with the SDK's generic applet system,
// providing context injection, routing, and asset serving.
type BiChatApplet struct {
	config *ModuleConfig
}

// NewBiChatApplet creates a new BiChatApplet instance.
// The config is optional and can be set later via SetConfig.
func NewBiChatApplet(config *ModuleConfig) *BiChatApplet {
	return &BiChatApplet{
		config: config,
	}
}

// SetConfig updates the applet configuration.
// This allows the applet to be created before the full module configuration is available.
func (a *BiChatApplet) SetConfig(config *ModuleConfig) {
	a.config = config
}

// Name returns the unique identifier for the BiChat applet.
// Using lowercase to match existing convention.
func (a *BiChatApplet) Name() string {
	return "bichat"
}

// BasePath returns the URL path where BiChat is mounted.
func (a *BiChatApplet) BasePath() string {
	return "/bi-chat"
}

// Config returns the applet configuration for BiChat.
// This configures how the SDK integrates with the BiChat React application.
//
// Note: This requires the application to be available in the context.
// The middleware uses composables.UseApp() which depends on prior middleware setup.
func (a *BiChatApplet) Config() applet.Config {
	return applet.Config{
		// WindowGlobal specifies the JavaScript global variable for context injection
		// Creates: window.__BICHAT_CONTEXT__ = { user, tenant, locale, config, ... }
		WindowGlobal: "__BICHAT_CONTEXT__",

		// Endpoints configures API endpoint paths
		Endpoints: applet.EndpointConfig{
			GraphQL: "/bi-chat/graphql", // GraphQL API endpoint
			Stream:  "/bi-chat/stream",  // SSE streaming endpoint
		},

		// Assets configuration for serving the built React app
		Assets: applet.AssetConfig{
			FS:       distFS,             // Sub-filesystem rooted at dist/ for direct file access
			BasePath: "/assets",          // URL prefix for asset serving (relative to applet base path)
			CSSPath:  "/assets/main.css", // CSS file path (relative to applet base path)
		},

		// Router uses MuxRouter to extract route parameters (e.g., /sessions/{id})
		Router: applet.NewMuxRouter(),

		// CustomContext injects BiChat feature flags into InitialContext.Extensions
		CustomContext: a.buildCustomContext,

		// Middleware: Required middleware stack for authenticated applet
		// Order matters: Authorize -> User -> Localizer -> PageContext
		Middleware: a.getMiddleware(),
	}
}

// getMiddleware returns the required middleware stack for BiChat.
// This creates the same middleware stack as the old WebController.
//
// Prerequisites:
//   - The global middleware must have already added the app to context via:
//     middleware.Provide(constants.AppKey, app)
//   - This is set up in internal/server/default.go
func (a *BiChatApplet) getMiddleware() []mux.MiddlewareFunc {
	return []mux.MiddlewareFunc{
		middleware.Authorize(),                // Check authentication token
		middleware.RedirectNotAuthenticated(), // Redirect to /login if not authenticated
		middleware.ProvideUser(),              // Add user to context from session
		a.provideLocalizerFromContext(),       // Add localizer using app from context
		middleware.WithPageContext(),          // Add page context (locale, etc.)
	}
}

// provideLocalizerFromContext creates a middleware that extracts the app from context
// and uses it to provide the localizer. This works around the issue that the applet
// config doesn't have direct access to the app instance, but the global middleware
// has already added it to the request context.
func (a *BiChatApplet) provideLocalizerFromContext() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get app from context (added by global middleware)
			app, err := application.UseApp(r.Context())
			if err != nil {
				panic("app not found in context - ensure middleware.Provide(constants.AppKey, app) runs first")
			}

			// Create the ProvideLocalizer middleware dynamically
			localizerMiddleware := middleware.ProvideLocalizer(app)

			// Apply it and continue
			localizerMiddleware(next).ServeHTTP(w, r)
		})
	}
}

// buildCustomContext creates custom context fields for the BiChat React app.
// This passes feature flags from ModuleConfig to the frontend via InitialContext.Extensions.
//
// Extensions structure:
//
//	{
//	  "features": {
//	    "vision": true,
//	    "webSearch": false,
//	    "codeInterpreter": true,
//	    "multiAgent": false
//	  }
//	}
func (a *BiChatApplet) buildCustomContext(ctx context.Context) (map[string]interface{}, error) {
	// If no config provided, return minimal defaults
	if a.config == nil {
		return map[string]interface{}{
			"features": map[string]bool{
				"vision":          false,
				"webSearch":       false,
				"codeInterpreter": false,
				"multiAgent":      false,
			},
		}, nil
	}

	// Extract feature flags from config
	features := map[string]bool{
		"vision":          a.config.EnableVision,
		"webSearch":       a.config.EnableWebSearch,
		"codeInterpreter": a.config.EnableCodeInterpreter,
		"multiAgent":      a.config.EnableMultiAgent,
	}

	return map[string]interface{}{
		"features": features,
	}, nil
}
