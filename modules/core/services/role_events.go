package services

import (
	"context"
	"strconv"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Versioned IDs of the role events.
const (
	RoleCreatedEventID           = "core.role.created.v1"
	RolePermissionGrantedEventID = "core.role.permission_granted.v1"
)

// NativeRole is the ID of the core.role native entity.
const NativeRole = "core.role"

// RoleEntity describes modules/core `roles` (key: bigint) for the extension
// runtime.
func RoleEntity() data.NativeEntity {
	return data.NativeEntity{
		ID: NativeRole, Table: "roles", KeyColumn: "id", KeyType: data.KeyInt64, TenantColumn: "tenant_id",
		ReadPermission: permissions.RoleRead.Name(),
		Label:          localized("Role", "Роль", "Rol", "Роль"),
		Fields: []data.NativeField{
			{ID: "name", Column: "name", Type: manifest.Type{Kind: manifest.KindString, MaxLength: 255}},
			{ID: "description", Column: "description", Type: manifest.Type{Kind: manifest.KindString, Optional: true, Nullable: true}},
			{ID: "type", Column: "type", Type: manifest.Type{Kind: manifest.KindString, MaxLength: 50}},
			{ID: "createdAt", Column: "created_at", Type: manifest.Type{Kind: manifest.KindDateTime, Optional: true, Nullable: true}},
			{ID: "updatedAt", Column: "updated_at", Type: manifest.Type{Kind: manifest.KindDateTime, Optional: true, Nullable: true}},
		},
	}
}

// RoleCreatedPayload is the payload of core.role.created.v1.
type RoleCreatedPayload struct {
	RoleID int64  `json:"roleId,string" wire:"int64"`
	Name   string `json:"name"`
}

// RolePermissionGrantedPayload is the payload of
// core.role.permission_granted.v1; permission is the RBAC permission name
// ("User.Read").
type RolePermissionGrantedPayload struct {
	RoleID     int64  `json:"roleId,string" wire:"int64"`
	Permission string `json:"permission"`
}

// RoleCreatedEvent is published when a role is created.
var RoleCreatedEvent = native.Event[RoleCreatedPayload](RoleCreatedEventID, native.EventSpec{
	Subject:     NativeRole,
	Label:       localized("Role created", "Роль создана", "Rol yaratildi", "Роль яратилди"),
	Description: "A role was created. Published in the transaction that creates the role.",
	Descriptions: localized(
		"A role was created.",
		"Создана роль.",
		"Rol yaratildi.",
		"Роль яратилди."),
	ReadPermission: permissions.RoleRead.Name(),
	Category:       "core.roles",
})

// RolePermissionGrantedEvent is published for every permission a role gained.
var RolePermissionGrantedEvent = native.Event[RolePermissionGrantedPayload](RolePermissionGrantedEventID, native.EventSpec{
	Subject:     NativeRole,
	Label:       localized("Permission granted to role", "Роли выдано право", "Rolga ruxsat berildi", "Ролга рухсат берилди"),
	Description: "A permission was granted to a role. Published once per added permission in the transaction of the change.",
	Descriptions: localized(
		"A permission was granted to a role.",
		"Роли выдано право.",
		"Rolga ruxsat berildi.",
		"Ролга рухсат берилди."),
	ReadPermission: permissions.RoleRead.Name(),
	Category:       "core.roles",
})

func roleRef(id uint) *spi.ObjectRef {
	return &spi.ObjectRef{Entity: NativeRole, Key: strconv.FormatUint(uint64(id), 10)}
}

// publishRoleCreated enqueues core.role.created.v1.
func publishRoleCreated(ctx context.Context, created role.Role) error {
	return serrors.Wrap("RoleService.publishRoleCreated", events.PublishInTx(ctx, created.TenantID(), RoleCreatedEvent, roleRef(created.ID()), RoleCreatedPayload{
		RoleID: int64(created.ID()), Name: created.Name(),
	}))
}

// publishRolePermissionsGranted enqueues core.role.permission_granted.v1 for
// every permission of after whose key (history diff) was added.
func publishRolePermissionsGranted(ctx context.Context, after role.Role, added []string) error {
	const op = serrors.Op("publishRolePermissionsGranted")
	wanted := make(map[string]bool, len(added))
	for _, key := range added {
		wanted[key] = true
	}
	for _, p := range after.Permissions() {
		if !wanted[permissionKey(p)] {
			continue
		}
		if err := publishRolePermission(ctx, after, p); err != nil {
			return serrors.Wrap(op, err)
		}
	}
	return nil
}

func publishRolePermission(ctx context.Context, r role.Role, p permission.Permission) error {
	return events.PublishInTx(ctx, r.TenantID(), RolePermissionGrantedEvent, roleRef(r.ID()), RolePermissionGrantedPayload{
		RoleID: int64(r.ID()), Permission: p.Name(),
	})
}
