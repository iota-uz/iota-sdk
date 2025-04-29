package persistence_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/group"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	permissions "github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPgGroupRepository_CRUD(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	// Setup repositories for test
	permissionRepository := persistence.NewPermissionRepository()
	roleRepository := persistence.NewRoleRepository()
	uploadRepository := persistence.NewUploadRepository()
	userRepository := persistence.NewUserRepository(uploadRepository)
	groupRepository := persistence.NewGroupRepository(userRepository, roleRepository)

	// Create roles for testing
	err := permissionRepository.Save(f.ctx, permissions.UserRead)
	require.NoError(t, err)

	// First role
	roleData := role.New(
		role.TypeUser,
		"test-role",
		role.WithDescription("test role description"),
		role.WithPermissions([]*permission.Permission{
			permissions.UserRead,
		}),
	)

	roleEntity, err := roleRepository.Create(f.ctx, roleData)
	require.NoError(t, err)

	// Second role for testing role filtering
	secondRoleData := role.New(
		role.TypeSystem,
		"admin-role",
		role.WithDescription("admin role description"),
		role.WithPermissions([]*permission.Permission{
			permissions.UserRead,
		}),
	)

	secondRoleEntity, err := roleRepository.Create(f.ctx, secondRoleData)
	require.NoError(t, err)

	// Create users for testing
	email, err := internet.NewEmail("test@gmail.com")
	require.NoError(t, err)

	userEntity := user.New(
		user.TypeUser,
		"John",
		"Doe",
		email,
		user.UILanguageEN,
		user.WithMiddleName(""),
	)

	createdUser, err := userRepository.Create(f.ctx, userEntity)
	require.NoError(t, err)

	secondEmail, err := internet.NewEmail("jane@gmail.com")
	require.NoError(t, err)

	secondUserEntity := user.New(
		user.TypeUser,
		"Jane",
		"Smith",
		secondEmail,
		user.UILanguageEN,
		user.WithMiddleName(""),
	)

	secondCreatedUser, err := userRepository.Create(f.ctx, secondUserEntity)
	require.NoError(t, err)

	// Test group creation
	t.Run("Create", func(t *testing.T) {
		groupID := uuid.New()
		// Create group with ID, name, description, users, and roles
		groupEntity := group.New(
			group.TypeUser,
			"Test Group",
			group.WithID(groupID),
			group.WithDescription("Test group description"),
			group.WithUsers([]user.User{createdUser}),
			group.WithRoles([]role.Role{roleEntity}),
		)

		// Save the group
		savedGroup, err := groupRepository.Save(f.ctx, groupEntity)
		require.NoError(t, err)
		assert.Equal(t, "Test Group", savedGroup.Name())
		assert.Equal(t, "Test group description", savedGroup.Description())
		assert.Len(t, savedGroup.Users(), 1)
		assert.Len(t, savedGroup.Roles(), 1)
		assert.Equal(t, createdUser.ID(), savedGroup.Users()[0].ID())
		assert.Equal(t, roleEntity.ID(), savedGroup.Roles()[0].ID())

		// Get the user again to check if the group ID was added to user
		updatedUser, err := userRepository.GetByID(f.ctx, createdUser.ID())
		require.NoError(t, err)

		// Verify that the user has this group ID
		groupIDs := updatedUser.GroupIDs()
		hasGroupID := false
		for _, id := range groupIDs {
			if id == groupID {
				hasGroupID = true
				break
			}
		}
		assert.True(t, hasGroupID, "User should have the group ID")

		// Create another group for testing
		secondGroupID := uuid.New()
		secondGroupEntity := group.New(
			group.TypeUser,
			"Second Group",
			group.WithID(secondGroupID),
			group.WithDescription("Second group description"),
			group.WithUsers([]user.User{secondCreatedUser}),
			group.WithRoles([]role.Role{secondRoleEntity}),
		)

		secondSavedGroup, err := groupRepository.Save(f.ctx, secondGroupEntity)
		require.NoError(t, err)
		assert.Equal(t, "Second Group", secondSavedGroup.Name())
		assert.Equal(t, "Second group description", secondSavedGroup.Description())
		assert.Len(t, secondSavedGroup.Users(), 1)
		assert.Len(t, secondSavedGroup.Roles(), 1)
		assert.Equal(t, secondCreatedUser.ID(), secondSavedGroup.Users()[0].ID())
		assert.Equal(t, secondRoleEntity.ID(), secondSavedGroup.Roles()[0].ID())
	})

	t.Run("GetByID", func(t *testing.T) {
		// Create a new group
		groupID := uuid.New()
		groupEntity := group.New(
			group.TypeUser,
			"Get By ID Group",
			group.WithID(groupID),
			group.WithDescription("Group for GetByID test"),
			group.WithUsers([]user.User{createdUser}),
			group.WithRoles([]role.Role{roleEntity}),
		)

		savedGroup, err := groupRepository.Save(f.ctx, groupEntity)
		require.NoError(t, err)

		// Test getting by ID
		retrievedGroup, err := groupRepository.GetByID(f.ctx, savedGroup.ID())
		require.NoError(t, err)
		assert.Equal(t, savedGroup.ID(), retrievedGroup.ID())
		assert.Equal(t, savedGroup.Name(), retrievedGroup.Name())
		assert.Equal(t, savedGroup.Description(), retrievedGroup.Description())
		assert.Len(t, retrievedGroup.Users(), 1)
		assert.Len(t, retrievedGroup.Roles(), 1)
		assert.Equal(t, createdUser.ID(), retrievedGroup.Users()[0].ID())
		assert.Equal(t, roleEntity.ID(), retrievedGroup.Roles()[0].ID())

		// Test with non-existent ID
		nonExistentID := uuid.New()
		_, err = groupRepository.GetByID(f.ctx, nonExistentID)
		require.Error(t, err)
		require.ErrorIs(t, err, persistence.ErrGroupNotFound)
	})

	t.Run("Update", func(t *testing.T) {
		// Create a new group
		groupID := uuid.New()
		groupEntity := group.New(
			group.TypeUser,
			"Update Group",
			group.WithID(groupID),
			group.WithDescription("Group for update test"),
			group.WithUsers([]user.User{createdUser}),
			group.WithRoles([]role.Role{roleEntity}),
		)

		savedGroup, err := groupRepository.Save(f.ctx, groupEntity)
		require.NoError(t, err)

		// Update the group
		updatedGroup := savedGroup.SetName("Updated Group Name").SetDescription("Updated description")
		// Add the second user to the group
		updatedGroup = updatedGroup.AddUser(secondCreatedUser)
		// Add the second role to the group
		updatedGroup = updatedGroup.AssignRole(secondRoleEntity)

		// Save the updated group
		savedUpdatedGroup, err := groupRepository.Save(f.ctx, updatedGroup)
		require.NoError(t, err)

		// Verify the updates
		assert.Equal(t, "Updated Group Name", savedUpdatedGroup.Name())
		assert.Equal(t, "Updated description", savedUpdatedGroup.Description())
		assert.Len(t, savedUpdatedGroup.Users(), 2)
		assert.Len(t, savedUpdatedGroup.Roles(), 2)

		// Verify original user and role are still there
		hasUser1 := false
		hasUser2 := false
		for _, u := range savedUpdatedGroup.Users() {
			if u.ID() == createdUser.ID() {
				hasUser1 = true
			}
			if u.ID() == secondCreatedUser.ID() {
				hasUser2 = true
			}
		}
		assert.True(t, hasUser1, "First user should still be in the group")
		assert.True(t, hasUser2, "Second user should have been added to the group")

		hasRole1 := false
		hasRole2 := false
		for _, r := range savedUpdatedGroup.Roles() {
			if r.ID() == roleEntity.ID() {
				hasRole1 = true
			}
			if r.ID() == secondRoleEntity.ID() {
				hasRole2 = true
			}
		}
		assert.True(t, hasRole1, "First role should still be in the group")
		assert.True(t, hasRole2, "Second role should have been added to the group")
	})

	t.Run("RemoveUserAndRole", func(t *testing.T) {
		// Create a group with both users and roles
		groupID := uuid.New()
		groupEntity := group.New(
			group.TypeUser,
			"Remove Test Group",
			group.WithID(groupID),
			group.WithDescription("Group for removal test"),
			group.WithUsers([]user.User{createdUser, secondCreatedUser}),
			group.WithRoles([]role.Role{roleEntity, secondRoleEntity}),
		)

		savedGroup, err := groupRepository.Save(f.ctx, groupEntity)
		require.NoError(t, err)
		assert.Len(t, savedGroup.Users(), 2)
		assert.Len(t, savedGroup.Roles(), 2)

		// Verify that both users have this group ID
		firstUser, err := userRepository.GetByID(f.ctx, createdUser.ID())
		require.NoError(t, err)
		secondUser, err := userRepository.GetByID(f.ctx, secondCreatedUser.ID())
		require.NoError(t, err)

		// Check if both users have the group ID
		hasGroupID := false
		for _, id := range firstUser.GroupIDs() {
			if id == groupID {
				hasGroupID = true
				break
			}
		}
		assert.True(t, hasGroupID, "First user should have the group ID")

		hasGroupID = false
		for _, id := range secondUser.GroupIDs() {
			if id == groupID {
				hasGroupID = true
				break
			}
		}
		assert.True(t, hasGroupID, "Second user should have the group ID")

		// Remove a user
		updatedGroup := savedGroup.RemoveUser(createdUser)
		// Remove a role
		updatedGroup = updatedGroup.RemoveRole(roleEntity)

		// Save the updated group
		savedUpdatedGroup, err := groupRepository.Save(f.ctx, updatedGroup)
		require.NoError(t, err)

		// Verify the updates
		assert.Len(t, savedUpdatedGroup.Users(), 1)
		assert.Len(t, savedUpdatedGroup.Roles(), 1)
		assert.Equal(t, secondCreatedUser.ID(), savedUpdatedGroup.Users()[0].ID())
		assert.Equal(t, secondRoleEntity.ID(), savedUpdatedGroup.Roles()[0].ID())

		// Check if the removed user no longer has this group ID
		firstUserAfterRemoval, err := userRepository.GetByID(f.ctx, createdUser.ID())
		require.NoError(t, err)

		hasGroupID = false
		for _, id := range firstUserAfterRemoval.GroupIDs() {
			if id == groupID {
				hasGroupID = true
				break
			}
		}
		assert.False(t, hasGroupID, "First user should no longer have the group ID after removal")

		// Check if the second user still has the group ID
		secondUserAfterUpdate, err := userRepository.GetByID(f.ctx, secondCreatedUser.ID())
		require.NoError(t, err)

		hasGroupID = false
		for _, id := range secondUserAfterUpdate.GroupIDs() {
			if id == groupID {
				hasGroupID = true
				break
			}
		}
		assert.True(t, hasGroupID, "Second user should still have the group ID")
	})

	t.Run("FilterByCreatedAt", func(t *testing.T) {
		// Get a timestamp for comparison
		now := time.Now()
		pastTime := now.Add(-24 * time.Hour)

		// Create a group with custom creation time
		customTimeGroupID := uuid.New()
		customTimeGroupEntity := group.New(
			group.TypeUser,
			"Time Filter Group",
			group.WithID(customTimeGroupID),
			group.WithDescription("Group for time filter test"),
			group.WithCreatedAt(pastTime.Add(-24*time.Hour)), // 2 days ago
			group.WithUsers([]user.User{createdUser}),
			group.WithRoles([]role.Role{roleEntity}),
		)

		_, err := groupRepository.Save(f.ctx, customTimeGroupEntity)
		require.NoError(t, err)

		// Test filtering by created_at with Gt expression
		params := &group.FindParams{
			Filters: []group.Filter{
				{
					Column: group.CreatedAt,
					Filter: repo.Gt(pastTime),
				},
			},
			SortBy: group.SortBy{
				Fields:    []group.Field{group.CreatedAt},
				Ascending: true,
			},
			Limit:  100,
			Offset: 0,
		}

		// Get groups created after yesterday
		groups, err := groupRepository.GetPaginated(f.ctx, params)
		require.NoError(t, err)

		// Should return groups created after yesterday
		for _, g := range groups {
			assert.True(t, g.CreatedAt().After(pastTime), "Group should be created after yesterday")
		}

		// Test with Lt expression
		params = &group.FindParams{
			Filters: []group.Filter{
				{
					Column: group.CreatedAt,
					Filter: repo.Lt(pastTime),
				},
			},
			SortBy: group.SortBy{
				Fields:    []group.Field{group.CreatedAt},
				Ascending: true,
			},
			Limit:  100,
			Offset: 0,
		}

		// Get groups created before yesterday
		groups, err = groupRepository.GetPaginated(f.ctx, params)
		require.NoError(t, err)

		// Should return groups created before yesterday
		for _, g := range groups {
			assert.True(t, g.CreatedAt().Before(pastTime), "Group should be created before yesterday")
		}

		// Test with Gte expression
		params = &group.FindParams{
			Filters: []group.Filter{
				{
					Column: group.CreatedAt,
					Filter: repo.Gte(pastTime),
				},
			},
			SortBy: group.SortBy{
				Fields:    []group.Field{group.CreatedAt},
				Ascending: true,
			},
			Limit:  100,
			Offset: 0,
		}

		// Get groups created on or after yesterday
		groups, err = groupRepository.GetPaginated(f.ctx, params)
		require.NoError(t, err)

		// Should return groups created on or after yesterday
		for _, g := range groups {
			assert.True(t, g.CreatedAt().After(pastTime) || g.CreatedAt().Equal(pastTime),
				"Group should be created on or after yesterday")
		}

		// Test with Lte expression
		params = &group.FindParams{
			Filters: []group.Filter{
				{
					Column: group.CreatedAt,
					Filter: repo.Lte(pastTime),
				},
			},
			SortBy: group.SortBy{
				Fields:    []group.Field{group.CreatedAt},
				Ascending: true,
			},
			Limit:  100,
			Offset: 0,
		}

		// Get groups created on or before yesterday
		groups, err = groupRepository.GetPaginated(f.ctx, params)
		require.NoError(t, err)

		// Should return groups created on or before yesterday
		for _, g := range groups {
			assert.True(t, g.CreatedAt().Before(pastTime) || g.CreatedAt().Equal(pastTime),
				"Group should be created on or before yesterday")
		}
	})

	t.Run("Sorting", func(t *testing.T) {
		// Test sorting by created_at ascending
		params := &group.FindParams{
			SortBy: group.SortBy{
				Fields:    []group.Field{group.CreatedAt},
				Ascending: true,
			},
			Limit:  100,
			Offset: 0,
		}

		groups, err := groupRepository.GetPaginated(f.ctx, params)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(groups), 2, "Should return at least 2 groups")

		// Verify ascending order
		for i := 1; i < len(groups); i++ {
			assert.True(t, groups[i-1].CreatedAt().Before(groups[i].CreatedAt()) ||
				groups[i-1].CreatedAt().Equal(groups[i].CreatedAt()),
				"Groups should be sorted by created_at in ascending order")
		}

		// Test sorting by created_at descending
		params.SortBy.Ascending = false
		groups, err = groupRepository.GetPaginated(f.ctx, params)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(groups), 2, "Should return at least 2 groups")

		// Verify descending order
		for i := 1; i < len(groups); i++ {
			assert.True(t, groups[i-1].CreatedAt().After(groups[i].CreatedAt()) ||
				groups[i-1].CreatedAt().Equal(groups[i].CreatedAt()),
				"Groups should be sorted by created_at in descending order")
		}

		// Test sorting by updated_at
		params.SortBy.Fields = []group.Field{group.UpdatedAt}
		params.SortBy.Ascending = true
		groups, err = groupRepository.GetPaginated(f.ctx, params)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(groups), 2, "Should return at least 2 groups")

		// Verify ascending order by updated_at
		for i := 1; i < len(groups); i++ {
			assert.True(t, groups[i-1].UpdatedAt().Before(groups[i].UpdatedAt()) ||
				groups[i-1].UpdatedAt().Equal(groups[i].UpdatedAt()),
				"Groups should be sorted by updated_at in ascending order")
		}
	})

	t.Run("Pagination", func(t *testing.T) {
		// Create multiple additional groups to test pagination
		for i := 0; i < 5; i++ {
			groupID := uuid.New()
			groupEntity := group.New(
				group.TypeUser,
				"Pagination Group "+string(rune(i+65)), // A, B, C, D, E
				group.WithID(groupID),
				group.WithDescription("Group for pagination test"),
				group.WithUsers([]user.User{createdUser}),
				group.WithRoles([]role.Role{roleEntity}),
			)

			_, err := groupRepository.Save(f.ctx, groupEntity)
			require.NoError(t, err)
		}

		// Test with limit and offset
		params := &group.FindParams{
			SortBy: group.SortBy{
				Fields:    []group.Field{group.CreatedAt},
				Ascending: true,
			},
			Limit:  3,
			Offset: 0,
		}

		// Get first page
		firstPage, err := groupRepository.GetPaginated(f.ctx, params)
		require.NoError(t, err)
		assert.Len(t, firstPage, 3, "First page should return 3 groups")

		// Get second page
		params.Offset = 3
		secondPage, err := groupRepository.GetPaginated(f.ctx, params)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(secondPage), 1, "Second page should return at least 1 group")

		// Verify pages don't overlap
		for _, g1 := range firstPage {
			for _, g2 := range secondPage {
				assert.NotEqual(t, g1.ID(), g2.ID(), "Groups shouldn't appear in both pages")
			}
		}

		// Test total count
		count, err := groupRepository.Count(f.ctx, &group.FindParams{})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, int64(8), "Total count should be at least 8 groups")
	})

	t.Run("Delete", func(t *testing.T) {
		// Create a group to delete
		groupID := uuid.New()
		groupEntity := group.New(
			group.TypeUser,
			"Delete Test Group",
			group.WithID(groupID),
			group.WithDescription("Group for delete test"),
			group.WithUsers([]user.User{createdUser}),
			group.WithRoles([]role.Role{roleEntity}),
		)

		savedGroup, err := groupRepository.Save(f.ctx, groupEntity)
		require.NoError(t, err)

		// Delete the group
		err = groupRepository.Delete(f.ctx, savedGroup.ID())
		require.NoError(t, err)

		// Verify the group was deleted
		_, err = groupRepository.GetByID(f.ctx, savedGroup.ID())
		require.Error(t, err)
		require.ErrorIs(t, err, persistence.ErrGroupNotFound)
	})
}
