package models

import (
	coremodels "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/lib/pq"
	"time"
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
	Name         string
	Results      []*InventoryCheckResult `gorm:"foreignKey:InventoryCheckID"`
	CreatedAt    time.Time
	FinishedAt   *time.Time
	CreatedByID  uint
	CreatedBy    *coremodels.User `gorm:"foreignKey:CreatedByID"`
	FinishedByID *uint
	FinishedBy   *coremodels.User `gorm:"foreignKey:FinishedByID"`
}

type InventoryPosition struct {
	ID       uint
	Title    string
	Quantity int
	RfidTags pq.StringArray `gorm:"type:text[]"`
}

type InventoryCheckResult struct {
	ID               uint
	InventoryCheckID uint
	PositionID       uint
	Position         *WarehousePosition
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
	WarehouseOrderID   uint
	WarehouseProductID uint
	CreatedAt          time.Time
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
