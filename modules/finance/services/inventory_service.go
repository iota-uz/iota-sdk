package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/inventory"
	"github.com/iota-uz/iota-sdk/pkg/composables"
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
	var result inventory.Inventory
	err := composables.InTx(ctx, func(txCtx context.Context) error {
		var createErr error
		result, createErr = s.repo.Create(txCtx, inv)
		return createErr
	})
	return result, err
}

func (s *InventoryService) GetByID(ctx context.Context, id uuid.UUID) (inventory.Inventory, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *InventoryService) Count(ctx context.Context, params *inventory.FindParams) (int64, error) {
	return s.repo.Count(ctx, params)
}

func (s *InventoryService) GetAll(ctx context.Context) ([]inventory.Inventory, error) {
	return s.repo.GetAll(ctx)
}

func (s *InventoryService) GetPaginated(ctx context.Context, params *inventory.FindParams) ([]inventory.Inventory, error) {
	return s.repo.GetPaginated(ctx, params)
}

func (s *InventoryService) Update(ctx context.Context, inv inventory.Inventory) error {
	return composables.InTx(ctx, func(txCtx context.Context) error {
		_, err := s.repo.Update(txCtx, inv)
		return err
	})
}

func (s *InventoryService) Delete(ctx context.Context, id uuid.UUID) error {
	return composables.InTx(ctx, func(txCtx context.Context) error {
		return s.repo.Delete(txCtx, id)
	})
}
