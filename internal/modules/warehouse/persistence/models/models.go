package models

import "time"

type WarehouseUnit struct {
	ID         int
	Title      string
	ShortTitle string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type InventoryCheck struct {
	ID        uint
	Status    string
	CreatedAt time.Time
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
	CreatedAt time.Time
}

type OrderItem struct {
	OrderID   uint
	ProductID uint
	CreatedAt time.Time
}

type WarehousePosition struct {
	ID        uint
	Title     string
	Barcode   string
	UnitID    uint
	CreatedAt time.Time
	UpdatedAt time.Time
}

type WarehouseProduct struct {
	ID         uint
	PositionID uint
	Rfid       string
	Status     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
