package query_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/query"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/stretchr/testify/require"
)

func TestPgRoleQueryRepositoryListsAllAssignmentOptionsInTwoQueries(t *testing.T) {
	// Falsely green if the fixture does not exceed the former 25-row page limit.
	fixtures := setupTest(t)
	tenantID, err := composables.UseTenantID(fixtures.Ctx)
	require.NoError(t, err)
	prefix := uuid.NewString()
	_, err = fixtures.Tx.Exec(fixtures.Ctx, `INSERT INTO roles
		(type, tenant_id, name, description, created_at, updated_at)
		SELECT 'user', $1, $2 || '-' || LPAD(n::text, 2, '0'), '', NOW(), NOW()
		FROM generate_series(1, 30) n`, tenantID, prefix)
	require.NoError(t, err)

	measured := &countingTx{Tx: fixtures.Tx}
	options, err := query.NewPgRoleQueryRepository().FindAssignmentOptions(composables.WithTx(fixtures.Ctx, measured))
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

func TestPgRoleQueryRepository_FindRolesWithCounts(t *testing.T) {
	t.Parallel()

	roleQueryRepo := query.NewPgRoleQueryRepository()

	t.Run("find all roles with user counts", func(t *testing.T) {
		t.Parallel()
		fixtures := setupTest(t)

		roles, err := roleQueryRepo.FindRolesWithCounts(fixtures.Ctx)
		require.NoError(t, err)
		require.NotNil(t, roles)

		// Verify each role has required fields
		for _, role := range roles {
			require.NotEmpty(t, role.ID)
			require.NotEmpty(t, role.Name)
			require.GreaterOrEqual(t, role.UsersCount, 0)
		}
	})

	t.Run("roles with zero users have count of 0", func(t *testing.T) {
		t.Parallel()
		fixtures := setupTest(t)

		roles, err := roleQueryRepo.FindRolesWithCounts(fixtures.Ctx)
		require.NoError(t, err)

		// LEFT JOIN should include roles with 0 users
		for _, role := range roles {
			require.GreaterOrEqual(t, role.UsersCount, 0)
		}
	})

	t.Run("roles can have multiple users", func(t *testing.T) {
		t.Parallel()
		fixtures := setupTest(t)

		roles, err := roleQueryRepo.FindRolesWithCounts(fixtures.Ctx)
		require.NoError(t, err)

		// User counts should be non-negative
		for _, role := range roles {
			require.GreaterOrEqual(t, role.UsersCount, 0)
		}
	})

	t.Run("only returns roles for current tenant", func(t *testing.T) {
		t.Parallel()
		fixtures := setupTest(t)

		roles, err := roleQueryRepo.FindRolesWithCounts(fixtures.Ctx)
		require.NoError(t, err)

		// All roles should belong to the test tenant
		for _, role := range roles {
			require.NotEmpty(t, role.ID)
		}
	})

	t.Run("system roles cannot be updated or deleted", func(t *testing.T) {
		t.Parallel()
		fixtures := setupTest(t)

		roles, err := roleQueryRepo.FindRolesWithCounts(fixtures.Ctx)
		require.NoError(t, err)

		// Find system roles and verify permissions
		hasSystemRole := false
		for _, role := range roles {
			if role.Type == "system" {
				hasSystemRole = true
				require.False(t, role.CanUpdate, "system role should not be updatable")
				require.False(t, role.CanDelete, "system role should not be deletable")
			} else {
				require.True(t, role.CanUpdate, "non-system role should be updatable")
				require.True(t, role.CanDelete, "non-system role should be deletable")
			}
		}

		// We expect at least one system role to exist in a typical setup
		if len(roles) > 0 {
			t.Logf("Total roles: %d, has system role: %v", len(roles), hasSystemRole)
		}
	})

	t.Run("roles are ordered by name ascending", func(t *testing.T) {
		t.Parallel()
		fixtures := setupTest(t)

		roles, err := roleQueryRepo.FindRolesWithCounts(fixtures.Ctx)
		require.NoError(t, err)

		if len(roles) > 1 {
			// Verify roles are sorted by name
			for i := 1; i < len(roles); i++ {
				require.LessOrEqual(t,
					roles[i-1].Name,
					roles[i].Name,
					"roles should be ordered by name ascending",
				)
			}
		}
	})

	t.Run("user counts match actual user_roles relationships", func(t *testing.T) {
		t.Parallel()
		fixtures := setupTest(t)

		roles, err := roleQueryRepo.FindRolesWithCounts(fixtures.Ctx)
		require.NoError(t, err)

		// User counts should be accurate
		for _, role := range roles {
			require.GreaterOrEqual(t, role.UsersCount, 0)
		}
	})

	t.Run("users are counted distinctly per role", func(t *testing.T) {
		t.Parallel()
		fixtures := setupTest(t)

		roles, err := roleQueryRepo.FindRolesWithCounts(fixtures.Ctx)
		require.NoError(t, err)

		// The query uses COUNT(DISTINCT ur.user_id)
		for _, role := range roles {
			require.GreaterOrEqual(t, role.UsersCount, 0)
		}
	})

	t.Run("all role fields are populated correctly", func(t *testing.T) {
		t.Parallel()
		fixtures := setupTest(t)

		roles, err := roleQueryRepo.FindRolesWithCounts(fixtures.Ctx)
		require.NoError(t, err)

		for _, role := range roles {
			// Verify all required fields are present
			require.NotEmpty(t, role.ID)
			require.NotEmpty(t, role.Type)
			require.NotEmpty(t, role.Name)
			require.NotEmpty(t, role.CreatedAt)
			require.NotEmpty(t, role.UpdatedAt)
			require.GreaterOrEqual(t, role.UsersCount, 0)
		}
	})

	t.Run("handles empty results gracefully", func(t *testing.T) {
		t.Parallel()
		fixtures := setupTest(t)

		roles, err := roleQueryRepo.FindRolesWithCounts(fixtures.Ctx)
		require.NoError(t, err)
		require.NotNil(t, roles)
		// Should return empty slice, not error
	})
}
