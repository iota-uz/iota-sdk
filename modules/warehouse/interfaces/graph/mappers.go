// Package graph provides this package.
package graph

import (
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/product"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/entities/inventory"
	model "github.com/iota-uz/iota-sdk/modules/warehouse/interfaces/graph/gqlmodels"
	"github.com/iota-uz/iota-sdk/modules/warehouse/interfaces/graph/mappers"
)

func ProductsToGraphModel(domainProducts []product.Product) []*model.Product {
	products := make([]*model.Product, 0, len(domainProducts))
	for _, p := range domainProducts {
		products = append(products, mappers.ProductToGraphModel(p))
	}

	return products
}

func ProductsToTags(domainProducts []product.Product) []string {
	tags := make([]string, 0, len(domainProducts))
	for _, p := range domainProducts {
		tags = append(tags, p.Rfid())
	}

	return tags
}

func InventoryPositionsToGraphModel(positions []*inventory.Position) []*model.InventoryPosition {
	inventoryPositions := make([]*model.InventoryPosition, 0, len(positions))
	for _, pos := range positions {
		inventoryPositions = append(inventoryPositions, mappers.InventoryPositionToGraphModel(pos))
	}

	return inventoryPositions
}
