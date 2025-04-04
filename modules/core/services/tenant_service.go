package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/tenant"
)

type TenantService struct {
	repo tenant.Repository
}

func NewTenantService(repo tenant.Repository) *TenantService {
	return &TenantService{
		repo: repo,
	}
}

func (s *TenantService) GetByID(ctx context.Context, id uuid.UUID) (*tenant.Tenant, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *TenantService) GetByDomain(ctx context.Context, domain string) (*tenant.Tenant, error) {
	return s.repo.GetByDomain(ctx, domain)
}

func (s *TenantService) Create(ctx context.Context, name, domain string) (*tenant.Tenant, error) {
	t := tenant.New(name, tenant.WithDomain(domain))
	return s.repo.Create(ctx, t)
}

func (s *TenantService) Update(ctx context.Context, t *tenant.Tenant) (*tenant.Tenant, error) {
	return s.repo.Update(ctx, t)
}

func (s *TenantService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *TenantService) List(ctx context.Context) ([]*tenant.Tenant, error) {
	return s.repo.List(ctx)
}