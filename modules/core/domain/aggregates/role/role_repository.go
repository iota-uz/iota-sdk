package role

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/repo"
)

type Field int

const (
	NameField Field = iota
	DescriptionField
	CreatedAtField
	PermissionIDField
)

type Filter = repo.FieldFilter[Field]
type SortBy = repo.SortBy[Field]

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
	Create(ctx context.Context, role Role) (Role, error)
	Update(ctx context.Context, role Role) (Role, error)
	Delete(ctx context.Context, id uint) error
}
