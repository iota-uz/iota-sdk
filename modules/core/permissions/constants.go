package permissions

import (
	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
)

const (
	ResourceUser   permission.Resource = "user"
	ResourceRole   permission.Resource = "role"
	ResourceGroup  permission.Resource = "group"
	ResourceUpload permission.Resource = "upload"
)

var (
	UserCreate = permission.MustCreate(
		uuid.MustParse("8b6060b3-af5e-4ae0-b32d-b33695141066"),
		"User.Create",
		ResourceUser,
		permission.ActionCreate,
		permission.ModifierAll,
	)
	UserRead = permission.MustCreate(
		uuid.MustParse("13f011c8-1107-4957-ad19-70cfc167a775"),
		"User.Read",
		ResourceUser,
		permission.ActionRead,
		permission.ModifierAll,
	)
	UserUpdate = permission.MustCreate(
		uuid.MustParse("1c351fd3-9a2b-40b9-80b1-11ba81e645c8"),
		"User.Update",
		ResourceUser,
		permission.ActionUpdate,
		permission.ModifierAll,
	)
	UserDelete = permission.MustCreate(
		uuid.MustParse("547cded3-6754-4a05-aeb0-a38d12ed05ee"),
		"User.Delete",
		ResourceUser,
		permission.ActionDelete,
		permission.ModifierAll,
	)
	RoleCreate = permission.MustCreate(
		uuid.MustParse("60f195ed-d373-41c3-a39d-bb7484850840"),
		"Role.Create",
		ResourceRole,
		permission.ActionCreate,
		permission.ModifierAll,
	)
	RoleRead = permission.MustCreate(
		uuid.MustParse("51d1025e-11fe-405e-9ab4-88078c28e110"),
		"Role.Read",
		ResourceRole,
		permission.ActionRead,
		permission.ModifierAll,
	)
	RoleUpdate = permission.MustCreate(
		uuid.MustParse("ea18e9d1-6ac4-4b2a-861c-cc89d95d7a19"),
		"Role.Update",
		ResourceRole,
		permission.ActionUpdate,
		permission.ModifierAll,
	)
	RoleDelete = permission.MustCreate(
		uuid.MustParse("5fcea09b-913e-4bbf-bb00-66586c29e930"),
		"Role.Delete",
		ResourceRole,
		permission.ActionDelete,
		permission.ModifierAll,
	)
	GroupCreate = permission.MustCreate(
		uuid.MustParse("7e8f9a0b-1c2d-3e4f-5a6b-7c8d9e0f1a2b"),
		"Group.Create",
		ResourceGroup,
		permission.ActionCreate,
		permission.ModifierAll,
	)
	GroupRead = permission.MustCreate(
		uuid.MustParse("8f9a0b1c-2d3e-4f5a-6b7c-8d9e0f1a2b3c"),
		"Group.Read",
		ResourceGroup,
		permission.ActionRead,
		permission.ModifierAll,
	)
	GroupUpdate = permission.MustCreate(
		uuid.MustParse("9a0b1c2d-3e4f-5a6b-7c8d-9e0f1a2b3c4d"),
		"Group.Update",
		ResourceGroup,
		permission.ActionUpdate,
		permission.ModifierAll,
	)
	GroupDelete = permission.MustCreate(
		uuid.MustParse("a0b1c2d3-4e5f-6a7b-8c9d-0e1f2a3b4c5d"),
		"Group.Delete",
		ResourceGroup,
		permission.ActionDelete,
		permission.ModifierAll,
	)
	UploadCreate = permission.MustCreate(
		uuid.MustParse("a1b2c3d4-e5f6-7890-abcd-ef1234567890"),
		"Upload.Create",
		ResourceUpload,
		permission.ActionCreate,
		permission.ModifierAll,
	)
	UploadRead = permission.MustCreate(
		uuid.MustParse("b2c3d4e5-f6a7-8901-bcde-f23456789012"),
		"Upload.Read",
		ResourceUpload,
		permission.ActionRead,
		permission.ModifierAll,
	)
	UploadUpdate = permission.MustCreate(
		uuid.MustParse("c3d4e5f6-a7b8-9012-cdef-345678901234"),
		"Upload.Update",
		ResourceUpload,
		permission.ActionUpdate,
		permission.ModifierAll,
	)
	UploadDelete = permission.MustCreate(
		uuid.MustParse("d4e5f6a7-b8c9-0123-defa-456789012345"),
		"Upload.Delete",
		ResourceUpload,
		permission.ActionDelete,
		permission.ModifierAll,
	)
)

var Permissions = []permission.Permission{
	UserCreate,
	UserRead,
	UserUpdate,
	UserDelete,
	RoleCreate,
	RoleRead,
	RoleUpdate,
	RoleDelete,
	GroupCreate,
	GroupRead,
	GroupUpdate,
	GroupDelete,
	UploadCreate,
	UploadRead,
	UploadUpdate,
	UploadDelete,
}
