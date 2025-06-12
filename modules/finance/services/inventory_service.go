package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/inventory"
)

type InventoryService struct {
	repo inventory.Repository
}

func NewInventoryService(repo inventory.Repository) *InventoryService {
	return &InventoryService{
		repo: repo,
	}
}

func (s *InventoryService) Create(ctx context.Context, inv inventory.Inventory) (inventory.Inventory, error) {
	return s.repo.Create(ctx, inv)
}

func (s *InventoryService) GetByID(ctx context.Context, id uuid.UUID) (inventory.Inventory, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *InventoryService) Count(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}

func (s *InventoryService) GetAll(ctx context.Context) ([]inventory.Inventory, error) {
	return s.repo.GetAll(ctx)
}

func (s *InventoryService) GetPaginated(ctx context.Context, params *inventory.FindParams) ([]inventory.Inventory, error) {
	return s.repo.GetPaginated(ctx, params)
}

func (s *InventoryService) Update(ctx context.Context, inv inventory.Inventory) error {
	_, err := s.repo.Update(ctx, inv)
	return err
}

func (s *InventoryService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
