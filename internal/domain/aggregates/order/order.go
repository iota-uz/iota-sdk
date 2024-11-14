package order

import (
	"github.com/iota-agency/iota-erp/internal/modules/warehouse/domain/entities/product"
	"time"
)

type Order struct {
	ID        uint
	Type      *Type
	Status    *Status
	Items     []*Item
	CreatedAt time.Time
}

type Item struct {
	Product   *product.Product
	CreatedAt time.Time
}
