package permissions

import (
	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
)

const (
	ResourceClient permission.Resource = "client"
)

var (
	ClientCreate = permission.MustCreate(
		uuid.MustParse("7d9454d8-607e-4f30-bc12-459bbcc939b5"),
		"Client.Create",
		ResourceClient,
		permission.ActionCreate,
		permission.ModifierAll,
	)
	ClientRead = permission.MustCreate(
		uuid.MustParse("a5a89cb3-5f9c-4dc1-bf37-bd56dad593f1"),
		"Client.Read",
		ResourceClient,
		permission.ActionRead,
		permission.ModifierAll,
	)
	ClientUpdate = permission.MustCreate(
		uuid.MustParse("7cbcf407-49f7-460c-9038-c2bc6035453e"),
		"Client.Update",
		ResourceClient,
		permission.ActionUpdate,
		permission.ModifierAll,
	)
	ClientDelete = permission.MustCreate(
		uuid.MustParse("080826f0-5667-40c0-a67d-b6e8fcf2a535"),
		"Client.Delete",
		ResourceClient,
		permission.ActionDelete,
		permission.ModifierAll,
	)
)

var Permissions = []permission.Permission{
	ClientCreate,
	ClientRead,
	ClientUpdate,
	ClientDelete,
}
