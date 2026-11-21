package integrations

// Wire types of the integrations screen's RPC. iota-solid generates the TypeScript contract from them
// (web/Integrations.solid.generated.ts); a result union of the issue's contract is one struct whose
// variant fields are optional.

// AuthKind is how a connector authenticates.
type AuthKind string

const (
	AuthAPIKey AuthKind = "api_key"
	AuthBasic  AuthKind = "basic"
	AuthOAuth2 AuthKind = "oauth2"
	AuthCustom AuthKind = "custom"
)

// Category groups connector cards.
type Category string

const (
	CategoryMessaging Category = "messaging"
	CategoryEmail     Category = "email"
	CategoryAI        Category = "ai"
	CategoryDocuments Category = "documents"
	CategoryOther     Category = "other"
)

// PlatformPolicy is who may use a platform credential.
type PlatformPolicy string

const (
	PolicyAuto  PlatformPolicy = "auto"
	PolicyOptIn PlatformPolicy = "opt_in"
)

// Status is the state of a connection.
type Status string

const (
	StatusConnected   Status = "connected"
	StatusNeedsReauth Status = "needs_reauth"
	StatusError       Status = "error"
	StatusDisabled    Status = "disabled"
)

// ErrorCode is the refusal of an action that the screen explains.
type ErrorCode string

const (
	ErrTestFailed      ErrorCode = "test_failed"
	ErrNameTaken       ErrorCode = "name_taken"
	ErrInvalidInput    ErrorCode = "invalid_input"
	ErrVersionConflict ErrorCode = "version_conflict"
	ErrConnectionInUse ErrorCode = "CONNECTION_IN_USE"
	ErrNeedsReauth     ErrorCode = "needs_reauth"
	ErrUnavailable     ErrorCode = "unavailable"
)

// Props are the initial props of the screen.
type Props struct {
	// BasePath is the page path; the screen keeps the open drawer in its query.
	BasePath string `json:"basePath"`
	// AutomationsPath links a use to the graph editor: <AutomationsPath>/<id>?node=<node>.
	AutomationsPath string `json:"automationsPath"`
	// Configured is false when no encryption key is configured: connections cannot be saved.
	Configured bool `json:"configured"`
}

// Empty is a request without parameters.
type Empty struct{}

// PlatformView is the platform credential of a connector; null when its policy is none.
type PlatformView struct {
	Policy           PlatformPolicy `json:"policy"`
	Available        bool           `json:"available"`
	EnabledForTenant bool           `json:"enabled_for_tenant"`
}

// ConnectionView is a connection without its credentials.
type ConnectionView struct {
	ID           string  `json:"id"`
	Kind         string  `json:"kind"`
	Name         string  `json:"name"`
	DisplayName  string  `json:"display_name"`
	Version      int     `json:"version"`
	Identity     *string `json:"identity"`
	Status       Status  `json:"status"`
	StatusReason *string `json:"status_reason"`
	UsedBy       int     `json:"used_by"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
	LastChecked  *string `json:"last_checked_at"`
	// Fields are the non-secret credential values for the edit form (administrators only).
	Fields map[string]string `json:"fields,omitempty"`
}

// ConnectorView is a registered connector and the tenant's connections.
type ConnectorView struct {
	Kind        string   `json:"kind"`
	Title       string   `json:"title"`
	Icon        string   `json:"icon"`
	Description string   `json:"description"`
	Category    Category `json:"category"`
	Auth        AuthKind `json:"auth"`
	// CredentialSchema is Descriptor.Credentials with titles in the user's locale; a field with
	// "x-secret": true is write-only.
	CredentialSchema any `json:"credential_schema"`
	// Nodes are the titles of the nodes a connection makes available.
	Nodes       []string         `json:"nodes"`
	Platform    *PlatformView    `json:"platform"`
	Connections []ConnectionView `json:"connections"`
}

// ListResponse is the result of automations.connectors.list.
type ListResponse struct {
	Connectors []ConnectorView `json:"connectors"`
	// CanManage is whether the caller holds Connections.Manage.
	CanManage bool `json:"can_manage"`
}

// IDRequest names a connection.
type IDRequest struct {
	ID string `json:"id"`
}

// UsageView is a graph version that uses a connection.
type UsageView struct {
	AutomationID string   `json:"automation_id"`
	Name         string   `json:"name"`
	Version      int      `json:"version"`
	Enabled      bool     `json:"enabled"`
	NodeIDs      []string `json:"node_ids"`
}

// UsageResponse is the result of automations.connections.usage.
type UsageResponse struct {
	Usages []UsageView `json:"usages"`
}

// CreateRequest creates a connection.
type CreateRequest struct {
	Kind        string         `json:"kind"`
	Name        string         `json:"name"`
	DisplayName string         `json:"display_name"`
	Credentials map[string]any `json:"credentials"`
}

// UpdateRequest changes a connection; credentials absent keep them.
type UpdateRequest struct {
	ID          string         `json:"id"`
	Version     int            `json:"version"`
	DisplayName *string        `json:"display_name,omitempty"`
	Credentials map[string]any `json:"credentials,omitempty"`
}

// SaveResponse is the result of create and update: ok with the connection, or the refusal.
type SaveResponse struct {
	OK         bool              `json:"ok"`
	Connection *ConnectionView   `json:"connection,omitempty"`
	Error      ErrorCode         `json:"error,omitempty"`
	Message    string            `json:"message,omitempty"`
	Fields     map[string]string `json:"fields,omitempty"`
}

// OAuthStartRequest begins an OAuth2 authorization; ConnectionID re-authorizes that connection.
type OAuthStartRequest struct {
	Kind         string `json:"kind"`
	Name         string `json:"name"`
	DisplayName  string `json:"display_name,omitempty"`
	ConnectionID string `json:"connection_id,omitempty"`
}

// OAuthStartResponse is where to send the browser.
type OAuthStartResponse struct {
	AuthorizeURL string `json:"authorize_url"`
}

// TestResponse is the result of automations.connections.test.
type TestResponse struct {
	OK       bool      `json:"ok"`
	Identity string    `json:"identity,omitempty"`
	Message  string    `json:"message,omitempty"`
	Error    ErrorCode `json:"error,omitempty"`
}

// SetDisabledRequest disables or enables a connection.
type SetDisabledRequest struct {
	ID       string `json:"id"`
	Disabled bool   `json:"disabled"`
}

// ConnectionResponse is a connection after an action.
type ConnectionResponse struct {
	Connection ConnectionView `json:"connection"`
}

// DeleteResponse is ok, or CONNECTION_IN_USE with the uses.
type DeleteResponse struct {
	OK     bool        `json:"ok"`
	Error  ErrorCode   `json:"error,omitempty"`
	Usages []UsageView `json:"usages,omitempty"`
}

// PlatformRequest grants or revokes the platform credential of a kind.
type PlatformRequest struct {
	Kind    string `json:"kind"`
	Enabled bool   `json:"enabled"`
}

// OKResponse is a plain success.
type OKResponse struct {
	OK bool `json:"ok"`
}
