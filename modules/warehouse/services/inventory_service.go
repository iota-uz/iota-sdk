package services

import (
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/entities/inventory"
	"github.com/iota-agency/iota-sdk/modules/warehouse/persistence"
	"github.com/iota-agency/iota-sdk/pkg/event"
)

type InventoryService struct {
	repo      inventory.Repository
	publisher event.Publisher
}

func NewInventoryService(publisher event.Publisher) *InventoryService {
	return &InventoryService{
		repo:      persistence.NewInventoryRepository(),
		publisher: publisher,
	}
}
