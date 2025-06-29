package builder

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/testutils"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestContext provides a fluent API for building test contexts
type TestContext struct {
	ctx     context.Context
	pool    *pgxpool.Pool
	tx      pgx.Tx
	app     application.Application
	tenant  *composables.Tenant
	user    user.User
	modules []application.Module
	dbName  string
}

// New creates a new TestContext builder
func New() *TestContext {
	return &TestContext{
		ctx:     context.Background(),
		modules: []application.Module{},
	}
}

// WithModules adds modules to the test context
func (tc *TestContext) WithModules(modules ...application.Module) *TestContext {
	tc.modules = append(tc.modules, modules...)
	return tc
}

// WithUser sets the user for the test context
func (tc *TestContext) WithUser(u user.User) *TestContext {
	tc.user = u
	return tc
}

// WithDBName sets a custom database name
func (tc *TestContext) WithDBName(t *testing.T, name string) *TestContext {
	t.Helper()
	if tc.dbName == "" {
		tc.dbName = name
	}
	return tc
}

// Build creates the test context with all dependencies
func (tc *TestContext) Build(t *testing.T) *TestEnvironment {
	t.Helper()

	// Set default db name if not set
	if tc.dbName == "" {
		tc.dbName = t.Name()
	}

	// Create test database
	testutils.CreateDB(tc.dbName)
	tc.pool = testutils.NewPool(testutils.DbOpts(tc.dbName))

	// Setup application
	app, err := testutils.SetupApplication(tc.pool, tc.modules...)
	if err != nil {
		t.Fatal(err)
	}
	tc.app = app

	// Create tenant
	tenant, err := testutils.CreateTestTenant(tc.ctx, tc.pool)
	if err != nil {
		t.Fatal(err)
	}
	tc.tenant = tenant

	// Begin transaction
	tx, err := tc.pool.Begin(tc.ctx)
	if err != nil {
		t.Fatal(err)
	}
	tc.tx = tx

	// Build context with all composables
	tc.ctx = tc.buildContext()

	// Setup cleanup
	t.Cleanup(func() {
		if err := tx.Rollback(tc.ctx); err != nil && err != pgx.ErrTxClosed {
			t.Logf("Warning: failed to rollback transaction: %v", err)
		}
		tc.pool.Close()
		// Release the database semaphore acquired in NewPool
		<-testutils.DbTestSemaphore
	})

	return &TestEnvironment{
		Ctx:    tc.ctx,
		Pool:   tc.pool,
		Tx:     tc.tx,
		App:    tc.app,
		Tenant: tc.tenant,
		User:   tc.user,
	}
}

func (tc *TestContext) buildContext() context.Context {
	ctx := tc.ctx
	ctx = composables.WithPool(ctx, tc.pool)
	ctx = composables.WithTenantID(ctx, tc.tenant.ID)
	ctx = composables.WithParams(ctx, testutils.DefaultParams())

	if tc.user != nil {
		ctx = composables.WithUser(ctx, tc.user)
	}

	ctx = composables.WithSession(ctx, &session.Session{})

	return ctx
}

// TestEnvironment contains all test dependencies
type TestEnvironment struct {
	Ctx    context.Context
	Pool   *pgxpool.Pool
	Tx     pgx.Tx
	App    application.Application
	Tenant *composables.Tenant
	User   user.User
}

// Service retrieves a service from the application
func (te *TestEnvironment) Service(service interface{}) interface{} {
	return te.App.Service(service)
}

// AssertNoError fails the test if err is not nil
func (te *TestEnvironment) AssertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// TenantID returns the test tenant ID
func (te *TestEnvironment) TenantID() uuid.UUID {
	return te.Tenant.ID
}
