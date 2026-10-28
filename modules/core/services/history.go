package services

import (
	"context"
	"sort"
	"strconv"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/department"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/group"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/userposition"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/logging/domain/aggregates/businessevent"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
)

const (
	UserResource         = "core.user"
	RoleResource         = "core.role"
	GroupResource        = "core.group"
	PermissionResource   = "core.permission"
	DepartmentResource   = "core.department"
	UserPositionResource = "core.user_position"

	UserCreated                 = "core.user.created"
	UserUpdated                 = "core.user.updated"
	UserDeleted                 = "core.user.deleted"
	UserBlocked                 = "core.user.blocked"
	UserUnblocked               = "core.user.unblocked"
	UserPasswordChanged         = "core.user.password_changed"
	UserTemporaryPasswordIssued = "core.user.temporary_password_issued"
	UserOnboardingCompleted     = "core.user.onboarding_completed"
	UserRoleAssigned            = "core.user.role_assigned"
	UserRoleUnassigned          = "core.user.role_unassigned"
	UserPermissionGranted       = "core.user.permission_granted"
	UserPermissionRevoked       = "core.user.permission_revoked"
	RoleCreated                 = "core.role.created"
	RoleUpdated                 = "core.role.updated"
	RoleDeleted                 = "core.role.deleted"
	RolePermissionGranted       = "core.role.permission_granted"
	RolePermissionRevoked       = "core.role.permission_revoked"
	GroupCreated                = "core.group.created"
	GroupUpdated                = "core.group.updated"
	GroupDeleted                = "core.group.deleted"
	GroupMemberAdded            = "core.group.member_added"
	GroupMemberRemoved          = "core.group.member_removed"
	GroupRoleAssigned           = "core.group.role_assigned"
	GroupRoleUnassigned         = "core.group.role_unassigned"
	DepartmentCreated           = "core.department.created"
	DepartmentUpdated           = "core.department.updated"
	DepartmentDeleted           = "core.department.deleted"
	UserPositionCreated         = "core.user_position.created"
	UserPositionUpdated         = "core.user_position.updated"
	UserPositionDeleted         = "core.user_position.deleted"
)

// HistoryDefinitions are the business history event types of the core module. Account, role
// and group changes are security events; the org structure is business data.
func HistoryDefinitions() []history.Definition {
	userRead := permissions.UserRead
	userResource := history.Resource{Name: UserResource, Fields: []history.Field{
		{Name: "first_name", LabelKey: "Users.Single.FirstName", Type: history.ValueString},
		{Name: "last_name", LabelKey: "Users.Single.LastName", Type: history.ValueString},
		{Name: "middle_name", LabelKey: "Users.Single.MiddleName", Type: history.ValueString},
		{Name: "email", LabelKey: "Users.Single.Email", Type: history.ValueString, ReadPermission: &userRead},
		{Name: "phone", LabelKey: "Users.Single.Phone", Type: history.ValueString, ReadPermission: &userRead},
		{Name: "ui_language", LabelKey: "Users.Single.Language", Type: history.ValueString},
		{Name: "status", LabelKey: "Users.List.FilterStatus", Type: history.ValueString},
		{Name: "blocked", LabelKey: "Users.Blocked", Type: history.ValueBoolean},
		{Name: "block_reason", LabelKey: "Users.Block.Reason", Type: history.ValueString},
		{Name: "password_expires_at", LabelKey: "Users.TemporaryPassword.ExpiresAt", Type: history.ValueDateTime},
	}}
	roleResource := history.Resource{Name: RoleResource, Fields: []history.Field{
		{Name: "name", LabelKey: "Roles.Single.Name.Label", Type: history.ValueString},
		{Name: "description", LabelKey: "Roles.Single._Description.Label", Type: history.ValueString},
	}}
	groupResource := history.Resource{Name: GroupResource, Fields: []history.Field{
		{Name: "name", LabelKey: "Groups.Single.Name", Type: history.ValueString},
		{Name: "description", LabelKey: "Groups.Single._Description", Type: history.ValueString},
	}}
	userRef := history.Resource{Name: UserResource}
	roleRef := history.Resource{Name: RoleResource}
	groupRef := history.Resource{Name: GroupResource}
	permissionRef := history.Resource{Name: PermissionResource}

	defs := make([]history.Definition, 0, 30)
	defs = append(defs, securityLifecycle("core.user", "Users.History", userResource)...)
	defs = append(defs, securityLifecycle("core.role", "Roles.History", roleResource)...)
	defs = append(defs, securityLifecycle("core.group", "Groups.History", groupResource)...)
	defs = append(defs,
		securityDefinition(UserBlocked, "Users.History.Blocked", userResource),
		securityDefinition(UserUnblocked, "Users.History.Unblocked", userResource),
		securityDefinition(UserPasswordChanged, "Users.History.PasswordChanged", userRef),
		securityDefinition(UserTemporaryPasswordIssued, "Users.History.TemporaryPasswordIssued", userResource),
		securityDefinition(UserOnboardingCompleted, "Users.History.OnboardingCompleted", userResource),
		securityDefinition(UserRoleAssigned, "Users.History.RoleAssigned", userRef, roleRef),
		securityDefinition(UserRoleUnassigned, "Users.History.RoleUnassigned", userRef, roleRef),
		securityDefinition(UserPermissionGranted, "Users.History.PermissionGranted", userRef, permissionRef),
		securityDefinition(UserPermissionRevoked, "Users.History.PermissionRevoked", userRef, permissionRef),
		securityDefinition(RolePermissionGranted, "Roles.History.PermissionGranted", roleRef, permissionRef),
		securityDefinition(RolePermissionRevoked, "Roles.History.PermissionRevoked", roleRef, permissionRef),
		securityDefinition(GroupMemberAdded, "Groups.History.MemberAdded", groupRef, userRef),
		securityDefinition(GroupMemberRemoved, "Groups.History.MemberRemoved", groupRef, userRef),
		securityDefinition(GroupRoleAssigned, "Groups.History.RoleAssigned", groupRef, roleRef),
		securityDefinition(GroupRoleUnassigned, "Groups.History.RoleUnassigned", groupRef, roleRef),
	)
	defs = append(defs, history.Lifecycle("core.department", "Departments.History", history.Resource{Name: DepartmentResource, Fields: []history.Field{
		{Name: "code", LabelKey: "Departments.Single.Code", Type: history.ValueString},
		{Name: "name", LabelKey: "Departments.Single.Name", Type: history.ValueString},
		{Name: "parent_id", LabelKey: "Departments.Single.Parent", Type: history.ValueString},
		{Name: "order", LabelKey: "Departments.Single.Order", Type: history.ValueInteger},
		{Name: "status", LabelKey: "Departments.Single.Status", Type: history.ValueString},
	}})...)
	defs = append(defs, history.Lifecycle("core.user_position", "UserPositions.History", history.Resource{Name: UserPositionResource, Fields: []history.Field{
		{Name: "title", LabelKey: "Positions.Single.Title", Type: history.ValueString},
		{Name: "user_id", LabelKey: "Positions.Single.User", Type: history.ValueInteger},
		{Name: "department_id", LabelKey: "Positions.Single.Department", Type: history.ValueString},
		{Name: "is_manager", LabelKey: "Positions.Single.IsManager", Type: history.ValueBoolean},
		{Name: "is_primary", LabelKey: "Positions.Single.IsPrimary", Type: history.ValueBoolean},
		{Name: "status", LabelKey: "Positions.Single.Status", Type: history.ValueString},
	}})...)
	return defs
}

func securityLifecycle(typePrefix, keyPrefix string, resource history.Resource) []history.Definition {
	defs := history.Lifecycle(typePrefix, keyPrefix, resource)
	for i := range defs {
		defs[i].Category = businessevent.CategorySecurity
	}
	return defs
}

func securityDefinition(eventType, keyPrefix string, resources ...history.Resource) history.Definition {
	return history.Definition{
		Type:           eventType,
		Version:        1,
		Category:       businessevent.CategorySecurity,
		TitleKey:       keyPrefix + ".Title",
		DescriptionKey: keyPrefix + ".Summary",
		Resources:      resources,
	}
}

func userKey(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}

func memberKey(u user.User) string {
	return userKey(u.ID())
}

func userSnapshot(u user.User) history.Snapshot {
	s := history.Snapshot{
		"first_name":  u.FirstName(),
		"last_name":   u.LastName(),
		"middle_name": u.MiddleName(),
		"ui_language": string(u.UILanguage()),
		"status":      string(u.Status()),
	}
	if u.Email() != nil {
		s["email"] = u.Email().Value()
	}
	if u.Phone() != nil {
		s["phone"] = u.Phone().Value()
	}
	return s
}

func userBlockSnapshot(u user.User) history.Snapshot {
	s := history.Snapshot{"blocked": u.IsBlocked()}
	if u.IsBlocked() {
		s["block_reason"] = u.BlockReason()
	}
	return s
}

func userCredentialsSnapshot(u user.User) history.Snapshot {
	return history.Snapshot{
		"status":              string(u.Status()),
		"password_expires_at": history.DateTime(u.PasswordExpiresAt()),
	}
}

func roleSnapshot(r role.Role) history.Snapshot {
	return history.Snapshot{"name": r.Name(), "description": r.Description()}
}

func groupSnapshot(g group.Group) history.Snapshot {
	return history.Snapshot{"name": g.Name(), "description": g.Description()}
}

func departmentSnapshot(d department.Department) history.Snapshot {
	s := history.Snapshot{
		"code":   d.Code(),
		"order":  d.Order(),
		"status": string(d.Status()),
	}
	if d.NameI18n() != nil {
		s["name"] = d.NameI18n().Default()
	}
	if d.ParentID() != nil {
		s["parent_id"] = history.UUID(*d.ParentID())
	}
	return s
}

func userPositionSnapshot(p userposition.UserPosition) history.Snapshot {
	s := history.Snapshot{
		"user_id":       int(p.UserID()),
		"department_id": history.UUID(p.DepartmentID()),
		"is_manager":    p.IsManager(),
		"is_primary":    p.IsPrimary(),
		"status":        string(p.Status()),
	}
	if p.TitleI18n() != nil {
		s["title"] = p.TitleI18n().Default()
	}
	return s
}

func ids[T any](items []T, id func(T) string) []string {
	result := make([]string, len(items))
	for i, item := range items {
		result[i] = id(item)
	}
	return result
}

func roleKey(r role.Role) string {
	return strconv.FormatUint(uint64(r.ID()), 10)
}

func permissionKey(p permission.Permission) string {
	return p.ID().String()
}

// recordUserAccess records the roles, groups and direct permissions the user gained or lost
// between before and after; before is nil for a new user.
func recordUserAccess(ctx context.Context, w history.Writer, before, after user.User) error {
	subject := businessevent.Subject{Resource: UserResource, ID: userKey(after.ID())}
	var beforeRoles, beforeGroups, beforePerms []string
	if before != nil {
		beforeRoles = ids(before.Roles(), roleKey)
		beforeGroups = ids(before.GroupIDs(), uuid.UUID.String)
		beforePerms = ids(before.Permissions(), permissionKey)
	}
	if err := recordLinks(ctx, w, subject, RoleResource, beforeRoles, ids(after.Roles(), roleKey), UserRoleAssigned, UserRoleUnassigned); err != nil {
		return err
	}
	if err := recordLinks(ctx, w, subject, GroupResource, beforeGroups, ids(after.GroupIDs(), uuid.UUID.String), GroupMemberAdded, GroupMemberRemoved); err != nil {
		return err
	}
	return recordLinks(ctx, w, subject, PermissionResource, beforePerms, ids(after.Permissions(), permissionKey), UserPermissionGranted, UserPermissionRevoked)
}

// recordGroupChange records the group's own changes and the members and roles it gained or
// lost; before is nil for a new group.
func recordGroupChange(ctx context.Context, w history.Writer, before, after group.Group) error {
	m := history.Mutation{Type: GroupUpdated, Version: 1, Resource: GroupResource, ID: after.ID().String(), After: groupSnapshot(after)}
	if before == nil {
		m.Type = GroupCreated
	} else {
		m.Before = groupSnapshot(before)
	}
	if err := history.RecordMutation(ctx, w, m); err != nil {
		return err
	}
	subject := businessevent.Subject{Resource: GroupResource, ID: after.ID().String()}
	var beforeMembers, beforeRoles []string
	if before != nil {
		beforeMembers = ids(before.Users(), memberKey)
		beforeRoles = ids(before.Roles(), roleKey)
	}
	if err := recordLinks(ctx, w, subject, UserResource, beforeMembers, ids(after.Users(), memberKey), GroupMemberAdded, GroupMemberRemoved); err != nil {
		return err
	}
	return recordLinks(ctx, w, subject, RoleResource, beforeRoles, ids(after.Roles(), roleKey), GroupRoleAssigned, GroupRoleUnassigned)
}

// recordRoleChange records the role's own changes and the permissions it gained or lost;
// before is nil for a new role.
func recordRoleChange(ctx context.Context, w history.Writer, before, after role.Role) error {
	id := roleKey(after)
	m := history.Mutation{Type: RoleUpdated, Version: 1, Resource: RoleResource, ID: id, After: roleSnapshot(after)}
	if before == nil {
		m.Type = RoleCreated
	} else {
		m.Before = roleSnapshot(before)
	}
	if err := history.RecordMutation(ctx, w, m); err != nil {
		return err
	}
	var beforePerms []string
	if before != nil {
		beforePerms = ids(before.Permissions(), permissionKey)
	}
	subject := businessevent.Subject{Resource: RoleResource, ID: id}
	return recordLinks(ctx, w, subject, PermissionResource, beforePerms, ids(after.Permissions(), permissionKey), RolePermissionGranted, RolePermissionRevoked)
}

// recordLinks records one event per object of resource linked to or unlinked from subject.
func recordLinks(ctx context.Context, w history.Writer, subject businessevent.Subject, resource string, before, after []string, linked, unlinked string) error {
	was := make(map[string]bool, len(before))
	for _, id := range before {
		was[id] = true
	}
	is := make(map[string]bool, len(after))
	for _, id := range after {
		is[id] = true
	}
	for _, id := range sortedKeys(is) {
		if !was[id] {
			if err := recordEvent(ctx, w, linked, subject, businessevent.Subject{Resource: resource, ID: id}); err != nil {
				return err
			}
		}
	}
	for _, id := range sortedKeys(was) {
		if !is[id] {
			if err := recordEvent(ctx, w, unlinked, subject, businessevent.Subject{Resource: resource, ID: id}); err != nil {
				return err
			}
		}
	}
	return nil
}

func recordEvent(ctx context.Context, w history.Writer, eventType string, subjects ...businessevent.Subject) error {
	_, err := w.Record(ctx, history.Input{
		Type:     eventType,
		Version:  1,
		Outcome:  businessevent.OutcomeSucceeded,
		Subjects: subjects,
	})
	return err
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
