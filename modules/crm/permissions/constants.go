package permissions

import (
	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
)

const (
	ResourceClient permission.Resource = "client"
)

var (
	ClientCreate = &permission.Permission{
		ID:       uuid.MustParse("7d9454d8-607e-4f30-bc12-459bbcc939b5"),
		Name:     "Client.Create",
		Resource: ResourceClient,
		Action:   permission.ActionCreate,
		Modifier: permission.ModifierAll,
	}
	ClientRead = &permission.Permission{
		ID:       uuid.MustParse("a5a89cb3-5f9c-4dc1-bf37-bd56dad593f1"),
		Name:     "Client.Read",
		Resource: ResourceClient,
		Action:   permission.ActionRead,
		Modifier: permission.ModifierAll,
	}
	ClientUpdate = &permission.Permission{
		ID:       uuid.MustParse("7cbcf407-49f7-460c-9038-c2bc6035453e"),
		Name:     "Client.Update",
		Resource: ResourceClient,
		Action:   permission.ActionUpdate,
		Modifier: permission.ModifierAll,
	}
	ClientDelete = &permission.Permission{
		ID:       uuid.MustParse("080826f0-5667-40c0-a67d-b6e8fcf2a535"),
		Name:     "Client.Delete",
		Resource: ResourceClient,
		Action:   permission.ActionDelete,
		Modifier: permission.ModifierAll,
	}
)

var Permissions = []*permission.Permission{
	ClientCreate,
	ClientRead,
	ClientUpdate,
	ClientDelete,
}
