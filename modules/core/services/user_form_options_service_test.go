package services_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/query"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type userFormOptionsRepoStub struct {
	set *query.UserFormOptionSet
}

func (r *userFormOptionsRepoStub) ListUserFormOptions(context.Context) (*query.UserFormOptionSet, error) {
	return r.set, nil
}

func TestUserFormOptionsServiceFiltersGrantableAndRetainsCurrentAssignments(t *testing.T) {
	// Falsely green if the retained choices are also grantable to the actor.
	allowed := permission.New(
		permission.WithID(uuid.New()), permission.WithName("allowed"),
		permission.WithResource("users"), permission.WithAction(permission.ActionRead),
		permission.WithModifier(permission.ModifierAll),
	)
	denied := permission.New(
		permission.WithID(uuid.New()), permission.WithName("denied"),
		permission.WithResource("secrets"), permission.WithAction(permission.ActionRead),
		permission.WithModifier(permission.ModifierAll),
	)
	retainedGroupID := uuid.New()
	repo := &userFormOptionsRepoStub{set: &query.UserFormOptionSet{
		Roles: []*query.AssignmentOption{
			{ID: "1", Type: "user", Name: "Allowed role", Permissions: []permission.Permission{allowed}},
			{ID: "2", Type: "user", Name: "Retained role", Permissions: []permission.Permission{denied}},
			{ID: "3", Type: "user", Name: "Hidden role", Permissions: []permission.Permission{denied}},
		},
		Groups: []*query.AssignmentOption{
			{ID: uuid.NewString(), Type: "user", Name: "Allowed group", Permissions: []permission.Permission{allowed}},
			{ID: retainedGroupID.String(), Type: "user", Name: "Retained group", Permissions: []permission.Permission{denied}},
			{ID: uuid.NewString(), Type: "user", Name: "Hidden group", Permissions: []permission.Permission{denied}},
		},
	}}
	policy := services.NewPrivilegeGrantPolicy(nil, nil, nil, nil, nil, logrus.New())
	service := services.NewUserFormOptionsService(repo, policy)
	actor := user.New("Form", "Actor", nil, user.UILanguageEN, user.WithPermissions([]permission.Permission{allowed}))
	ctx := composables.WithUser(context.Background(), actor)

	options, err := service.Build(ctx, services.UserFormSelection{
		SelectedRoleIDs:  []uint{2},
		SelectedGroupIDs: []string{retainedGroupID.String()},
		RetainedRoleIDs:  []uint{2},
		RetainedGroupIDs: []uuid.UUID{retainedGroupID},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"Allowed role", "Retained role"}, roleOptionNames(options.Roles))
	assert.Equal(t, []string{"Retained role"}, roleOptionNames(options.SelectedRoles))
	assert.Equal(t, []string{"Allowed group", "Retained group"}, groupOptionNames(options.Groups))
	assert.Equal(t, []string{retainedGroupID.String()}, options.SelectedGroupIDs)
}

func roleOptionNames(options []*viewmodels.Role) []string {
	result := make([]string, 0, len(options))
	for _, option := range options {
		result = append(result, option.Name)
	}
	return result
}

func groupOptionNames(options []*viewmodels.Group) []string {
	result := make([]string, 0, len(options))
	for _, option := range options {
		result = append(result, option.Name)
	}
	return result
}
