package controllers

import (
	"context"
	"io"
	"strconv"

	"github.com/sirupsen/logrus"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/query"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/mappers"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/users"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
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

const userFormGroupsLimit = 1000

func userGroupsFindParams(limit int) *query.GroupFindParams {
	return &query.GroupFindParams{
		Limit:   limit,
		Offset:  0,
		Filters: []query.GroupFilter{},
	}
}

func loadUserFormOptions(
	ctx context.Context,
	roleService *services.RoleService,
	groupQueryService *services.GroupQueryService,
) ([]*viewmodels.Role, []*viewmodels.Group, error) {
	const op = serrors.Op("controllers.loadUserFormOptions")

	roles, err := roleService.GetAll(ctx)
	if err != nil {
		return nil, nil, serrors.E(op, err)
	}

	groups, _, err := groupQueryService.FindGroups(ctx, userGroupsFindParams(userFormGroupsLimit))
	if err != nil {
		return nil, nil, serrors.E(op, err)
	}

	return mapping.MapViewModels(roles, mappers.RoleToViewModel), groups, nil
}

func selectedRoleViewModels(allRoles []*viewmodels.Role, selectedIDs []uint) []*viewmodels.Role {
	if len(selectedIDs) == 0 {
		return nil
	}

	selected := make(map[string]struct{}, len(selectedIDs))
	for _, roleID := range selectedIDs {
		selected[strconv.FormatUint(uint64(roleID), 10)] = struct{}{}
	}

	selectedRoles := make([]*viewmodels.Role, 0, len(selectedIDs))
	for _, role := range allRoles {
		if _, ok := selected[role.ID]; ok {
			selectedRoles = append(selectedRoles, role)
		}
	}

	return selectedRoles
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

func decorateBlockedByUser(
	ctx context.Context,
	logger *logrus.Entry,
	userService *services.UserService,
	userViewModel *viewmodels.User,
) {
	if !userViewModel.IsBlocked || userViewModel.BlockedBy == "" || userViewModel.BlockedBy == "0" {
		return
	}

	blockedByID, err := strconv.ParseUint(userViewModel.BlockedBy, 10, 64)
	if err != nil {
		logger.WithField("blockedBy", userViewModel.BlockedBy).WithError(err).Warn("failed to parse blocked by user id")
		return
	}

	blockerUser, err := userService.GetByID(ctx, uint(blockedByID))
	if err != nil {
		logger.WithField("blockedBy", userViewModel.BlockedBy).WithError(err).Warn("failed to load blocker user")
		return
	}

	userViewModel.BlockedByUser = mappers.UserToViewModel(blockerUser).Title()
}

func (c *UsersController) buildCreateFormProps(
	ctx context.Context,
	roleService *services.RoleService,
	groupQueryService *services.GroupQueryService,
	state *userCreateFormState,
) (*users.CreateFormProps, error) {
	const op = serrors.Op("controllers.buildCreateFormProps")

	roleViewModels, groups, err := loadUserFormOptions(ctx, roleService, groupQueryService)
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
				GroupIDs:   state.DTO.GroupIDs,
				Roles:      selectedRoleViewModels(roleViewModels, state.DTO.RoleIDs),
				Language:   state.DTO.Language,
				AvatarID:   strconv.FormatUint(uint64(state.DTO.AvatarID), 10),
			}
		}
	}

	return &users.CreateFormProps{
		User:                     userViewModel,
		Roles:                    roleViewModels,
		Groups:                   groups,
		ResourcePermissionGroups: c.resourcePermissionGroups(),
		Errors:                   errors,
	}, nil
}

func (c *UsersController) buildEditFormProps(
	ctx context.Context,
	logger *logrus.Entry,
	userService *services.UserService,
	roleService *services.RoleService,
	groupQueryService *services.GroupQueryService,
	userID uint,
	state *userEditFormState,
) (*users.EditFormProps, error) {
	const op = serrors.Op("controllers.buildEditFormProps")

	roleViewModels, groups, err := loadUserFormOptions(ctx, roleService, groupQueryService)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	us, err := userService.GetByID(ctx, userID)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	canDelete, err := userService.CanUserBeDeleted(ctx, userID)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	userViewModel := mappers.UserToViewModel(us)
	userViewModel.CanDelete = canDelete
	decorateBlockedByUser(ctx, logger, userService, userViewModel)

	selectedPermissions := us.Permissions()
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
			userViewModel.GroupIDs = state.DTO.GroupIDs
			userViewModel.AvatarID = strconv.FormatUint(uint64(state.DTO.AvatarID), 10)
			userViewModel.Roles = selectedRoleViewModels(roleViewModels, state.DTO.RoleIDs)
			selectedPermissions = c.selectedPermissionsFromIDs(state.DTO.PermissionIDs)
		}
	}

	return &users.EditFormProps{
		User:                     userViewModel,
		Roles:                    roleViewModels,
		Groups:                   groups,
		ResourcePermissionGroups: c.resourcePermissionGroups(selectedPermissions...),
		Errors:                   errors,
		CanDelete:                canDelete,
	}, nil
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
