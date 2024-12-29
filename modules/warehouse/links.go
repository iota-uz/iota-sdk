package warehouse

import (
	"github.com/iota-uz/iota-sdk/components/icons"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/warehouse/permissions"
	"github.com/iota-uz/iota-sdk/pkg/types"
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
	UnitsItem = types.NavigationItem{
		Name: "NavigationLinks.WarehouseUnits",
		Href: "/warehouse/units",
		Permissions: []permission.Permission{
			permissions.UnitRead,
		},
		Children: nil,
	}
	WarehouseItem = types.NavigationItem{
		Name:     "NavigationLinks.Warehouse",
		Icon:     icons.Warehouse(icons.Props{Size: "20"}),
		Href:     "/warehouse",
		Children: []types.NavigationItem{ProductsItem, PositionsItem, OrdersItem, UnitsItem, InventoryItem},
	}
)

var NavItems = []types.NavigationItem{WarehouseItem}
