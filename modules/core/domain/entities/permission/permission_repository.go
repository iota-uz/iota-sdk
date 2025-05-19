package permission

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/repo"
)

type Field int

const (
	NameField Field = iota
	ResourceField
	ActionField
	ModifierField
)

type SortByField = repo.SortByField[Field]
type SortBy = repo.SortBy[Field]

type FindParams struct {
	Limit  int
	Offset int
	RoleID uint
	SortBy SortBy
}

type Repository interface {
	GetPaginated(ctx context.Context, params *FindParams) ([]*Permission, error)
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context) ([]*Permission, error)
	GetByID(ctx context.Context, id string) (*Permission, error)
	Save(ctx context.Context, p *Permission) error
	Delete(ctx context.Context, id string) error
}
