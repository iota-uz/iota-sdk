package persistence_test

import (
	"context"
	"os"
	"testing"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/testutils"
	"github.com/jackc/pgx/v5/pgxpool"
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

	// Setup application and run migrations first (outside the transaction)
	app, err := testutils.SetupApplication(pool, modules.BuiltInModules...)
	if err != nil {
		t.Fatal(err)
	}

	// Run migrations first to create all tables including tenants
	if err := app.Migrations().Run(); err != nil {
		t.Fatal(err)
	}

	// Create a test tenant outside transaction
	tenant, err := testutils.CreateTestTenant(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}

	// Now start the transaction
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		// Rollback instead of commit to ensure clean state
		// This is safer as it ensures tests don't affect each other
		if err := tx.Rollback(ctx); err != nil {
			// Only fatal if it's not already committed
			if err.Error() != "sql: transaction has already been committed or rolled back" {
				t.Fatal(err)
			}
		}
		pool.Close()
	})

	// Add transaction and tenant to context
	ctx = composables.WithTx(ctx, tx)
	ctx = composables.WithTenantID(ctx, tenant.ID)

	return &testFixtures{
		ctx:  ctx,
		pool: pool,
		app:  app,
	}
}
