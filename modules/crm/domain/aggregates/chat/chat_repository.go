// Package chat provides this package.
package chat

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/repo"
)

type Field int

const (
	CreatedAtField Field = iota
	LastMessageAtField
)

type SortByField = repo.SortByField[Field]
type SortBy = repo.SortBy[Field]

type FindParams struct {
	Limit  int
	Offset int
	Search string
	SortBy SortBy
}

type Repository interface {
	Count(ctx context.Context) (int64, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]Chat, error)
	GetByID(ctx context.Context, id uint) (Chat, error)
	// LockByID locks the chat row of the current tenant until the transaction of
	// ctx ends, so concurrent appends to one chat run one after another and each
	// reads the messages the previous one stored.
	LockByID(ctx context.Context, id uint) error
	GetByClientID(ctx context.Context, clientID uint) (Chat, error)
	GetMemberByContact(ctx context.Context, contactType string, contactValue string) (Member, error)
	Save(ctx context.Context, data Chat) (Chat, error)
	Delete(ctx context.Context, id uint) error
}
