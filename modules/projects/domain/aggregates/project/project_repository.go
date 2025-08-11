package project

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

type Field int

const (
	ID Field = iota
	Name
	CounterpartyID
	Description
	CreatedAt
	UpdatedAt
)

type SortBy = repo.SortBy[Field]

type Repository interface {
	Save(ctx context.Context, project Project) (Project, error)
	Delete(ctx context.Context, id uuid.UUID) error
	GetByID(ctx context.Context, id uuid.UUID) (Project, error)
	GetAll(ctx context.Context) ([]Project, error)
	GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]Project, error)
	GetByCounterpartyID(ctx context.Context, counterpartyID uuid.UUID) ([]Project, error)
}
