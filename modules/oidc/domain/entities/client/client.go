package client

import (
	"time"

	"github.com/google/uuid"
)

// Option is a functional option for configuring Client
type Option func(*client)

// --- Option setters ---

func WithID(id uuid.UUID) Option {
	return func(c *client) {
		c.id = id
	}
}

func WithClientSecretHash(hash string) Option {
	return func(c *client) {
		c.clientSecretHash = &hash
	}
}

func WithGrantTypes(grantTypes []string) Option {
	return func(c *client) {
		c.grantTypes = grantTypes
	}
}

func WithResponseTypes(responseTypes []string) Option {
	return func(c *client) {
		c.responseTypes = responseTypes
	}
}

func WithScopes(scopes []string) Option {
	return func(c *client) {
		c.scopes = scopes
	}
}

func WithAuthMethod(authMethod string) Option {
	return func(c *client) {
		c.authMethod = authMethod
	}
}

func WithAccessTokenLifetime(lifetime time.Duration) Option {
	return func(c *client) {
		c.accessTokenLifetime = lifetime
	}
}

func WithIDTokenLifetime(lifetime time.Duration) Option {
	return func(c *client) {
		c.idTokenLifetime = lifetime
	}
}

func WithRefreshTokenLifetime(lifetime time.Duration) Option {
	return func(c *client) {
		c.refreshTokenLifetime = lifetime
	}
}

func WithRequirePKCE(require bool) Option {
	return func(c *client) {
		c.requirePKCE = require
	}
}

func WithIsActive(isActive bool) Option {
	return func(c *client) {
		c.isActive = isActive
	}
}

func WithCreatedAt(createdAt time.Time) Option {
	return func(c *client) {
		c.createdAt = createdAt
	}
}

func WithUpdatedAt(updatedAt time.Time) Option {
	return func(c *client) {
		c.updatedAt = updatedAt
	}
}

// --- Interface ---

// Client represents an OIDC/OAuth2 client application
type Client interface {
	ID() uuid.UUID
	ClientID() string
	ClientSecretHash() *string
	Name() string
	ApplicationType() string
	RedirectURIs() []string
	GrantTypes() []string
	ResponseTypes() []string
	Scopes() []string
	AuthMethod() string
	AccessTokenLifetime() time.Duration
	IDTokenLifetime() time.Duration
	RefreshTokenLifetime() time.Duration
	RequirePKCE() bool
	IsActive() bool
	CreatedAt() time.Time
	UpdatedAt() time.Time

	// Business methods
	ValidateRedirectURI(uri string) bool
	ValidateGrantType(grantType string) bool
	ValidateScope(scope string) bool
	SetClientSecretHash(hash string) Client
	SetRedirectURIs(uris []string) Client
	SetScopes(scopes []string) Client
	AddScope(scope string) Client
	RemoveScope(scope string) Client
	Activate() Client
	Deactivate() Client
}

// --- Implementation ---

// New creates a new Client with required fields and default values
func New(
	clientID string,
	name string,
	applicationType string,
	redirectURIs []string,
	opts ...Option,
) Client {
	c := &client{
		id:                   uuid.New(),
		clientID:             clientID,
		clientSecretHash:     nil,
		name:                 name,
		applicationType:      applicationType,
		redirectURIs:         redirectURIs,
		grantTypes:           []string{"authorization_code"},
		responseTypes:        []string{"code"},
		scopes:               []string{"openid", "profile", "email"},
		authMethod:           "client_secret_basic",
		accessTokenLifetime:  time.Hour,
		idTokenLifetime:      time.Hour,
		refreshTokenLifetime: 720 * time.Hour,
		requirePKCE:          true,
		isActive:             true,
		createdAt:            time.Now(),
		updatedAt:            time.Now(),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

type client struct {
	id                   uuid.UUID
	clientID             string
	clientSecretHash     *string
	name                 string
	applicationType      string
	redirectURIs         []string
	grantTypes           []string
	responseTypes        []string
	scopes               []string
	authMethod           string
	accessTokenLifetime  time.Duration
	idTokenLifetime      time.Duration
	refreshTokenLifetime time.Duration
	requirePKCE          bool
	isActive             bool
	createdAt            time.Time
	updatedAt            time.Time
}

// --- Getters ---

func (c *client) ID() uuid.UUID {
	return c.id
}

func (c *client) ClientID() string {
	return c.clientID
}

func (c *client) ClientSecretHash() *string {
	return c.clientSecretHash
}

func (c *client) Name() string {
	return c.name
}

func (c *client) ApplicationType() string {
	return c.applicationType
}

func (c *client) RedirectURIs() []string {
	return c.redirectURIs
}

func (c *client) GrantTypes() []string {
	return c.grantTypes
}

func (c *client) ResponseTypes() []string {
	return c.responseTypes
}

func (c *client) Scopes() []string {
	return c.scopes
}

func (c *client) AuthMethod() string {
	return c.authMethod
}

func (c *client) AccessTokenLifetime() time.Duration {
	return c.accessTokenLifetime
}

func (c *client) IDTokenLifetime() time.Duration {
	return c.idTokenLifetime
}

func (c *client) RefreshTokenLifetime() time.Duration {
	return c.refreshTokenLifetime
}

func (c *client) RequirePKCE() bool {
	return c.requirePKCE
}

func (c *client) IsActive() bool {
	return c.isActive
}

func (c *client) CreatedAt() time.Time {
	return c.createdAt
}

func (c *client) UpdatedAt() time.Time {
	return c.updatedAt
}

// --- Business methods (Immutable - return new instance) ---

// ValidateRedirectURI checks if the provided redirect URI is registered
func (c *client) ValidateRedirectURI(uri string) bool {
	for _, registeredURI := range c.redirectURIs {
		if registeredURI == uri {
			return true
		}
	}
	return false
}

// ValidateGrantType checks if the provided grant type is allowed
func (c *client) ValidateGrantType(grantType string) bool {
	for _, allowed := range c.grantTypes {
		if allowed == grantType {
			return true
		}
	}
	return false
}

// ValidateScope checks if the provided scope is allowed
func (c *client) ValidateScope(scope string) bool {
	for _, allowed := range c.scopes {
		if allowed == scope {
			return true
		}
	}
	return false
}

// SetClientSecretHash sets the client secret hash (immutable pattern)
func (c *client) SetClientSecretHash(hash string) Client {
	result := *c
	result.clientSecretHash = &hash
	result.updatedAt = time.Now()
	return &result
}

// SetRedirectURIs sets the redirect URIs (immutable pattern)
func (c *client) SetRedirectURIs(uris []string) Client {
	result := *c
	result.redirectURIs = uris
	result.updatedAt = time.Now()
	return &result
}

// SetScopes sets the scopes (immutable pattern)
func (c *client) SetScopes(scopes []string) Client {
	result := *c
	result.scopes = scopes
	result.updatedAt = time.Now()
	return &result
}

// AddScope adds a new scope to the client (immutable pattern)
func (c *client) AddScope(scope string) Client {
	// Check if scope already exists
	for _, s := range c.scopes {
		if s == scope {
			return c
		}
	}
	result := *c
	result.scopes = append(result.scopes, scope)
	result.updatedAt = time.Now()
	return &result
}

// RemoveScope removes a scope from the client (immutable pattern)
func (c *client) RemoveScope(scope string) Client {
	result := *c
	filteredScopes := make([]string, 0, len(c.scopes))
	for _, s := range c.scopes {
		if s != scope {
			filteredScopes = append(filteredScopes, s)
		}
	}
	result.scopes = filteredScopes
	result.updatedAt = time.Now()
	return &result
}

// Activate activates the client (immutable pattern)
func (c *client) Activate() Client {
	result := *c
	result.isActive = true
	result.updatedAt = time.Now()
	return &result
}

// Deactivate deactivates the client (immutable pattern)
func (c *client) Deactivate() Client {
	result := *c
	result.isActive = false
	result.updatedAt = time.Now()
	return &result
}
