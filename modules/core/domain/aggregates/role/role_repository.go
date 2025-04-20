package role

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/repo"
)

type Field int

const (
	Name Field = iota
	Description
	CreatedAt
	PermissionID
)

type SortBy repo.SortBy[Field]
type Filter = repo.FieldFilter[Field]

type FindParams struct {
	Search            string
	AttachPermissions bool
	Limit             int
	Offset            int
	SortBy            SortBy
	Filters           []Filter
}

type Repository interface {
	Count(ctx context.Context, params *FindParams) (int64, error)
	GetAll(ctx context.Context) ([]Role, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]Role, error)
	GetByID(ctx context.Context, id uint) (Role, error)
	Create(ctx context.Context, upload Role) (Role, error)
	Update(ctx context.Context, upload Role) (Role, error)
	Delete(ctx context.Context, id uint) error
}
