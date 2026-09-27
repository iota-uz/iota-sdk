package query_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/query"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

type countingTx struct {
	pgx.Tx
	queries int
}

func (tx *countingTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	tx.queries++
	return tx.Tx.Query(ctx, sql, args...)
}

func (tx *countingTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	tx.queries++
	return tx.Tx.QueryRow(ctx, sql, args...)
}

func TestPgGroupQueryRepositoryRequiresTenantBeforeSQL(t *testing.T) {
	// Falsely green if a transaction is not installed and UseTx fails before tenant validation.
	fixtures := setupTest(t)
	measured := &countingTx{Tx: fixtures.Tx}
	ctx := composables.WithTx(context.Background(), measured)

	_, _, err := query.NewPgGroupQueryRepository().FindGroups(ctx, &query.GroupFindParams{})
	require.Error(t, err)
	require.Zero(t, measured.queries)
}

func TestPgGroupQueryRepositoryBatchesRelationsAndAssignmentOptions(t *testing.T) {
	// Falsely green if the list contains one group or the measured call bypasses the counting transaction.
	fixtures := setupTest(t)
	tenantID, err := composables.UseTenantID(fixtures.Ctx)
	require.NoError(t, err)
	prefix := uuid.NewString()
	_, err = fixtures.Tx.Exec(fixtures.Ctx, `INSERT INTO user_groups
		(id, type, tenant_id, name, description, created_at, updated_at)
		SELECT gen_random_uuid(), 'user', $1, $2 || '-' || LPAD(n::text, 2, '0'), '', NOW(), NOW()
		FROM generate_series(1, 30) n`, tenantID, prefix)
	require.NoError(t, err)

	repository := query.NewPgGroupQueryRepository()
	queryCount := func(limit int) int {
		measured := &countingTx{Tx: fixtures.Tx}
		ctx := composables.WithTx(fixtures.Ctx, measured)
		groups, _, findErr := repository.FindGroups(ctx, &query.GroupFindParams{Limit: limit, SortBy: query.SortBy{Fields: []repo.SortByField[query.Field]{{Field: query.GroupFieldID, Ascending: false}}}})
		require.NoError(t, findErr)
		require.Len(t, groups, limit)
		return measured.queries
	}
	require.Equal(t, queryCount(1), queryCount(25))

	measured := &countingTx{Tx: fixtures.Tx}
	options, err := repository.FindAssignmentOptions(composables.WithTx(fixtures.Ctx, measured))
	require.NoError(t, err)
	require.Equal(t, 2, measured.queries)
	found := 0
	for _, option := range options {
		if len(option.Name) >= len(prefix) && option.Name[:len(prefix)] == prefix {
			found++
		}
	}
	require.Equal(t, 30, found)
}

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
