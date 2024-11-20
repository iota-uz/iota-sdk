package permissions

import "github.com/iota-agency/iota-sdk/pkg/domain/entities/permission"

const (
	ResourceProduct  permission.Resource = "product"
	ResourcePosition permission.Resource = "position"
	ResourceOrder    permission.Resource = "order"
	ResourceUnit     permission.Resource = "unit"
)

var (
	ProductCreate = permission.Permission{
		ID:       35,
		Name:     "Product.Create",
		Resource: ResourceProduct,
		Action:   permission.ActionCreate,
		Modifier: permission.ModifierAll,
	}
	ProductRead = permission.Permission{
		ID:       36,
		Name:     "Product.Read",
		Resource: ResourceProduct,
		Action:   permission.ActionRead,
		Modifier: permission.ModifierAll,
	}
	ProductUpdate = permission.Permission{
		ID:       37,
		Name:     "Product.Update",
		Resource: ResourceProduct,
		Action:   permission.ActionUpdate,
		Modifier: permission.ModifierAll,
	}
	ProductDelete = permission.Permission{
		ID:       38,
		Name:     "Product.Delete",
		Resource: ResourceProduct,
		Action:   permission.ActionDelete,
		Modifier: permission.ModifierAll,
	}
	PositionCreate = permission.Permission{
		ID:       39,
		Name:     "Position.Create",
		Resource: ResourcePosition,
		Action:   permission.ActionCreate,
		Modifier: permission.ModifierAll,
	}
	PositionRead = permission.Permission{
		ID:       40,
		Name:     "Position.Read",
		Resource: ResourcePosition,
		Action:   permission.ActionRead,
		Modifier: permission.ModifierAll,
	}
	PositionUpdate = permission.Permission{
		ID:       41,
		Name:     "Position.Update",
		Resource: ResourcePosition,
		Action:   permission.ActionUpdate,
		Modifier: permission.ModifierAll,
	}
	PositionDelete = permission.Permission{
		ID:       42,
		Name:     "Position.Delete",
		Resource: ResourcePosition,
		Action:   permission.ActionDelete,
		Modifier: permission.ModifierAll,
	}
	OrderCreate = permission.Permission{
		ID:       43,
		Name:     "Order.Create",
		Resource: ResourceOrder,
		Action:   permission.ActionCreate,
		Modifier: permission.ModifierAll,
	}
	OrderRead = permission.Permission{
		ID:       43,
		Name:     "Order.Read",
		Resource: ResourceOrder,
		Action:   permission.ActionRead,
		Modifier: permission.ModifierAll,
	}
	OrderUpdate = permission.Permission{
		ID:       44,
		Name:     "Order.Update",
		Resource: ResourceOrder,
		Action:   permission.ActionUpdate,
		Modifier: permission.ModifierAll,
	}
	OrderDelete = permission.Permission{
		ID:       45,
		Name:     "Order.Delete",
		Resource: ResourceOrder,
		Action:   permission.ActionDelete,
		Modifier: permission.ModifierAll,
	}
	UnitCreate = permission.Permission{
		ID:       46,
		Name:     "Unit.Create",
		Resource: ResourceUnit,
		Action:   permission.ActionCreate,
		Modifier: permission.ModifierAll,
	}
	UnitRead = permission.Permission{
		ID:       47,
		Name:     "Unit.Read",
		Resource: ResourceUnit,
		Action:   permission.ActionRead,
		Modifier: permission.ModifierAll,
	}
	UnitUpdate = permission.Permission{
		ID:       48,
		Name:     "Unit.Update",
		Resource: ResourceUnit,
		Action:   permission.ActionUpdate,
		Modifier: permission.ModifierAll,
	}
	UnitDelete = permission.Permission{
		ID:       49,
		Name:     "Unit.Delete",
		Resource: ResourceUnit,
		Action:   permission.ActionDelete,
		Modifier: permission.ModifierAll,
	}
)
