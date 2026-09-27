package itf

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"testing"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain moves the working directory to the repository root so the harness
// finds the migrations directory when it compiles the full module set — the
// same convention the module controller tests use.
func TestMain(m *testing.M) {
	if err := os.Chdir("../.."); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func TestSuite_MountAll_RegistersApplicationControllers(t *testing.T) {
	t.Parallel()

	suite := NewSuite(t, modules.Components()...).
		AsUser(User(permissions.UserRead)).
		MountAll()

	// /users is served by a core controller that NewSuite does not mount on
	// its own; MountAll must have registered the production route table, so
	// the page renders instead of 404.
	suite.GET("/users").Expect(t).Status(200)
}

func TestSuite_RoutesStay404WithoutMounting(t *testing.T) {
	t.Parallel()

	suite := NewSuite(t, modules.Components()...).
		AsUser(User(permissions.UserRead))

	suite.GET("/users").Expect(t).Status(404)
}

func TestCommitTx_MakesScopeWritesVisibleToPoolReaders(t *testing.T) {
	t.Parallel()

	env := Setup(t, WithComponents(modules.Components()...))

	table := fmt.Sprintf("commit_tx_probe_%d", atomic.AddUint32(&probeSeq, 1))
	_, err := env.Tx.Exec(env.Ctx, fmt.Sprintf("CREATE TABLE %s (id int PRIMARY KEY)", table))
	require.NoError(t, err)
	_, err = env.Tx.Exec(env.Ctx, fmt.Sprintf("INSERT INTO %s (id) VALUES (1)", table))
	require.NoError(t, err)

	// Before the commit a pool reader (its own connection, outside the scope
	// transaction) cannot see the table.
	_, err = env.Pool.Exec(context.Background(), fmt.Sprintf("SELECT id FROM %s", table))
	require.Error(t, err, "uncommitted scope writes must not be visible to pool readers")

	env.CommitTx(t)

	_, err = env.Pool.Exec(context.Background(), fmt.Sprintf("SELECT id FROM %s WHERE id = 1", table))
	assert.NoError(t, err, "committed scope writes must be visible to pool readers")
}

func TestCommitTx_KeepsRollbackIsolationForLaterWork(t *testing.T) {
	t.Parallel()

	env := Setup(t, WithComponents(modules.Components()...))

	table := fmt.Sprintf("commit_tx_iso_%d", atomic.AddUint32(&probeSeq, 1))
	_, err := env.Tx.Exec(env.Ctx, fmt.Sprintf("CREATE TABLE %s (id int PRIMARY KEY)", table))
	require.NoError(t, err)
	env.CommitTx(t)

	// Writes through the replacement transaction roll back with the test.
	_, err = env.Tx.Exec(env.Ctx, fmt.Sprintf("INSERT INTO %s (id) VALUES (2)", table))
	require.NoError(t, err)
	var n int
	require.NoError(t, env.Tx.QueryRow(env.Ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&n))
	assert.Equal(t, 1, n) // sees its own uncommitted insert

	pool := env.Pool
	require.NoError(t, env.Tx.Rollback(env.Ctx))
	require.NoError(t, pool.QueryRow(context.Background(), fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&n))
	assert.Equal(t, 0, n, "post-commit scope writes must roll back")
}

func TestFreshTx_DiscardsUncommittedWrites(t *testing.T) {
	t.Parallel()

	env := Setup(t, WithComponents(modules.Components()...))

	table := fmt.Sprintf("fresh_tx_probe_%d", atomic.AddUint32(&probeSeq, 1))
	_, err := env.Pool.Exec(context.Background(), fmt.Sprintf("CREATE TABLE %s (id int PRIMARY KEY)", table))
	require.NoError(t, err)

	_, err = env.Tx.Exec(env.Ctx, fmt.Sprintf("INSERT INTO %s (id) VALUES (1)", table))
	require.NoError(t, err)

	env.FreshTx(t)

	var n int
	require.NoError(t, env.Tx.QueryRow(env.Ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&n))
	assert.Equal(t, 0, n, "uncommitted writes must be gone after FreshTx")
}

func TestScopeTx_IsGuardedAgainstConcurrentUse(t *testing.T) {
	t.Parallel()

	env := Setup(t, WithComponents(modules.Components()...))

	// The rollback scope transaction must be wrapped so that overlapping
	// calls fail fast with repo.ErrTxInUse instead of racing the connection
	// (behaviour itself is covered by the repo.GuardedTx unit tests).
	_, ok := env.Tx.(*repo.GuardedTx)
	require.True(t, ok, "scope transaction must be a repo.GuardedTx, got %T", env.Tx)
}

func TestScopeTx_ComposablesUseTxSeesReplacement(t *testing.T) {
	t.Parallel()

	env := Setup(t, WithComponents(modules.Components()...))

	table := fmt.Sprintf("use_tx_probe_%d", atomic.AddUint32(&probeSeq, 1))
	_, err := env.Tx.Exec(env.Ctx, fmt.Sprintf("CREATE TABLE %s (id int PRIMARY KEY)", table))
	require.NoError(t, err)

	env.CommitTx(t)

	// The refreshed env.Ctx must carry the replacement transaction: UseTx
	// resolves to it, and writes through it behave like scope writes.
	tx, err := composables.UseTx(env.Ctx)
	require.NoError(t, err)
	_, err = tx.Exec(env.Ctx, fmt.Sprintf("INSERT INTO %s (id) VALUES (1)", table))
	require.NoError(t, err)

	// The value stored in the environment IS the guarded replacement.
	assert.Equal(t, env.Tx, tx)
}

func TestCommitTx_ReappliesTxSettings(t *testing.T) {
	t.Parallel()

	env := Setup(t, WithComponents(modules.Components()...))

	env.CommitTx(t)

	// applyTxSettings defaults: lock_timeout 2s. SET LOCAL is per
	// transaction, so the replacement must have re-applied it.
	var lockTimeout string
	require.NoError(
		t,
		env.Tx.QueryRow(env.Ctx, "SHOW lock_timeout").Scan(&lockTimeout),
	)
	assert.Equal(t, "2s", lockTimeout, "replacement scope tx must re-apply tx settings")
}

var probeSeq uint32
