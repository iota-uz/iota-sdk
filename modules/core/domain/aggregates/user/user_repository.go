package user

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

type Field = int

const (
	FirstNameField Field = iota
	LastNameField
	MiddleNameField
	EmailField
	PhoneField
	GroupIDField
	RoleIDField
	PermissionIDField
	LastLoginField
	CreatedAtField
	UpdatedAtField
	TenantIDField
)

type SortBy = repo.SortBy[Field]
type Filter = repo.FieldFilter[Field]

type FindParams struct {
	Limit   int
	Offset  int
	SortBy  SortBy
	Search  string
	Filters []Filter
}

type Repository interface {
	Count(ctx context.Context, params *FindParams) (int64, error)
	CountByTenantID(ctx context.Context, tenantID uuid.UUID) (int64, error)
	GetAll(ctx context.Context) ([]User, error)
	GetByEmail(ctx context.Context, email string) (User, error)
	GetByPhone(ctx context.Context, phone string) (User, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]User, error)
	GetByID(ctx context.Context, id uint) (User, error)
	PhoneExists(ctx context.Context, phone string) (bool, error)
	EmailExists(ctx context.Context, email string) (bool, error)
	Create(ctx context.Context, user User) (User, error)
	Update(ctx context.Context, user User) error
	UpdateLastAction(ctx context.Context, id uint) error
	UpdateLastLogin(ctx context.Context, id uint) error
	Delete(ctx context.Context, id uint) error
}
