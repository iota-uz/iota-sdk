package bichat

import (
	"context"
	"net/http"
	"os"
	"strings"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-uz/applets"
	"github.com/iota-uz/iota-sdk/components/sidebar"
	"github.com/iota-uz/iota-sdk/modules/bichat/presentation/assets"
	bichatrpc "github.com/iota-uz/iota-sdk/modules/bichat/rpc"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/layouts"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

var distFS = assets.AppletFS()

// BiChatApplet implements the applets.Applet interface for BiChat.
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

// Name returns the unique identifier for the BiChat applets.
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
func (a *BiChatApplet) Config() applets.Config {
	return applets.Config{
		// WindowGlobal specifies the JavaScript global variable for context injection
		// Creates: window.__APPLET_CONTEXT__ = { user, tenant, locale, config, ... }
		WindowGlobal: "__APPLET_CONTEXT__",

		// Endpoints configures API endpoint paths
		Endpoints: applets.EndpointConfig{
			Stream: "/bi-chat/stream", // SSE streaming endpoint
		},

		// Assets configuration for serving the built React app
		// Uses Vite manifest for hashed asset resolution
		Assets: applets.AssetConfig{
			FS:           distFS,                // Sub-filesystem rooted at dist/ for direct file access
			BasePath:     "/assets",             // URL prefix for asset serving (relative to applet base path)
			ManifestPath: ".vite/manifest.json", // Vite with manifest: true writes to dist/.vite/manifest.json
			Entrypoint:   "index.html",          // Entry point file name (Vite default, matches manifest key)
			Dev:          bichatDevAssets(),
		},

		Mount: applets.MountConfig{
			Tag: "bi-chat-root",
			Attributes: map[string]string{
				"base-path":   a.BasePath(),
				"router-mode": "url",
				"shell-mode":  "embedded",
				"style":       "display: flex; flex: 1; flex-direction: column; min-height: 0; height: 100%; width: 100%;",
			},
		},

		// Router uses MuxRouter to extract route parameters (e.g., /sessions/{id})
		Router: applets.NewMuxRouter(),

		// CustomContext injects BiChat feature flags into InitialContext.Extensions
		CustomContext: a.buildCustomContext,

		// Middleware: Required middleware stack for authenticated applet
		// Order matters: Authorize -> User -> Localizer -> PageContext
		Middleware: a.getMiddleware(),

		Shell: applets.ShellConfig{
			Mode: applets.ShellModeEmbedded,
			Layout: func(title string) templ.Component {
				return layouts.Authenticated(layouts.AuthenticatedProps{
					BaseProps: layouts.BaseProps{Title: title},
				})
			},
			Title: "BiChat",
		},

		RPC: func() *applets.RPCConfig {
			if a.config == nil {
				return nil
			}
			chatSvc := a.config.ChatService()
			artifactSvc := a.config.ArtifactService()
			if chatSvc == nil || artifactSvc == nil {
				return nil
			}
			cfg := bichatrpc.Router(chatSvc, artifactSvc).Config()
			// Expose internal error details in development mode.
			if configuration.Use().IsDev() {
				t := true
				cfg.ExposeInternalErrors = &t
			}
			return cfg
		}(),
	}
}

func bichatDevAssets() *applets.DevAssetConfig {
	enabled := configuration.Use().IsDev()
	target := strings.TrimSpace(os.Getenv("IOTA_APPLET_VITE_URL_BICHAT"))
	if target == "" {
		target = "http://localhost:5173"
	}
	entry := strings.TrimSpace(os.Getenv("IOTA_APPLET_ENTRY_BICHAT"))
	if entry == "" {
		entry = "/src/main.tsx"
	}
	client := strings.TrimSpace(os.Getenv("IOTA_APPLET_CLIENT_BICHAT"))
	if client == "" {
		client = "/@vite/client"
	}

	return &applets.DevAssetConfig{
		Enabled:      enabled,
		TargetURL:    target,
		EntryModule:  entry,
		ClientModule: client,
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
		middleware.Authorize(),                                        // Check authentication token
		middleware.RedirectNotAuthenticated(),                         // Redirect to /login if not authenticated
		middleware.ProvideUser(),                                      // Add user to context from session
		a.provideLocalizerFromContext(),                               // Add localizer using app from context
		middleware.NavItemsWithInitialState(sidebar.SidebarCollapsed), // Ensure iota sidebar is visible + collapsed by default
		middleware.WithPageContext(),                                  // Add page context (locale, etc.)
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
				configuration.Use().Logger().
					WithError(err).
					Error("BiChat applet localizer middleware missing app in request context")
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
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
//	  },
//	  "debug": {
//	    "limits": {
//	      "policyMaxTokens": 180000,
//	      "modelMaxTokens": 272000,
//	      "effectiveMaxTokens": 180000,
//	      "completionReserveTokens": 8000
//	    }
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
			"llm": map[string]interface{}{
				"provider":         "",
				"apiKeyConfigured": false,
			},
			"debug": map[string]interface{}{
				"limits": map[string]int{
					"policyMaxTokens":         0,
					"modelMaxTokens":          0,
					"effectiveMaxTokens":      0,
					"completionReserveTokens": 0,
				},
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

	policyMax := a.config.ContextPolicy.ContextWindow
	modelMax := 0
	if a.config.Model != nil {
		modelMax = a.config.Model.Info().ContextWindow
	}

	effectiveMax := 0
	switch {
	case policyMax > 0 && modelMax > 0:
		if policyMax < modelMax {
			effectiveMax = policyMax
		} else {
			effectiveMax = modelMax
		}
	case policyMax > 0:
		effectiveMax = policyMax
	case modelMax > 0:
		effectiveMax = modelMax
	}

	limits := map[string]int{
		"policyMaxTokens":         policyMax,
		"modelMaxTokens":          modelMax,
		"effectiveMaxTokens":      effectiveMax,
		"completionReserveTokens": a.config.ContextPolicy.CompletionReserve,
	}

	modelProvider := ""
	if a.config.Model != nil {
		modelProvider = strings.ToLower(strings.TrimSpace(a.config.Model.Info().Provider))
	}
	apiKeyConfigured := true
	if modelProvider == "openai" {
		apiKeyConfigured = strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) != ""
	}

	return map[string]interface{}{
		"features": features,
		"llm": map[string]interface{}{
			"provider":         modelProvider,
			"apiKeyConfigured": apiKeyConfigured,
		},
		"debug": map[string]interface{}{
			"limits": limits,
		},
	}, nil
}
