package tenant

import "context"

type Repository interface {
	GetByID(ctx context.Context, id uint) (*Tenant, error)
	GetByDomain(ctx context.Context, domain string) (*Tenant, error)
	Create(ctx context.Context, tenant *Tenant) (*Tenant, error)
	Update(ctx context.Context, tenant *Tenant) (*Tenant, error)
	Delete(ctx context.Context, id uint) error
	List(ctx context.Context) ([]*Tenant, error)
}
