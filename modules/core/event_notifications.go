package core

import (
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/notification"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
)

// labeledEvent is a native event with its localized presentation metadata
// (*native.EventDef satisfies it).
type labeledEvent interface {
	notifications.EventRef
	Label() manifest.Label
	Descriptions() manifest.Label
}

func affectedUser(field string) notifications.RecipientDefinition {
	return notifications.DataUserRecipient("affected_user", map[string]string{
		"en": "Affected user", "ru": "Затронутый пользователь", "uz": "Ta'sirlangan foydalanuvchi", "uz-Cyrl": "Таъсирланган фойдаланувчи",
	}, field)
}

// coreEventNotifications are the definitions of the core events except
// core.user.created.v1 (userCreatedNotification, which keeps its legacy
// identity).
func coreEventNotifications() []notifications.Definition {
	user := func(ev labeledEvent, level notification.Level, body func(notifications.Event) string, recipients ...notifications.RecipientDefinition) notifications.Definition {
		return notifications.FromEvent(ev,
			notifications.WithModule("core"),
			notifications.WithRequiredPermission(permissions.UserRead),
			notifications.WithLevel(level),
			notifications.WithRecipients(append([]notifications.RecipientDefinition{notifications.ActorRecipient()}, recipients...)...),
			notifications.WithRender(notifications.EventContent(ev.Label(), ev.Descriptions(), body, notifications.DataURL("/users/", "user_id"))),
		)
	}
	userName := func(e notifications.Event) string { return notifications.DataName(e, "first_name", "last_name") }
	roleDef := func(ev labeledEvent, body func(notifications.Event) string) notifications.Definition {
		return notifications.FromEvent(ev,
			notifications.WithModule("core"),
			notifications.WithRequiredPermission(permissions.RoleRead),
			notifications.WithRecipients(notifications.ActorRecipient()),
			notifications.WithRender(notifications.EventContent(ev.Label(), ev.Descriptions(), body, notifications.DataURL("/roles/", "role_id"))),
		)
	}
	groupDef := func(ev labeledEvent, body func(notifications.Event) string, recipients ...notifications.RecipientDefinition) notifications.Definition {
		return notifications.FromEvent(ev,
			notifications.WithModule("core"),
			notifications.WithRequiredPermission(permissions.GroupRead),
			notifications.WithRecipients(append([]notifications.RecipientDefinition{notifications.ActorRecipient()}, recipients...)...),
			notifications.WithRender(notifications.EventContent(ev.Label(), ev.Descriptions(), body, notifications.DataURL("/groups/", "group_id"))),
		)
	}
	groupName := func(e notifications.Event) string { return e.Data["group_name"] }

	return []notifications.Definition{
		user(services.UserUpdatedEvent, notification.LevelInfo, userName, affectedUser("user_id")),
		user(services.UserBlockedEvent, notification.LevelWarning, func(e notifications.Event) string { return e.Data["block_reason"] }, affectedUser("user_id")),
		user(services.UserUnblockedEvent, notification.LevelInfo, nil, affectedUser("user_id")),
		user(services.UserDeletedEvent, notification.LevelWarning, userName),
		user(services.UserRoleAssignedEvent, notification.LevelInfo, nil, affectedUser("user_id")),
		roleDef(services.RoleCreatedEvent, func(e notifications.Event) string { return e.Data["name"] }),
		roleDef(services.RolePermissionGrantedEvent, func(e notifications.Event) string { return e.Data["permission"] }),
		groupDef(services.GroupCreatedEvent, func(e notifications.Event) string { return e.Data["name"] }),
		groupDef(services.GroupMemberAddedEvent, groupName, affectedUser("user_id")),
		groupDef(services.GroupMemberRemovedEvent, groupName, affectedUser("user_id")),
	}
}
