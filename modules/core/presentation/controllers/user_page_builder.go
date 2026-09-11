package controllers

import (
	"context"
	"io"
	"strconv"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/mappers"
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
	formOptionsService *services.UserFormOptionsService,
	state *userCreateFormState,
) (*users.CreateFormProps, error) {
	const op = serrors.Op("controllers.buildCreateFormProps")

	selection := services.UserFormSelection{}
	if state != nil && state.DTO != nil {
		selection.SelectedRoleIDs = state.DTO.RoleIDs
		selection.SelectedGroupIDs = state.DTO.GroupIDs
	}
	options, err := formOptionsService.Build(ctx, selection)
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
				Roles:      options.SelectedRoles,
				GroupIDs:   options.SelectedGroupIDs,
				Language:   state.DTO.Language,
				AvatarID:   strconv.FormatUint(uint64(state.DTO.AvatarID), 10),
			}
		}
	}

	return &users.CreateFormProps{
		User:                     userViewModel,
		Roles:                    options.Roles,
		Groups:                   options.Groups,
		ResourcePermissionGroups: c.grantableResourcePermissionGroups(ctx),
		Errors:                   errors,
	}, nil
}

func (c *UsersController) buildEditFormProps(
	ctx context.Context,
	logger *logrus.Entry,
	userService *services.UserService,
	formOptionsService *services.UserFormOptionsService,
	policy *services.PrivilegeGrantPolicy,
	userID uint,
	state *userEditFormState,
) (*users.EditFormProps, error) {
	const op = serrors.Op("controllers.buildEditFormProps")

	us, err := userService.GetByID(ctx, userID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	currentRoleIDs := make([]uint, 0, len(us.Roles()))
	for _, assignedRole := range us.Roles() {
		currentRoleIDs = append(currentRoleIDs, assignedRole.ID())
	}
	selection := services.UserFormSelection{
		SelectedRoleIDs:  currentRoleIDs,
		SelectedGroupIDs: uuidStrings(us.GroupIDs()),
		RetainedRoleIDs:  currentRoleIDs,
		RetainedGroupIDs: us.GroupIDs(),
	}
	if state != nil && state.DTO != nil {
		selection.SelectedRoleIDs = state.DTO.RoleIDs
		selection.SelectedGroupIDs = state.DTO.GroupIDs
	}
	options, err := formOptionsService.Build(ctx, selection)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	canDelete, err := userService.CanUserBeDeleted(ctx, userID)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	userViewModel := mappers.UserToViewModel(us)
	userViewModel.Roles = options.SelectedRoles
	userViewModel.GroupIDs = options.SelectedGroupIDs
	actor, err := composables.UseUser(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	canManage := policy.CanManageUser(actor, us)
	userViewModel.CanUpdate = userViewModel.CanUpdate && canManage
	userViewModel.CanDelete = userViewModel.CanDelete && canDelete && canManage
	userViewModel.CanBeBlocked = userViewModel.CanBeBlocked && canManage
	canDelete = canDelete && canManage
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
			userViewModel.GroupIDs = options.SelectedGroupIDs
			userViewModel.AvatarID = strconv.FormatUint(uint64(state.DTO.AvatarID), 10)
			userViewModel.Roles = options.SelectedRoles
			selectedPermissions = c.selectedPermissionsFromIDs(state.DTO.PermissionIDs)
		}
	}

	return &users.EditFormProps{
		User:                     userViewModel,
		Roles:                    options.Roles,
		Groups:                   options.Groups,
		ResourcePermissionGroups: c.grantableResourcePermissionGroups(ctx, selectedPermissions...),
		Errors:                   errors,
		CanDelete:                canDelete,
	}, nil
}

func uuidStrings(values []uuid.UUID) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.String())
	}
	return result
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
