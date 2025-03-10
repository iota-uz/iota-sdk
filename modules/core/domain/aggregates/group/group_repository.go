package group

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/repo"
)

type Field = int

const (
	CreatedAt Field = iota
	UpdatedAt
)

type SortBy repo.SortBy[Field]

type FindParams struct {
	Limit     int
	Offset    int
	SortBy    SortBy
	RoleID    *repo.Filter
	CreatedAt *repo.Filter
	UpdateAt  *repo.Filter
}

type Repository interface {
	Count(ctx context.Context, params *FindParams) (int64, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]Group, error)
	GetByID(ctx context.Context, id GroupID) (Group, error)
	Save(ctx context.Context, group Group) (Group, error)
	Delete(ctx context.Context, id GroupID) error
}
