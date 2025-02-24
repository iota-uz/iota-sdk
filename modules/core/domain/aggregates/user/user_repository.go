package user

import (
	"context"
)

type FindParams struct {
	Limit  int
	Offset int
	SortBy []string
}

type SessionFindParams struct {
	Limit  int
	Offset int
	SortBy []string
}

type Repository interface {
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context) ([]User, error)
	GetByEmail(ctx context.Context, email string) (User, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]User, error)
	GetByID(ctx context.Context, id UserID) (User, error)
	Create(ctx context.Context, user User) (User, error)
	Update(ctx context.Context, user User) error
	UpdateLastAction(ctx context.Context, id UserID) error
	UpdateLastLogin(ctx context.Context, id UserID) error
	Delete(ctx context.Context, id uint) error
}

type SessionRepository interface {
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context) ([]*Session, error)
	GetPaginated(ctx context.Context, params *SessionFindParams) ([]*Session, error)
	GetByID(ctx context.Context, token SessionID) (*Session, error)
	Create(ctx context.Context, user *Session) error
	Update(ctx context.Context, user *Session) error
	Delete(ctx context.Context, token SessionID) error
}
