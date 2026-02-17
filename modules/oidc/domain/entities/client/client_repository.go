package client

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

type Field int

const (
	ClientIDField Field = iota
	NameField
	ApplicationTypeField
	IsActiveField
	CreatedAtField
	UpdatedAtField
)

type SortByField = repo.SortByField[Field]
type SortBy = repo.SortBy[Field]
type Filter = repo.FieldFilter[Field]

type FindParams struct {
	Limit   int
	Offset  int
	SortBy  SortBy
	Search  string
	Filters []Filter
}

// Repository defines the interface for OIDC client persistence operations
type Repository interface {
	// Count returns the total number of clients matching the given parameters
	Count(ctx context.Context, params *FindParams) (int64, error)

	// GetAll retrieves all clients
	GetAll(ctx context.Context) ([]Client, error)

	// GetPaginated retrieves clients with pagination and filtering
	GetPaginated(ctx context.Context, params *FindParams) ([]Client, error)

	// GetByID retrieves a client by its UUID
	GetByID(ctx context.Context, id uuid.UUID) (Client, error)

	// GetByClientID retrieves a client by its client_id (OIDC identifier)
	GetByClientID(ctx context.Context, clientID string) (Client, error)

	// ClientIDExists checks if a client_id already exists
	ClientIDExists(ctx context.Context, clientID string) (bool, error)

	// Create stores a new client
	Create(ctx context.Context, client Client) (Client, error)

	// Update modifies an existing client
	Update(ctx context.Context, client Client) error

	// Delete removes a client by its UUID
	Delete(ctx context.Context, id uuid.UUID) error
}
