// Package services provides this package.
package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/inventory"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

type InventoryService struct {
	repo          inventory.Repository
	historyWriter history.Writer
}

func NewInventoryService(repo inventory.Repository, historyWriter history.Writer) *InventoryService {
	return &InventoryService{
		repo:          repo,
		historyWriter: historyWriter,
	}
}

func (s *InventoryService) Create(ctx context.Context, inv inventory.Inventory) (inventory.Inventory, error) {
	var result inventory.Inventory
	err := composables.InTx(ctx, func(txCtx context.Context) error {
		var createErr error
		result, createErr = s.repo.Create(txCtx, inv)
		if createErr != nil {
			return createErr
		}
		return history.RecordMutation(txCtx, s.historyWriter, history.Mutation{
			Type: InventoryCreated, Version: 1, Resource: InventoryResource, ID: result.ID().String(),
			After: inventorySnapshot(result),
		})
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

func (s *InventoryService) Update(ctx context.Context, inv inventory.Inventory) (inventory.Inventory, error) {
	var result inventory.Inventory
	err := composables.InTx(ctx, func(txCtx context.Context) error {
		before, err := s.repo.GetByID(txCtx, inv.ID())
		if err != nil {
			return err
		}
		result, err = s.repo.Update(txCtx, inv)
		if err != nil {
			return err
		}
		return history.RecordMutation(txCtx, s.historyWriter, history.Mutation{
			Type: InventoryUpdated, Version: 1, Resource: InventoryResource, ID: inv.ID().String(),
			Before: inventorySnapshot(before), After: inventorySnapshot(result),
		})
	})
	return result, err
}

func (s *InventoryService) Delete(ctx context.Context, id uuid.UUID) error {
	return composables.InTx(ctx, func(txCtx context.Context) error {
		entity, err := s.repo.GetByID(txCtx, id)
		if err != nil {
			return err
		}
		if err := s.repo.Delete(txCtx, id); err != nil {
			return err
		}
		return history.RecordMutation(txCtx, s.historyWriter, history.Mutation{
			Type: InventoryDeleted, Version: 1, Resource: InventoryResource, ID: id.String(),
			Before: inventorySnapshot(entity),
		})
	})
}
