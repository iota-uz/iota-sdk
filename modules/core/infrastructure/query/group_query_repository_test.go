package query_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/query"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/stretchr/testify/require"
)

func TestPgGroupQueryRepository_FindGroups(t *testing.T) {
	t.Parallel()

	// Create repository
	groupQueryRepo := query.NewPgGroupQueryRepository()

	// Test without any filters
	t.Run("find all groups", func(t *testing.T) {
		fixtures := setupTest(t)

		params := &query.GroupFindParams{
			Limit:  10,
			Offset: 0,
		}

		groups, count, err := groupQueryRepo.FindGroups(fixtures.Ctx, params)
		require.NoError(t, err)
		require.NotNil(t, groups)
		require.GreaterOrEqual(t, count, 0)
	})

	// Test with search
	t.Run("search groups", func(t *testing.T) {
		fixtures := setupTest(t)

		params := &query.GroupFindParams{
			Limit:  10,
			Offset: 0,
			Search: "admin",
		}

		groups, count, err := groupQueryRepo.SearchGroups(fixtures.Ctx, params)
		require.NoError(t, err)
		require.NotNil(t, groups)
		require.GreaterOrEqual(t, count, 0)
	})

	// Test find by ID
	t.Run("find group by ID", func(t *testing.T) {
		fixtures := setupTest(t)

		// Since we don't have a group yet, this should return an error
		group, err := groupQueryRepo.FindGroupByID(fixtures.Ctx, "non-existent-id")
		require.Error(t, err)
		require.Nil(t, group)
	})

	// Test with filters
	t.Run("find groups with filters", func(t *testing.T) {
		fixtures := setupTest(t)

		params := &query.GroupFindParams{
			Limit:  10,
			Offset: 0,
			Filters: []query.GroupFilter{
				{
					Column: query.GroupFieldType,
					Filter: repo.Eq("user"),
				},
			},
		}

		groups, count, err := groupQueryRepo.FindGroups(fixtures.Ctx, params)
		require.NoError(t, err)
		require.NotNil(t, groups)
		require.GreaterOrEqual(t, count, 0)
	})

	// Test with sorting
	t.Run("find groups with sorting", func(t *testing.T) {
		fixtures := setupTest(t)

		params := &query.GroupFindParams{
			Limit:  10,
			Offset: 0,
			SortBy: query.SortBy{
				Fields: []repo.SortByField[query.Field]{
					{Field: query.GroupFieldName, Ascending: true},
				},
			},
		}

		groups, count, err := groupQueryRepo.FindGroups(fixtures.Ctx, params)
		require.NoError(t, err)
		require.NotNil(t, groups)
		require.GreaterOrEqual(t, count, 0)
	})

	// Test with multiple sort fields
	t.Run("find groups with multiple sort fields", func(t *testing.T) {
		fixtures := setupTest(t)

		params := &query.GroupFindParams{
			Limit:  10,
			Offset: 0,
			SortBy: query.SortBy{
				Fields: []repo.SortByField[query.Field]{
					{Field: query.GroupFieldType, Ascending: false},
					{Field: query.GroupFieldName, Ascending: true},
				},
			},
		}

		groups, count, err := groupQueryRepo.FindGroups(fixtures.Ctx, params)
		require.NoError(t, err)
		require.NotNil(t, groups)
		require.GreaterOrEqual(t, count, 0)
	})

	// Test with sorting and nulls last
	t.Run("find groups with nulls last sorting", func(t *testing.T) {
		fixtures := setupTest(t)

		params := &query.GroupFindParams{
			Limit:  10,
			Offset: 0,
			SortBy: query.SortBy{
				Fields: []repo.SortByField[query.Field]{
					{Field: query.GroupFieldUpdatedAt, Ascending: false, NullsLast: true},
				},
			},
		}

		groups, count, err := groupQueryRepo.FindGroups(fixtures.Ctx, params)
		require.NoError(t, err)
		require.NotNil(t, groups)
		require.GreaterOrEqual(t, count, 0)
	})

	// Test search with sorting
	t.Run("search groups with sorting", func(t *testing.T) {
		fixtures := setupTest(t)

		params := &query.GroupFindParams{
			Limit:  10,
			Offset: 0,
			Search: "test",
			SortBy: query.SortBy{
				Fields: []repo.SortByField[query.Field]{
					{Field: query.GroupFieldName, Ascending: true},
				},
			},
		}

		groups, count, err := groupQueryRepo.SearchGroups(fixtures.Ctx, params)
		require.NoError(t, err)
		require.NotNil(t, groups)
		require.GreaterOrEqual(t, count, 0)
	})

	// Test with combined filters and sorting
	t.Run("find groups with filters and sorting", func(t *testing.T) {
		fixtures := setupTest(t)

		params := &query.GroupFindParams{
			Limit:  10,
			Offset: 0,
			Filters: []query.GroupFilter{
				{
					Column: query.GroupFieldType,
					Filter: repo.NotEq("system"),
				},
			},
			SortBy: query.SortBy{
				Fields: []repo.SortByField[query.Field]{
					{Field: query.GroupFieldCreatedAt, Ascending: false},
				},
			},
		}

		groups, count, err := groupQueryRepo.FindGroups(fixtures.Ctx, params)
		require.NoError(t, err)
		require.NotNil(t, groups)
		require.GreaterOrEqual(t, count, 0)
	})

	// Test with IN filter
	t.Run("find groups with IN filter", func(t *testing.T) {
		fixtures := setupTest(t)

		params := &query.GroupFindParams{
			Limit:  10,
			Offset: 0,
			Filters: []query.GroupFilter{
				{
					Column: query.GroupFieldType,
					Filter: repo.In([]string{"user", "system"}),
				},
			},
		}

		groups, count, err := groupQueryRepo.FindGroups(fixtures.Ctx, params)
		require.NoError(t, err)
		require.NotNil(t, groups)
		require.GreaterOrEqual(t, count, 0)
	})

	// Test pagination
	t.Run("find groups with pagination", func(t *testing.T) {
		fixtures := setupTest(t)

		// First page
		params1 := &query.GroupFindParams{
			Limit:  5,
			Offset: 0,
			SortBy: query.SortBy{
				Fields: []repo.SortByField[query.Field]{
					{Field: query.GroupFieldID, Ascending: true},
				},
			},
		}

		groups1, count1, err := groupQueryRepo.FindGroups(fixtures.Ctx, params1)
		require.NoError(t, err)
		require.NotNil(t, groups1)

		// Second page
		params2 := &query.GroupFindParams{
			Limit:  5,
			Offset: 5,
			SortBy: query.SortBy{
				Fields: []repo.SortByField[query.Field]{
					{Field: query.GroupFieldID, Ascending: true},
				},
			},
		}

		groups2, count2, err := groupQueryRepo.FindGroups(fixtures.Ctx, params2)
		require.NoError(t, err)
		require.NotNil(t, groups2)

		// Total count should be the same
		require.Equal(t, count1, count2)
	})
}
