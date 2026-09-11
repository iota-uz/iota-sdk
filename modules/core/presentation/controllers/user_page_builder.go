package controllers

import (
	"context"
	"io"
	"strconv"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/group"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/users"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type userCreateFormState struct {
	DTO    *dtos.CreateUserDTO
	Errors map[string]string
}

type userEditFormState struct {
	DTO    *dtos.UpdateUserDTO
	Errors map[string]string
}

type userFormOptionSelection struct {
	selectedRoleIDs  []uint
	selectedGroupIDs []string
	retainedRoleIDs  []uint
	retainedGroupIDs []string
}

type userFormOptions struct {
	roles            []*viewmodels.Role
	groups           []*viewmodels.Group
	selectedRoles    []*viewmodels.Role
	selectedGroupIDs []string
}

func loadUserFormOptions(
	ctx context.Context,
	roleQueryService *services.RoleQueryService,
	groupQueryService *services.GroupQueryService,
	policy *services.PrivilegeGrantPolicy,
	selection userFormOptionSelection,
) (*userFormOptions, error) {
	const op = serrors.Op("controllers.loadUserFormOptions")
	roleOptions, err := roleQueryService.FindAssignmentOptions(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	groupOptions, err := groupQueryService.FindAssignmentOptions(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	actor, err := composables.UseUser(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	selectedRoles := uintSet(selection.selectedRoleIDs)
	retainedRoles := uintSet(selection.retainedRoleIDs)
	selectedGroups := stringSet(selection.selectedGroupIDs)
	retainedGroups := stringSet(selection.retainedGroupIDs)
	result := &userFormOptions{
		roles: make([]*viewmodels.Role, 0, len(roleOptions)), groups: make([]*viewmodels.Group, 0, len(groupOptions)),
		selectedRoles: make([]*viewmodels.Role, 0, len(selectedRoles)), selectedGroupIDs: make([]string, 0, len(selectedGroups)),
	}
	for _, option := range roleOptions {
		id64, parseErr := strconv.ParseUint(option.ID, 10, 64)
		if parseErr != nil {
			continue
		}
		id := uint(id64)
		if !policy.CanGrantRoleOption(actor, role.Type(option.Type), option.Permissions) {
			if _, keep := retainedRoles[id]; !keep {
				continue
			}
		}
		vm := option.Role()
		result.roles = append(result.roles, vm)
		if _, selected := selectedRoles[id]; selected {
			result.selectedRoles = append(result.selectedRoles, vm)
		}
	}
	for _, option := range groupOptions {
		if !policy.CanGrantGroupOption(actor, group.Type(option.Type), option.Permissions) {
			if _, keep := retainedGroups[option.ID]; !keep {
				continue
			}
		}
		result.groups = append(result.groups, option.Group())
		if _, selected := selectedGroups[option.ID]; selected {
			result.selectedGroupIDs = append(result.selectedGroupIDs, option.ID)
		}
	}
	return result, nil
}

func uintSet(values []uint) map[uint]struct{} {
	result := make(map[uint]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func stringSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func (c *UsersController) selectedPermissionsFromIDs(permissionIDs []string) []permission.Permission {
	if len(permissionIDs) == 0 || c.permissionSchema == nil {
		return nil
	}

	requested := make(map[string]struct{}, len(permissionIDs))
	for _, permissionID := range permissionIDs {
		requested[permissionID] = struct{}{}
	}

	selected := make([]permission.Permission, 0, len(permissionIDs))
	seen := make(map[string]struct{}, len(permissionIDs))

	for _, set := range c.permissionSchema.Sets {
		for _, perm := range set.Permissions {
			permissionID := perm.ID().String()
			if _, ok := requested[permissionID]; !ok {
				continue
			}
			if _, ok := seen[permissionID]; ok {
				continue
			}
			seen[permissionID] = struct{}{}
			selected = append(selected, perm)
		}
	}

	return selected
}

func (c *UsersController) buildCreateFormProps(
	ctx context.Context,
	roleQueryService *services.RoleQueryService,
	groupQueryService *services.GroupQueryService,
	policy *services.PrivilegeGrantPolicy,
	state *userCreateFormState,
) (*users.CreateFormProps, error) {
	const op = serrors.Op("controllers.buildCreateFormProps")

	selection := userFormOptionSelection{}
	if state != nil && state.DTO != nil {
		selection.selectedRoleIDs = state.DTO.RoleIDs
		selection.selectedGroupIDs = state.DTO.GroupIDs
	}
	options, err := loadUserFormOptions(ctx, roleQueryService, groupQueryService, policy, selection)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	userViewModel := viewmodels.User{}
	errors := map[string]string{}

	if state != nil {
		errors = state.Errors
		if state.DTO != nil {
			userViewModel = viewmodels.User{
				FirstName:  state.DTO.FirstName,
				LastName:   state.DTO.LastName,
				MiddleName: state.DTO.MiddleName,
				Email:      state.DTO.Email,
				Phone:      state.DTO.Phone,
				Roles:      options.selectedRoles,
				GroupIDs:   options.selectedGroupIDs,
				Language:   state.DTO.Language,
				AvatarID:   strconv.FormatUint(uint64(state.DTO.AvatarID), 10),
			}
		}
	}

	return &users.CreateFormProps{
		User:                     userViewModel,
		Roles:                    options.roles,
		Groups:                   options.groups,
		ResourcePermissionGroups: c.grantableResourcePermissionGroups(ctx),
		Errors:                   errors,
	}, nil
}

func (c *UsersController) buildEditFormProps(
	ctx context.Context,
	userQueryService *services.UserQueryService,
	roleQueryService *services.RoleQueryService,
	groupQueryService *services.GroupQueryService,
	policy *services.PrivilegeGrantPolicy,
	userID uint,
	state *userEditFormState,
) (*users.EditFormProps, error) {
	const op = serrors.Op("controllers.buildEditFormProps")

	userViewModel, err := userQueryService.FindUserByID(ctx, int(userID))
	if err != nil {
		return nil, serrors.E(op, err)
	}
	currentRoleIDs := make([]uint, 0, len(userViewModel.Roles))
	for _, assignedRole := range userViewModel.Roles {
		id, parseErr := strconv.ParseUint(assignedRole.ID, 10, 64)
		if parseErr != nil {
			return nil, serrors.E(op, parseErr)
		}
		currentRoleIDs = append(currentRoleIDs, uint(id))
	}
	selection := userFormOptionSelection{
		selectedRoleIDs:  currentRoleIDs,
		selectedGroupIDs: userViewModel.GroupIDs,
		retainedRoleIDs:  currentRoleIDs,
		retainedGroupIDs: userViewModel.GroupIDs,
	}
	if state != nil && state.DTO != nil {
		selection.selectedRoleIDs = state.DTO.RoleIDs
		selection.selectedGroupIDs = state.DTO.GroupIDs
	}
	options, err := loadUserFormOptions(ctx, roleQueryService, groupQueryService, policy, selection)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	canDelete, err := userQueryService.CanDeleteUser(ctx, int(userID))
	if err != nil {
		return nil, serrors.E(op, err)
	}

	userViewModel.Roles = options.selectedRoles
	userViewModel.GroupIDs = options.selectedGroupIDs
	actor, err := composables.UseUser(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	tenantID, err := uuid.Parse(userViewModel.TenantID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	targetPermissions, err := permissionsFromViewModel(userViewModel.EffectivePermissions)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	canManage := policy.CanManageUserProjection(actor, userID, tenantID, user.Type(userViewModel.Type), targetPermissions)
	userViewModel.CanUpdate = userViewModel.CanUpdate && canManage
	userViewModel.CanDelete = userViewModel.CanDelete && canDelete && canManage
	userViewModel.CanBeBlocked = userViewModel.CanBeBlocked && canManage
	canDelete = canDelete && canManage
	selectedPermissions, err := permissionsFromViewModel(userViewModel.DirectPermissions)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	errors := map[string]string{}

	if state != nil {
		errors = state.Errors
		if state.DTO != nil {
			userViewModel.FirstName = state.DTO.FirstName
			userViewModel.LastName = state.DTO.LastName
			userViewModel.MiddleName = state.DTO.MiddleName
			userViewModel.Email = state.DTO.Email
			userViewModel.Phone = state.DTO.Phone
			userViewModel.Language = state.DTO.Language
			userViewModel.GroupIDs = options.selectedGroupIDs
			userViewModel.AvatarID = strconv.FormatUint(uint64(state.DTO.AvatarID), 10)
			userViewModel.Roles = options.selectedRoles
			selectedPermissions = c.selectedPermissionsFromIDs(state.DTO.PermissionIDs)
		}
	}

	return &users.EditFormProps{
		User:                     userViewModel,
		Roles:                    options.roles,
		Groups:                   options.groups,
		ResourcePermissionGroups: c.grantableResourcePermissionGroups(ctx, selectedPermissions...),
		Errors:                   errors,
		CanDelete:                canDelete,
	}, nil
}

func permissionsFromViewModel(values []*viewmodels.Permission) ([]permission.Permission, error) {
	result := make([]permission.Permission, 0, len(values))
	for _, value := range values {
		id, err := uuid.Parse(value.ID)
		if err != nil {
			return nil, err
		}
		result = append(result, permission.New(permission.WithID(id), permission.WithName(value.Name),
			permission.WithResource(permission.Resource(value.Resource)), permission.WithAction(permission.Action(value.Action)),
			permission.WithModifier(permission.Modifier(value.Modifier))))
	}
	return result, nil
}

func renderBlockDrawer(
	ctx context.Context,
	w io.Writer,
	userViewModel *viewmodels.User,
	errors map[string]string,
) error {
	return users.BlockDrawer(&users.BlockDrawerProps{
		User:   userViewModel,
		Errors: errors,
	}).Render(ctx, w)
}

func renderEditContentOOB(
	ctx context.Context,
	w io.Writer,
	props *users.EditFormProps,
) error {
	return users.EditFormContentOOB(props).Render(ctx, w)
}
