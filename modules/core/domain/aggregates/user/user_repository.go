package user

import (
	"context"
)

type Field = int

const (
	FirstName Field = iota
	LastName
	MiddleName
	Email
	LastLogin
	CreatedAt
)

type SortBy struct {
	Fields    []Field
	Ascending bool
}

type FindParams struct {
	Limit  int
	Offset int
	SortBy SortBy
	RoleID uint
	Name   string
}

type Repository interface {
	Count(ctx context.Context, params *FindParams) (int64, error)
	GetAll(ctx context.Context) ([]User, error)
	GetByEmail(ctx context.Context, email string) (User, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]User, error)
	GetByID(ctx context.Context, id uint) (User, error)
	Create(ctx context.Context, user User) (User, error)
	Update(ctx context.Context, user User) error
	UpdateLastAction(ctx context.Context, id uint) error
	UpdateLastLogin(ctx context.Context, id uint) error
	Delete(ctx context.Context, id uint) error
}
