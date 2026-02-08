package session

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/repo"
)

type Field int

const (
	ExpiresAt Field = iota
	CreatedAt
)

type SortByField = repo.SortByField[Field]
type SortBy = repo.SortBy[Field]

type FindParams struct {
	Limit  int
	Offset int
	SortBy SortBy
	Token  string
	Search string // Search query for filtering by user name/email
}

type Repository interface {
	Count(ctx context.Context) (int64, error)
	CountFiltered(ctx context.Context, search string) (int64, error)
	GetAll(ctx context.Context) ([]Session, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]Session, error)
	GetByToken(ctx context.Context, token string) (Session, error)
	GetByTokenAndAudience(ctx context.Context, token string, audience SessionAudience) (Session, error)
	GetByUserID(ctx context.Context, userID uint) ([]Session, error)
	Create(ctx context.Context, user Session) error
	Update(ctx context.Context, user Session) error
	Delete(ctx context.Context, token string) error
	DeleteByUserId(ctx context.Context, userId uint) ([]Session, error)
	DeleteAllExceptToken(ctx context.Context, userID uint, exceptToken string) (int, error)
}
