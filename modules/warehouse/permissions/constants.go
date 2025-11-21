package permissions

import (
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
)

const (
	ResourceProduct   permission.Resource = "product"
	ResourcePosition  permission.Resource = "position"
	ResourceOrder     permission.Resource = "order"
	ResourceUnit      permission.Resource = "unit"
	ResourceInventory permission.Resource = "inventory"
)

var (
	ProductCreate = permission.MustCreate(
		uuid.MustParse("b6aacbc4-f93d-4b31-8456-e30c87aaeea0"),
		"Product.Create",
		ResourceProduct,
		permission.ActionCreate,
		permission.ModifierAll,
	)
	ProductRead = permission.MustCreate(
		uuid.MustParse("40d7db94-1cb5-468b-9eeb-8bfbbf7450db"),
		"Product.Read",
		ResourceProduct,
		permission.ActionRead,
		permission.ModifierAll,
	)
	ProductUpdate = permission.MustCreate(
		uuid.MustParse("290cfc14-bdcf-406d-8da5-928ea8974e1b"),
		"Product.Update",
		ResourceProduct,
		permission.ActionUpdate,
		permission.ModifierAll,
	)
	ProductDelete = permission.MustCreate(
		uuid.MustParse("6e996ec5-88a0-434a-9f89-caab5d312a29"),
		"Product.Delete",
		ResourceProduct,
		permission.ActionDelete,
		permission.ModifierAll,
	)
	PositionCreate = permission.MustCreate(
		uuid.MustParse("bbc1695e-40c8-4b23-b40d-cda81394a599"),
		"Position.Create",
		ResourcePosition,
		permission.ActionCreate,
		permission.ModifierAll,
	)
	PositionRead = permission.MustCreate(
		uuid.MustParse("8a9451f1-4fea-4039-b97f-378397b59dac"),
		"Position.Read",
		ResourcePosition,
		permission.ActionRead,
		permission.ModifierAll,
	)
	PositionUpdate = permission.MustCreate(
		uuid.MustParse("1f84b55d-8450-4c59-9e78-9be80440f52b"),
		"Position.Update",
		ResourcePosition,
		permission.ActionUpdate,
		permission.ModifierAll,
	)
	PositionDelete = permission.MustCreate(
		uuid.MustParse("9d6c588e-8418-4bd5-99a3-2bd7b60274c5"),
		"Position.Delete",
		ResourcePosition,
		permission.ActionDelete,
		permission.ModifierAll,
	)
	OrderCreate = permission.MustCreate(
		uuid.MustParse("d6d59911-c92d-4012-b794-3c4903ec779b"),
		"Order.Create",
		ResourceOrder,
		permission.ActionCreate,
		permission.ModifierAll,
	)
	OrderRead = permission.MustCreate(
		uuid.MustParse("b1773521-f7ee-4335-8afc-0b642d3d2577"),
		"Order.Read",
		ResourceOrder,
		permission.ActionRead,
		permission.ModifierAll,
	)
	OrderUpdate = permission.MustCreate(
		uuid.MustParse("23d07cb9-3d2f-482f-9d12-c423674e1ff3"),
		"Order.Update",
		ResourceOrder,
		permission.ActionUpdate,
		permission.ModifierAll,
	)
	OrderDelete = permission.MustCreate(
		uuid.MustParse("04d31d1a-a26d-4f33-8699-ce03a6b3afa8"),
		"Order.Delete",
		ResourceOrder,
		permission.ActionDelete,
		permission.ModifierAll,
	)
	UnitCreate = permission.MustCreate(
		uuid.MustParse("1fd40255-8705-4c49-b60c-90ab66d3c344"),
		"Unit.Create",
		ResourceUnit,
		permission.ActionCreate,
		permission.ModifierAll,
	)
	UnitRead = permission.MustCreate(
		uuid.MustParse("3d7b6564-4fc3-4299-99d8-9c532c7da582"),
		"Unit.Read",
		ResourceUnit,
		permission.ActionRead,
		permission.ModifierAll,
	)
	UnitUpdate = permission.MustCreate(
		uuid.MustParse("815f2f19-44c8-47b6-8101-3023621c7c5d"),
		"Unit.Update",
		ResourceUnit,
		permission.ActionUpdate,
		permission.ModifierAll,
	)
	UnitDelete = permission.MustCreate(
		uuid.MustParse("cb59ad46-1b54-4818-b795-85c7d5b5f25b"),
		"Unit.Delete",
		ResourceUnit,
		permission.ActionDelete,
		permission.ModifierAll,
	)
	InventoryCreate = permission.MustCreate(
		uuid.MustParse("0193cef6-2b1a-74a1-8081-cd7c9c6270bf"),
		"Inventory.Create",
		ResourceInventory,
		permission.ActionCreate,
		permission.ModifierAll,
	)
	InventoryRead = permission.MustCreate(
		uuid.MustParse("0193cef6-5382-7619-89ee-570e5d814f17"),
		"Inventory.Read",
		ResourceInventory,
		permission.ActionRead,
		permission.ModifierAll,
	)
	InventoryUpdate = permission.MustCreate(
		uuid.MustParse("0193cef6-6b32-789a-8cc5-fcd7418cc0b6"),
		"Inventory.Update",
		ResourceInventory,
		permission.ActionUpdate,
		permission.ModifierAll,
	)
	InventoryDelete = permission.MustCreate(
		uuid.MustParse("0193cef6-858d-7eae-a069-908068574bea"),
		"Inventory.Delete",
		ResourceInventory,
		permission.ActionDelete,
		permission.ModifierAll,
	)
)

var Permissions = []permission.Permission{
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
