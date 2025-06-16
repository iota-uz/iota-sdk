package testutils

import (
	"context"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestEnv holds common test dependencies
type TestEnv struct {
	Ctx    context.Context
	Pool   *pgxpool.Pool
	Tx     pgx.Tx
	Tenant *composables.Tenant
	App    application.Application
}

// Setup creates a test environment with database and transaction
func Setup(t *testing.T, modules ...application.Module) *TestEnv {
	t.Helper()

	// Create test database
	CreateDB(t.Name())
	pool := NewPool(DbOpts(t.Name()))

	// Setup application
	app, err := SetupApplication(pool, modules...)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// Create tenant
	tenant, err := CreateTestTenant(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}

	// Start transaction
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Build context
	ctx = composables.WithPool(ctx, pool)
	ctx = composables.WithTx(ctx, tx)
	ctx = composables.WithTenantID(ctx, tenant.ID)

	// Cleanup
	t.Cleanup(func() {
		if err := tx.Rollback(ctx); err != nil {
			t.Logf("Failed to rollback transaction: %v", err)
		}
		pool.Close()
	})

	return &TestEnv{
		Ctx:    ctx,
		Pool:   pool,
		Tx:     tx,
		Tenant: tenant,
		App:    app,
	}
}
