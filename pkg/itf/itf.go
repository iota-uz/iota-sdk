package itf

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/jackc/pgx/v5"
)

// Setup creates a new test harness with database and application setup
func Setup(tb testing.TB, opts ...Option) *TestEnvironment {
	tb.Helper()
	return NewTestContext().applyOptions(opts...).Build(tb)
}

// HTTP creates a new test suite for HTTP handlers
func HTTP(tb testing.TB, modules ...application.Module) *Suite {
	tb.Helper()
	return NewSuite(tb, modules...)
}

// Excel creates a new Excel file builder
func Excel() *TestExcelBuilder {
	return NewTestExcelBuilder()
}

// User creates a test user with the given permissions
func User(permissions ...*permission.Permission) user.User {
	r := role.New(
		"admin",
		role.WithID(1),
		role.WithPermissions(permissions),
		role.WithCreatedAt(time.Now()),
		role.WithUpdatedAt(time.Now()),
		role.WithTenantID(uuid.Nil), // tenant_id will be set correctly in repository
	)

	email, err := internet.NewEmail("test@example.com")
	if err != nil {
		panic(err)
	}

	return user.New(
		"", // firstName
		"", // lastName
		email,
		"", // uiLanguage
		user.WithID(1),
		user.WithRoles([]role.Role{r}),
		user.WithCreatedAt(time.Now()),
		user.WithUpdatedAt(time.Now()),
	)
}

// Transaction begins a new transaction for testing
func Transaction(tb testing.TB, env *TestEnvironment) pgx.Tx {
	tb.Helper()
	tx, err := env.Pool.Begin(env.Ctx)
	if err != nil {
		tb.Fatalf("Failed to begin transaction: %v", err)
	}
	tb.Cleanup(func() {
		if err := tx.Rollback(env.Ctx); err != nil && err != pgx.ErrTxClosed {
			tb.Logf("Warning: failed to rollback transaction: %v", err)
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
