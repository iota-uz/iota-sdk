package applet

// InitialContext contains all server-side context passed to the React/Next.js frontend.
// This is serialized as JSON and injected into window[Config.WindowGlobal].
type InitialContext struct {
	User    UserContext            `json:"user"`
	Tenant  TenantContext          `json:"tenant"`
	Locale  LocaleContext          `json:"locale"`
	Config  AppConfig              `json:"config"`
	Route   RouteContext           `json:"route"`
	Session SessionContext         `json:"session"`
	Custom  map[string]interface{} `json:"custom,omitempty"` // Optional custom context from ContextExtender
}

// UserContext contains user information passed to frontend
type UserContext struct {
	ID          int64    `json:"id"`
	Email       string   `json:"email"`
	FirstName   string   `json:"firstName"`
	LastName    string   `json:"lastName"`
	Permissions []string `json:"permissions"` // All user permissions, not filtered
}

// TenantContext contains tenant information
type TenantContext struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// LocaleContext contains locale and translation data
type LocaleContext struct {
	Language     string            `json:"language"`     // e.g., "en", "ru", "uz"
	Translations map[string]string `json:"translations"` // All translations from bundle, not filtered
}

// AppConfig contains application configuration endpoints
type AppConfig struct {
	GraphQLEndpoint string `json:"graphQLEndpoint,omitempty"` // Optional: e.g., "/bichat/graphql"
	StreamEndpoint  string `json:"streamEndpoint,omitempty"`  // Optional: e.g., "/bichat/stream"
	RESTEndpoint    string `json:"restEndpoint,omitempty"`    // Optional: e.g., "/bichat/api"
}

// RouteContext contains URL routing information for deep linking
type RouteContext struct {
	Path   string            `json:"path"`   // Path after BasePath, e.g., "/sessions/123"
	Params map[string]string `json:"params"` // Route parameters extracted by router, e.g., {"id": "123"}
	Query  map[string]string `json:"query"`  // URL query parameters, e.g., {"tab": "history"}
}

// SessionContext contains authentication and session information
type SessionContext struct {
	ExpiresAt  int64  `json:"expiresAt"`  // Unix timestamp (milliseconds) when session expires
	RefreshURL string `json:"refreshURL"` // Endpoint to refresh session, e.g., "/auth/refresh"
	CSRFToken  string `json:"csrfToken"`  // Current CSRF token for POST requests
}
