package permissions

import "github.com/iota-agency/iota-erp/internal/domain/entities/permission"

const (
	ResourceProduct  permission.Resource = "product"
	ResourcePosition permission.Resource = "position"
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
)
