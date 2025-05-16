package testutils

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	_ "github.com/lib/pq"
)

type TestFixtures struct {
	SQLDB   *sql.DB
	Pool    *pgxpool.Pool
	Context context.Context
	Tx      pgx.Tx
	App     application.Application
}

func MockUser(permissions ...*permission.Permission) user.User {
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

func MockSession() *session.Session {
	return &session.Session{
		Token:     "",
		UserID:    0,
		IP:        "",
		UserAgent: "",
		ExpiresAt: time.Now(),
		CreatedAt: time.Now(),
	}
}

func NewPool(dbOpts string) *pgxpool.Pool {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	pool, err := pgxpool.New(ctx, dbOpts)
	if err != nil {
		panic(err)
	}
	return pool
}

func DefaultParams() *composables.Params {
	return &composables.Params{
		IP:            "",
		UserAgent:     "",
		Authenticated: true,
		Request:       nil,
		Writer:        nil,
	}
}

// CreateTestTenant creates a test tenant for testing
func CreateTestTenant(ctx context.Context, pool *pgxpool.Pool) (*composables.Tenant, error) {
	testTenant := &composables.Tenant{
		ID:     uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		Name:   "Test Tenant",
		Domain: "test.com",
	}

	// Try to insert the tenant - if it fails because table doesn't exist, we'll catch it
	_, err := pool.Exec(ctx, "INSERT INTO tenants (id, name, domain, created_at, updated_at) VALUES ($1, $2, $3, $4, $5) ON CONFLICT (id) DO NOTHING",
		testTenant.ID,
		testTenant.Name,
		testTenant.Domain,
		time.Now(),
		time.Now(),
	)

	// If there's no error, we're done
	if err == nil {
		return testTenant, nil
	}

	// If the error is not about missing table, return it
	if !strings.Contains(err.Error(), "relation") && !strings.Contains(err.Error(), "does not exist") {
		return nil, err
	}

	// We can simply return the tenant object for test context
	// The actual tenant will be created when migrations run
	return testTenant, nil
}

// sanitizeDBName replaces special characters in database names with underscores
func sanitizeDBName(name string) string {
	sanitized := strings.ReplaceAll(name, "/", "_")
	sanitized = strings.ReplaceAll(sanitized, " ", "_")
	sanitized = strings.ReplaceAll(sanitized, "-", "_")
	return sanitized
}

func CreateDB(name string) {
	sanitizedName := sanitizeDBName(name)

	c := configuration.Use()
	db, err := sql.Open("postgres", c.Database.ConnectionString())
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			panic(err)
		}
	}()
	_, err = db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", sanitizedName))
	if err != nil {
		panic(err)
	}
	_, err = db.Exec(fmt.Sprintf("CREATE DATABASE %s", sanitizedName))
	if err != nil {
		panic(err)
	}
}

func DbOpts(name string) string {
	sanitizedName := sanitizeDBName(name)

	c := configuration.Use()
	return fmt.Sprintf(
		"host=%s port=%s user=%s dbname=%s password=%s sslmode=disable",
		c.Database.Host, c.Database.Port, c.Database.User, strings.ToLower(sanitizedName), c.Database.Password,
	)
}

func SetupApplication(pool *pgxpool.Pool, mods ...application.Module) (application.Application, error) {
	conf := configuration.Use()
	app := application.New(pool, eventbus.NewEventPublisher(conf.Logger()))
	if err := modules.Load(app, mods...); err != nil {
		return nil, err
	}
	if err := app.Migrations().Run(); err != nil {
		return nil, err
	}
	return app, nil
}

func GetTestContext() *TestFixtures {
	conf := configuration.Use()
	pool := NewPool(conf.Database.Opts)
	app := application.New(pool, eventbus.NewEventPublisher(conf.Logger()))
	if err := modules.Load(app, modules.BuiltInModules...); err != nil {
		panic(err)
	}
	if err := app.Migrations().Rollback(); err != nil {
		panic(err)
	}
	if err := app.Migrations().Run(); err != nil {
		panic(err)
	}

	sqlDB := stdlib.OpenDB(*pool.Config().ConnConfig)
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		panic(err)
	}
	ctx = composables.WithTx(ctx, tx)
	ctx = composables.WithParams(
		ctx,
		DefaultParams(),
	)

	return &TestFixtures{
		SQLDB:   sqlDB,
		Pool:    pool,
		Tx:      tx,
		Context: ctx,
		App:     app,
	}
}
