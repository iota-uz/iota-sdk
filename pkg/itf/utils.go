// Package itf provides this package.
package itf

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/config"
	envprov "github.com/iota-uz/iota-sdk/pkg/config/providers/env"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/dbconfig"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/serrors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
)

const (
	opCreateDBE = serrors.Op("itf.CreateDB")
	opDropDBE   = serrors.Op("itf.DropDB")
)

type TestFixtures struct {
	SQLDB     *sql.DB
	Pool      *pgxpool.Pool
	Context   context.Context
	Tx        pgx.Tx
	App       application.Application
	Container *composition.Container
}

func (f *TestFixtures) Close() error {
	if f == nil {
		return nil
	}

	var closeErr error
	if f.Tx != nil {
		closeErr = errors.Join(closeErr, f.Tx.Rollback(context.Background()))
	}
	if f.Container != nil {
		stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		closeErr = errors.Join(closeErr, composition.Stop(stopCtx, f.Container))
		cancel()
	}
	if f.SQLDB != nil {
		closeErr = errors.Join(closeErr, f.SQLDB.Close())
	}
	if f.Pool != nil {
		f.Pool.Close()
	}
	return closeErr
}

func MockSession() session.Session {
	return session.New("mock-token", 0, uuid.Nil, "127.0.0.1", "test-agent")
}

func NewPool(dbOpts string) *pgxpool.Pool {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	cfg, err := pgxpool.ParseConfig(dbOpts)
	if err != nil {
		panic(err)
	}

	// With increased PostgreSQL max_connections (500), we can use reasonable limits
	cfg.MaxConns = 4
	cfg.MinConns = 0
	cfg.MaxConnLifetime = time.Minute * 5
	cfg.MaxConnIdleTime = time.Second * 30

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		panic(fmt.Errorf("failed to create database pool: %w", err))
	}

	return pool
}

// DatabaseManager handles database lifecycle for tests
type DatabaseManager struct {
	pool   *pgxpool.Pool
	dbName string
}

// NewDatabaseManager creates a new database and returns a manager that handles cleanup automatically.
// It reads DB connection details from the standard .env files.
func NewDatabaseManager(t *testing.T) *DatabaseManager {
	t.Helper()

	db := LoadDBConfigFromEnv()
	dbName := t.Name()
	CreateDB(dbName, db)
	pool := NewPool(DBOpts(dbName, db))

	dm := &DatabaseManager{
		pool:   pool,
		dbName: dbName,
	}

	t.Cleanup(func() {
		dm.Close()
	})

	return dm
}

// Pool returns the database pool
func (dm *DatabaseManager) Pool() *pgxpool.Pool {
	return dm.pool
}

// Close closes the pool
func (dm *DatabaseManager) Close() {
	if dm.pool != nil {
		dm.pool.Close()
		dm.pool = nil
	}
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
	tenantID := uuid.New()
	testTenant := &composables.Tenant{
		ID:     tenantID,
		Name:   "Test Tenant " + tenantID.String()[:8],
		Domain: tenantID.String()[:8] + ".test.com",
	}

	_, err := pool.Exec(ctx, "INSERT INTO tenants (id, name, domain, created_at, updated_at) VALUES ($1, $2, $3, $4, $5) ON CONFLICT (id) DO NOTHING",
		testTenant.ID,
		testTenant.Name,
		testTenant.Domain,
		time.Now(),
		time.Now(),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create test tenant: %w", err)
	}

	return testTenant, nil
}

const (
	// PostgreSQL database name maximum length is 63 characters
	maxDBNameLength = 63
	// Reserve space for hash suffix when truncating (8 chars + underscore)
	hashSuffixLength = 9
)

var nonIdentifierChars = regexp.MustCompile(`[^a-z0-9_]`)

// sanitizeDBName replaces special characters in database names with underscores
// and ensures the name doesn't exceed PostgreSQL's 63-character limit
func sanitizeDBName(name string) string {
	sanitized := strings.ToLower(name)
	sanitized = nonIdentifierChars.ReplaceAllString(sanitized, "_")

	for strings.Contains(sanitized, "__") {
		sanitized = strings.ReplaceAll(sanitized, "__", "_")
	}
	sanitized = strings.Trim(sanitized, "_")
	if sanitized == "" {
		sanitized = "test_db"
	}
	if len(sanitized) <= maxDBNameLength {
		return sanitized
	}
	return truncateWithHash(sanitized, name)
}

func truncateWithHash(sanitized, original string) string {
	hasher := sha256.New()
	hasher.Write([]byte(original))
	hash := fmt.Sprintf("%x", hasher.Sum(nil))[:8]
	maxNameLength := maxDBNameLength - hashSuffixLength
	truncated := intelligentTruncate(sanitized, maxNameLength)
	return fmt.Sprintf("%s_%s", truncated, hash)
}

func intelligentTruncate(name string, maxLength int) string {
	if len(name) <= maxLength {
		return name
	}
	parts := strings.Split(name, "_")
	if len(parts) > 1 {
		first := parts[0]
		last := parts[len(parts)-1]
		combined := first + "_" + last
		if len(combined) <= maxLength && first != last {
			return combined
		}
		if len(first) <= maxLength/2 {
			result := first
			remaining := maxLength - len(first) - 1
			for i := 1; i < len(parts) && len(result) < maxLength; i++ {
				part := parts[i]
				if len(part)+1 <= remaining {
					result += "_" + part
					remaining -= len(part) + 1
				} else {
					if remaining > 4 {
						result += "_" + part[:remaining-1]
					}
					break
				}
			}
			return result
		}
	}
	return name[:maxLength]
}

// CreateDB creates a test database using an explicit dbconfig.Config for the admin connection.
func CreateDB(name string, db dbconfig.Config) {
	sanitizedName := sanitizeDBName(name)
	adminConnStr := fmt.Sprintf(
		"host=%s port=%s user=%s dbname=postgres password=%s sslmode=disable",
		db.Host, db.Port, db.User, db.Password,
	)
	conn, err := sql.Open("postgres", adminConnStr)
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			log.Printf("[WARNING] Error closing CreateDB connection: %v", err)
		}
	}()
	_, err = conn.ExecContext(context.Background(), fmt.Sprintf(`DROP DATABASE IF EXISTS "%s"`, sanitizedName))
	if err != nil {
		panic(err)
	}
	_, err = conn.ExecContext(context.Background(), fmt.Sprintf(`CREATE DATABASE "%s"`, sanitizedName))
	if err != nil {
		panic(err)
	}
}

// CreateDBE creates a test database and returns an error instead of panicking.
func CreateDBE(name string, db dbconfig.Config) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = serrors.E(opCreateDBE, fmt.Errorf("failed to create test database %q: %v", sanitizeDBName(name), r))
		}
	}()
	CreateDB(name, db)
	return nil
}

// DropDB drops a test database. Used for cleanup after tests to free disk space.
func DropDB(name string, db dbconfig.Config) error {
	if err := dropDB(name, db); err != nil {
		log.Printf("[WARNING] DropDB failed: %v", err)
		return err
	}
	return nil
}

func dropDB(name string, db dbconfig.Config) error {
	sanitizedName := sanitizeDBName(name)
	adminConnStr := fmt.Sprintf(
		"host=%s port=%s user=%s dbname=postgres password=%s sslmode=disable",
		db.Host, db.Port, db.User, db.Password,
	)
	conn, err := sql.Open("postgres", adminConnStr)
	if err != nil {
		log.Printf("[WARNING] Failed to open connection for DropDB: %v", err)
		return err
	}
	defer func() {
		if err := conn.Close(); err != nil {
			log.Printf("[WARNING] Error closing DropDB connection: %v", err)
		}
	}()

	terminateSQL := `
		SELECT pg_terminate_backend(pg_stat_activity.pid)
		FROM pg_stat_activity
		WHERE pg_stat_activity.datname = $1
		AND pid <> pg_backend_pid()
	`
	_, _ = conn.ExecContext(context.Background(), terminateSQL, sanitizedName)
	_, err = conn.ExecContext(context.Background(), fmt.Sprintf(`DROP DATABASE IF EXISTS "%s"`, sanitizedName))
	return err
}

// DropDBE drops a test database and returns an error instead of panicking.
func DropDBE(name string, db dbconfig.Config) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = serrors.E(opDropDBE, fmt.Errorf("failed to drop test database %q: %v", sanitizeDBName(name), r))
		}
	}()
	err = dropDB(name, db)
	return err
}

// DBOpts returns a libpq-style connection string for the named database
// using an explicit dbconfig.Config.
func DBOpts(name string, db dbconfig.Config) string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s dbname=%s password=%s sslmode=disable",
		db.Host, db.Port, db.User, strings.ToLower(sanitizeDBName(name)), db.Password,
	)
}

// setupApplicationWithSource bootstraps an application and composition container for tests.
// When src is non-nil it is forwarded to the build context for ProvideConfig[T].
func setupApplicationWithSource(
	pool *pgxpool.Pool,
	logger *logrus.Logger,
	components []composition.Component,
	sources ...config.Source,
) (application.Application, *composition.Container, error) {
	if logger == nil {
		logger = logrus.New()
	}
	bundle := application.LoadBundle()
	app, err := application.New(&application.ApplicationOptions{
		Pool:               pool,
		Bundle:             bundle,
		EventBus:           eventbus.NewEventPublisher(logger),
		Logger:             logger,
		SupportedLanguages: application.DefaultSupportedLanguages(),
	})
	if err != nil {
		return nil, nil, err
	}
	var container *composition.Container
	if len(components) > 0 {
		engine := composition.NewEngine()
		if err := engine.Register(components...); err != nil {
			return nil, nil, serrors.E(serrors.Op("itf.SetupApplication"), err, "register components")
		}
		var buildCtx composition.BuildContext
		switch {
		case len(sources) > 0 && sources[0] != nil:
			buildCtx = composition.NewBuildContext(app, sources[0], composition.WithLogger(logger))
		default:
			// No explicit source supplied — build one from the standard env file
			// chain so stdconfig auto-registration still works in tests that
			// don't go through itf.SuiteBuilder.WithSource.
			fallback, err := config.Build(envprov.New(".env", ".env.local", ".env.testing"))
			if err != nil {
				return nil, nil, serrors.E(serrors.Op("itf.SetupApplication"), err, "build fallback config source")
			}
			buildCtx = composition.NewBuildContext(app, fallback, composition.WithLogger(logger))
		}
		container, err = engine.Compile(
			buildCtx,
			composition.CapabilityAPI,
			composition.CapabilityWorker,
		)
		if err != nil {
			return nil, nil, serrors.E(serrors.Op("itf.SetupApplication"), err, "compile components")
		}
	}
	if container != nil {
		startCtx, cancelStart := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelStart()
		if err := composition.Start(startCtx, container); err != nil {
			return nil, nil, serrors.E(serrors.Op("itf.SetupApplication"), err, "start runtime")
		}
	}
	return app, container, nil
}

// LoadDBConfigFromEnv builds a dbconfig.Config from env files (.env, .env.local).
// Panics on failure — only used in test setup paths where panics are acceptable.
func LoadDBConfigFromEnv() dbconfig.Config {
	src, err := config.Build(envprov.New(".env", ".env.local"))
	if err != nil {
		panic(fmt.Errorf("itf: load DB config from env: %w", err))
	}
	reg := config.NewRegistry(src)
	cfg, err := config.Register[dbconfig.Config](reg, "db")
	if err != nil {
		panic(fmt.Errorf("itf: register dbconfig: %w", err))
	}
	return *cfg
}
