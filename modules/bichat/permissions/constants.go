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
		uuid.MustParse("534ad655-f8d2-422f-a8b2-b6b6a78fa912"),
		"BiChat.Access",
		ResourceBIChat,
		permission.ActionRead,
		permission.ModifierAll,
	)
	BiChatReadAll = permission.MustCreate(
		uuid.MustParse("4f46b1af-3181-4e6b-88ab-01f5ec0d50e0"),
		"BiChat.ReadAll",
		ResourceBIChat,
		permission.ActionRead,
		permission.ModifierAll,
	)
	BiChatExport = permission.MustCreate(
		uuid.MustParse("7e6b8eba-c372-4248-b744-b94d4c623c32"),
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
