package acceptance

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/money"
)

// ErrStatus means the document cannot move to the requested status: only a
// draft can be signed, and a cancelled document stays cancelled.
var ErrStatus = errors.New("acceptance document cannot change to this status")

type Repository interface {
	GetByID(ctx context.Context, id uuid.UUID) (Document, error)
	GetByProjectID(ctx context.Context, projectID uuid.UUID) ([]Document, error)
	// SignedTotals sums the signed documents of the projects, one amount per
	// currency.
	SignedTotals(ctx context.Context, projectIDs []uuid.UUID) ([]*money.Money, error)
	Create(ctx context.Context, document Document) (Document, error)
	// UpdateStatus saves the document status only while the stored status is
	// one of from, and returns ErrStatus otherwise.
	UpdateStatus(ctx context.Context, document Document, from ...Status) (Document, error)
	Delete(ctx context.Context, id uuid.UUID) error
}
