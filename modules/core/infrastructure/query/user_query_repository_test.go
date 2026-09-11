package query_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/query"
	permissions "github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/stretchr/testify/require"
)

func TestPgUserQueryRepositoryBatchesRelations(t *testing.T) {
	// Falsely green if the list contains one user or the measured call bypasses the counting transaction.
	fixtures := setupTest(t)
	tenantID, err := composables.UseTenantID(fixtures.Ctx)
	require.NoError(t, err)
	prefix := uuid.NewString()
	_, err = fixtures.Tx.Exec(fixtures.Ctx, `INSERT INTO users
		(tenant_id, type, first_name, last_name, email, ui_language, created_at, updated_at)
		SELECT $1, 'user', 'Batch', n::text, $2 || '-' || n::text || '@example.test', 'en', NOW(), NOW()
		FROM generate_series(1, 30) n`, tenantID, prefix)
	require.NoError(t, err)

	repository := query.NewPgUserQueryRepository()
	queryCount := func(limit int) int {
		measured := &countingTx{Tx: fixtures.Tx}
		ctx := composables.WithTx(fixtures.Ctx, measured)
		users, _, findErr := repository.FindUsers(ctx, &query.FindParams{Limit: limit, SortBy: query.SortBy{Fields: []repo.SortByField[query.Field]{{Field: query.FieldID, Ascending: false}}}})
		require.NoError(t, findErr)
		require.Len(t, users, limit)
		return measured.queries
	}
	require.Equal(t, queryCount(1), queryCount(25))
}

func TestPgUserQueryRepositoryIncludesGroupRolePermissions(t *testing.T) {
	// Falsely green if the permission is also assigned directly or through a direct user role.
	fixtures := setupTest(t)
	tenantID, err := composables.UseTenantID(fixtures.Ctx)
	require.NoError(t, err)
	require.NoError(t, persistence.NewPermissionRepository().Save(fixtures.Ctx, permissions.UserRead))
	permissionID := permissions.UserRead.ID()
	var userID uint
	require.NoError(t, fixtures.Tx.QueryRow(fixtures.Ctx, `INSERT INTO users
		(tenant_id, type, first_name, last_name, email, ui_language)
		VALUES ($1, 'user', 'Group', 'Permission', $2, 'en') RETURNING id`, tenantID, uuid.NewString()+"@example.test").Scan(&userID))
	var roleID uint
	require.NoError(t, fixtures.Tx.QueryRow(fixtures.Ctx, `INSERT INTO roles
		(type, tenant_id, name, description) VALUES ('user', $1, $2, '') RETURNING id`, tenantID, uuid.NewString()).Scan(&roleID))
	groupID := uuid.New()
	_, err = fixtures.Tx.Exec(fixtures.Ctx, `INSERT INTO user_groups (id, type, tenant_id, name, description) VALUES ($1, 'user', $2, $3, '')`, groupID, tenantID, uuid.NewString())
	require.NoError(t, err)
	_, err = fixtures.Tx.Exec(fixtures.Ctx, `INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2)`, roleID, permissionID)
	require.NoError(t, err)
	_, err = fixtures.Tx.Exec(fixtures.Ctx, `INSERT INTO group_roles (group_id, role_id) VALUES ($1, $2)`, groupID, roleID)
	require.NoError(t, err)
	_, err = fixtures.Tx.Exec(fixtures.Ctx, `INSERT INTO group_users (group_id, user_id) VALUES ($1, $2)`, groupID, userID)
	require.NoError(t, err)

	result, err := query.NewPgUserQueryRepository().FindUserByID(fixtures.Ctx, int(userID))
	require.NoError(t, err)
	require.Empty(t, result.Permissions)
	require.Empty(t, result.DirectPermissions)
	require.Len(t, result.EffectivePermissions, 1)
	require.Equal(t, permissionID.String(), result.EffectivePermissions[0].ID)
}

func TestPgUserQueryRepository_FindUsers(t *testing.T) {
	t.Parallel()

	// Create repository
	userQueryRepo := query.NewPgUserQueryRepository()

	// Test without any filters
	t.Run("find all users", func(t *testing.T) {
		fixtures := setupTest(t)

		params := &query.FindParams{
			Limit:  10,
			Offset: 0,
		}

		users, count, err := userQueryRepo.FindUsers(fixtures.Ctx, params)
		require.NoError(t, err)
		require.NotNil(t, users)
		require.GreaterOrEqual(t, count, 0)
	})

	// Test with search
	t.Run("search users", func(t *testing.T) {
		fixtures := setupTest(t)

		params := &query.FindParams{
			Limit:  10,
			Offset: 0,
			Search: "admin",
		}

		users, count, err := userQueryRepo.SearchUsers(fixtures.Ctx, params)
		require.NoError(t, err)
		require.NotNil(t, users)
		require.GreaterOrEqual(t, count, 0)
	})

	// Test find by ID
	t.Run("find user by ID", func(t *testing.T) {
		fixtures := setupTest(t)

		// Since we don't have a user yet, this should return an error
		user, err := userQueryRepo.FindUserByID(fixtures.Ctx, 999999)
		require.Error(t, err)
		require.Nil(t, user)
	})

	// Test with filters
	t.Run("find users with filters", func(t *testing.T) {
		fixtures := setupTest(t)

		params := &query.FindParams{
			Limit:  10,
			Offset: 0,
			Filters: []query.Filter{
				{
					Column: query.FieldType,
					Filter: repo.Eq("admin"),
				},
			},
		}

		users, count, err := userQueryRepo.FindUsers(fixtures.Ctx, params)
		require.NoError(t, err)
		require.NotNil(t, users)
		require.GreaterOrEqual(t, count, 0)
	})

	// Test with sorting
	t.Run("find users with sorting", func(t *testing.T) {
		fixtures := setupTest(t)

		params := &query.FindParams{
			Limit:  10,
			Offset: 0,
			SortBy: query.SortBy{
				Fields: []repo.SortByField[query.Field]{
					{Field: query.FieldFirstName, Ascending: true},
				},
			},
		}

		users, count, err := userQueryRepo.FindUsers(fixtures.Ctx, params)
		require.NoError(t, err)
		require.NotNil(t, users)
		require.GreaterOrEqual(t, count, 0)
	})

	// Test with multiple sort fields
	t.Run("find users with multiple sort fields", func(t *testing.T) {
		fixtures := setupTest(t)

		params := &query.FindParams{
			Limit:  10,
			Offset: 0,
			SortBy: query.SortBy{
				Fields: []repo.SortByField[query.Field]{
					{Field: query.FieldLastName, Ascending: false},
					{Field: query.FieldFirstName, Ascending: true},
				},
			},
		}

		users, count, err := userQueryRepo.FindUsers(fixtures.Ctx, params)
		require.NoError(t, err)
		require.NotNil(t, users)
		require.GreaterOrEqual(t, count, 0)
	})

	// Test with sorting and nulls last
	t.Run("find users with nulls last sorting", func(t *testing.T) {
		fixtures := setupTest(t)

		params := &query.FindParams{
			Limit:  10,
			Offset: 0,
			SortBy: query.SortBy{
				Fields: []repo.SortByField[query.Field]{
					{Field: query.FieldUpdatedAt, Ascending: false, NullsLast: true},
				},
			},
		}

		users, count, err := userQueryRepo.FindUsers(fixtures.Ctx, params)
		require.NoError(t, err)
		require.NotNil(t, users)
		require.GreaterOrEqual(t, count, 0)
	})

	// Test search with sorting
	t.Run("search users with sorting", func(t *testing.T) {
		fixtures := setupTest(t)

		params := &query.FindParams{
			Limit:  10,
			Offset: 0,
			Search: "test",
			SortBy: query.SortBy{
				Fields: []repo.SortByField[query.Field]{
					{Field: query.FieldEmail, Ascending: true},
				},
			},
		}

		users, count, err := userQueryRepo.SearchUsers(fixtures.Ctx, params)
		require.NoError(t, err)
		require.NotNil(t, users)
		require.GreaterOrEqual(t, count, 0)
	})

	// Test with combined filters and sorting
	t.Run("find users with filters and sorting", func(t *testing.T) {
		fixtures := setupTest(t)

		params := &query.FindParams{
			Limit:  10,
			Offset: 0,
			Filters: []query.Filter{
				{
					Column: query.FieldType,
					Filter: repo.NotEq("guest"),
				},
			},
			SortBy: query.SortBy{
				Fields: []repo.SortByField[query.Field]{
					{Field: query.FieldCreatedAt, Ascending: false},
				},
			},
		}

		users, count, err := userQueryRepo.FindUsers(fixtures.Ctx, params)
		require.NoError(t, err)
		require.NotNil(t, users)
		require.GreaterOrEqual(t, count, 0)
	})

	// Test with IN filter
	t.Run("find users with IN filter", func(t *testing.T) {
		fixtures := setupTest(t)

		params := &query.FindParams{
			Limit:  10,
			Offset: 0,
			Filters: []query.Filter{
				{
					Column: query.FieldType,
					Filter: repo.In([]string{"admin", "user"}),
				},
			},
		}

		users, count, err := userQueryRepo.FindUsers(fixtures.Ctx, params)
		require.NoError(t, err)
		require.NotNil(t, users)
		require.GreaterOrEqual(t, count, 0)
	})

	// Test pagination
	t.Run("find users with pagination", func(t *testing.T) {
		fixtures := setupTest(t)

		// First page
		params1 := &query.FindParams{
			Limit:  5,
			Offset: 0,
			SortBy: query.SortBy{
				Fields: []repo.SortByField[query.Field]{
					{Field: query.FieldID, Ascending: true},
				},
			},
		}

		users1, count1, err := userQueryRepo.FindUsers(fixtures.Ctx, params1)
		require.NoError(t, err)
		require.NotNil(t, users1)

		// Second page
		params2 := &query.FindParams{
			Limit:  5,
			Offset: 5,
			SortBy: query.SortBy{
				Fields: []repo.SortByField[query.Field]{
					{Field: query.FieldID, Ascending: true},
				},
			},
		}

		users2, count2, err := userQueryRepo.FindUsers(fixtures.Ctx, params2)
		require.NoError(t, err)
		require.NotNil(t, users2)

		// Total count should be the same
		require.Equal(t, count1, count2)
	})
}
