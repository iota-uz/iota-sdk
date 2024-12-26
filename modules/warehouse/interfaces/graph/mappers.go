package graph

import (
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/product"
	model "github.com/iota-agency/iota-sdk/modules/warehouse/interfaces/graph/gqlmodels"
	"github.com/iota-agency/iota-sdk/modules/warehouse/interfaces/graph/mappers"
	"github.com/iota-agency/iota-sdk/pkg/fp"
)

var (
	ProductsToGraphModel = fp.Map[*product.Product, *model.Product](mappers.ProductToGraphModel)
	ProductsToTags       = fp.Map[*product.Product, string](func(p *product.Product) string { return p.Rfid })
)
