package itf

import (
	"testing"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/jackc/pgx/v5"
)

// Setup creates a new test harness with database and application setup
func Setup(t testing.TB, opts ...Option) *TestEnvironment {
	t.Helper()
	return NewTestContext().applyOptions(opts...).Build(t)
}

// HTTP is an alias for Scenario for clearer intent
func HTTP(t testing.TB, modules ...application.Module) *Suite {
	t.Helper()
	return NewSuite(t, modules...)
}

// Excel creates a new Excel file builder
func Excel() *TestExcelBuilder {
	return NewTestExcelBuilder()
}

// User creates a test user with the given permissions
func User(permissions ...*permission.Permission) user.User {
	return MockUser(permissions...)
}

// Transaction begins a new transaction for testing
func Transaction(t testing.TB, env *TestEnvironment) pgx.Tx {
	t.Helper()
	tx, err := env.Pool.Begin(env.Ctx)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}
	t.Cleanup(func() {
		if err := tx.Rollback(env.Ctx); err != nil && err != pgx.ErrTxClosed {
			t.Logf("Warning: failed to rollback transaction: %v", err)
		}
	})
	return tx
}

// Option configures the test setup
type Option func(*TestContext)

// WithModules adds modules to the test context
func WithModules(modules ...application.Module) Option {
	return func(tc *TestContext) {
		tc.modules = append(tc.modules, modules...)
	}
}

// WithDatabase sets a custom database name
func WithDatabase(name string) Option {
	return func(tc *TestContext) {
		tc.dbName = name
	}
}

// WithUser sets the default user for the test context
func WithUser(u user.User) Option {
	return func(tc *TestContext) {
		tc.user = u
	}
}

// applyOptions applies all options to the test context
func (tc *TestContext) applyOptions(opts ...Option) *TestContext {
	for _, opt := range opts {
		opt(tc)
	}
	return tc
}
