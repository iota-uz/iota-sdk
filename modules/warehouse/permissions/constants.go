package permissions

import (
	"github.com/google/uuid"
	permission2 "github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
)

const (
	ResourceProduct   permission2.Resource = "product"
	ResourcePosition  permission2.Resource = "position"
	ResourceOrder     permission2.Resource = "order"
	ResourceUnit      permission2.Resource = "unit"
	ResourceInventory permission2.Resource = "inventory"
)

var (
	ProductCreate = permission2.Permission{
		ID:       uuid.MustParse("b6aacbc4-f93d-4b31-8456-e30c87aaeea0"),
		Name:     "Product.Create",
		Resource: ResourceProduct,
		Action:   permission2.ActionCreate,
		Modifier: permission2.ModifierAll,
	}
	ProductRead = permission2.Permission{
		ID:       uuid.MustParse("40d7db94-1cb5-468b-9eeb-8bfbbf7450db"),
		Name:     "Product.Read",
		Resource: ResourceProduct,
		Action:   permission2.ActionRead,
		Modifier: permission2.ModifierAll,
	}
	ProductUpdate = permission2.Permission{
		ID:       uuid.MustParse("290cfc14-bdcf-406d-8da5-928ea8974e1b"),
		Name:     "Product.Update",
		Resource: ResourceProduct,
		Action:   permission2.ActionUpdate,
		Modifier: permission2.ModifierAll,
	}
	ProductDelete = permission2.Permission{
		ID:       uuid.MustParse("6e996ec5-88a0-434a-9f89-caab5d312a29"),
		Name:     "Product.Delete",
		Resource: ResourceProduct,
		Action:   permission2.ActionDelete,
		Modifier: permission2.ModifierAll,
	}
	PositionCreate = permission2.Permission{
		ID:       uuid.MustParse("bbc1695e-40c8-4b23-b40d-cda81394a599"),
		Name:     "Position.Create",
		Resource: ResourcePosition,
		Action:   permission2.ActionCreate,
		Modifier: permission2.ModifierAll,
	}
	PositionRead = permission2.Permission{
		ID:       uuid.MustParse("8a9451f1-4fea-4039-b97f-378397b59dac"),
		Name:     "Position.Read",
		Resource: ResourcePosition,
		Action:   permission2.ActionRead,
		Modifier: permission2.ModifierAll,
	}
	PositionUpdate = permission2.Permission{
		ID:       uuid.MustParse("1f84b55d-8450-4c59-9e78-9be80440f52b"),
		Name:     "Position.Update",
		Resource: ResourcePosition,
		Action:   permission2.ActionUpdate,
		Modifier: permission2.ModifierAll,
	}
	PositionDelete = permission2.Permission{
		ID:       uuid.MustParse("9d6c588e-8418-4bd5-99a3-2bd7b60274c5"),
		Name:     "Position.Delete",
		Resource: ResourcePosition,
		Action:   permission2.ActionDelete,
		Modifier: permission2.ModifierAll,
	}
	OrderCreate = permission2.Permission{
		ID:       uuid.MustParse("d6d59911-c92d-4012-b794-3c4903ec779b"),
		Name:     "Order.Create",
		Resource: ResourceOrder,
		Action:   permission2.ActionCreate,
		Modifier: permission2.ModifierAll,
	}
	OrderRead = permission2.Permission{
		ID:       uuid.MustParse("b1773521-f7ee-4335-8afc-0b642d3d2577"),
		Name:     "Order.Read",
		Resource: ResourceOrder,
		Action:   permission2.ActionRead,
		Modifier: permission2.ModifierAll,
	}
	OrderUpdate = permission2.Permission{
		ID:       uuid.MustParse("23d07cb9-3d2f-482f-9d12-c423674e1ff3"),
		Name:     "Order.Update",
		Resource: ResourceOrder,
		Action:   permission2.ActionUpdate,
		Modifier: permission2.ModifierAll,
	}
	OrderDelete = permission2.Permission{
		ID:       uuid.MustParse("04d31d1a-a26d-4f33-8699-ce03a6b3afa8"),
		Name:     "Order.Delete",
		Resource: ResourceOrder,
		Action:   permission2.ActionDelete,
		Modifier: permission2.ModifierAll,
	}
	UnitCreate = permission2.Permission{
		ID:       uuid.MustParse("1fd40255-8705-4c49-b60c-90ab66d3c344"),
		Name:     "Unit.Create",
		Resource: ResourceUnit,
		Action:   permission2.ActionCreate,
		Modifier: permission2.ModifierAll,
	}
	UnitRead = permission2.Permission{
		ID:       uuid.MustParse("3d7b6564-4fc3-4299-99d8-9c532c7da582"),
		Name:     "Unit.Read",
		Resource: ResourceUnit,
		Action:   permission2.ActionRead,
		Modifier: permission2.ModifierAll,
	}
	UnitUpdate = permission2.Permission{
		ID:       uuid.MustParse("815f2f19-44c8-47b6-8101-3023621c7c5d"),
		Name:     "Unit.Update",
		Resource: ResourceUnit,
		Action:   permission2.ActionUpdate,
		Modifier: permission2.ModifierAll,
	}
	UnitDelete = permission2.Permission{
		ID:       uuid.MustParse("cb59ad46-1b54-4818-b795-85c7d5b5f25b"),
		Name:     "Unit.Delete",
		Resource: ResourceUnit,
		Action:   permission2.ActionDelete,
		Modifier: permission2.ModifierAll,
	}
	InventoryCreate = permission2.Permission{
		ID:       uuid.MustParse("0193cef6-2b1a-74a1-8081-cd7c9c6270bf"),
		Name:     "Inventory.Create",
		Resource: ResourceInventory,
		Action:   permission2.ActionCreate,
		Modifier: permission2.ModifierAll,
	}
	InventoryRead = permission2.Permission{
		ID:       uuid.MustParse("0193cef6-5382-7619-89ee-570e5d814f17"),
		Name:     "Inventory.Read",
		Resource: ResourceInventory,
		Action:   permission2.ActionRead,
		Modifier: permission2.ModifierAll,
	}
	InventoryUpdate = permission2.Permission{
		ID:       uuid.MustParse("0193cef6-6b32-789a-8cc5-fcd7418cc0b6"),
		Name:     "Inventory.Update",
		Resource: ResourceInventory,
		Action:   permission2.ActionUpdate,
		Modifier: permission2.ModifierAll,
	}
	InventoryDelete = permission2.Permission{
		ID:       uuid.MustParse("0193cef6-858d-7eae-a069-908068574bea"),
		Name:     "Inventory.Delete",
		Resource: ResourceInventory,
		Action:   permission2.ActionDelete,
		Modifier: permission2.ModifierAll,
	}
)

var Permissions = []permission2.Permission{
	ProductCreate,
	ProductRead,
	ProductUpdate,
	ProductDelete,
	PositionCreate,
	PositionRead,
	PositionUpdate,
	PositionDelete,
	OrderCreate,
	OrderRead,
	OrderUpdate,
	OrderDelete,
	UnitCreate,
	UnitRead,
	UnitUpdate,
	UnitDelete,
	InventoryCreate,
	InventoryRead,
	InventoryUpdate,
	InventoryDelete,
}
