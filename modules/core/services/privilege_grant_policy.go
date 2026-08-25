package services

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/group"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/authorization"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	corepermissions "github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/sirupsen/logrus"
)

const (
	denialGrantCeiling    = "Authorization.Errors.GrantCeiling"
	denialTargetDominance = "Authorization.Errors.TargetDominance"
	denialTenantBoundary  = "Authorization.Errors.TenantBoundary"
	denialSystemRole      = "Authorization.Errors.SystemRole"
	denialSystemGroup     = "Authorization.Errors.SystemGroup"
	denialSelfAdmin       = "Authorization.Errors.SelfAdmin"
	denialInvalidChoice   = "Authorization.Errors.InvalidSelection"
)

// PrivilegeDeniedError is safe to expose to an administrator. It intentionally
// contains no role names, permission names, tenant IDs, or submitted payloads.
type PrivilegeDeniedError struct {
	Code      string
	LocaleKey string
	Action    string
}

func (e *PrivilegeDeniedError) Error() string { return e.Code }

func (e *PrivilegeDeniedError) Is(target error) bool {
	return target == composables.ErrForbidden
}

type PrivilegeGrantPolicy struct {
	locks          authorization.Repository
	users          user.Repository
	roles          role.Repository
	groups         group.Repository
	permissions    permission.Repository
	securityLogger *logrus.Logger
}

func NewPrivilegeGrantPolicy(
	locks authorization.Repository,
	users user.Repository,
	roles role.Repository,
	groups group.Repository,
	permissions permission.Repository,
	logger *logrus.Logger,
) *PrivilegeGrantPolicy {
	return &PrivilegeGrantPolicy{
		locks: locks, users: users, roles: roles, groups: groups,
		permissions: permissions, securityLogger: logger,
	}
}

func (p *PrivilegeGrantPolicy) deny(actor user.User, action, targetType, targetID, code, localeKey string) error {
	fields := logrus.Fields{
		"security_event": "administrative_action_denied",
		"action":         action,
		"target_type":    targetType,
		"reason_code":    code,
	}
	if actor != nil {
		fields["actor_id"] = actor.ID()
	}
	if targetID != "" {
		fields["target_id"] = targetID
	}
	if p.securityLogger != nil {
		p.securityLogger.WithFields(fields).Warn("administrative action denied")
	}
	return &PrivilegeDeniedError{Code: code, LocaleKey: localeKey, Action: action}
}

func IsPrivilegeDenied(err error) bool {
	var denied *PrivilegeDeniedError
	return errors.As(err, &denied)
}

func PrivilegeDenialLocaleKey(err error) string {
	var denied *PrivilegeDeniedError
	if errors.As(err, &denied) {
		return denied.LocaleKey
	}
	return ""
}

func Dominates(actorPermissions, requestedPermissions []permission.Permission) bool {
	for _, requested := range requestedPermissions {
		covered := false
		for _, held := range actorPermissions {
			if held.Resource() != requested.Resource() || held.Action() != requested.Action() {
				continue
			}
			if held.Modifier() == permission.ModifierAll || held.Modifier() == requested.Modifier() {
				covered = true
				break
			}
		}
		if !covered {
			return false
		}
	}
	return true
}

func (p *PrivilegeGrantPolicy) actor(ctx context.Context, additionalUserIDs ...uint) (user.User, error) {
	contextActor, err := composables.UseUser(ctx)
	if err != nil {
		return nil, serrors.E(serrors.Op("PrivilegeGrantPolicy.actor"), serrors.PermissionDenied, err)
	}
	if err := p.locks.LockTenant(ctx); err != nil {
		return nil, err
	}
	ids := append([]uint{contextActor.ID()}, additionalUserIDs...)
	if err := p.locks.LockUsers(ctx, ids...); err != nil {
		return nil, err
	}
	actor, err := p.users.GetByID(ctx, contextActor.ID())
	if err != nil {
		return nil, err
	}

	roleIDs := make([]uint, 0, len(actor.Roles()))
	for _, assignedRole := range actor.Roles() {
		roleIDs = append(roleIDs, assignedRole.ID())
	}
	if err := p.locks.LockRoles(ctx, roleIDs...); err != nil {
		return nil, err
	}
	if err := p.locks.LockGroups(ctx, actor.GroupIDs()...); err != nil {
		return nil, err
	}
	for _, groupID := range actor.GroupIDs() {
		assignedGroup, err := p.groups.GetByID(ctx, groupID)
		if err != nil {
			return nil, err
		}
		for _, assignedRole := range assignedGroup.Roles() {
			roleIDs = append(roleIDs, assignedRole.ID())
		}
	}
	if err := p.locks.LockRoles(ctx, roleIDs...); err != nil {
		return nil, err
	}

	return p.users.GetByID(ctx, contextActor.ID())
}

func (p *PrivilegeGrantPolicy) validateTenant(ctx context.Context, actor user.User, tenantID uuid.UUID, action, targetType, targetID string) error {
	contextTenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return err
	}
	if actor.TenantID() != contextTenantID || tenantID != contextTenantID {
		return p.deny(actor, action, targetType, targetID, "tenant_boundary", denialTenantBoundary)
	}
	return nil
}

func (p *PrivilegeGrantPolicy) resolveRoles(ctx context.Context, actor user.User, requested []role.Role, action, targetType, targetID string) ([]role.Role, error) {
	ids := make([]uint, 0, len(requested))
	seen := make(map[uint]struct{}, len(requested))
	for _, candidate := range requested {
		if candidate == nil || candidate.ID() == 0 {
			return nil, p.deny(actor, action, targetType, targetID, "invalid_selection", denialInvalidChoice)
		}
		if _, ok := seen[candidate.ID()]; ok {
			return nil, p.deny(actor, action, targetType, targetID, "invalid_selection", denialInvalidChoice)
		}
		seen[candidate.ID()] = struct{}{}
		ids = append(ids, candidate.ID())
	}
	if err := p.locks.LockRoles(ctx, ids...); err != nil {
		return nil, err
	}
	resolved := make([]role.Role, 0, len(ids))
	for _, id := range ids {
		entity, err := p.roles.GetByID(ctx, id)
		if err != nil {
			return nil, p.deny(actor, action, targetType, targetID, "invalid_selection", denialInvalidChoice)
		}
		if entity.Type() == role.TypeSystem && !actor.Can(corepermissions.RoleAssignSystem) {
			return nil, p.deny(actor, action, targetType, targetID, "system_role", denialSystemRole)
		}
		resolved = append(resolved, entity)
	}
	return resolved, nil
}

func (p *PrivilegeGrantPolicy) resolveGroups(ctx context.Context, actor user.User, requested []uuid.UUID, action, targetType, targetID string) ([]group.Group, error) {
	seen := make(map[uuid.UUID]struct{}, len(requested))
	for _, id := range requested {
		if id == uuid.Nil {
			return nil, p.deny(actor, action, targetType, targetID, "invalid_selection", denialInvalidChoice)
		}
		if _, ok := seen[id]; ok {
			return nil, p.deny(actor, action, targetType, targetID, "invalid_selection", denialInvalidChoice)
		}
		seen[id] = struct{}{}
	}
	if err := p.locks.LockGroups(ctx, requested...); err != nil {
		return nil, err
	}
	resolved := make([]group.Group, 0, len(requested))
	for _, id := range requested {
		entity, err := p.groups.GetByID(ctx, id)
		if err != nil {
			return nil, p.deny(actor, action, targetType, targetID, "invalid_selection", denialInvalidChoice)
		}
		if entity.Type() == group.TypeSystem && !actor.Can(corepermissions.GroupAssignSystem) {
			return nil, p.deny(actor, action, targetType, targetID, "system_group", denialSystemGroup)
		}
		resolved = append(resolved, entity)
	}
	return resolved, nil
}

func (p *PrivilegeGrantPolicy) resolvePermissions(ctx context.Context, actor user.User, requested []permission.Permission, action, targetType, targetID string) ([]permission.Permission, error) {
	seen := make(map[uuid.UUID]struct{}, len(requested))
	resolved := make([]permission.Permission, 0, len(requested))
	for _, candidate := range requested {
		if candidate == nil || candidate.ID() == uuid.Nil {
			return nil, p.deny(actor, action, targetType, targetID, "invalid_selection", denialInvalidChoice)
		}
		if _, ok := seen[candidate.ID()]; ok {
			return nil, p.deny(actor, action, targetType, targetID, "invalid_selection", denialInvalidChoice)
		}
		seen[candidate.ID()] = struct{}{}
		entity, err := p.permissions.GetByID(ctx, candidate.ID().String())
		if err != nil {
			return nil, p.deny(actor, action, targetType, targetID, "invalid_selection", denialInvalidChoice)
		}
		resolved = append(resolved, entity)
	}
	return resolved, nil
}

func permissionsForRoles(roles []role.Role) []permission.Permission {
	result := make([]permission.Permission, 0)
	for _, entity := range roles {
		result = append(result, entity.Permissions()...)
	}
	return result
}

func permissionsForGroups(groups []group.Group) []permission.Permission {
	result := make([]permission.Permission, 0)
	for _, entity := range groups {
		result = append(result, permissionsForRoles(entity.Roles())...)
	}
	return result
}

func (p *PrivilegeGrantPolicy) canonicalUser(ctx context.Context, actor user.User, desired user.User, action string) (user.User, error) {
	targetID := strconv.FormatUint(uint64(desired.ID()), 10)
	if err := p.validateTenant(ctx, actor, desired.TenantID(), action, "user", targetID); err != nil {
		return nil, err
	}
	roles, err := p.resolveRoles(ctx, actor, desired.Roles(), action, "user", targetID)
	if err != nil {
		return nil, err
	}
	groups, err := p.resolveGroups(ctx, actor, desired.GroupIDs(), action, "user", targetID)
	if err != nil {
		return nil, err
	}
	direct, err := p.resolvePermissions(ctx, actor, desired.Permissions(), action, "user", targetID)
	if err != nil {
		return nil, err
	}
	postState := append([]permission.Permission{}, direct...)
	postState = append(postState, permissionsForRoles(roles)...)
	postState = append(postState, permissionsForGroups(groups)...)
	if !Dominates(user.EffectivePermissions(actor), postState) {
		return nil, p.deny(actor, action, "user", targetID, "grant_ceiling", denialGrantCeiling)
	}
	return desired.SetRoles(roles).SetPermissions(direct), nil
}

func (p *PrivilegeGrantPolicy) AuthorizeUserCreate(ctx context.Context, desired user.User) (user.User, error) {
	actor, err := p.actor(ctx)
	if err != nil {
		return nil, err
	}
	return p.canonicalUser(ctx, actor, desired, "user.create")
}

func (p *PrivilegeGrantPolicy) AuthorizeUserUpdate(ctx context.Context, desired user.User) (user.User, error) {
	actor, err := p.actor(ctx, desired.ID())
	if err != nil {
		return nil, err
	}
	if actor.ID() == desired.ID() {
		return nil, p.deny(actor, "user.update", "user", strconv.FormatUint(uint64(desired.ID()), 10), "self_admin_route", denialSelfAdmin)
	}
	current, err := p.users.GetByID(ctx, desired.ID())
	if err != nil {
		return nil, err
	}
	if current.Type() == user.TypeSystem || !Dominates(user.EffectivePermissions(actor), user.EffectivePermissions(current)) {
		return nil, p.deny(actor, "user.update", "user", strconv.FormatUint(uint64(desired.ID()), 10), "target_dominance", denialTargetDominance)
	}
	return p.canonicalUser(ctx, actor, desired, "user.update")
}

// AuthorizeSelfUpdate joins the self-service path to the same tenant lock used
// by administrative privilege changes and returns the canonical latest row.
// The caller may copy only self-editable fields onto this authorization state.
func (p *PrivilegeGrantPolicy) AuthorizeSelfUpdate(ctx context.Context, desired user.User) (user.User, error) {
	actor, err := p.actor(ctx, desired.ID())
	if err != nil {
		return nil, err
	}
	if actor.ID() != desired.ID() {
		return nil, p.deny(actor, "user.update_self", "user", strconv.FormatUint(uint64(desired.ID()), 10), "self_admin_route", denialSelfAdmin)
	}
	if err := p.validateTenant(ctx, actor, desired.TenantID(), "user.update_self", "user", strconv.FormatUint(uint64(desired.ID()), 10)); err != nil {
		return nil, err
	}
	return p.users.GetByID(ctx, actor.ID())
}

func (p *PrivilegeGrantPolicy) AuthorizeUserTarget(ctx context.Context, userID uint, action string) (user.User, error) {
	actor, err := p.actor(ctx, userID)
	if err != nil {
		return nil, err
	}
	target, err := p.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if err := p.validateTenant(ctx, actor, target.TenantID(), action, "user", strconv.FormatUint(uint64(userID), 10)); err != nil {
		return nil, err
	}
	if actor.ID() == target.ID() || target.Type() == user.TypeSystem || !Dominates(user.EffectivePermissions(actor), user.EffectivePermissions(target)) {
		return nil, p.deny(actor, action, "user", strconv.FormatUint(uint64(userID), 10), "target_dominance", denialTargetDominance)
	}
	return target, nil
}

func (p *PrivilegeGrantPolicy) AuthorizeRoleCreate(ctx context.Context, desired role.Role) (role.Role, error) {
	actor, err := p.actor(ctx)
	if err != nil {
		return nil, err
	}
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, err
	}
	desired = desired.SetTenantID(tenantID)
	resolved, err := p.resolvePermissions(ctx, actor, desired.Permissions(), "role.create", "role", "")
	if err != nil {
		return nil, err
	}
	if !Dominates(user.EffectivePermissions(actor), resolved) {
		return nil, p.deny(actor, "role.create", "role", "", "grant_ceiling", denialGrantCeiling)
	}
	return desired.SetPermissions(resolved), nil
}

func (p *PrivilegeGrantPolicy) AuthorizeRoleUpdate(ctx context.Context, desired role.Role) (role.Role, error) {
	actor, err := p.actor(ctx)
	if err != nil {
		return nil, err
	}
	if err := p.locks.LockRoles(ctx, desired.ID()); err != nil {
		return nil, err
	}
	current, err := p.roles.GetByID(ctx, desired.ID())
	if err != nil {
		return nil, err
	}
	if current.Type() == role.TypeSystem || !Dominates(user.EffectivePermissions(actor), current.Permissions()) {
		return nil, p.deny(actor, "role.update", "role", strconv.FormatUint(uint64(desired.ID()), 10), "target_dominance", denialTargetDominance)
	}
	resolved, err := p.resolvePermissions(ctx, actor, desired.Permissions(), "role.update", "role", strconv.FormatUint(uint64(desired.ID()), 10))
	if err != nil {
		return nil, err
	}
	if !Dominates(user.EffectivePermissions(actor), resolved) {
		return nil, p.deny(actor, "role.update", "role", strconv.FormatUint(uint64(desired.ID()), 10), "grant_ceiling", denialGrantCeiling)
	}
	return desired.SetTenantID(current.TenantID()).SetPermissions(resolved), nil
}

func (p *PrivilegeGrantPolicy) AuthorizeRoleDelete(ctx context.Context, id uint) (role.Role, error) {
	actor, err := p.actor(ctx)
	if err != nil {
		return nil, err
	}
	if err := p.locks.LockRoles(ctx, id); err != nil {
		return nil, err
	}
	target, err := p.roles.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if target.Type() == role.TypeSystem || !Dominates(user.EffectivePermissions(actor), target.Permissions()) {
		return nil, p.deny(actor, "role.delete", "role", strconv.FormatUint(uint64(id), 10), "target_dominance", denialTargetDominance)
	}
	return target, nil
}

func (p *PrivilegeGrantPolicy) canonicalGroup(ctx context.Context, actor user.User, desired group.Group, action string) (group.Group, error) {
	targetID := desired.ID().String()
	if err := p.validateTenant(ctx, actor, desired.TenantID(), action, "group", targetID); err != nil {
		return nil, err
	}
	roles, err := p.resolveRoles(ctx, actor, desired.Roles(), action, "group", targetID)
	if err != nil {
		return nil, err
	}
	if !Dominates(user.EffectivePermissions(actor), permissionsForRoles(roles)) {
		return nil, p.deny(actor, action, "group", targetID, "grant_ceiling", denialGrantCeiling)
	}
	return desired.SetRoles(roles), nil
}

func (p *PrivilegeGrantPolicy) AuthorizeGroupCreate(ctx context.Context, desired group.Group) (group.Group, error) {
	actor, err := p.actor(ctx)
	if err != nil {
		return nil, err
	}
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, err
	}
	return p.canonicalGroup(ctx, actor, desired.SetTenantID(tenantID), "group.create")
}

func (p *PrivilegeGrantPolicy) AuthorizeGroupUpdate(ctx context.Context, desired group.Group) (group.Group, group.Group, error) {
	actor, err := p.actor(ctx)
	if err != nil {
		return nil, nil, err
	}
	if err := p.locks.LockGroups(ctx, desired.ID()); err != nil {
		return nil, nil, err
	}
	current, err := p.groups.GetByID(ctx, desired.ID())
	if err != nil {
		return nil, nil, err
	}
	if current.Type() == group.TypeSystem || !Dominates(user.EffectivePermissions(actor), permissionsForRoles(current.Roles())) {
		return nil, nil, p.deny(actor, "group.update", "group", desired.ID().String(), "target_dominance", denialTargetDominance)
	}
	canonical, err := p.canonicalGroup(ctx, actor, desired.SetTenantID(current.TenantID()), "group.update")
	return current, canonical, err
}

func (p *PrivilegeGrantPolicy) AuthorizeGroupDelete(ctx context.Context, id uuid.UUID) (group.Group, error) {
	actor, err := p.actor(ctx)
	if err != nil {
		return nil, err
	}
	if err := p.locks.LockGroups(ctx, id); err != nil {
		return nil, err
	}
	target, err := p.groups.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if target.Type() == group.TypeSystem || !Dominates(user.EffectivePermissions(actor), permissionsForRoles(target.Roles())) {
		return nil, p.deny(actor, "group.delete", "group", id.String(), "target_dominance", denialTargetDominance)
	}
	return target, nil
}

func (p *PrivilegeGrantPolicy) AuthorizeGroupMembership(ctx context.Context, groupID uuid.UUID, userID uint, add bool) (group.Group, user.User, error) {
	action := "group.remove_user"
	if add {
		action = "group.add_user"
	}
	actor, err := p.actor(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	if err := p.locks.LockGroups(ctx, groupID); err != nil {
		return nil, nil, err
	}
	targetGroup, err := p.groups.GetByID(ctx, groupID)
	if err != nil {
		return nil, nil, err
	}
	targetUser, err := p.users.GetByID(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	actorPermissions := user.EffectivePermissions(actor)
	if targetGroup.Type() == group.TypeSystem || !Dominates(actorPermissions, permissionsForRoles(targetGroup.Roles())) || !Dominates(actorPermissions, user.EffectivePermissions(targetUser)) {
		return nil, nil, p.deny(actor, action, "group", groupID.String(), "target_dominance", denialTargetDominance)
	}
	if add {
		postState := append([]permission.Permission{}, user.EffectivePermissions(targetUser)...)
		postState = append(postState, permissionsForRoles(targetGroup.Roles())...)
		if !Dominates(actorPermissions, postState) {
			return nil, nil, p.deny(actor, action, "user", strconv.FormatUint(uint64(userID), 10), "grant_ceiling", denialGrantCeiling)
		}
	}
	return targetGroup, targetUser, nil
}

func (p *PrivilegeGrantPolicy) AuthorizeGroupRoleChange(ctx context.Context, groupID uuid.UUID, roleID uint, add bool) (group.Group, role.Role, error) {
	action := "group.remove_role"
	if add {
		action = "group.assign_role"
	}
	actor, err := p.actor(ctx)
	if err != nil {
		return nil, nil, err
	}
	if err := p.locks.LockGroups(ctx, groupID); err != nil {
		return nil, nil, err
	}
	resolvedRoles, err := p.resolveRoles(ctx, actor, []role.Role{role.New("", role.WithID(roleID))}, action, "group", groupID.String())
	if err != nil {
		return nil, nil, err
	}
	target, err := p.groups.GetByID(ctx, groupID)
	if err != nil {
		return nil, nil, err
	}
	actorPermissions := user.EffectivePermissions(actor)
	if target.Type() == group.TypeSystem || !Dominates(actorPermissions, permissionsForRoles(target.Roles())) {
		return nil, nil, p.deny(actor, action, "group", groupID.String(), "target_dominance", denialTargetDominance)
	}
	if add {
		postState := append(permissionsForRoles(target.Roles()), resolvedRoles[0].Permissions()...)
		if !Dominates(actorPermissions, postState) {
			return nil, nil, p.deny(actor, action, "group", groupID.String(), "grant_ceiling", denialGrantCeiling)
		}
	}
	return target, resolvedRoles[0], nil
}

func (p *PrivilegeGrantPolicy) CanGrantRole(actor user.User, candidate role.Role) bool {
	if actor == nil || candidate == nil {
		return false
	}
	if candidate.Type() == role.TypeSystem && !actor.Can(corepermissions.RoleAssignSystem) {
		return false
	}
	return Dominates(user.EffectivePermissions(actor), candidate.Permissions())
}

func (p *PrivilegeGrantPolicy) CanGrantPermission(actor user.User, candidate permission.Permission) bool {
	return actor != nil && candidate != nil && Dominates(user.EffectivePermissions(actor), []permission.Permission{candidate})
}

func (p *PrivilegeGrantPolicy) CanGrantGroup(actor user.User, candidate group.Group) bool {
	if actor == nil || candidate == nil {
		return false
	}
	if candidate.Type() == group.TypeSystem && !actor.Can(corepermissions.GroupAssignSystem) {
		return false
	}
	return Dominates(user.EffectivePermissions(actor), permissionsForRoles(candidate.Roles()))
}

func (p *PrivilegeGrantPolicy) CanGrantGroupID(ctx context.Context, actor user.User, id uuid.UUID) (bool, error) {
	candidate, err := p.groups.GetByID(ctx, id)
	if err != nil {
		return false, err
	}
	return p.CanGrantGroup(actor, candidate), nil
}

func (p *PrivilegeGrantPolicy) CanManageUser(actor user.User, target user.User) bool {
	if actor == nil || target == nil || actor.ID() == target.ID() || target.Type() == user.TypeSystem {
		return false
	}
	return actor.TenantID() == target.TenantID() && Dominates(user.EffectivePermissions(actor), user.EffectivePermissions(target))
}

func (p *PrivilegeGrantPolicy) CanManageRole(actor user.User, target role.Role) bool {
	if actor == nil || target == nil || target.Type() == role.TypeSystem {
		return false
	}
	return actor.TenantID() == target.TenantID() && Dominates(user.EffectivePermissions(actor), target.Permissions())
}

func (p *PrivilegeGrantPolicy) CanManageGroup(actor user.User, target group.Group) bool {
	if actor == nil || target == nil || target.Type() == group.TypeSystem {
		return false
	}
	return actor.TenantID() == target.TenantID() && Dominates(user.EffectivePermissions(actor), permissionsForRoles(target.Roles()))
}

func (p *PrivilegeGrantPolicy) InvalidSelection(ctx context.Context, action, targetType, targetID string) error {
	actor, err := composables.UseUser(ctx)
	if err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}
	return p.deny(actor, action, targetType, targetID, "invalid_selection", denialInvalidChoice)
}
