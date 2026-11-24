package services

import (
	"context"
	"reflect"
	"slices"
	"strconv"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Versioned IDs of the user events. Extension packages subscribe to them by
// these IDs.
const (
	// UserCreatedEventID is the versioned ID of the business event published
	// when a user is created.
	UserCreatedEventID      = "core.user.created.v1"
	UserUpdatedEventID      = "core.user.updated.v1"
	UserBlockedEventID      = "core.user.blocked.v1"
	UserUnblockedEventID    = "core.user.unblocked.v1"
	UserDeletedEventID      = "core.user.deleted.v1"
	UserRoleAssignedEventID = "core.user.role_assigned.v1"
)

// localized builds the label of an event in the locales the SDK checks.
func localized(en, ru, uz, uzCyrl string) manifest.Label {
	return manifest.Label{"en": en, "ru": ru, "uz": uz, "uz-Cyrl": uzCyrl}
}

// UserCreatedPayload is the versioned payload of core.user.created.v1.
type UserCreatedPayload struct {
	UserID    int64  `json:"userId,string" wire:"int64"`
	Email     string `json:"email"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	UserType  string `json:"userType"`
}

// UserUpdatedPayload is the payload of core.user.updated.v1: the user after
// the update and the IDs of the core.user fields the update changed.
type UserUpdatedPayload struct {
	UserID    int64    `json:"userId,string" wire:"int64"`
	FirstName string   `json:"firstName"`
	LastName  string   `json:"lastName"`
	Changed   []string `json:"changed"`
}

// UserBlockedPayload is the payload of core.user.blocked.v1.
type UserBlockedPayload struct {
	UserID      int64  `json:"userId,string" wire:"int64"`
	BlockReason string `json:"blockReason"`
}

// UserUnblockedPayload is the payload of core.user.unblocked.v1.
type UserUnblockedPayload struct {
	UserID int64 `json:"userId,string" wire:"int64"`
}

// UserDeletedPayload is the payload of core.user.deleted.v1.
type UserDeletedPayload struct {
	UserID    int64  `json:"userId,string" wire:"int64"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

// UserRoleAssignedPayload is the payload of core.user.role_assigned.v1.
type UserRoleAssignedPayload struct {
	UserID int64 `json:"userId,string" wire:"int64"`
	RoleID int64 `json:"roleId,string" wire:"int64"`
}

// UserCreatedEvent declares the event and its versioned payload. The core
// module contributes it to the application's native set (Natives), next to
// the events declared by extension packages.
var UserCreatedEvent = native.Event[UserCreatedPayload](UserCreatedEventID, native.EventSpec{
	Subject: "core.user",
	Label: manifest.Label{
		"en": "User created", "ru": "Создан пользователь", "uz": "Foydalanuvchi yaratildi",
		"uz-Cyrl": "Фойдаланувчи яратилди", "pt-BR": "Usuário criado", "zh": "用户已创建",
	},
	Description: "A user account was created. Published in the transaction that creates the user.",
	Descriptions: manifest.Label{
		"en":      "A user account was created.",
		"ru":      "Создана учётная запись пользователя.",
		"uz":      "Foydalanuvchi hisobi yaratildi.",
		"uz-Cyrl": "Фойдаланувчи ҳисоби яратилди.",
		"pt-BR":   "Uma conta de usuário foi criada.",
		"zh":      "已创建用户账户。",
	},
	ReadPermission: permissions.UserRead.Name(),
	Sensitive:      map[string][]string{"email": {permissions.UserRead.Name()}},
	Category:       "core.users",
})

// UserUpdatedEvent is published when an update changes the user's profile.
var UserUpdatedEvent = native.Event[UserUpdatedPayload](UserUpdatedEventID, native.EventSpec{
	Subject:     "core.user",
	Label:       localized("User updated", "Пользователь изменён", "Foydalanuvchi o'zgartirildi", "Фойдаланувчи ўзгартирилди"),
	Description: "A user was updated. Published in the transaction of the update that changed a profile field; changed lists the core.user fields it changed.",
	Descriptions: localized(
		"A user was updated; the changed fields are listed.",
		"Пользователь изменён; перечислены изменённые поля.",
		"Foydalanuvchi o'zgartirildi; o'zgargan maydonlar ko'rsatiladi.",
		"Фойдаланувчи ўзгартирилди; ўзгарган майдонлар кўрсатилади."),
	ReadPermission: permissions.UserRead.Name(),
	Category:       "core.users",
})

// UserBlockedEvent is published when a user is blocked.
var UserBlockedEvent = native.Event[UserBlockedPayload](UserBlockedEventID, native.EventSpec{
	Subject:     "core.user",
	Label:       localized("User blocked", "Пользователь заблокирован", "Foydalanuvchi bloklandi", "Фойдаланувчи блокланди"),
	Description: "A user was blocked. Published in the transaction that blocks the user.",
	Descriptions: localized(
		"A user was blocked.",
		"Пользователь заблокирован.",
		"Foydalanuvchi bloklandi.",
		"Фойдаланувчи блокланди."),
	ReadPermission: permissions.UserRead.Name(),
	Category:       "core.users",
})

// UserUnblockedEvent is published when a user is unblocked.
var UserUnblockedEvent = native.Event[UserUnblockedPayload](UserUnblockedEventID, native.EventSpec{
	Subject:     "core.user",
	Label:       localized("User unblocked", "Пользователь разблокирован", "Foydalanuvchi blokdan chiqarildi", "Фойдаланувчи блокдан чиқарилди"),
	Description: "A user was unblocked. Published in the transaction that unblocks the user.",
	Descriptions: localized(
		"A user was unblocked.",
		"Пользователь разблокирован.",
		"Foydalanuvchi blokdan chiqarildi.",
		"Фойдаланувчи блокдан чиқарилди."),
	ReadPermission: permissions.UserRead.Name(),
	Category:       "core.users",
})

// UserDeletedEvent is published when a user is deleted.
var UserDeletedEvent = native.Event[UserDeletedPayload](UserDeletedEventID, native.EventSpec{
	Subject:     "core.user",
	Label:       localized("User deleted", "Пользователь удалён", "Foydalanuvchi o'chirildi", "Фойдаланувчи ўчирилди"),
	Description: "A user was deleted. Published in the transaction that deletes the user.",
	Descriptions: localized(
		"A user was deleted.",
		"Пользователь удалён.",
		"Foydalanuvchi o'chirildi.",
		"Фойдаланувчи ўчирилди."),
	ReadPermission: permissions.UserRead.Name(),
	Category:       "core.users",
})

// UserRoleAssignedEvent is published for every role a user gained.
var UserRoleAssignedEvent = native.Event[UserRoleAssignedPayload](UserRoleAssignedEventID, native.EventSpec{
	Subject:     "core.user",
	Label:       localized("Role assigned to user", "Пользователю назначена роль", "Foydalanuvchiga rol tayinlandi", "Фойдаланувчига роль тайинланди"),
	Description: "A role was assigned to a user. Published once per added role in the transaction of the change.",
	Descriptions: localized(
		"A role was assigned to a user.",
		"Пользователю назначена роль.",
		"Foydalanuvchiga rol tayinlandi.",
		"Фойдаланувчига роль тайинланди."),
	ReadPermission: permissions.UserRead.Name(),
	Category:       "core.users",
})

// Natives is the core module's native registration: the core.user, core.group
// and core.role entities and the user, role and group events. Applications
// compose it into their native.Set (modules.Natives).
func Natives(s *native.Set) error {
	commands, err := automationActions()
	if err != nil {
		return err
	}
	if err := s.Add(commands...); err != nil {
		return err
	}
	return s.Add(
		native.Entity(data.CoreUser()), native.Entity(GroupEntity()), native.Entity(RoleEntity()),
		UserCreatedEvent, UserUpdatedEvent, UserBlockedEvent, UserUnblockedEvent, UserDeletedEvent, UserRoleAssignedEvent,
		RoleCreatedEvent, RolePermissionGrantedEvent,
		GroupCreatedEvent, GroupMemberAddedEvent, GroupMemberRemovedEvent,
	)
}

func userRef(id uint) *spi.ObjectRef {
	return &spi.ObjectRef{Entity: "core.user", Key: strconv.FormatUint(uint64(id), 10)}
}

// publishUserCreated enqueues core.user.created.v1 in the transaction held by
// ctx, so the event commits or rolls back with the user. The event belongs to
// the created user's tenant; the acting user, when there is one, is the event
// initiator.
func publishUserCreated(ctx context.Context, created user.User) error {
	const op = serrors.Op("UserService.publishUserCreated")
	tenantID := created.TenantID()
	if tenantID == uuid.Nil {
		var err error
		if tenantID, err = composables.UseTenantID(ctx); err != nil {
			return serrors.Wrap(op, err)
		}
	}
	return serrors.Wrap(op, events.PublishInTx(ctx, tenantID, UserCreatedEvent, userRef(created.ID()), UserCreatedPayload{
		UserID:    int64(created.ID()),
		Email:     created.Email().Value(),
		FirstName: created.FirstName(),
		LastName:  created.LastName(),
		UserType:  string(created.Type()),
	}))
}

// userEntityFields maps the history snapshot keys of a user to the core.user
// field IDs a changed list may name.
var userEntityFields = []struct{ snapshot, field string }{
	{"first_name", "firstName"}, {"last_name", "lastName"}, {"middle_name", "middleName"},
	{"email", "email"}, {"phone", "phone"}, {"ui_language", "uiLanguage"},
}

// userChangedFields lists the core.user fields whose values differ.
func userChangedFields(before, after history.Snapshot) []string {
	out := []string{}
	for _, f := range userEntityFields {
		if !reflect.DeepEqual(before[f.snapshot], after[f.snapshot]) {
			out = append(out, f.field)
		}
	}
	slices.Sort(out)
	return out
}

// publishUserUpdated enqueues core.user.updated.v1 when the update changed a
// profile field; an update that changed nothing publishes nothing.
func publishUserUpdated(ctx context.Context, before, after user.User) error {
	const op = serrors.Op("UserService.publishUserUpdated")
	changed := userChangedFields(userSnapshot(before), userSnapshot(after))
	if len(changed) == 0 {
		return nil
	}
	return serrors.Wrap(op, events.PublishInTx(ctx, after.TenantID(), UserUpdatedEvent, userRef(after.ID()), UserUpdatedPayload{
		UserID: int64(after.ID()), FirstName: after.FirstName(), LastName: after.LastName(), Changed: changed,
	}))
}

// publishUserBlocked enqueues core.user.blocked.v1.
func publishUserBlocked(ctx context.Context, blocked user.User) error {
	return serrors.Wrap("UserService.publishUserBlocked", events.PublishInTx(ctx, blocked.TenantID(), UserBlockedEvent, userRef(blocked.ID()), UserBlockedPayload{
		UserID: int64(blocked.ID()), BlockReason: blocked.BlockReason(),
	}))
}

// publishUserUnblocked enqueues core.user.unblocked.v1.
func publishUserUnblocked(ctx context.Context, unblocked user.User) error {
	return serrors.Wrap("UserService.publishUserUnblocked", events.PublishInTx(ctx, unblocked.TenantID(), UserUnblockedEvent, userRef(unblocked.ID()), UserUnblockedPayload{
		UserID: int64(unblocked.ID()),
	}))
}

// publishUserDeleted enqueues core.user.deleted.v1.
func publishUserDeleted(ctx context.Context, deleted user.User) error {
	return serrors.Wrap("UserService.publishUserDeleted", events.PublishInTx(ctx, deleted.TenantID(), UserDeletedEvent, userRef(deleted.ID()), UserDeletedPayload{
		UserID: int64(deleted.ID()), FirstName: deleted.FirstName(), LastName: deleted.LastName(),
	}))
}

// publishUserRolesAssigned enqueues core.user.role_assigned.v1 for every role
// ID (decimal strings of the history diff) the user gained.
func publishUserRolesAssigned(ctx context.Context, subject user.User, added []string) error {
	const op = serrors.Op("UserService.publishUserRolesAssigned")
	for _, id := range added {
		roleID, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			return serrors.Wrap(op, err)
		}
		if err := events.PublishInTx(ctx, subject.TenantID(), UserRoleAssignedEvent, userRef(subject.ID()), UserRoleAssignedPayload{
			UserID: int64(subject.ID()), RoleID: roleID,
		}); err != nil {
			return serrors.Wrap(op, err)
		}
	}
	return nil
}
