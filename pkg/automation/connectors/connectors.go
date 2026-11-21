// Package connectors holds the connector interfaces of automation graphs and the connections of
// tenants: the AES-256-GCM encrypted store in automation.connections with key rotation, platform
// credentials and their tenant grants, usage accounting, an OAuth2 helper and the bridge that serves
// connection secrets to extension packages (host.SecretResolver).
//
// connectors imports catalog; catalog never imports connectors.
package connectors

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
)

// Connector is a kind of external system: what it adds to the catalog, how it authenticates and how
// its credentials are checked.
type Connector interface {
	catalog.ConnectorContribution
	Descriptor() Descriptor
	// Test checks credentials with the provider and returns the identity behind them (a bot's name, a
	// number, an email). Neither its result nor its error carries credential values.
	Test(ctx context.Context, c Credentials) (Identity, error)
}

// AuthKind is how a connector authenticates.
type AuthKind string

const (
	AuthAPIKey AuthKind = "api_key"
	AuthBasic  AuthKind = "basic"
	AuthOAuth2 AuthKind = "oauth2"
	AuthCustom AuthKind = "custom"
)

// PlatformPolicy is who may use the platform credential of a connector: none; auto — every tenant;
// opt_in — a tenant whose administrator enabled it.
type PlatformPolicy string

const (
	PlatformNone  PlatformPolicy = "none"
	PlatformAuto  PlatformPolicy = "auto"
	PlatformOptIn PlatformPolicy = "opt_in"
)

// Category groups connectors on the integrations page; empty is CategoryOther.
type Category string

const (
	CategoryMessaging Category = "messaging"
	CategoryEmail     Category = "email"
	CategoryAI        Category = "ai"
	CategoryDocuments Category = "documents"
	CategoryOther     Category = "other"
)

// Descriptor describes a connector for the integrations page and the store.
type Descriptor struct {
	Kind  catalog.ConnectorKind
	Title manifest.Label
	// Description is one line on the connector's card (optional).
	Description manifest.Label
	// Category groups the card (empty: CategoryOther).
	Category Category
	Icon     string
	Auth     AuthKind
	// Credentials is the JSON Schema of the credentials form; a property with "x-secret": true is never
	// returned to a client.
	Credentials json.RawMessage
	OAuth2      *OAuth2Config // for Auth == oauth2
	Platform    PlatformPolicy
	Inbound     bool // the connector receives (a webhook or polling)
}

// Credentials are the decrypted fields of a connection.
type Credentials = catalog.Credentials

// Identity is who the credentials belong to at the provider.
type Identity struct {
	Display  string `json:"display"`
	External string `json:"external"` // the provider's identifier
}

// ConnectionStatus is the state of a connection.
type ConnectionStatus string

const (
	StatusConnected   ConnectionStatus = "connected"
	StatusNeedsReauth ConnectionStatus = "needs_reauth"
	StatusError       ConnectionStatus = "error"
	StatusDisabled    ConnectionStatus = "disabled"
)

func (s ConnectionStatus) valid() bool {
	switch s {
	case StatusConnected, StatusNeedsReauth, StatusError, StatusDisabled:
		return true
	}
	return false
}

// Connection is a tenant's connection, without its credentials.
type Connection struct {
	ID           uuid.UUID             `json:"id"`
	TenantID     uuid.UUID             `json:"tenant_id"`
	Kind         catalog.ConnectorKind `json:"kind"`
	Name         string                `json:"name"` // slug ^[a-z][a-z0-9_]{0,39}$, unique in (tenant, kind)
	DisplayName  string                `json:"display_name"`
	Identity     Identity              `json:"identity"`
	Status       ConnectionStatus      `json:"status"`
	StatusReason string                `json:"status_reason"`
	Version      int                   `json:"version"` // optimistic concurrency
	KeyID        string                `json:"key_id"`
	CreatedBy    int64                 `json:"created_by"`
	CreatedAt    time.Time             `json:"created_at"`
	UpdatedAt    time.Time             `json:"updated_at"`
}

// ConnectionStore keeps the connections of tenants. Every query is scoped to the tenant; a connection
// of another tenant is ErrNotFound.
type ConnectionStore interface {
	catalog.Availability
	// List is the connections of a tenant (kind "": of every kind), without credentials.
	List(ctx context.Context, tenantID uuid.UUID, kind catalog.ConnectorKind) ([]Connection, error)
	// Get returns the credentials too; only the engine and connectors call it, never presentation or
	// an API.
	Get(ctx context.Context, tenantID, id uuid.UUID) (Connection, Credentials, error)
	// Save creates (Version 0) or updates a connection when Version matches, else ErrVersionConflict.
	// creds == nil on an update keeps the credentials.
	Save(ctx context.Context, c Connection, creds Credentials) (Connection, error)
	SetStatus(ctx context.Context, tenantID, id uuid.UUID, s ConnectionStatus, reason string) error
	// Delete refuses a connection in use with ErrConnectionInUse (an *InUseError).
	Delete(ctx context.Context, tenantID, id uuid.UUID) error
	// UpdateInbound changes the inbound state (a polling cursor, a subscription id) under the row lock.
	// The state is stored in the clear: a webhook signing secret belongs in the credentials (Save).
	UpdateInbound(ctx context.Context, tenantID, id uuid.UUID, fn func(state json.RawMessage) (json.RawMessage, error)) error
	// Rewrap re-encrypts up to batch rows sealed with older keys with the active one; idempotent. It
	// returns how many rows it rewrapped: call it until it returns 0.
	Rewrap(ctx context.Context, batch int) (int, error)
}

// CredentialSource is how the engine opens the connection a call runs with: a connection ID or
// "platform:<kind>", as catalog.ResolveConnection picked it. The store implements it.
type CredentialSource interface {
	Open(ctx context.Context, tenantID uuid.UUID, ref string) (catalog.ConnectionInfo, Credentials, error)
}

// Directory is every registered connector with its Descriptor, configured or not (the integrations
// page).
type Directory interface {
	// Register adds a connector and its contribution to the catalog registry; a kind twice is an error.
	Register(Connector) error
	List() []Connector // by Kind
	Get(kind catalog.ConnectorKind) (Connector, bool)
}

// Keyring is the master keys from configuration: id → 32 bytes; Active encrypts new records.
type Keyring interface {
	Active() (id string, key []byte)
	Key(id string) ([]byte, bool)
}

// UsageIndex is where a connection is used (the engine implements it over graph versions).
type UsageIndex interface {
	// Uses lists the uses that block deleting the connection: versions that are enabled or have
	// unfinished runs.
	Uses(ctx context.Context, tenantID, connectionID uuid.UUID) ([]Use, error)
}

// Use is a graph version that uses a connection.
type Use struct {
	AutomationID uuid.UUID
	Name         string
	Version      int
	NodeIDs      []string
	Enabled      bool
}

// InUseError is ErrConnectionInUse with the uses.
type InUseError struct {
	Uses []Use
}

func (e *InUseError) Error() string {
	names := make([]string, 0, len(e.Uses))
	for _, u := range e.Uses {
		names = append(names, fmt.Sprintf("%s v%d", u.Name, u.Version))
	}
	return ErrConnectionInUse.Error() + ": " + strings.Join(names, ", ")
}

func (e *InUseError) Is(target error) bool { return target == ErrConnectionInUse }

// PlatformCredentials are credentials from the application's environment or configuration, one per
// name (a connector kind, or the name of an OAuth2 client credential).
type PlatformCredentials interface {
	Platform(kind catalog.ConnectorKind) (Credentials, Identity, bool)
}

// PlatformCatalog is the optional part of PlatformCredentials the store lists platform connections
// with: the kinds offered to tenants and their policy (NewPlatformCredentials implements it). Without it
// no platform credential is available to tenants.
type PlatformCatalog interface {
	Kinds() []catalog.ConnectorKind
	Policy(kind catalog.ConnectorKind) PlatformPolicy
}

// PlatformGrants is whether a tenant administrator enabled an opt_in platform credential (the platform
// AI key is available to a tenant only after that).
type PlatformGrants interface {
	Enabled(ctx context.Context, tenantID uuid.UUID, kind catalog.ConnectorKind) (bool, error)
	SetEnabled(ctx context.Context, tenantID uuid.UUID, kind catalog.ConnectorKind, enabled bool, by int64) error
}

// OAuth2Config is how a connector authorizes with OAuth2 (authorization code with PKCE).
type OAuth2Config struct {
	AuthURL, TokenURL string
	Scopes            []string
	// ClientCredential is the name of the platform credential holding client_id and client_secret;
	// empty: the connector's kind.
	ClientCredential string
}

// OAuth2 authorizes connections and keeps their tokens fresh.
type OAuth2 interface {
	// Begin returns the provider's authorization URL and the state to check on the callback.
	Begin(ctx context.Context, tenantID uuid.UUID, kind catalog.ConnectorKind, redirectURL string) (authURL string, state string, err error)
	// Complete exchanges the code and saves the connection (a new one, or the one of the same identity).
	Complete(ctx context.Context, tenantID uuid.UUID, state, code string) (Connection, error)
	// Token is a valid access token, refreshed 15 minutes before it expires under the row lock; when
	// the provider refuses the refresh the connection becomes needs_reauth and ErrNeedsReauth is returned.
	Token(ctx context.Context, tenantID, connectionID uuid.UUID) (string, error)
}

var (
	ErrNotConfigured   = errors.New("automation connections: encryption key is not configured")
	ErrNotFound        = errors.New("automation connections: not found")
	ErrVersionConflict = errors.New("automation connections: version conflict")
	ErrConnectionInUse = errors.New("automation connections: connection is in use")
	ErrNeedsReauth     = errors.New("automation connections: re-authorization required")
	// ErrNameTaken: another connection of the tenant and kind has the name.
	ErrNameTaken = errors.New("automation connections: name is taken")
	// ErrNotConnected: the connection exists but is not connected (Open).
	ErrNotConnected = errors.New("automation connections: connection is not connected")
	// ErrUndecryptable: the credentials do not decrypt (a row changed outside the store).
	ErrUndecryptable = errors.New("automation connections: credentials do not decrypt")
	// ErrInvalid: a connection or an argument the store refuses.
	ErrInvalid = errors.New("automation connections: invalid")
)
