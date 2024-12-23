package warehouse

import (
	"github.com/iota-agency/iota-sdk/modules/warehouse/permissions"
	"github.com/iota-agency/iota-sdk/pkg/domain/entities/permission"
	"github.com/iota-agency/iota-sdk/pkg/presentation/templates/icons"
	"github.com/iota-agency/iota-sdk/pkg/types"
)

var (
	ProductsItem = types.NavigationItem{
		Name: "NavigationLinks.Products",
		Href: "/warehouse/products",
		Permissions: []permission.Permission{
			permissions.ProductRead,
		},
		Children: nil,
	}
	PositionsItem = types.NavigationItem{
		Name: "NavigationLinks.WarehousePositions",
		Href: "/warehouse/positions",
		Permissions: []permission.Permission{
			permissions.PositionRead,
		},
		Children: nil,
	}
	OrdersItem = types.NavigationItem{
		Name: "NavigationLinks.WarehouseOrders",
		Href: "/warehouse/orders",
		Permissions: []permission.Permission{
			permissions.OrderRead,
		},
		Children: nil,
	}
	InventoryItem = types.NavigationItem{
		Name: "NavigationLinks.WarehouseInventory",
		Href: "/warehouse/inventory",
		Permissions: []permission.Permission{
			permissions.InventoryRead,
		},
		Children: nil,
	}
	WarehouseItem = types.NavigationItem{
		Name:     "NavigationLinks.Warehouse",
		Icon:     icons.Warehouse(icons.Props{Size: "20"}),
		Href:     "/warehouse",
		Children: []types.NavigationItem{ProductsItem, PositionsItem, OrdersItem, InventoryItem},
	}
)

var NavItems = []types.NavigationItem{WarehouseItem}
