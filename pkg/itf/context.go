package itf

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type controllerCloser interface {
	Close() error
}

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

// newTestContext creates a new internal TestContext builder.
func newTestContext() *TestContext {
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
func (tc *TestContext) WithDBName(tb testing.TB, name string) *TestContext {
	tb.Helper()
	if tc.dbName == "" {
		tc.dbName = name
	}
	return tc
}

// Build creates the test context with all dependencies
func (tc *TestContext) Build(tb testing.TB) *TestEnvironment {
	tb.Helper()

	// Set default db name if not set, adding a unique suffix to avoid conflicts
	// between parallel test runs
	if tc.dbName == "" {
		// Generate a short unique suffix using UUID
		uniqueSuffix := uuid.New().String()[:8]
		tc.dbName = tb.Name() + "_" + uniqueSuffix
	}
	h := NewHarness(tb, HarnessConfig{
		Name:    tc.dbName,
		Modules: tc.modules,
		Database: DatabaseConfig{
			Provisioning: ProvisioningPerTestDatabase,
			Cleanup:      CleanupDropOnExit,
		},
		Migration: MigrationConfig{
			Policy: MigrationApplyOnce,
		},
		Isolation: IsolationConfig{
			Mode: IsolationRollback,
		},
		Context: ContextConfig{
			User: tc.user,
		},
	})

	scope := h.Scope(tb)

	tc.ctx = scope.Ctx
	tc.pool = scope.Pool
	tc.tx = scope.Tx
	tc.app = scope.App
	tc.tenant = scope.Tenant

	return &TestEnvironment{
		Ctx:    scope.Ctx,
		Pool:   scope.Pool,
		Tx:     scope.Tx,
		App:    scope.App,
		Tenant: scope.Tenant,
		User:   tc.user,
	}
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

// GetService is a generic helper that retrieves and casts a service
func GetService[T any](te *TestEnvironment) *T {
	var zero T
	service := te.App.Service(zero)
	if service == nil {
		return nil
	}
	return service.(*T)
}

// AssertNoError fails the test if err is not nil
func (te *TestEnvironment) AssertNoError(tb testing.TB, err error) {
	tb.Helper()
	if err != nil {
		tb.Fatal(err)
	}
}

// TenantID returns the test tenant ID
func (te *TestEnvironment) TenantID() uuid.UUID {
	return te.Tenant.ID
}

// WithTx returns a new context with the test transaction
func (te *TestEnvironment) WithTx(ctx context.Context) context.Context {
	return composables.WithTx(ctx, te.Tx)
}

func closeControllerResources(tb testing.TB, controllers []application.Controller) {
	tb.Helper()
	for _, controller := range controllers {
		closer, ok := controller.(controllerCloser)
		if !ok {
			continue
		}
		if err := closer.Close(); err != nil {
			tb.Logf("Warning: failed to close controller %T: %v", controller, err)
		}
	}
}
