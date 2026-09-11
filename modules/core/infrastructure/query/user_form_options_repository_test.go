package query_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/query"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
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

func TestPgUserFormOptionsRepositoryListsEveryTenantOptionWithConstantQueries(t *testing.T) {
	// Falsely green if the test does not exceed the standard 25-row page or if
	// the measured call can bypass the counting transaction.
	t.Parallel()
	fixtures := setupTest(t)
	tenantID, err := composables.UseTenantID(fixtures.Ctx)
	require.NoError(t, err)
	otherTenant, err := itf.CreateTestTenant(fixtures.Ctx, fixtures.Pool)
	require.NoError(t, err)
	prefix := uuid.NewString()

	_, err = fixtures.Tx.Exec(fixtures.Ctx, `
		INSERT INTO roles (type, tenant_id, name, description, created_at, updated_at)
		SELECT 'user', $1, $2 || '-' || LPAD(n::text, 2, '0'), '', NOW(), NOW()
		FROM generate_series(1, 40) n`, tenantID, prefix)
	require.NoError(t, err)
	_, err = fixtures.Tx.Exec(fixtures.Ctx, `
		INSERT INTO user_groups (id, type, tenant_id, name, description, created_at, updated_at)
		VALUES ($1, 'user', $2, $3, '', NOW(), NOW())`, uuid.New(), tenantID, prefix+"-group")
	require.NoError(t, err)
	_, err = fixtures.Tx.Exec(fixtures.Ctx, `
		INSERT INTO roles (type, tenant_id, name, description, created_at, updated_at)
		VALUES ('user', $1, $2, '', NOW(), NOW())`, otherTenant.ID, prefix+"-other-role")
	require.NoError(t, err)
	_, err = fixtures.Tx.Exec(fixtures.Ctx, `
		INSERT INTO user_groups (id, type, tenant_id, name, description, created_at, updated_at)
		VALUES ($1, 'user', $2, $3, '', NOW(), NOW())`, uuid.New(), otherTenant.ID, prefix+"-other-group")
	require.NoError(t, err)

	measuredTx := &countingTx{Tx: fixtures.Tx}
	ctx := composables.WithTx(fixtures.Ctx, measuredTx)
	set, err := query.NewPgUserFormOptionsRepository().ListUserFormOptions(ctx)
	require.NoError(t, err)
	assert.Equal(t, 4, measuredTx.queries, "option loading must stay constant as role/group cardinality grows")

	roleNames := make([]string, 0, len(set.Roles))
	roleSortKeys := make([]string, 0, len(set.Roles))
	for _, option := range set.Roles {
		roleNames = append(roleNames, option.Name)
		roleSortKeys = append(roleSortKeys, strings.ToLower(option.Name))
		assert.NotEqual(t, prefix+"-other-role", option.Name)
	}
	for n := 1; n <= 40; n++ {
		assert.Contains(t, roleNames, fmt.Sprintf("%s-%02d", prefix, n))
	}
	groupNames := make([]string, 0, len(set.Groups))
	groupSortKeys := make([]string, 0, len(set.Groups))
	for _, option := range set.Groups {
		groupNames = append(groupNames, option.Name)
		groupSortKeys = append(groupSortKeys, strings.ToLower(option.Name))
		assert.NotEqual(t, prefix+"-other-group", option.Name)
	}
	assert.Contains(t, groupNames, prefix+"-group")
	assert.IsNonDecreasing(t, roleSortKeys)
	assert.IsNonDecreasing(t, groupSortKeys)
}
