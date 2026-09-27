package acceptance

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/money"
)

type Repository interface {
	GetByID(ctx context.Context, id uuid.UUID) (Document, error)
	GetByProjectID(ctx context.Context, projectID uuid.UUID) ([]Document, error)
	// SignedTotals sums the signed documents of the projects, one amount per
	// currency.
	SignedTotals(ctx context.Context, projectIDs []uuid.UUID) ([]*money.Money, error)
	Create(ctx context.Context, document Document) (Document, error)
	Update(ctx context.Context, document Document) (Document, error)
	Delete(ctx context.Context, id uuid.UUID) error
}
