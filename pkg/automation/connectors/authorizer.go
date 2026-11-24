package connectors

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
)

// ConnectionAuthorizer is an optional interface of a Connector that signs a connection in with an
// OAuth2 authorization server of the connection's own, found from what the administrator entered (an
// MCP server's protected resource metadata): the endpoints and the client are the connection's, not the
// platform's (OAuth2Config). The integrations page saves such a connection first, then sends the
// administrator's browser to BeginAuthorization's URL and finishes on the callback.
type ConnectionAuthorizer interface {
	// NeedsAuthorization reports whether credentials (as validated) need a sign-in in the browser before
	// the connection works: no token yet, or one issued for other settings.
	NeedsAuthorization(c Credentials) bool
	// BeginAuthorization discovers the authorization server, registers a client when the credentials
	// have none (stored at once, under the row lock, so a second sign-in does not register again) and
	// returns where to send the browser. The state is sealed, one-time on the page's side and expires.
	BeginAuthorization(ctx context.Context, in AuthorizationStart) (string, string, error)
	// CompleteAuthorization exchanges the code of the callback and stores the tokens in the connection
	// (under the row lock); it returns the credentials as stored.
	CompleteAuthorization(ctx context.Context, in AuthorizationReturn) (Credentials, error)
	// FreshCredentials are a connection's credentials with an access token valid for at least
	// RefreshBefore, refreshed under the row lock when needed. A refresh the server refuses sets
	// needs_reauth (unless check: a disabled connection being checked records nothing) and is
	// ErrNeedsReauth.
	FreshCredentials(ctx context.Context, tenantID, connectionID uuid.UUID, check bool) (Credentials, error)
}

// AuthorizationStart is a sign-in of a saved connection.
type AuthorizationStart struct {
	TenantID     uuid.UUID
	ConnectionID uuid.UUID
	RedirectURL  string
}

// AuthorizationReturn is the callback of a sign-in.
type AuthorizationReturn struct {
	TenantID     uuid.UUID
	ConnectionID uuid.UUID
	State, Code  string
}

// StoreBinder is an optional interface of a Connector that needs the connection store itself (to refresh
// its connections' tokens under the row lock): the application binds the store once it is built.
type StoreBinder interface {
	BindStore(Store)
}

// BindStore gives every connector of the directory that is a StoreBinder the store.
func BindStore(dir Directory, s Store) {
	if dir == nil || s == nil {
		return
	}
	for _, c := range dir.List() {
		if b, ok := c.(StoreBinder); ok {
			b.BindStore(s)
		}
	}
}

// InternalFields are the credential fields whose schema property has "x-internal": true: the connector
// writes them itself (tokens of a sign-in), a client never sees or sets them, and the form does not show
// them.
func InternalFields(d Descriptor) map[string]bool {
	var doc struct {
		Properties map[string]struct {
			Internal bool `json:"x-internal"`
		} `json:"properties"`
	}
	out := map[string]bool{}
	if len(d.Credentials) == 0 || json.Unmarshal(d.Credentials, &doc) != nil {
		return out
	}
	for name, p := range doc.Properties {
		if p.Internal {
			out[name] = true
		}
	}
	return out
}
