package services

import (
	"context"
	"strconv"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/group"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/query"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type UserFormSelection struct {
	SelectedRoleIDs  []uint
	SelectedGroupIDs []string
	RetainedRoleIDs  []uint
	RetainedGroupIDs []uuid.UUID
}

type UserFormOptions struct {
	Roles            []*viewmodels.Role
	Groups           []*viewmodels.Group
	SelectedRoles    []*viewmodels.Role
	SelectedGroupIDs []string
}

type UserFormOptionsService struct {
	repo   query.UserFormOptionsRepository
	policy *PrivilegeGrantPolicy
}

func NewUserFormOptionsService(repo query.UserFormOptionsRepository, policy *PrivilegeGrantPolicy) *UserFormOptionsService {
	return &UserFormOptionsService{repo: repo, policy: policy}
}

func (s *UserFormOptionsService) Build(ctx context.Context, selection UserFormSelection) (*UserFormOptions, error) {
	const op = serrors.Op("UserFormOptionsService.Build")

	set, err := s.repo.ListUserFormOptions(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	actor, err := composables.UseUser(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	selectedRoles := uintSet(selection.SelectedRoleIDs)
	retainedRoles := uintSet(selection.RetainedRoleIDs)
	selectedGroups := stringSet(selection.SelectedGroupIDs)
	retainedGroups := uuidStringSet(selection.RetainedGroupIDs)
	result := &UserFormOptions{
		Roles:            make([]*viewmodels.Role, 0, len(set.Roles)),
		Groups:           make([]*viewmodels.Group, 0, len(set.Groups)),
		SelectedRoles:    make([]*viewmodels.Role, 0, len(selectedRoles)),
		SelectedGroupIDs: make([]string, 0, len(selectedGroups)),
	}

	for _, option := range set.Roles {
		id64, parseErr := strconv.ParseUint(option.ID, 10, 64)
		if parseErr != nil {
			continue
		}
		id := uint(id64)
		if !s.policy.CanGrantRoleOption(actor, role.Type(option.Type), option.Permissions) {
			if _, keep := retainedRoles[id]; !keep {
				continue
			}
		}
		vm := &viewmodels.Role{ID: option.ID, Type: option.Type, Name: option.Name, Description: option.Description}
		result.Roles = append(result.Roles, vm)
		if _, selected := selectedRoles[id]; selected {
			result.SelectedRoles = append(result.SelectedRoles, vm)
		}
	}

	for _, option := range set.Groups {
		if !s.policy.CanGrantGroupOption(actor, group.Type(option.Type), option.Permissions) {
			if _, keep := retainedGroups[option.ID]; !keep {
				continue
			}
		}
		result.Groups = append(result.Groups, &viewmodels.Group{
			ID: option.ID, Type: option.Type, Name: option.Name, Description: option.Description,
		})
		if _, selected := selectedGroups[option.ID]; selected {
			result.SelectedGroupIDs = append(result.SelectedGroupIDs, option.ID)
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

func uuidStringSet(values []uuid.UUID) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value.String()] = struct{}{}
	}
	return result
}
