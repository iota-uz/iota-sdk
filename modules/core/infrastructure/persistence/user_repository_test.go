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
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/phone"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/stretchr/testify/assert"
)

func TestGormUserRepository_CRUD(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	permissionRepository := persistence.NewPermissionRepository()
	roleRepository := persistence.NewRoleRepository()
	uploadRepository := persistence.NewUploadRepository()
	userRepository := persistence.NewUserRepository(uploadRepository)

	roleData, err := role.New(
		"test",
		"test",
		[]*permission.Permission{
			permissions.UserRead,
		},
	)
	assert.NoError(t, err)

	err = permissionRepository.Save(f.ctx, permissions.UserRead)
	assert.NoError(t, err)

	roleEntity, err := roleRepository.Create(f.ctx, roleData)
	assert.NoError(t, err)

	secondRoleData, err := role.New(
		"admin",
		"admin role",
		[]*permission.Permission{
			permissions.UserRead,
		},
	)
	assert.NoError(t, err)

	secondRoleEntity, err := roleRepository.Create(f.ctx, secondRoleData)
	assert.NoError(t, err)

	email, err := internet.NewEmail("test@gmail.com")
	assert.NoError(t, err)

	// Create a phone object
	phoneObj, err := phone.NewFromE164("12345678901")
	assert.NoError(t, err)

	userEntity := user.New(
		"John",
		"Doe",
		email,
		user.UILanguageEN,
		user.WithMiddleName(""),
		user.WithRoles([]role.Role{roleEntity}),
		user.WithPhone(phoneObj),
	)

	createdUser, err := userRepository.Create(f.ctx, userEntity)
	assert.NoError(t, err)

	secondEmail, err := internet.NewEmail("jane@gmail.com")
	assert.NoError(t, err)

	secondUserEntity := user.New(
		"Jane",
		"Smith",
		secondEmail,
		user.UILanguageEN,
		user.WithMiddleName(""),
		user.WithRoles([]role.Role{secondRoleEntity}),
	)

	secondCreatedUser, err := userRepository.Create(f.ctx, secondUserEntity)
	assert.NoError(t, err)

	t.Run("Get", func(t *testing.T) {
		dbUser, err := userRepository.GetByID(f.ctx, createdUser.ID())
		assert.NoError(t, err)
		assert.Equal(t, "John", dbUser.FirstName())
		assert.Equal(t, "Doe", dbUser.LastName())
		assert.Equal(t, "", dbUser.MiddleName())

		// Check if phone is properly persisted
		assert.NotNil(t, dbUser.Phone(), "Phone should not be nil")
		assert.Equal(t, "12345678901", dbUser.Phone().Value(), "Phone number should match")

		roles := dbUser.Roles()
		assert.Len(t, roles, 1)
		assert.Equal(t, "test", roles[0].Name())

		perms := roles[0].Permissions()
		assert.Len(t, perms, 1)
		assert.Equal(t, permissions.UserRead.Name, perms[0].Name)

		groupIDs := dbUser.GroupIDs()
		assert.Len(t, groupIDs, 0)
	})

	t.Run("Update", func(t *testing.T) {
		groupRepository := persistence.NewGroupRepository(userRepository, roleRepository)

		firstGroup := group.New("First Test Group", group.WithID(uuid.New()))
		_, err = groupRepository.Save(f.ctx, firstGroup)
		assert.NoError(t, err)

		secondGroup := group.New("Second Test Group", group.WithID(uuid.New()))
		secondGroup, err := groupRepository.Save(f.ctx, secondGroup)
		assert.NoError(t, err)

		firstGroupID := firstGroup.ID()
		secondGroupID := secondGroup.ID()

		// Create new phone number for update
		newPhoneObj, err := phone.NewFromE164("9876543210")
		assert.NoError(t, err)

		updatedUser := createdUser.SetName("Alice", "Karen", "Smith")
		updatedUser = updatedUser.AddGroupID(firstGroupID)
		updatedUser = updatedUser.SetPhone(newPhoneObj)

		err = userRepository.Update(f.ctx, updatedUser)
		assert.NoError(t, err)

		dbUser, err := userRepository.GetByID(f.ctx, createdUser.ID())
		assert.NoError(t, err)
		assert.Equal(t, "Alice", dbUser.FirstName())
		assert.Equal(t, "Karen", dbUser.LastName())
		assert.Equal(t, "Smith", dbUser.MiddleName())
		// Verify phone was properly updated
		assert.NotNil(t, dbUser.Phone(), "Phone should not be nil after update")
		assert.Equal(t, "9876543210", dbUser.Phone().Value(), "Updated phone number should match")
		assert.True(t, dbUser.UpdatedAt().After(createdUser.UpdatedAt()))

		groupIDs := dbUser.GroupIDs()
		assert.Len(t, groupIDs, 1)
		assert.Equal(t, firstGroupID, groupIDs[0])

		updatedUser = dbUser.AddGroupID(secondGroupID)
		err = userRepository.Update(f.ctx, updatedUser)
		assert.NoError(t, err)

		dbUser, err = userRepository.GetByID(f.ctx, createdUser.ID())
		assert.NoError(t, err)

		groupIDs = dbUser.GroupIDs()
		assert.Len(t, groupIDs, 2)

		hasFirstGroup := false
		hasSecondGroup := false
		for _, id := range groupIDs {
			if id == firstGroupID {
				hasFirstGroup = true
			}
			if id == secondGroupID {
				hasSecondGroup = true
			}
		}
		assert.True(t, hasFirstGroup, "User should have the first group ID")
		assert.True(t, hasSecondGroup, "User should have the second group ID")

		updatedUser = dbUser.RemoveGroupID(firstGroupID)
		err = userRepository.Update(f.ctx, updatedUser)
		assert.NoError(t, err)

		dbUser, err = userRepository.GetByID(f.ctx, createdUser.ID())
		assert.NoError(t, err)

		groupIDs = dbUser.GroupIDs()
		assert.Len(t, groupIDs, 1)
		assert.Equal(t, secondGroupID, groupIDs[0], "User should only have the second group ID")
	})

	t.Run("FilterByRoleID", func(t *testing.T) {
		params := &user.FindParams{
			RoleID: &repo.Filter{
				Expr:  repo.Eq,
				Value: roleEntity.ID(),
			},
			SortBy: user.SortBy{
				Fields:    []user.Field{user.FirstName},
				Ascending: true,
			},
			Limit:  10,
			Offset: 0,
		}

		users, err := userRepository.GetPaginated(f.ctx, params)
		assert.NoError(t, err)

		assert.Len(t, users, 1)
		assert.Equal(t, createdUser.ID(), users[0].ID())

		count, err := userRepository.Count(f.ctx, params)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), count)

		params.RoleID = &repo.Filter{
			Expr:  repo.Eq,
			Value: secondRoleEntity.ID(),
		}

		users, err = userRepository.GetPaginated(f.ctx, params)
		assert.NoError(t, err)

		assert.Len(t, users, 1)
		assert.Equal(t, secondCreatedUser.ID(), users[0].ID())

		count, err = userRepository.Count(f.ctx, params)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), count)
	})

	t.Run("FilterByRoleID_NotEq", func(t *testing.T) {
		params := &user.FindParams{
			RoleID: &repo.Filter{
				Expr:  repo.NotEq,
				Value: roleEntity.ID(),
			},
			SortBy: user.SortBy{
				Fields:    []user.Field{user.FirstName},
				Ascending: true,
			},
			Limit:  10,
			Offset: 0,
		}

		users, err := userRepository.GetPaginated(f.ctx, params)
		assert.NoError(t, err)

		assert.Len(t, users, 1)
		assert.Equal(t, secondCreatedUser.ID(), users[0].ID())

		count, err := userRepository.Count(f.ctx, params)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), count)
	})

	t.Run("FilterByRoleID_In", func(t *testing.T) {
		params := &user.FindParams{
			RoleID: &repo.Filter{
				Expr:  repo.In,
				Value: []interface{}{roleEntity.ID(), secondRoleEntity.ID()},
			},
			SortBy: user.SortBy{
				Fields:    []user.Field{user.FirstName},
				Ascending: true,
			},
			Limit:  10,
			Offset: 0,
		}

		users, err := userRepository.GetPaginated(f.ctx, params)
		assert.NoError(t, err)

		assert.Len(t, users, 2)

		count, err := userRepository.Count(f.ctx, params)
		assert.NoError(t, err)
		assert.Equal(t, int64(2), count)
	})

	t.Run("FilterByGroupID", func(t *testing.T) {
		groupRepository := persistence.NewGroupRepository(userRepository, roleRepository)

		testGroupA := group.New("Test Group A", group.WithID(uuid.New()))
		testGroupA, err := groupRepository.Save(f.ctx, testGroupA)
		assert.NoError(t, err)

		testGroupB := group.New("Test Group B", group.WithID(uuid.New()))
		testGroupB, err = groupRepository.Save(f.ctx, testGroupB)
		assert.NoError(t, err)

		updatedUser := createdUser.AddGroupID(testGroupA.ID())
		err = userRepository.Update(f.ctx, updatedUser)
		assert.NoError(t, err)

		secondUpdatedUser := secondCreatedUser.AddGroupID(testGroupB.ID())
		err = userRepository.Update(f.ctx, secondUpdatedUser)
		assert.NoError(t, err)

		params := &user.FindParams{
			GroupID: &repo.Filter{
				Expr:  repo.Eq,
				Value: testGroupB.ID().String(),
			},
			SortBy: user.SortBy{
				Fields:    []user.Field{user.FirstName},
				Ascending: true,
			},
			Limit:  10,
			Offset: 0,
		}

		users, err := userRepository.GetPaginated(f.ctx, params)
		assert.NoError(t, err)

		assert.Len(t, users, 1)
		assert.Equal(t, secondCreatedUser.ID(), users[0].ID())

		count, err := userRepository.Count(f.ctx, params)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), count)

		params.GroupID = &repo.Filter{
			Expr:  repo.NotEq,
			Value: testGroupB.ID().String(),
		}

		users, err = userRepository.GetPaginated(f.ctx, params)
		assert.NoError(t, err)

		assert.Len(t, users, 1)
		assert.Equal(t, createdUser.ID(), users[0].ID())

		testGroupC := group.New("Test Group C", group.WithID(uuid.New()))
		testGroupC, err = groupRepository.Save(f.ctx, testGroupC)
		assert.NoError(t, err)

		updatedUser = updatedUser.AddGroupID(testGroupC.ID())
		err = userRepository.Update(f.ctx, updatedUser)
		assert.NoError(t, err)

		params.GroupID = &repo.Filter{
			Expr:  repo.In,
			Value: []interface{}{testGroupB.ID().String(), testGroupC.ID().String()},
		}

		users, err = userRepository.GetPaginated(f.ctx, params)
		assert.NoError(t, err)

		assert.Len(t, users, 2)

		count, err = userRepository.Count(f.ctx, params)
		assert.NoError(t, err)
		assert.Equal(t, int64(2), count)
	})

	t.Run("FilterByEmail_Eq", func(t *testing.T) {
		params := &user.FindParams{
			Email: &repo.Filter{
				Expr:  repo.Eq,
				Value: "test@gmail.com",
			},
			SortBy: user.SortBy{
				Fields:    []user.Field{user.FirstName},
				Ascending: true,
			},
			Limit:  10,
			Offset: 0,
		}

		users, err := userRepository.GetPaginated(f.ctx, params)
		assert.NoError(t, err)

		assert.Len(t, users, 1)
		assert.Equal(t, createdUser.ID(), users[0].ID())
	})

	t.Run("FilterByEmail_NotEq", func(t *testing.T) {
		params := &user.FindParams{
			Email: &repo.Filter{
				Expr:  repo.NotEq,
				Value: "test@gmail.com",
			},
			SortBy: user.SortBy{
				Fields:    []user.Field{user.FirstName},
				Ascending: true,
			},
			Limit:  10,
			Offset: 0,
		}

		users, err := userRepository.GetPaginated(f.ctx, params)
		assert.NoError(t, err)

		assert.Len(t, users, 1)
		assert.Equal(t, secondCreatedUser.ID(), users[0].ID())
	})

	t.Run("FilterByEmail_Like", func(t *testing.T) {
		params := &user.FindParams{
			Email: &repo.Filter{
				Expr:  repo.Like,
				Value: "%gmail.com",
			},
			SortBy: user.SortBy{
				Fields:    []user.Field{user.FirstName},
				Ascending: true,
			},
			Limit:  10,
			Offset: 0,
		}

		users, err := userRepository.GetPaginated(f.ctx, params)
		assert.NoError(t, err)

		assert.Len(t, users, 2)
	})

	t.Run("FilterByEmail_In", func(t *testing.T) {
		params := &user.FindParams{
			Email: &repo.Filter{
				Expr:  repo.In,
				Value: []interface{}{"test@gmail.com", "jane@gmail.com"},
			},
			SortBy: user.SortBy{
				Fields:    []user.Field{user.FirstName},
				Ascending: true,
			},
			Limit:  10,
			Offset: 0,
		}

		users, err := userRepository.GetPaginated(f.ctx, params)
		assert.NoError(t, err)

		assert.Len(t, users, 2)
	})

	t.Run("GetByPhone", func(t *testing.T) {
		// Get the user that has the updated phone field from the database first
		userWithPhone, err := userRepository.GetByID(f.ctx, createdUser.ID())
		assert.NoError(t, err)
		assert.NotNil(t, userWithPhone.Phone())
		phoneValue := userWithPhone.Phone().Value()

		// Test fetching user by phone number
		fetchedUser, err := userRepository.GetByPhone(f.ctx, phoneValue)
		assert.NoError(t, err)
		assert.NotNil(t, fetchedUser)
		assert.Equal(t, createdUser.ID(), fetchedUser.ID())
		assert.Equal(t, phoneValue, fetchedUser.Phone().Value())

		// Test fetching with non-existent phone number
		_, err = userRepository.GetByPhone(f.ctx, "1111111111")
		assert.Error(t, err)
		assert.ErrorIs(t, err, persistence.ErrUserNotFound)
	})

	t.Run("FilterByName", func(t *testing.T) {
		params := &user.FindParams{
			Name: "John",
			SortBy: user.SortBy{
				Fields:    []user.Field{user.FirstName},
				Ascending: true,
			},
			Limit:  10,
			Offset: 0,
		}

		users, err := userRepository.GetPaginated(f.ctx, params)
		assert.NoError(t, err)

		assert.Len(t, users, 1)
		assert.Equal(t, createdUser.ID(), users[0].ID())

		params.Name = "Jan"

		users, err = userRepository.GetPaginated(f.ctx, params)
		assert.NoError(t, err)

		assert.Len(t, users, 1)
		assert.Equal(t, secondCreatedUser.ID(), users[0].ID())
	})

	t.Run("FilterByCreatedAt", func(t *testing.T) {
		now := time.Now()
		pastTime := now.Add(-24 * time.Hour)

		params := &user.FindParams{
			CreatedAt: &repo.Filter{
				Expr:  repo.Gt,
				Value: pastTime,
			},
			SortBy: user.SortBy{
				Fields:    []user.Field{user.FirstName},
				Ascending: true,
			},
			Limit:  10,
			Offset: 0,
		}

		users, err := userRepository.GetPaginated(f.ctx, params)
		assert.NoError(t, err)

		assert.Len(t, users, 2)

		params.CreatedAt = &repo.Filter{
			Expr:  repo.Lt,
			Value: now.Add(24 * time.Hour),
		}

		users, err = userRepository.GetPaginated(f.ctx, params)
		assert.NoError(t, err)

		assert.Len(t, users, 2)

		params.CreatedAt = &repo.Filter{
			Expr:  repo.Gte,
			Value: pastTime,
		}

		users, err = userRepository.GetPaginated(f.ctx, params)
		assert.NoError(t, err)

		assert.Len(t, users, 2)

		params.CreatedAt = &repo.Filter{
			Expr:  repo.Lte,
			Value: now.Add(24 * time.Hour),
		}

		users, err = userRepository.GetPaginated(f.ctx, params)
		assert.NoError(t, err)

		assert.Len(t, users, 2)
	})

	t.Run("FilterByLastLogin", func(t *testing.T) {
		err := userRepository.UpdateLastLogin(f.ctx, createdUser.ID())
		assert.NoError(t, err)

		now := time.Now()
		pastTime := now.Add(-24 * time.Hour)

		params := &user.FindParams{
			LastLogin: &repo.Filter{
				Expr:  repo.Gt,
				Value: pastTime,
			},
			SortBy: user.SortBy{
				Fields:    []user.Field{user.FirstName},
				Ascending: true,
			},
			Limit:  10,
			Offset: 0,
		}

		users, err := userRepository.GetPaginated(f.ctx, params)
		assert.NoError(t, err)

		assert.Len(t, users, 1)
		assert.Equal(t, createdUser.ID(), users[0].ID())

		params.LastLogin = &repo.Filter{
			Expr:  repo.Lt,
			Value: now.Add(24 * time.Hour),
		}

		users, err = userRepository.GetPaginated(f.ctx, params)
		assert.NoError(t, err)

		assert.Len(t, users, 1)
		assert.Equal(t, createdUser.ID(), users[0].ID())
	})

	t.Run("CombinedFilters", func(t *testing.T) {
		params := &user.FindParams{
			Name: "John",
			RoleID: &repo.Filter{
				Expr:  repo.Eq,
				Value: roleEntity.ID(),
			},
			SortBy: user.SortBy{
				Fields:    []user.Field{user.FirstName},
				Ascending: true,
			},
			Limit:  10,
			Offset: 0,
		}

		users, err := userRepository.GetPaginated(f.ctx, params)
		assert.NoError(t, err)

		assert.Len(t, users, 1)
		assert.Equal(t, createdUser.ID(), users[0].ID())

		params = &user.FindParams{
			Name: "NonExistentName",
			RoleID: &repo.Filter{
				Expr:  repo.Eq,
				Value: secondRoleEntity.ID(),
			},
			SortBy: user.SortBy{
				Fields:    []user.Field{user.FirstName},
				Ascending: true,
			},
			Limit:  10,
			Offset: 0,
		}

		users, err = userRepository.GetPaginated(f.ctx, params)
		assert.NoError(t, err)

		assert.Empty(t, users)
	})

	t.Run("Delete", func(t *testing.T) {
		assert.NoError(t, userRepository.Delete(f.ctx, 1))

		_, err := userRepository.GetByID(f.ctx, 1)
		assert.Error(t, err)
		assert.ErrorIs(t, err, persistence.ErrUserNotFound)
	})
}
