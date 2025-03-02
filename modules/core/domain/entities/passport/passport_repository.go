package passport

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	GetByID(ctx context.Context, id uuid.UUID) (Passport, error)
	GetByPassportNumber(ctx context.Context, series, number string) (Passport, error)
	Exists(ctx context.Context, id uuid.UUID) (bool, error)
	Save(ctx context.Context, data Passport) (Passport, error) // Create or Update
	Create(ctx context.Context, data Passport) (Passport, error)
	Update(ctx context.Context, id uuid.UUID, data Passport) (Passport, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

