package bichat

import "github.com/iota-uz/iota-sdk/pkg/applet"

// BiChatApplet implements the applet.Applet interface for BiChat.
// This enables BiChat to integrate with the SDK's generic applet system,
// providing context injection, routing, and asset serving.
type BiChatApplet struct{}

// Name returns the unique identifier for the BiChat applet.
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

		// Assets: BiChat uses separate asset serving (not embedded FS)
		// Router: Use default router (no custom route parameter extraction)
		// CustomContext: Use standard context (no custom fields)
		// Middleware: Use standard SDK middleware (Auth, User, Localizer, PageContext)
	}
}
