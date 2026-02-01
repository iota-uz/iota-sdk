package models

import (
	"database/sql"
	"time"

	"github.com/lib/pq"
)

// Client represents the database model for oidc_clients table
type Client struct {
	ID                   string
	ClientID             string
	ClientSecretHash     sql.NullString
	Name                 string
	ApplicationType      string
	RedirectURIs         pq.StringArray // TEXT[]
	GrantTypes           pq.StringArray // VARCHAR(50)[]
	ResponseTypes        pq.StringArray // VARCHAR(50)[]
	Scopes               pq.StringArray // TEXT[]
	AuthMethod           string
	AccessTokenLifetime  time.Duration // INTERVAL
	IDTokenLifetime      time.Duration // INTERVAL
	RefreshTokenLifetime time.Duration // INTERVAL
	RequirePKCE          bool
	IsActive             bool
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// AuthRequest represents the database model for oidc_auth_requests table
type AuthRequest struct {
	ID                  string
	ClientID            string
	RedirectURI         string
	Scopes              pq.StringArray // TEXT[]
	State               sql.NullString
	Nonce               sql.NullString
	ResponseType        string
	CodeChallenge       sql.NullString
	CodeChallengeMethod sql.NullString
	UserID              sql.NullInt64
	TenantID            sql.NullString
	AuthTime            sql.NullTime
	CreatedAt           time.Time
	ExpiresAt           time.Time
}

// RefreshToken represents the database model for oidc_refresh_tokens table
type RefreshToken struct {
	ID        string
	TokenHash string
	ClientID  string
	UserID    int
	TenantID  string
	Scopes    pq.StringArray // TEXT[]
	Audience  pq.StringArray // TEXT[]
	AuthTime  time.Time
	AMR       pq.StringArray // TEXT[]
	ExpiresAt time.Time
	CreatedAt time.Time
}

// SigningKey represents the database model for oidc_signing_keys table
type SigningKey struct {
	ID         string
	KeyID      string
	Algorithm  string
	PrivateKey []byte // BYTEA (AES-encrypted)
	PublicKey  []byte // BYTEA (PEM-encoded)
	IsActive   bool
	CreatedAt  time.Time
	ExpiresAt  sql.NullTime
}
