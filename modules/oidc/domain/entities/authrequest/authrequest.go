package authrequest

import (
	"time"

	"github.com/google/uuid"
)

// Option is a functional option for configuring AuthRequest
type Option func(*authRequest)

// --- Option setters ---

func WithID(id uuid.UUID) Option {
	return func(a *authRequest) {
		a.id = id
	}
}

func WithState(state string) Option {
	return func(a *authRequest) {
		a.state = &state
	}
}

func WithNonce(nonce string) Option {
	return func(a *authRequest) {
		a.nonce = &nonce
	}
}

func WithCodeChallenge(challenge, method string) Option {
	return func(a *authRequest) {
		a.codeChallenge = &challenge
		a.codeChallengeMethod = &method
	}
}

func WithUserID(userID int) Option {
	return func(a *authRequest) {
		a.userID = &userID
	}
}

func WithTenantID(tenantID uuid.UUID) Option {
	return func(a *authRequest) {
		a.tenantID = &tenantID
	}
}

func WithAuthTime(authTime time.Time) Option {
	return func(a *authRequest) {
		a.authTime = &authTime
	}
}

func WithCreatedAt(createdAt time.Time) Option {
	return func(a *authRequest) {
		a.createdAt = createdAt
	}
}

func WithExpiresAt(expiresAt time.Time) Option {
	return func(a *authRequest) {
		a.expiresAt = expiresAt
	}
}

// ---- Interface ----

// AuthRequest represents an OAuth2/OIDC authorization request
type AuthRequest interface {
	ID() uuid.UUID
	ClientID() string
	RedirectURI() string
	Scopes() []string
	State() *string
	Nonce() *string
	ResponseType() string
	CodeChallenge() *string
	CodeChallengeMethod() *string
	UserID() *int
	TenantID() *uuid.UUID
	AuthTime() *time.Time
	CreatedAt() time.Time
	ExpiresAt() time.Time

	// Business logic methods
	SetState(state string) AuthRequest
	SetNonce(nonce string) AuthRequest
	SetPKCE(challenge, method string) AuthRequest
	CompleteAuthentication(userID int, tenantID uuid.UUID) AuthRequest
	IsExpired() bool
	IsAuthenticated() bool
}

// ---- Implementation ----

// New creates a new AuthRequest
func New(
	clientID string,
	redirectURI string,
	scopes []string,
	responseType string,
	opts ...Option,
) AuthRequest {
	now := time.Now()
	a := &authRequest{
		id:           uuid.New(),
		clientID:     clientID,
		redirectURI:  redirectURI,
		scopes:       scopes,
		responseType: responseType,
		createdAt:    now,
		expiresAt:    now.Add(5 * time.Minute), // 5-minute TTL by default
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

type authRequest struct {
	id                  uuid.UUID
	clientID            string
	redirectURI         string
	scopes              []string
	state               *string
	nonce               *string
	responseType        string
	codeChallenge       *string
	codeChallengeMethod *string
	userID              *int
	tenantID            *uuid.UUID
	authTime            *time.Time
	createdAt           time.Time
	expiresAt           time.Time
}

// Getters
func (a *authRequest) ID() uuid.UUID                { return a.id }
func (a *authRequest) ClientID() string             { return a.clientID }
func (a *authRequest) RedirectURI() string          { return a.redirectURI }
func (a *authRequest) Scopes() []string             { return a.scopes }
func (a *authRequest) State() *string               { return a.state }
func (a *authRequest) Nonce() *string               { return a.nonce }
func (a *authRequest) ResponseType() string         { return a.responseType }
func (a *authRequest) CodeChallenge() *string       { return a.codeChallenge }
func (a *authRequest) CodeChallengeMethod() *string { return a.codeChallengeMethod }
func (a *authRequest) UserID() *int                 { return a.userID }
func (a *authRequest) TenantID() *uuid.UUID         { return a.tenantID }
func (a *authRequest) AuthTime() *time.Time         { return a.authTime }
func (a *authRequest) CreatedAt() time.Time         { return a.createdAt }
func (a *authRequest) ExpiresAt() time.Time         { return a.expiresAt }

// SetState returns a new AuthRequest with the state set
func (a *authRequest) SetState(state string) AuthRequest {
	c := *a
	c.state = &state
	return &c
}

// SetNonce returns a new AuthRequest with the nonce set
func (a *authRequest) SetNonce(nonce string) AuthRequest {
	c := *a
	c.nonce = &nonce
	return &c
}

// SetPKCE returns a new AuthRequest with PKCE parameters set
func (a *authRequest) SetPKCE(challenge, method string) AuthRequest {
	c := *a
	c.codeChallenge = &challenge
	c.codeChallengeMethod = &method
	return &c
}

// CompleteAuthentication returns a new AuthRequest with user authentication details
func (a *authRequest) CompleteAuthentication(userID int, tenantID uuid.UUID) AuthRequest {
	c := *a
	now := time.Now()
	c.userID = &userID
	c.tenantID = &tenantID
	c.authTime = &now
	return &c
}

// IsExpired returns true if the auth request has expired
func (a *authRequest) IsExpired() bool {
	return time.Now().After(a.expiresAt)
}

// IsAuthenticated returns true if the auth request has been authenticated
func (a *authRequest) IsAuthenticated() bool {
	return a.userID != nil && a.tenantID != nil
}
