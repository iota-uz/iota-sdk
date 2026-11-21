// Package token provides this package.
package token

import (
	"context"

	"github.com/google/uuid"
)

// Repository defines the interface for RefreshToken persistence
type Repository interface {
	// Create stores a new refresh token
	Create(ctx context.Context, token RefreshToken) error

	// GetByTokenHash retrieves an active token by its SHA-256 hash; consumed
	// or revoked tokens are not returned
	GetByTokenHash(ctx context.Context, tokenHash string) (RefreshToken, error)

	// LockGrant serializes rotation and reuse revocation of one user's grant
	// to one client for the rest of the transaction in ctx
	LockGrant(ctx context.Context, userID int, clientID string) error

	// Consume marks an active token as used by a rotation and links it to its
	// replacement; a token that is not active returns NotFound
	Consume(ctx context.Context, tokenHash string, replacedBy uuid.UUID) error

	// RevokeReused revokes every token issued from tokenHash by rotation when
	// tokenHash was already consumed (refresh token reuse, RFC 9700 4.14.2);
	// it reports whether tokenHash was a consumed token. It takes the grant
	// lock first, so a rotation running concurrently is either fully visible
	// to it or starts after it; ctx must carry a transaction
	RevokeReused(ctx context.Context, tokenHash string) (bool, error)

	// Delete removes a refresh token (revocation)
	Delete(ctx context.Context, id uuid.UUID) error

	// DeleteByTokenHash removes a token by its hash
	DeleteByTokenHash(ctx context.Context, tokenHash string) error

	// DeleteByUserAndClient removes all tokens for a user + client combination (logout/session termination)
	DeleteByUserAndClient(ctx context.Context, userID int, clientID string) error

	// DeleteByUserID removes all tokens of a user (credential reset)
	DeleteByUserID(ctx context.Context, userID int) error

	// DeleteExpired removes all expired tokens (cleanup)
	DeleteExpired(ctx context.Context) error
}
