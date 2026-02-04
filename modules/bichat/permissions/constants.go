package permissions

import (
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
)

const (
	ResourceBIChat permission.Resource = "bichat"
)

var (
	BiChatAccess = permission.MustCreate(
		uuid.MustParse("1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d"),
		"BiChat.Access",
		ResourceBIChat,
		permission.ActionRead,
		permission.ModifierAll,
	)
	BiChatReadAll = permission.MustCreate(
		uuid.MustParse("2b3c4d5e-6f7a-8b9c-0d1e-2f3a4b5c6d7e"),
		"BiChat.ReadAll",
		ResourceBIChat,
		permission.ActionRead,
		permission.ModifierAll,
	)
	BiChatExport = permission.MustCreate(
		uuid.MustParse("3c4d5e6f-7a8b-9c0d-1e2f-3a4b5c6d7e8f"),
		"BiChat.Export",
		ResourceBIChat,
		permission.ActionCreate,
		permission.ModifierAll,
	)
)

var Permissions = []permission.Permission{
	BiChatAccess,
	BiChatReadAll,
	BiChatExport,
}
