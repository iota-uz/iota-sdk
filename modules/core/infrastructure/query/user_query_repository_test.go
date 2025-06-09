package query_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/query"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/stretchr/testify/require"
)

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

		users, count, err := userQueryRepo.FindUsers(fixtures.ctx, params)
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

		users, count, err := userQueryRepo.SearchUsers(fixtures.ctx, params)
		require.NoError(t, err)
		require.NotNil(t, users)
		require.GreaterOrEqual(t, count, 0)
	})

	// Test find by ID
	t.Run("find user by ID", func(t *testing.T) {
		fixtures := setupTest(t)

		// Since we don't have a user yet, this should return an error
		user, err := userQueryRepo.FindUserByID(fixtures.ctx, 999999)
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

		users, count, err := userQueryRepo.FindUsers(fixtures.ctx, params)
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

		users, count, err := userQueryRepo.FindUsers(fixtures.ctx, params)
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

		users, count, err := userQueryRepo.FindUsers(fixtures.ctx, params)
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

		users, count, err := userQueryRepo.FindUsers(fixtures.ctx, params)
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

		users, count, err := userQueryRepo.SearchUsers(fixtures.ctx, params)
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

		users, count, err := userQueryRepo.FindUsers(fixtures.ctx, params)
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

		users, count, err := userQueryRepo.FindUsers(fixtures.ctx, params)
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

		users1, count1, err := userQueryRepo.FindUsers(fixtures.ctx, params1)
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

		users2, count2, err := userQueryRepo.FindUsers(fixtures.ctx, params2)
		require.NoError(t, err)
		require.NotNil(t, users2)

		// Total count should be the same
		require.Equal(t, count1, count2)
	})
}
