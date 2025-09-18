package expense

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

type Field int

const (
	ID Field = iota
	TransactionID
	CategoryID
	CreatedAt
	UpdatedAt
)

type SortBy = repo.SortBy[Field]

type Filter = repo.FieldFilter[Field]

type FindParams struct {
	ID      uuid.UUID
	Limit   int
	Offset  int
	SortBy  SortBy
	Filters []Filter
	Search  string
}

type Repository interface {
	Count(ctx context.Context, params *FindParams) (int64, error)
	GetByID(ctx context.Context, id uuid.UUID) (Expense, error)
	GetAll(ctx context.Context) ([]Expense, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]Expense, error)
	Create(ctx context.Context, data Expense) (Expense, error)
	Update(ctx context.Context, data Expense) (Expense, error)
	Delete(ctx context.Context, id uuid.UUID) error

	// Attachment methods
	GetAttachments(ctx context.Context, expenseID uuid.UUID) ([]uint, error)
	AttachFile(ctx context.Context, expenseID uuid.UUID, uploadID uint) error
	DetachFile(ctx context.Context, expenseID uuid.UUID, uploadID uint) error
}
