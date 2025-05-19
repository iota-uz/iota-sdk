package persistence_test

import (
	"context"
	"os"
	"testing"

	"github.com/iota-uz/utils/random"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/testutils"
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

func setupBenchmark(b *testing.B) *testFixtures {
	b.Helper()

	dbName := b.Name() + random.String(5, random.LowerCharSet)
	testutils.CreateDB(dbName)
	pool := testutils.NewPool(testutils.DbOpts(dbName))

	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		b.Fatal(err)
	}

	b.Cleanup(func() {
		if err := tx.Commit(ctx); err != nil {
			b.Fatal(err)
		}
		pool.Close()
	})

	ctx = composables.WithTx(ctx, tx)
	ctx = composables.WithSession(ctx, &session.Session{})

	// Setup application and run migrations
	app, err := testutils.SetupApplication(pool, modules.BuiltInModules...)
	if err != nil {
		b.Fatal(err)
	}

	// Run migrations first to create all tables including tenants
	if err := app.Migrations().Run(); err != nil {
		b.Fatal(err)
	}

	// Create a test tenant and add it to the context
	tenant, err := testutils.CreateTestTenant(ctx, pool)
	if err != nil {
		b.Fatal(err)
	}
	ctx = composables.WithTenant(ctx, tenant)

	return &testFixtures{
		ctx:  ctx,
		pool: pool,
		app:  app,
	}
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
	ctx = composables.WithSession(ctx, &session.Session{})

	// Setup application and run migrations
	app, err := testutils.SetupApplication(pool, modules.BuiltInModules...)
	if err != nil {
		t.Fatal(err)
	}

	// Run migrations first to create all tables including tenants
	if err := app.Migrations().Run(); err != nil {
		t.Fatal(err)
	}

	// Create a test tenant and add it to the context
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
