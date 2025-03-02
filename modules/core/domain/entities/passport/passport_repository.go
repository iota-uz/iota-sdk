package passport

import (
	"context"

	"github.com/google/uuid"
)

type PassportRepository interface {
	Create(ctx context.Context, data Passport) (Passport, error)
	GetByID(ctx context.Context, id uuid.UUID) (Passport, error)
	GetByPassportNumber(ctx context.Context, series, number string) (Passport, error)
	Update(ctx context.Context, id uuid.UUID, data Passport) (Passport, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

