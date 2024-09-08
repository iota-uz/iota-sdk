package persistence

import "time"

type Unit struct {
	ID          int
	Name        string
	Description *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type InventoryCheck struct {
	ID        int64
	Status    string
	CreatedAt time.Time
}

type InventoryCheckResult struct {
	ID               int64
	InventoryCheckID int64
	PositionID       int64
	ExpectedQuantity int
	ActualQuantity   int
	Difference       int
	CreatedAt        time.Time
}

type Order struct {
	ID        int64
	Type      string
	Status    string
	CreatedAt time.Time
}

type OrderItem struct {
	OrderID   int64
	ProductID int64
	CreatedAt time.Time
}

type Position struct {
	ID        int64
	Title     string
	Barcode   string
	UnitID    int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Product struct {
	ID         int64
	PositionID int64
	Rfid       string
	Status     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
