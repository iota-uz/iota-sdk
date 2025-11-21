package order

import (
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/position"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/product"
)

// OrderCreatedEvent is emitted when a new order is created
type OrderCreatedEvent struct {
	OrderID   uint
	Type      Type
	TenantID  uuid.UUID
	Timestamp time.Time
}

// ItemAddedEvent is emitted when an item is added to an order
type ItemAddedEvent struct {
	OrderID   uint
	Position  position.Position
	Products  []product.Product
	Timestamp time.Time
}

// OrderCompletedEvent is emitted when an order is marked as complete
type OrderCompletedEvent struct {
	OrderID   uint
	Timestamp time.Time
}
