package bichat

import (
	"context"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/bichat/presentation/assets"
	"github.com/iota-uz/iota-sdk/pkg/applet"
)

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
	return "/bichat"
}

// Config returns the applet configuration for BiChat.
// This configures how the SDK integrates with the BiChat React application.
func (a *BiChatApplet) Config() applet.Config {
	return applet.Config{
		// WindowGlobal specifies the JavaScript global variable for context injection
		// Creates: window.__BICHAT_CONTEXT__ = { user, tenant, locale, config, ... }
		WindowGlobal: "__BICHAT_CONTEXT__",

		// Endpoints configures API endpoint paths
		Endpoints: applet.EndpointConfig{
			GraphQL: "/bichat/graphql", // GraphQL API endpoint
			Stream:  "/bichat/stream",  // SSE streaming endpoint
		},

		// Assets configuration for serving the built React app
		Assets: applet.AssetConfig{
			FS:       &assets.DistFS,            // Embedded filesystem from presentation/assets/dist/
			BasePath: "/bichat/assets",          // URL prefix for asset serving
			CSSPath:  "/bichat/assets/main.css", // CSS file path (updated when React build available)
		},

		// Router uses MuxRouter to extract route parameters (e.g., /sessions/{id})
		Router: applet.NewMuxRouter(),

		// CustomContext injects BiChat feature flags into InitialContext.Extensions
		CustomContext: a.buildCustomContext,

		// Middleware: Use standard SDK middleware (Auth, User, Localizer, PageContext)
		// BiChat doesn't need custom middleware - relies on SDK defaults
		Middleware: []mux.MiddlewareFunc{},
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
