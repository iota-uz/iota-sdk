package controllers

import (
	"encoding/json"
	"html/template"
	"net/http"

	"github.com/gorilla/csrf"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/applet"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// WebController handles React SPA rendering with server context injection.
// This controller is responsible for serving the BiChat React application
// and injecting server-side context (user, tenant, locale, config) into
// the client application via window.__BICHAT_CONTEXT__.
type WebController struct {
	app application.Application
}

// NewWebController creates a new web controller instance.
// The controller requires the application instance to access
// middleware and configuration.
func NewWebController(app application.Application) *WebController {
	return &WebController{
		app: app,
	}
}

// Key returns the unique controller identifier for dependency injection.
func (c *WebController) Key() string {
	return "bichat.WebController"
}

// Register configures HTTP routes and middleware for the web controller.
// The controller applies standard authentication and localization middleware
// to ensure the React app receives proper context.
func (c *WebController) Register(r *mux.Router) {
	commonMiddleware := []mux.MiddlewareFunc{
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.ProvideDynamicLogo(c.app),
		middleware.ProvideLocalizer(c.app),
		middleware.NavItems(),
		middleware.WithPageContext(),
	}

	subRouter := r.PathPrefix("/bichat").Subrouter()
	subRouter.Use(commonMiddleware...)

	// React app routes - both root and trailing slash
	subRouter.HandleFunc("", c.RenderChatApp).Methods("GET")
	subRouter.HandleFunc("/", c.RenderChatApp).Methods("GET")
	subRouter.HandleFunc("/app", c.RenderChatApp).Methods("GET")
}

// RenderChatApp renders the React SPA with injected server context.
//
// This handler performs the following steps:
//  1. Extracts user, tenant, and locale information from request context
//  2. Builds InitialContext object with all necessary data for React
//  3. Serializes context to JSON and injects into HTML template
//  4. Includes CSRF token for secure form submissions
//
// The rendered HTML includes:
//   - React app mounting point (#app div)
//   - window.__BICHAT_CONTEXT__ with server context
//   - window.__CSRF_TOKEN__ for CSRF protection
//   - React bundle script reference
//
// Response: HTML page with React app and injected context
// Status: 200 OK on success
// Status: 500 Internal Server Error if context build or rendering fails
func (c *WebController) RenderChatApp(w http.ResponseWriter, r *http.Request) {
	const op serrors.Op = "WebController.RenderChatApp"

	ctx := r.Context()
	logger := configuration.Use().Logger()

	// Get BiChat applet configuration from registry
	// Note: In production, retrieve from app.AppletRegistry().Get("bichat")
	// For now, create inline to get config
	config := applet.Config{
		WindowGlobal: "__BICHAT_CONTEXT__",
		Endpoints: applet.EndpointConfig{
			GraphQL: "/bichat/graphql",
			Stream:  "/bichat/stream",
		},
	}

	// Build initial context for React using the applet package
	// This extracts user, tenant, locale, and config from request context
	builder := applet.NewContextBuilder(config)
	initialContext, err := builder.Build(ctx, r)
	if err != nil {
		logger.WithError(serrors.E(op, err)).Error("Failed to build context")
		http.Error(w, "Failed to build context", http.StatusInternalServerError)
		return
	}

	// CSRF token is already included in initialContext.Session
	csrfToken := csrf.Token(r)

	// Serialize context to JSON for injection into HTML
	contextJSON, err := json.Marshal(initialContext)
	if err != nil {
		logger.WithError(serrors.E(op, err)).Error("Failed to serialize context")
		http.Error(w, "Failed to serialize context", http.StatusInternalServerError)
		return
	}

	// Render HTML template with injected context
	tmpl := template.Must(template.New("app").Parse(appTemplate))
	data := map[string]interface{}{
		"ContextJSON": template.JS(contextJSON),
		"CSRFToken":   csrfToken,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		logger.WithError(serrors.E(op, err)).Error("Failed to render template")
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
		return
	}
}

// appTemplate defines the HTML structure for the React SPA.
// It includes:
//   - Standard HTML5 boilerplate with UTF-8 charset
//   - Viewport meta tag for responsive design
//   - React app mounting point (#app div)
//   - Server context injection via window.__BICHAT_CONTEXT__
//   - CSRF token injection via window.__CSRF_TOKEN__
//   - React bundle script reference
//
// The template uses Go's html/template syntax to safely inject
// JSON data and strings, preventing XSS attacks.
const appTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>BI Chat</title>
    <script type="module" src="/bichat/assets/index.js"></script>
</head>
<body>
    <div id="app"></div>
    <script>
        window.__BICHAT_CONTEXT__ = {{.ContextJSON}};
        window.__CSRF_TOKEN__ = "{{.CSRFToken}}";
    </script>
</body>
</html>`
