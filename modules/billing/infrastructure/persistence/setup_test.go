package persistence_test

import (
	"context"
	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/testutils"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	if err := os.Chdir("../../../../"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

// testFixtures contains common test dependencies
type testFixtures struct {
	ctx  context.Context
	pool *pgxpool.Pool
	app  application.Application
}

// setupTest creates all necessary dependencies for tests
func setupTest(t *testing.T) *testFixtures {
	t.Helper()

	testutils.CreateDB(t.Name())
	pool := testutils.NewPool(testutils.DbOpts(t.Name()))

	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		pool.Close()
	})

	ctx = composables.WithTx(ctx, tx)
	app, err := testutils.SetupApplication(pool, modules.BuiltInModules...)
	if err != nil {
		t.Fatal(err)
	}

	// Create a test tenant outside transaction
	tenant, err := testutils.CreateTestTenant(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}

	ctx = composables.WithTenant(ctx, tenant)

	return &testFixtures{
		ctx:  ctx,
		pool: pool,
		app:  app,
	}
}
