package applet

import "time"

// InitialContext contains all server-side context passed to the React/Next.js frontend.
// This is serialized as JSON and injected into window[Config.WindowGlobal].
type InitialContext struct {
	User       UserContext            `json:"user"`
	Tenant     TenantContext          `json:"tenant"`
	Locale     LocaleContext          `json:"locale"`
	Config     AppConfig              `json:"config"`
	Route      RouteContext           `json:"route"`
	Session    SessionContext         `json:"session"`
	Error      *ErrorContext          `json:"error"`                // Required: error handling metadata
	Extensions map[string]interface{} `json:"extensions,omitempty"` // Optional: custom context from ContextExtender
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
	BasePath        string `json:"basePath,omitempty"`        // e.g., "/bi-chat"
	AssetsBasePath  string `json:"assetsBasePath,omitempty"`  // e.g., "/bi-chat/assets"
	RPCUIEndpoint   string `json:"rpcUIEndpoint,omitempty"`   // e.g., "/bi-chat/rpc"
	ShellMode       string `json:"shellMode,omitempty"`       // "embedded" | "standalone"
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

// ErrorContext provides error handling metadata for frontend error boundaries.
// Enriched by optional ErrorContextEnricher implementation.
type ErrorContext struct {
	SupportEmail string            `json:"supportEmail,omitempty"` // Support contact email
	DebugMode    bool              `json:"debugMode"`              // Enable debug logging in frontend
	ErrorCodes   map[string]string `json:"errorCodes,omitempty"`   // Code -> localized message mapping
	RetryConfig  *RetryConfig      `json:"retryConfig,omitempty"`  // Retry behavior configuration
}

// RetryConfig configures frontend retry behavior for failed requests.
type RetryConfig struct {
	MaxAttempts int   `json:"maxAttempts"` // Maximum retry attempts (0 = no retry)
	BackoffMs   int64 `json:"backoffMs"`   // Initial backoff duration in milliseconds
}

// StreamContext is a lightweight context for SSE streaming endpoints.
// Contains only essential data without heavy translations for optimal performance.
type StreamContext struct {
	UserID      int64                  `json:"userId"`
	TenantID    string                 `json:"tenantId"`
	Permissions []string               `json:"permissions"`
	CSRFToken   string                 `json:"csrfToken"`
	Session     SessionContext         `json:"session"`
	Extensions  map[string]interface{} `json:"extensions,omitempty"` // Optional custom context
}

// SessionConfig configures session expiry and refresh behavior.
type SessionConfig struct {
	ExpiryDuration time.Duration // Session expiry duration (default: 24h)
	RefreshURL     string        // Session refresh endpoint (default: "/auth/refresh")
	RenewBefore    time.Duration // Renew session N before expiry (default: 5min)
}

// DefaultSessionConfig provides sensible defaults for session configuration.
var DefaultSessionConfig = SessionConfig{
	ExpiryDuration: 24 * time.Hour,
	RefreshURL:     "/auth/refresh",
	RenewBefore:    5 * time.Minute,
}
