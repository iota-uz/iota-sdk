package token

import (
	"context"

	"github.com/google/uuid"
)

// Repository defines the interface for RefreshToken persistence
type Repository interface {
	// Create stores a new refresh token
	Create(ctx context.Context, token RefreshToken) error

	// GetByTokenHash retrieves a token by its SHA-256 hash
	GetByTokenHash(ctx context.Context, tokenHash string) (RefreshToken, error)

	// Delete removes a refresh token (revocation)
	Delete(ctx context.Context, id uuid.UUID) error

	// DeleteByTokenHash removes a token by its hash
	DeleteByTokenHash(ctx context.Context, tokenHash string) error

	// DeleteByUserAndClient removes all tokens for a user + client combination (logout/session termination)
	DeleteByUserAndClient(ctx context.Context, userID int, clientID string) error

	// DeleteExpired removes all expired tokens (cleanup)
	DeleteExpired(ctx context.Context) error
}
