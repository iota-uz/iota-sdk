package tenant

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*Tenant, error)
	GetByDomain(ctx context.Context, domain string) (*Tenant, error)
	Create(ctx context.Context, tenant *Tenant) (*Tenant, error)
	Update(ctx context.Context, tenant *Tenant) (*Tenant, error)
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context) ([]*Tenant, error)
}
