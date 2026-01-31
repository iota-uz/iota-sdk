package authrequest

import (
	"context"

	"github.com/google/uuid"
)

// Repository defines the interface for AuthRequest persistence
type Repository interface {
	// Create stores a new authorization request
	Create(ctx context.Context, req AuthRequest) error

	// GetByID retrieves an auth request by its ID
	GetByID(ctx context.Context, id uuid.UUID) (AuthRequest, error)

	// Update modifies an existing auth request
	Update(ctx context.Context, req AuthRequest) error

	// Delete removes an auth request
	Delete(ctx context.Context, id uuid.UUID) error

	// DeleteExpired removes all expired auth requests (cleanup)
	DeleteExpired(ctx context.Context) error
}
