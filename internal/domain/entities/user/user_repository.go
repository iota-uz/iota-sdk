package user

import (
	"context"
)

type Repository interface {
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context) ([]*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*User, error)
	GetByID(ctx context.Context, id int64) (*User, error)
	Create(ctx context.Context, user *User) error
	CreateOrUpdate(ctx context.Context, user *User) error
	Update(ctx context.Context, user *User) error
	UpdateLastAction(ctx context.Context, id int64) error
	UpdateLastLogin(ctx context.Context, id int64) error
	Delete(ctx context.Context, id int64) error
}
