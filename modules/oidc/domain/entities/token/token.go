package token

import (
	"time"

	"github.com/google/uuid"
)

// Option is a functional option for configuring RefreshToken
type Option func(*refreshToken)

// --- Option setters ---

func WithID(id uuid.UUID) Option {
	return func(t *refreshToken) {
		t.id = id
	}
}

func WithAudience(audience []string) Option {
	return func(t *refreshToken) {
		t.audience = audience
	}
}

func WithAMR(amr []string) Option {
	return func(t *refreshToken) {
		t.amr = amr
	}
}

func WithExpiresAt(expiresAt time.Time) Option {
	return func(t *refreshToken) {
		t.expiresAt = expiresAt
	}
}

func WithCreatedAt(createdAt time.Time) Option {
	return func(t *refreshToken) {
		t.createdAt = createdAt
	}
}

// ---- Interface ----

// RefreshToken represents an OIDC refresh token
type RefreshToken interface {
	ID() uuid.UUID
	TokenHash() string
	ClientID() string
	UserID() int
	TenantID() uuid.UUID
	Scopes() []string
	Audience() []string
	AuthTime() time.Time
	AMR() []string
	ExpiresAt() time.Time
	CreatedAt() time.Time

	// Business logic methods
	IsExpired() bool
	SetAudience(audience []string) RefreshToken
	SetAMR(amr []string) RefreshToken
}

// ---- Implementation ----

// New creates a new RefreshToken
func New(
	tokenHash string,
	clientID string,
	userID int,
	tenantID uuid.UUID,
	scopes []string,
	authTime time.Time,
	lifetime time.Duration,
	opts ...Option,
) RefreshToken {
	t := &refreshToken{
		id:        uuid.New(),
		tokenHash: tokenHash,
		clientID:  clientID,
		userID:    userID,
		tenantID:  tenantID,
		scopes:    scopes,
		audience:  []string{},
		authTime:  authTime,
		amr:       []string{"pwd"}, // Default: password authentication
		expiresAt: time.Now().Add(lifetime),
		createdAt: time.Now(),
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

type refreshToken struct {
	id        uuid.UUID
	tokenHash string
	clientID  string
	userID    int
	tenantID  uuid.UUID
	scopes    []string
	audience  []string
	authTime  time.Time
	amr       []string
	expiresAt time.Time
	createdAt time.Time
}

// Getters
func (t *refreshToken) ID() uuid.UUID        { return t.id }
func (t *refreshToken) TokenHash() string    { return t.tokenHash }
func (t *refreshToken) ClientID() string     { return t.clientID }
func (t *refreshToken) UserID() int          { return t.userID }
func (t *refreshToken) TenantID() uuid.UUID  { return t.tenantID }
func (t *refreshToken) Scopes() []string     { return t.scopes }
func (t *refreshToken) Audience() []string   { return t.audience }
func (t *refreshToken) AuthTime() time.Time  { return t.authTime }
func (t *refreshToken) AMR() []string        { return t.amr }
func (t *refreshToken) ExpiresAt() time.Time { return t.expiresAt }
func (t *refreshToken) CreatedAt() time.Time { return t.createdAt }

// IsExpired returns true if the token has expired
func (t *refreshToken) IsExpired() bool {
	return time.Now().After(t.expiresAt)
}

// SetAudience returns a new RefreshToken with the audience set
func (t *refreshToken) SetAudience(audience []string) RefreshToken {
	c := *t
	c.audience = audience
	return &c
}

// SetAMR returns a new RefreshToken with the AMR set
func (t *refreshToken) SetAMR(amr []string) RefreshToken {
	c := *t
	c.amr = amr
	return &c
}
