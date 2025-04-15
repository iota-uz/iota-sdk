package models

import (
	"database/sql"
	"time"

	coremodels "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"

	"github.com/lib/pq"
)

type WarehouseUnit struct {
	ID         uint
	TenantID   string
	Title      string
	ShortTitle string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type InventoryCheck struct {
	ID           uint
	TenantID     string
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
	RfidTags pq.StringArray
}

type InventoryCheckResult struct {
	ID               uint
	TenantID         string
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
	TenantID  string
	Type      string
	Status    string
	CreatedAt time.Time
}

type WarehouseOrderItem struct {
	WarehouseOrderID   uint
	WarehouseProductID uint
}

type WarehousePosition struct {
	ID        uint
	TenantID  string
	Title     string
	Barcode   string
	UnitID    sql.NullInt32
	Images    []coremodels.Upload `gorm:"many2many:warehouse_position_images;"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type WarehouseProduct struct {
	ID         uint
	TenantID   string
	PositionID uint
	Rfid       sql.NullString
	Status     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type WarehousePositionImage struct {
	UploadID            uint
	WarehousePositionID uint
}
