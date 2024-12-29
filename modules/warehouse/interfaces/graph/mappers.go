package graph

import (
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/product"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/entities/inventory"
	model "github.com/iota-uz/iota-sdk/modules/warehouse/interfaces/graph/gqlmodels"
	"github.com/iota-uz/iota-sdk/modules/warehouse/interfaces/graph/mappers"
	"github.com/iota-uz/iota-sdk/pkg/fp"
)

var (
	ProductsToGraphModel           = fp.Map[*product.Product, *model.Product](mappers.ProductToGraphModel)
	ProductsToTags                 = fp.Map[*product.Product, string](func(p *product.Product) string { return p.Rfid })
	InventoryPositionsToGraphModel = fp.Map[*inventory.Position, *model.InventoryPosition](mappers.InventoryPositionToGraphModel)
)
