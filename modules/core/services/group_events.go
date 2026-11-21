package services

import (
	"context"
	"strconv"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/group"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Versioned IDs of the group events.
const (
	GroupCreatedEventID       = "core.group.created.v1"
	GroupMemberAddedEventID   = "core.group.member_added.v1"
	GroupMemberRemovedEventID = "core.group.member_removed.v1"
)

// NativeGroup is the ID of the core.group native entity.
const NativeGroup = "core.group"

// GroupEntity describes modules/core `user_groups` (key: uuid) for the
// extension runtime.
func GroupEntity() data.NativeEntity {
	return data.NativeEntity{
		ID: NativeGroup, Table: "user_groups", KeyColumn: "id", KeyType: data.KeyUUID, TenantColumn: "tenant_id",
		ReadPermission: permissions.GroupRead.Name(),
		Label:          localized("Group", "Группа", "Guruh", "Гуруҳ"),
		Fields: []data.NativeField{
			{ID: "name", Column: "name", Type: manifest.Type{Kind: manifest.KindString, MaxLength: 255}},
			{ID: "description", Column: "description", Type: manifest.Type{Kind: manifest.KindString, Optional: true, Nullable: true}},
			{ID: "type", Column: "type", Type: manifest.Type{Kind: manifest.KindString, MaxLength: 50}},
			{ID: "createdAt", Column: "created_at", Type: manifest.Type{Kind: manifest.KindDateTime, Optional: true, Nullable: true}},
			{ID: "updatedAt", Column: "updated_at", Type: manifest.Type{Kind: manifest.KindDateTime, Optional: true, Nullable: true}},
		},
	}
}

// GroupCreatedPayload is the payload of core.group.created.v1.
type GroupCreatedPayload struct {
	GroupID uuid.UUID `json:"groupId"`
	Name    string    `json:"name"`
}

// GroupMemberPayload is the payload of core.group.member_added.v1 and
// core.group.member_removed.v1.
type GroupMemberPayload struct {
	GroupID   uuid.UUID `json:"groupId"`
	UserID    int64     `json:"userId,string" wire:"int64"`
	GroupName string    `json:"groupName"`
}

// GroupCreatedEvent is published when a group is created.
var GroupCreatedEvent = native.Event[GroupCreatedPayload](GroupCreatedEventID, native.EventSpec{
	Subject:     NativeGroup,
	Label:       localized("Group created", "Группа создана", "Guruh yaratildi", "Гуруҳ яратилди"),
	Description: "A user group was created. Published in the transaction that creates the group.",
	Descriptions: localized(
		"A user group was created.",
		"Создана группа пользователей.",
		"Foydalanuvchilar guruhi yaratildi.",
		"Фойдаланувчилар гуруҳи яратилди."),
	ReadPermission: permissions.GroupRead.Name(),
	Category:       "core.groups",
})

// GroupMemberAddedEvent is published for every user a group gained, whether
// the change was made on the group or on the user form.
var GroupMemberAddedEvent = native.Event[GroupMemberPayload](GroupMemberAddedEventID, native.EventSpec{
	Subject:     NativeGroup,
	Label:       localized("User added to group", "Пользователь добавлен в группу", "Foydalanuvchi guruhga qo'shildi", "Фойдаланувчи гуруҳга қўшилди"),
	Description: "A user joined a group. Published once per added member in the transaction of the change, on the group or on the user form.",
	Descriptions: localized(
		"A user joined a group.",
		"Пользователь добавлен в группу.",
		"Foydalanuvchi guruhga qo'shildi.",
		"Фойдаланувчи гуруҳга қўшилди."),
	ReadPermission: permissions.GroupRead.Name(),
	Category:       "core.groups",
})

// GroupMemberRemovedEvent is published for every user a group lost.
var GroupMemberRemovedEvent = native.Event[GroupMemberPayload](GroupMemberRemovedEventID, native.EventSpec{
	Subject:     NativeGroup,
	Label:       localized("User removed from group", "Пользователь удалён из группы", "Foydalanuvchi guruhdan chiqarildi", "Фойдаланувчи гуруҳдан чиқарилди"),
	Description: "A user left a group. Published once per removed member in the transaction of the change, on the group or on the user form.",
	Descriptions: localized(
		"A user left a group.",
		"Пользователь удалён из группы.",
		"Foydalanuvchi guruhdan chiqarildi.",
		"Фойдаланувчи гуруҳдан чиқарилди."),
	ReadPermission: permissions.GroupRead.Name(),
	Category:       "core.groups",
})

func groupRef(id uuid.UUID) *spi.ObjectRef {
	return &spi.ObjectRef{Entity: NativeGroup, Key: id.String()}
}

// publishGroupCreated enqueues core.group.created.v1.
func publishGroupCreated(ctx context.Context, created group.Group) error {
	return serrors.Wrap("GroupService.publishGroupCreated", events.PublishInTx(ctx, created.TenantID(), GroupCreatedEvent, groupRef(created.ID()), GroupCreatedPayload{
		GroupID: created.ID(), Name: created.Name(),
	}))
}

// publishGroupMember enqueues core.group.member_added.v1 (added) or
// core.group.member_removed.v1 for one member of a group of the tenant.
func publishGroupMember(ctx context.Context, tenantID, groupID uuid.UUID, groupName string, userID int64, added bool) error {
	ev := GroupMemberRemovedEvent
	if added {
		ev = GroupMemberAddedEvent
	}
	return serrors.Wrap("publishGroupMember", events.PublishInTx(ctx, tenantID, ev, groupRef(groupID), GroupMemberPayload{
		GroupID: groupID, UserID: userID, GroupName: groupName,
	}))
}

// publishGroupMembers publishes the member events of a group's diff: member
// is the history key (decimal user ID) of every added and removed member.
func publishGroupMembers(ctx context.Context, after group.Group, added, removed []string) error {
	const op = serrors.Op("publishGroupMembers")
	for _, set := range []struct {
		keys  []string
		added bool
	}{{added, true}, {removed, false}} {
		for _, key := range set.keys {
			userID, err := strconv.ParseInt(key, 10, 64)
			if err != nil {
				return serrors.Wrap(op, err)
			}
			if err := publishGroupMember(ctx, after.TenantID(), after.ID(), after.Name(), userID, set.added); err != nil {
				return serrors.Wrap(op, err)
			}
		}
	}
	return nil
}

// groupNamer resolves the name of a group of the context's tenant.
type groupNamer func(ctx context.Context, id uuid.UUID) (string, error)

// publishUserGroups publishes the member events of a user's group diff, the
// same events recordGroupChange publishes when the group is changed itself, so
// a group membership changed on the user form is never missed.
func publishUserGroups(ctx context.Context, tenantID uuid.UUID, userID int64, name groupNamer, added, removed []string) error {
	const op = serrors.Op("publishUserGroups")
	for _, set := range []struct {
		keys  []string
		added bool
	}{{added, true}, {removed, false}} {
		for _, key := range set.keys {
			groupID, err := uuid.Parse(key)
			if err != nil {
				return serrors.Wrap(op, err)
			}
			groupName, err := name(ctx, groupID)
			if err != nil {
				return serrors.Wrap(op, err)
			}
			if err := publishGroupMember(ctx, tenantID, groupID, groupName, userID, set.added); err != nil {
				return serrors.Wrap(op, err)
			}
		}
	}
	return nil
}

// groupName resolves the name of a group of the context's tenant.
func (p *PrivilegeGrantPolicy) groupName(ctx context.Context, id uuid.UUID) (string, error) {
	g, err := p.groups.GetByID(ctx, id)
	if err != nil {
		return "", err
	}
	return g.Name(), nil
}
