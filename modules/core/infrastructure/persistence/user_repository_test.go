package persistence_test

import (
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	permissions "github.com/iota-uz/iota-sdk/modules/core/permissions"
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

	// Create first role
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

	// Create second role for testing role filtering
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

	// Create first user with first role
	email, err := internet.NewEmail("test@gmail.com")
	assert.NoError(t, err)
	
	userEntity := user.New(
		"John",
		"Doe",
		email,
		user.UILanguageEN,
		user.WithMiddleName(""),
		user.WithRoles([]role.Role{roleEntity}),
	)

	createdUser, err := userRepository.Create(f.ctx, userEntity)
	assert.NoError(t, err)

	// Create second user with second role
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
		
		roles := dbUser.Roles()
		assert.Len(t, roles, 1)
		assert.Equal(t, "test", roles[0].Name())
		
		perms := roles[0].Permissions()
		assert.Len(t, perms, 1)
		assert.Equal(t, permissions.UserRead.Name, perms[0].Name)
	})

	t.Run("Update", func(t *testing.T) {
		err := userRepository.Update(
			f.ctx,
			createdUser.SetName("Alice", "Karen", "Smith"),
		)
		assert.NoError(t, err)
		
		dbUser, err := userRepository.GetByID(f.ctx, createdUser.ID())
		assert.NoError(t, err)
		assert.Equal(t, "Alice", dbUser.FirstName())
		assert.Equal(t, "Karen", dbUser.LastName())
		assert.Equal(t, "Smith", dbUser.MiddleName())
		assert.True(t, dbUser.UpdatedAt().After(createdUser.UpdatedAt()))
	})

	t.Run("FilterByRoleID", func(t *testing.T) {
		// Test filtering by the first role
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

		// Get users with first role
		users, err := userRepository.GetPaginated(f.ctx, params)
		assert.NoError(t, err)

		// Should return only one user with the first role
		assert.Len(t, users, 1)
		assert.Equal(t, createdUser.ID(), users[0].ID())

		// Test count with role filter
		count, err := userRepository.Count(f.ctx, params)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), count)

		// Test filtering by the second role
		params.RoleID = &repo.Filter{
			Expr:  repo.Eq,
			Value: secondRoleEntity.ID(),
		}

		// Get users with second role
		users, err = userRepository.GetPaginated(f.ctx, params)
		assert.NoError(t, err)

		// Should return only one user with the second role
		assert.Len(t, users, 1)
		assert.Equal(t, secondCreatedUser.ID(), users[0].ID())

		// Test count with second role filter
		count, err = userRepository.Count(f.ctx, params)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), count)
	})

	t.Run("FilterByRoleID_NotEq", func(t *testing.T) {
		// Test filtering with NotEq expression
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

		// Get users without the first role
		users, err := userRepository.GetPaginated(f.ctx, params)
		assert.NoError(t, err)
		
		// Should return only the second user (that has the second role)
		assert.Len(t, users, 1)
		assert.Equal(t, secondCreatedUser.ID(), users[0].ID())
		
		// Test count with NotEq filter
		count, err := userRepository.Count(f.ctx, params)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), count)
	})
	
	t.Run("FilterByRoleID_In", func(t *testing.T) {
		// Test filtering with In expression
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

		// Get users with either first or second role
		users, err := userRepository.GetPaginated(f.ctx, params)
		assert.NoError(t, err)
		
		// Should return both users
		assert.Len(t, users, 2)
		
		// Test count with In filter
		count, err := userRepository.Count(f.ctx, params)
		assert.NoError(t, err)
		assert.Equal(t, int64(2), count)
	})

	t.Run("FilterByEmail_Eq", func(t *testing.T) {
		// Test filtering by email with Eq expression
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

		// Get users with email = test@gmail.com
		users, err := userRepository.GetPaginated(f.ctx, params)
		assert.NoError(t, err)
		
		// Should return the first user
		assert.Len(t, users, 1)
		assert.Equal(t, createdUser.ID(), users[0].ID())
	})
	
	t.Run("FilterByEmail_NotEq", func(t *testing.T) {
		// Test filtering by email with NotEq expression
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

		// Get users with email != test@gmail.com
		users, err := userRepository.GetPaginated(f.ctx, params)
		assert.NoError(t, err)
		
		// Should return the second user
		assert.Len(t, users, 1)
		assert.Equal(t, secondCreatedUser.ID(), users[0].ID())
	})
	
	t.Run("FilterByEmail_Like", func(t *testing.T) {
		// Test filtering by email with Like expression
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

		// Get users with email like %gmail.com
		users, err := userRepository.GetPaginated(f.ctx, params)
		assert.NoError(t, err)
		
		// Should return both users
		assert.Len(t, users, 2)
	})
	
	t.Run("FilterByEmail_In", func(t *testing.T) {
		// Test filtering by email with In expression
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

		// Get users with email in the list
		users, err := userRepository.GetPaginated(f.ctx, params)
		assert.NoError(t, err)
		
		// Should return both users
		assert.Len(t, users, 2)
	})
	
	t.Run("FilterByName", func(t *testing.T) {
		// Test filtering by name
		params := &user.FindParams{
			Name: "Ali",  // Updated to match "Alice" from the previous test
			SortBy: user.SortBy{
				Fields:    []user.Field{user.FirstName},
				Ascending: true,
			},
			Limit:  10,
			Offset: 0,
		}

		// Get users with name containing "Ali"
		users, err := userRepository.GetPaginated(f.ctx, params)
		assert.NoError(t, err)
		
		// Should return the first user (Alice)
		assert.Len(t, users, 1)
		assert.Equal(t, createdUser.ID(), users[0].ID())
		
		// Change name filter to match second user
		params.Name = "Jan"
		
		// Get users with name containing "Jan"
		users, err = userRepository.GetPaginated(f.ctx, params)
		assert.NoError(t, err)
		
		// Should return the second user (Jane)
		assert.Len(t, users, 1)
		assert.Equal(t, secondCreatedUser.ID(), users[0].ID())
	})
	
	t.Run("FilterByCreatedAt", func(t *testing.T) {
		// Get a timestamp for comparison
		now := time.Now()
		pastTime := now.Add(-24 * time.Hour)
		
		// Test filtering by created_at with Gt expression
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

		// Get users created after yesterday
		users, err := userRepository.GetPaginated(f.ctx, params)
		assert.NoError(t, err)
		
		// Should return both users
		assert.Len(t, users, 2)
		
		// Test with Lt expression
		params.CreatedAt = &repo.Filter{
			Expr:  repo.Lt,
			Value: now.Add(24 * time.Hour), // tomorrow
		}
		
		// Get users created before tomorrow
		users, err = userRepository.GetPaginated(f.ctx, params)
		assert.NoError(t, err)
		
		// Should return both users
		assert.Len(t, users, 2)
		
		// Test with Gte expression
		params.CreatedAt = &repo.Filter{
			Expr:  repo.Gte,
			Value: pastTime,
		}
		
		// Get users created on or after yesterday
		users, err = userRepository.GetPaginated(f.ctx, params)
		assert.NoError(t, err)
		
		// Should return both users
		assert.Len(t, users, 2)
		
		// Test with Lte expression
		params.CreatedAt = &repo.Filter{
			Expr:  repo.Lte,
			Value: now.Add(24 * time.Hour), // tomorrow
		}
		
		// Get users created on or before tomorrow
		users, err = userRepository.GetPaginated(f.ctx, params)
		assert.NoError(t, err)
		
		// Should return both users
		assert.Len(t, users, 2)
	})
	
	t.Run("FilterByLastLogin", func(t *testing.T) {
		// Update last login for the first user
		err := userRepository.UpdateLastLogin(f.ctx, createdUser.ID())
		assert.NoError(t, err)
		
		// Get a timestamp for comparison
		now := time.Now()
		pastTime := now.Add(-24 * time.Hour)
		
		// Test filtering by last_login with Gt expression
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

		// Get users with last login after yesterday
		users, err := userRepository.GetPaginated(f.ctx, params)
		assert.NoError(t, err)
		
		// Should return only the first user
		assert.Len(t, users, 1)
		assert.Equal(t, createdUser.ID(), users[0].ID())
		
		// Test with Lt expression
		params.LastLogin = &repo.Filter{
			Expr:  repo.Lt,
			Value: now.Add(24 * time.Hour), // tomorrow
		}
		
		// Get users with last login before tomorrow
		users, err = userRepository.GetPaginated(f.ctx, params)
		assert.NoError(t, err)
		
		// Should return only the first user (second user has no last login)
		assert.Len(t, users, 1)
		assert.Equal(t, createdUser.ID(), users[0].ID())
	})

	t.Run("CombinedFilters", func(t *testing.T) {
		// Test combining multiple filters
		params := &user.FindParams{
			Name: "Ali", // Matches "Alice" from the Update test
			RoleID: &repo.Filter{
				Expr:  repo.Eq,
				Value: roleEntity.ID(), // First role
			},
			SortBy: user.SortBy{
				Fields:    []user.Field{user.FirstName},
				Ascending: true,
			},
			Limit:  10,
			Offset: 0,
		}

		// Get users matching both filters
		users, err := userRepository.GetPaginated(f.ctx, params)
		assert.NoError(t, err)
		
		// Should return the first user
		assert.Len(t, users, 1)
		assert.Equal(t, createdUser.ID(), users[0].ID())
		
		// Test with non-matching combination
		params = &user.FindParams{
			Name: "Ali", // Matches "Alice" 
			RoleID: &repo.Filter{
				Expr:  repo.Eq,
				Value: secondRoleEntity.ID(), // Second role
			},
			SortBy: user.SortBy{
				Fields:    []user.Field{user.FirstName},
				Ascending: true,
			},
			Limit:  10,
			Offset: 0,
		}

		// Get users matching both filters
		users, err = userRepository.GetPaginated(f.ctx, params)
		assert.NoError(t, err)
		
		// Should return no users (Alice doesn't have the second role)
		assert.Empty(t, users)
	})

	t.Run("Delete", func(t *testing.T) {
		assert.NoError(t, userRepository.Delete(f.ctx, 1))
		
		_, err := userRepository.GetByID(f.ctx, 1)
		assert.Error(t, err)
		assert.ErrorIs(t, err, persistence.ErrUserNotFound)
	})
}