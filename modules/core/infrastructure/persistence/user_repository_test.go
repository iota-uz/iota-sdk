package persistence_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/group"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	permissions "github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/iota-uz/iota-sdk/pkg/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPgUserRepository_CRUD(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	// Create needed repositories
	permissionRepository := persistence.NewPermissionRepository()
	roleRepository := persistence.NewRoleRepository()
	uploadRepository := persistence.NewUploadRepository()
	userRepository := persistence.NewUserRepository(uploadRepository)
	groupRepository := persistence.NewGroupRepository(userRepository, roleRepository)

	// Create roles for testing
	err := permissionRepository.Save(f.ctx, permissions.UserRead)
	require.NoError(t, err)

	tenant, err := composables.UseTenantID(f.ctx)
	require.NoError(t, err)

	// First role
	roleData := role.New(
		"test-role",
		role.WithDescription("test role description"),
		role.WithPermissions([]*permission.Permission{
			permissions.UserRead,
		}),
		role.WithTenantID(tenant),
	)
	require.NoError(t, err)

	roleEntity, err := roleRepository.Create(f.ctx, roleData)
	require.NoError(t, err)

	// Second role for testing role filtering
	secondRoleData := role.New(
		"admin-role",
		role.WithDescription("admin role description"),
		role.WithPermissions([]*permission.Permission{
			permissions.UserRead,
		}),
		role.WithTenantID(tenant),
	)

	secondRoleEntity, err := roleRepository.Create(f.ctx, secondRoleData)
	require.NoError(t, err)

	// Create a group to test filtering
	groupID := uuid.New()
	groupEntity := group.New(
		"Test Group",
		group.WithID(groupID),
		group.WithDescription("Test group description"),
		group.WithTenantID(tenant),
	)
	_, err = groupRepository.Save(f.ctx, groupEntity)
	require.NoError(t, err)

	// Second group
	secondGroupID := uuid.New()
	secondGroupEntity := group.New(
		"Second Group",
		group.WithID(secondGroupID),
		group.WithDescription("Second group description"),
		group.WithTenantID(tenant),
	)
	_, err = groupRepository.Save(f.ctx, secondGroupEntity)
	require.NoError(t, err)

	t.Run("Create", func(t *testing.T) {
		// Basic user creation
		email, err := internet.NewEmail("test@gmail.com")
		require.NoError(t, err)

		userEntity := user.New(
			"John",
			"Doe",
			email,
			user.UILanguageEN,
			user.WithMiddleName("Middle"),
			user.WithTenantID(tenant),
		)

		createdUser, err := userRepository.Create(f.ctx, userEntity)
		require.NoError(t, err)
		assert.NotEqual(t, uint(0), createdUser.ID())
		assert.Equal(t, "John", createdUser.FirstName())
		assert.Equal(t, "Doe", createdUser.LastName())
		assert.Equal(t, "Middle", createdUser.MiddleName())
		assert.Equal(t, email.Value(), createdUser.Email().Value())
		assert.Nil(t, createdUser.Phone())
		assert.Equal(t, user.UILanguageEN, createdUser.UILanguage())
		assert.Nil(t, createdUser.Avatar())
		assert.True(t, createdUser.LastLogin().IsZero())
		assert.Equal(t, "", createdUser.LastIP())
		assert.NotNil(t, createdUser.CreatedAt())
		assert.NotNil(t, createdUser.UpdatedAt())
		assert.Empty(t, createdUser.Roles())
		assert.Empty(t, createdUser.GroupIDs())
		assert.Empty(t, createdUser.Permissions())

		// With roles
		secondEmail, err := internet.NewEmail("admin@gmail.com")
		require.NoError(t, err)

		userWithRoles := user.New(
			"Admin",
			"User",
			secondEmail,
			user.UILanguageEN,
			user.WithRoles([]role.Role{roleEntity}),
			user.WithTenantID(tenant),
		)

		createdUserWithRoles, err := userRepository.Create(f.ctx, userWithRoles)
		require.NoError(t, err)
		assert.Len(t, createdUserWithRoles.Roles(), 1)
		assert.Equal(t, roleEntity.ID(), createdUserWithRoles.Roles()[0].ID())

		// With group IDs
		thirdEmail, err := internet.NewEmail("group@gmail.com")
		require.NoError(t, err)

		userWithGroup := user.New(
			"Group",
			"User",
			thirdEmail,
			user.UILanguageEN,
			user.WithGroupIDs([]uuid.UUID{groupID}),
			user.WithTenantID(tenant),
		)

		createdUserWithGroup, err := userRepository.Create(f.ctx, userWithGroup)
		require.NoError(t, err)
		assert.Len(t, createdUserWithGroup.GroupIDs(), 1)
		assert.Equal(t, groupID, createdUserWithGroup.GroupIDs()[0])
	})

	t.Run("GetByID", func(t *testing.T) {
		email, err := internet.NewEmail("getbyid@gmail.com")
		require.NoError(t, err)

		userEntity := user.New(
			"Get",
			"ByID",
			email,
			user.UILanguageEN,
			user.WithTenantID(tenant),
		)

		createdUser, err := userRepository.Create(f.ctx, userEntity)
		require.NoError(t, err)

		retrievedUser, err := userRepository.GetByID(f.ctx, createdUser.ID())
		require.NoError(t, err)
		assert.Equal(t, createdUser.ID(), retrievedUser.ID())
		assert.Equal(t, "Get", retrievedUser.FirstName())
		assert.Equal(t, "ByID", retrievedUser.LastName())
		assert.Equal(t, email.Value(), retrievedUser.Email().Value())
	})

	t.Run("GetByEmail", func(t *testing.T) {
		emailStr := "getbyemail@gmail.com"
		email, err := internet.NewEmail(emailStr)
		require.NoError(t, err)

		userEntity := user.New(
			"Get",
			"ByEmail",
			email,
			user.UILanguageEN,
			user.WithTenantID(tenant),
		)

		_, err = userRepository.Create(f.ctx, userEntity)
		require.NoError(t, err)

		retrievedUser, err := userRepository.GetByEmail(f.ctx, emailStr)
		require.NoError(t, err)
		assert.Equal(t, "Get", retrievedUser.FirstName())
		assert.Equal(t, "ByEmail", retrievedUser.LastName())
		assert.Equal(t, emailStr, retrievedUser.Email().Value())
	})

	t.Run("Update", func(t *testing.T) {
		email, err := internet.NewEmail("update@gmail.com")
		require.NoError(t, err)

		userEntity := user.New(
			"Before",
			"Update",
			email,
			user.UILanguageEN,
			user.WithTenantID(tenant),
		)

		createdUser, err := userRepository.Create(f.ctx, userEntity)
		require.NoError(t, err)

		updatedUser := createdUser.SetName("After", "Updated", createdUser.MiddleName())
		err = userRepository.Update(f.ctx, updatedUser)
		require.NoError(t, err)

		retrievedUser, err := userRepository.GetByID(f.ctx, createdUser.ID())
		require.NoError(t, err)
		assert.Equal(t, "After", retrievedUser.FirstName())
		assert.Equal(t, "Updated", retrievedUser.LastName())
	})

	t.Run("UpdateRoles", func(t *testing.T) {
		email, err := internet.NewEmail("updateroles@gmail.com")
		require.NoError(t, err)

		userEntity := user.New(
			"User",
			"Roles",
			email,
			user.UILanguageEN,
			user.WithTenantID(tenant),
		)

		createdUser, err := userRepository.Create(f.ctx, userEntity)
		require.NoError(t, err)
		assert.Empty(t, createdUser.Roles())

		// Add a role
		updatedUser := createdUser.AddRole(roleEntity)
		err = userRepository.Update(f.ctx, updatedUser)
		require.NoError(t, err)

		retrievedUser, err := userRepository.GetByID(f.ctx, createdUser.ID())
		require.NoError(t, err)
		assert.Len(t, retrievedUser.Roles(), 1)
		assert.Equal(t, roleEntity.ID(), retrievedUser.Roles()[0].ID())

		// Add another role
		updatedUser = retrievedUser.AddRole(secondRoleEntity)
		err = userRepository.Update(f.ctx, updatedUser)
		require.NoError(t, err)

		retrievedUser, err = userRepository.GetByID(f.ctx, createdUser.ID())
		require.NoError(t, err)
		assert.Len(t, retrievedUser.Roles(), 2)

		// Remove a role
		updatedUser = retrievedUser.RemoveRole(roleEntity)
		err = userRepository.Update(f.ctx, updatedUser)
		require.NoError(t, err)

		retrievedUser, err = userRepository.GetByID(f.ctx, createdUser.ID())
		require.NoError(t, err)
		assert.Len(t, retrievedUser.Roles(), 1)
		assert.Equal(t, secondRoleEntity.ID(), retrievedUser.Roles()[0].ID())
	})

	t.Run("UpdateGroupIDs", func(t *testing.T) {
		email, err := internet.NewEmail("updategroups@gmail.com")
		require.NoError(t, err)

		userEntity := user.New(
			"User",
			"Groups",
			email,
			user.UILanguageEN,
			user.WithTenantID(tenant),
		)

		createdUser, err := userRepository.Create(f.ctx, userEntity)
		require.NoError(t, err)
		assert.Empty(t, createdUser.GroupIDs())

		// Add a group
		updatedUser := createdUser.SetGroupIDs([]uuid.UUID{groupID})
		err = userRepository.Update(f.ctx, updatedUser)
		require.NoError(t, err)

		retrievedUser, err := userRepository.GetByID(f.ctx, createdUser.ID())
		require.NoError(t, err)
		assert.Len(t, retrievedUser.GroupIDs(), 1)
		assert.Equal(t, groupID, retrievedUser.GroupIDs()[0])

		// Change to a different group
		updatedUser = retrievedUser.SetGroupIDs([]uuid.UUID{secondGroupID})
		err = userRepository.Update(f.ctx, updatedUser)
		require.NoError(t, err)

		retrievedUser, err = userRepository.GetByID(f.ctx, createdUser.ID())
		require.NoError(t, err)
		assert.Len(t, retrievedUser.GroupIDs(), 1)
		assert.Equal(t, secondGroupID, retrievedUser.GroupIDs()[0])
	})

	t.Run("FilterByRoleID", func(t *testing.T) {
		params := &user.FindParams{
			Filters: []user.Filter{
				{
					Column: user.RoleIDField,
					Filter: repo.Eq(roleEntity.ID()),
				},
			},
			SortBy: user.SortBy{
				Fields: []repo.SortByField[user.Field]{{Field: user.FirstNameField, Ascending: true}},
			},
			Limit:  10,
			Offset: 0,
		}

		users, err := userRepository.GetPaginated(f.ctx, params)
		require.NoError(t, err)

		// Verify all returned users have the role
		for _, u := range users {
			hasRole := false
			for _, r := range u.Roles() {
				if r.ID() == roleEntity.ID() {
					hasRole = true
					break
				}
			}
			assert.True(t, hasRole, "User should have the role")
		}
	})

	t.Run("FilterByRoleID_NotEq", func(t *testing.T) {
		params := &user.FindParams{
			Filters: []user.Filter{
				{
					Column: user.RoleIDField,
					Filter: repo.NotEq(roleEntity.ID()),
				},
			},
			SortBy: user.SortBy{
				Fields: []repo.SortByField[user.Field]{{Field: user.FirstNameField, Ascending: true}},
			},
			Limit:  10,
			Offset: 0,
		}

		users, err := userRepository.GetPaginated(f.ctx, params)
		require.NoError(t, err)

		// Verify none of the returned users have the role
		for _, u := range users {
			hasRole := false
			for _, r := range u.Roles() {
				if r.ID() == roleEntity.ID() {
					hasRole = true
					break
				}
			}
			assert.False(t, hasRole, "User should not have the role")
		}
	})

	t.Run("FilterByRoleID_In", func(t *testing.T) {
		params := &user.FindParams{
			Filters: []user.Filter{
				{
					Column: user.RoleIDField,
					Filter: repo.In([]interface{}{roleEntity.ID(), secondRoleEntity.ID()}),
				},
			},
			SortBy: user.SortBy{
				Fields: []repo.SortByField[user.Field]{{Field: user.FirstNameField, Ascending: true}},
			},
			Limit:  10,
			Offset: 0,
		}

		users, err := userRepository.GetPaginated(f.ctx, params)
		require.NoError(t, err)

		// Verify all returned users have one of the roles
		for _, u := range users {
			hasRole := false
			for _, r := range u.Roles() {
				if r.ID() == roleEntity.ID() || r.ID() == secondRoleEntity.ID() {
					hasRole = true
					break
				}
			}
			assert.True(t, hasRole, "User should have one of the roles")
		}
	})

	t.Run("FilterByGroupID", func(t *testing.T) {
		params := &user.FindParams{
			Filters: []user.Filter{
				{
					Column: user.GroupIDField,
					Filter: repo.Eq(groupID.String()),
				},
			},
			SortBy: user.SortBy{
				Fields: []repo.SortByField[user.Field]{{Field: user.FirstNameField, Ascending: true}},
			},
			Limit:  10,
			Offset: 0,
		}

		users, err := userRepository.GetPaginated(f.ctx, params)
		require.NoError(t, err)

		// Verify all returned users have the group
		for _, u := range users {
			hasGroup := false
			for _, g := range u.GroupIDs() {
				if g == groupID {
					hasGroup = true
					break
				}
			}
			assert.True(t, hasGroup, "User should have the group")
		}
	})

	t.Run("GetPaginated_TenantFiltering", func(t *testing.T) {
		// Create a second tenant for testing cross-tenant isolation
		secondTenant, err := testutils.CreateTestTenant(f.ctx, f.pool)
		require.NoError(t, err)

		// Create users in the first tenant (current context tenant)
		email1, err := internet.NewEmail("tenant1user1@gmail.com")
		require.NoError(t, err)
		user1 := user.New("Tenant1", "User1", email1, user.UILanguageEN, user.WithTenantID(tenant))
		_, err = userRepository.Create(f.ctx, user1)
		require.NoError(t, err)

		email2, err := internet.NewEmail("tenant1user2@gmail.com")
		require.NoError(t, err)
		user2 := user.New("Tenant1", "User2", email2, user.UILanguageEN, user.WithTenantID(tenant))
		_, err = userRepository.Create(f.ctx, user2)
		require.NoError(t, err)

		// Create users in the second tenant by temporarily switching context
		secondTenantCtx := composables.WithTenantID(f.ctx, secondTenant.ID)

		email3, err := internet.NewEmail("tenant2user1@gmail.com")
		require.NoError(t, err)
		user3 := user.New("Tenant2", "User1", email3, user.UILanguageEN, user.WithTenantID(secondTenant.ID))
		_, err = userRepository.Create(secondTenantCtx, user3)
		require.NoError(t, err)

		email4, err := internet.NewEmail("tenant2user2@gmail.com")
		require.NoError(t, err)
		user4 := user.New("Tenant2", "User2", email4, user.UILanguageEN, user.WithTenantID(secondTenant.ID))
		_, err = userRepository.Create(secondTenantCtx, user4)
		require.NoError(t, err)

		// Test GetPaginated with first tenant context (should only return first tenant users)
		params := &user.FindParams{
			SortBy: user.SortBy{
				Fields: []repo.SortByField[user.Field]{{Field: user.FirstNameField, Ascending: true}},
			},
			Limit:  10,
			Offset: 0,
		}

		firstTenantUsers, err := userRepository.GetPaginated(f.ctx, params)
		require.NoError(t, err)

		// Verify all returned users belong to the first tenant
		for _, u := range firstTenantUsers {
			assert.Equal(t, tenant.String(), u.TenantID().String(), "User should belong to first tenant")
		}

		// Count users that match our test pattern (Tenant1*)
		tenant1UserCount := 0
		for _, u := range firstTenantUsers {
			if u.FirstName() == "Tenant1" {
				tenant1UserCount++
			}
		}
		assert.GreaterOrEqual(t, tenant1UserCount, 2, "Should find at least 2 users for first tenant")

		// Test GetPaginated with second tenant context (should only return second tenant users)
		secondTenantUsers, err := userRepository.GetPaginated(secondTenantCtx, params)
		require.NoError(t, err)

		// Verify all returned users belong to the second tenant
		for _, u := range secondTenantUsers {
			assert.Equal(t, secondTenant.ID.String(), u.TenantID().String(), "User should belong to second tenant")
		}

		// Count users that match our test pattern (Tenant2*)
		tenant2UserCount := 0
		for _, u := range secondTenantUsers {
			if u.FirstName() == "Tenant2" {
				tenant2UserCount++
			}
		}
		assert.GreaterOrEqual(t, tenant2UserCount, 2, "Should find at least 2 users for second tenant")

		// Verify no cross-tenant contamination
		for _, u := range firstTenantUsers {
			for _, u2 := range secondTenantUsers {
				assert.NotEqual(t, u.ID(), u2.ID(), "No user should appear in both tenant results")
			}
		}
	})

	t.Run("Delete", func(t *testing.T) {
		email, err := internet.NewEmail("delete@gmail.com")
		require.NoError(t, err)

		userEntity := user.New(
			"Delete",
			"User",
			email,
			user.UILanguageEN,
			user.WithTenantID(tenant),
		)

		createdUser, err := userRepository.Create(f.ctx, userEntity)
		require.NoError(t, err)

		err = userRepository.Delete(f.ctx, createdUser.ID())
		require.NoError(t, err)

		_, err = userRepository.GetByID(f.ctx, createdUser.ID())
		require.Error(t, err)
		require.ErrorIs(t, err, persistence.ErrUserNotFound)
	})
}
