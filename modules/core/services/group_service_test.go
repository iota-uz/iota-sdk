package services_test

import (
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/group"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	permissions "github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupService_GetByID(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	// Setup repositories for test
	permissionRepository := persistence.NewPermissionRepository()
	roleRepository := persistence.NewRoleRepository()
	uploadRepository := persistence.NewUploadRepository()
	userRepository := persistence.NewUserRepository(uploadRepository)
	groupRepository := persistence.NewGroupRepository(userRepository, roleRepository)

	tenant, err := composables.UseTenantID(f.Ctx)
	require.NoError(t, err)

	// Create test permission
	err = permissionRepository.Save(f.Ctx, permissions.UserRead)
	require.NoError(t, err)

	// Create test data
	groupID := uuid.New()
	testGroup := group.New("Test Group", group.WithID(groupID), group.WithTenantID(tenant))

	// Setup service
	bus := eventbus.NewEventPublisher(logrus.New())
	service := services.NewGroupService(groupRepository, bus)

	// Add the group to the repository
	savedGroup, err := groupRepository.Save(f.Ctx, testGroup)
	require.NoError(t, err)

	// Execute
	result, err := service.GetByID(f.Ctx, savedGroup.ID())

	// Assert
	require.NoError(t, err)
	assert.Equal(t, savedGroup.ID(), result.ID())
	assert.Equal(t, savedGroup.Name(), result.Name())
}

func TestGroupService_Count(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	// Setup repositories for test
	permissionRepository := persistence.NewPermissionRepository()
	roleRepository := persistence.NewRoleRepository()
	uploadRepository := persistence.NewUploadRepository()
	userRepository := persistence.NewUserRepository(uploadRepository)
	groupRepository := persistence.NewGroupRepository(userRepository, roleRepository)

	tenant, err := composables.UseTenantID(f.Ctx)
	require.NoError(t, err)

	// Create test permission
	err = permissionRepository.Save(f.Ctx, permissions.UserRead)
	require.NoError(t, err)

	// Setup service
	bus := eventbus.NewEventPublisher(logrus.New())
	service := services.NewGroupService(groupRepository, bus)

	// Add some test groups
	for i := 1; i <= 5; i++ {
		groupName := "Group " + string(rune(i+64)) // A, B, C, D, E
		groupEntity := group.New(groupName, group.WithID(uuid.New()), group.WithTenantID(tenant))
		_, err := groupRepository.Save(f.Ctx, groupEntity)
		require.NoError(t, err)
	}

	// Execute
	result, err := service.Count(f.Ctx, &group.FindParams{})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, int64(5), result)
}

func TestGroupService_GetPaginated(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	// Setup repositories for test
	permissionRepository := persistence.NewPermissionRepository()
	roleRepository := persistence.NewRoleRepository()
	uploadRepository := persistence.NewUploadRepository()
	userRepository := persistence.NewUserRepository(uploadRepository)
	groupRepository := persistence.NewGroupRepository(userRepository, roleRepository)

	tenant, err := composables.UseTenantID(f.Ctx)
	require.NoError(t, err)

	// Create test permission
	err = permissionRepository.Save(f.Ctx, permissions.UserRead)
	require.NoError(t, err)

	// Setup service
	bus := eventbus.NewEventPublisher(logrus.New())
	service := services.NewGroupService(groupRepository, bus)

	// Create time markers for sorting and filtering
	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)

	// Add test groups with different creation times
	groupOlder := group.New(
		"Older Group",
		group.WithID(uuid.New()),
		group.WithCreatedAt(yesterday),
		group.WithTenantID(tenant),
	)
	groupNewer := group.New(
		"Newer Group",
		group.WithID(uuid.New()),
		group.WithCreatedAt(now),
		group.WithTenantID(tenant),
	)

	_, err = groupRepository.Save(f.Ctx, groupOlder)
	require.NoError(t, err)
	_, err = groupRepository.Save(f.Ctx, groupNewer)
	require.NoError(t, err)

	// Test pagination
	params := &group.FindParams{
		Limit:  10,
		Offset: 0,
		SortBy: group.SortBy{
			Fields: []repo.SortByField[group.Field]{
				{Field: group.CreatedAtField, Ascending: true},
			},
		},
	}

	// Execute
	result, err := service.GetPaginated(f.Ctx, params)

	// Assert
	require.NoError(t, err)
	assert.Len(t, result, 2)

	// Verify sorting - older group should come first with ascending sort
	assert.Equal(t, "Older Group", result[0].Name())
	assert.Equal(t, "Newer Group", result[1].Name())

	// Test reverse sorting
	params.SortBy.Fields[0].Ascending = false
	result, err = service.GetPaginated(f.Ctx, params)

	// Assert
	require.NoError(t, err)
	assert.Len(t, result, 2)

	// Verify sorting - newer group should come first with descending sort
	assert.Equal(t, "Newer Group", result[0].Name())
	assert.Equal(t, "Older Group", result[1].Name())
}
