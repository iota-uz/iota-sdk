package models

import (
	"time"

	"github.com/iota-agency/iota-sdk/pkg/infrastructure/persistence/models"
	coremodels "github.com/iota-agency/iota-sdk/pkg/infrastructure/persistence/models"
)

type WarehouseUnit struct {
	ID         uint
	Title      string
	ShortTitle string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type InventoryCheck struct {
	ID           uint
	Status       string
	Type         string
	Name         string
	Results      []*InventoryCheckResult `gorm:"foreignKey:InventoryCheckID"`
	CreatedAt    time.Time
	FinishedAt   *time.Time
	CreatedByID  uint
	CreatedBy    *models.User `gorm:"foreignKey:CreatedByID"`
	FinishedByID *uint
	FinishedBy   *models.User `gorm:"foreignKey:FinishedByID"`
}

type InventoryCheckResult struct {
	ID               uint
	InventoryCheckID uint
	PositionID       uint
	ExpectedQuantity int
	ActualQuantity   int
	Difference       int
	CreatedAt        time.Time
}

type WarehouseOrder struct {
	ID        uint
	Type      string
	Status    string
	Products  []*WarehouseProduct `gorm:"many2many:warehouse_order_items;"`
	CreatedAt time.Time
}

type WarehouseOrderItem struct {
	WarehouseOrderID uint
	ProductID        uint
	CreatedAt        time.Time
}

type WarehousePosition struct {
	ID        uint
	Title     string
	Barcode   string
	UnitID    uint
	Unit      *WarehouseUnit
	Images    []coremodels.Upload `gorm:"many2many:warehouse_position_images;"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type WarehouseProduct struct {
	ID         uint
	PositionID uint
	Position   *WarehousePosition
	Rfid       *string
	Status     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type WarehousePositionImage struct {
	UploadID            uint
	WarehousePositionID uint
}
