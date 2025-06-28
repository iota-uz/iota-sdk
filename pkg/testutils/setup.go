package testutils

import (
	"context"
	"testing"
	"time"

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

	// Cleanup with proper connection handling
	t.Cleanup(func() {
		// Log initial state for debugging
		LogPoolStats(pool, "Before cleanup")

		// Rollback transaction first
		if err := tx.Rollback(ctx); err != nil {
			// Only log if transaction is still active
			if err != pgx.ErrTxClosed {
				t.Logf("Failed to rollback transaction: %v", err)
			}
		}

		// Close pool with timeout to prevent hanging
		closeCtx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()

		// Wait for all connections to be returned before closing
		for pool.Stat().AcquiredConns() > 0 && closeCtx.Err() == nil {
			time.Sleep(time.Millisecond * 10)
		}

		// Log final state
		LogPoolStats(pool, "Before close")
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
