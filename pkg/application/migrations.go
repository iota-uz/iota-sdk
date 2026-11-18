// Package application provides this package.
package application

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-gorp/gorp/v3"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	migrate "github.com/rubenv/sql-migrate"
	"github.com/sirupsen/logrus"

	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/dbconfig"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// MigrationStatus represents the status of a single migration
type MigrationStatus struct {
	ID        string
	Applied   bool
	AppliedAt *time.Time
}

// MigrationManager is an interface for handling database migrations.
//
// Run, Rollback and RollbackAll hold a Postgres session advisory lock for the
// whole plan-and-apply cycle, so concurrent callers (for example several
// replicas starting at once) apply each migration exactly once.
//
// Migrations marked with "-- +migrate Up notransaction" or
// "-- +migrate Down notransaction" run outside a transaction, which is
// required for statements such as CREATE INDEX CONCURRENTLY.
type MigrationManager interface {
	// Run applies all pending migrations to the database.
	Run(ctx context.Context) error
	// Rollback rolls back the last steps applied migrations, newest first.
	// steps must be at least 1.
	Rollback(ctx context.Context, steps int) error
	// RollbackAll rolls back every applied migration.
	RollbackAll(ctx context.Context) error
	// Status returns the status of all migrations (applied and pending)
	Status(ctx context.Context) ([]MigrationStatus, error)
}

const (
	// DefaultMigrationLockTimeout is the lock_timeout applied to migration
	// statements unless overridden with WithMigrationLockTimeout. It bounds how
	// long a migration waits for a table lock held by live traffic.
	DefaultMigrationLockTimeout = 30 * time.Second

	// DefaultMigrationAdvisoryLockKey is the pg_advisory_lock key that
	// serializes migration runs against one database.
	DefaultMigrationAdvisoryLockKey int64 = 0x696f74616d696772 // "iotamigr"
)

const (
	opMigrationsRun         serrors.Op = "application.MigrationManager.Run"
	opMigrationsRollback    serrors.Op = "application.MigrationManager.Rollback"
	opMigrationsRollbackAll serrors.Op = "application.MigrationManager.RollbackAll"
	opMigrationsLock        serrors.Op = "application.MigrationManager.lock"
)

// MigrationOption customizes a MigrationManager created by NewMigrationManager.
type MigrationOption func(*migrationManager)

// WithMigrationLockTimeout sets the lock_timeout for migration statements.
// Zero or a negative value disables the timeout (waits indefinitely).
// It does not limit the wait for the migration advisory lock; bound that
// wait with the context passed to Run or Rollback.
func WithMigrationLockTimeout(d time.Duration) MigrationOption {
	return func(m *migrationManager) {
		m.lockTimeout = d
	}
}

// WithMigrationAdvisoryLockKey overrides the advisory lock key used to
// serialize migration runs.
func WithMigrationAdvisoryLockKey(key int64) MigrationOption {
	return func(m *migrationManager) {
		m.advisoryLockKey = key
	}
}

// NewMigrationManager creates a MigrationManager driven by an explicit
// dbconfig.Config.
func NewMigrationManager(pool *pgxpool.Pool, db dbconfig.Config, logger logrus.FieldLogger, opts ...MigrationOption) MigrationManager {
	m := &migrationManager{
		migrationsDir:   db.MigrationsDir,
		logger:          logger,
		pool:            pool,
		lockTimeout:     DefaultMigrationLockTimeout,
		advisoryLockKey: DefaultMigrationAdvisoryLockKey,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// migrationManager implements the MigrationManager interface
type migrationManager struct {
	migrationsDir   string
	logger          logrus.FieldLogger
	pool            *pgxpool.Pool
	lockTimeout     time.Duration
	advisoryLockKey int64
}

// newTxError annotates err with the failing migration and keeps it
// unwrappable, so callers can inspect the underlying *pgconn.PgError.
func newTxError(migration *migrate.PlannedMigration, err error) error {
	return fmt.Errorf("migration %s: %w", migration.Id, err)
}

func (m *migrationManager) applyMigrations(
	ctx context.Context,
	dir migrate.MigrationDirection,
	migrations []*migrate.PlannedMigration,
	dbMap *gorp.DbMap,
) (int, error) {
	applied := 0
	for _, migration := range migrations {
		if err := applyMigration(ctx, dir, migration, dbMap); err != nil {
			return applied, err
		}
		applied++
	}
	return applied, nil
}

func applyMigration(
	ctx context.Context,
	dir migrate.MigrationDirection,
	migration *migrate.PlannedMigration,
	dbMap *gorp.DbMap,
) error {
	var executor gorp.SqlExecutor
	var tx *gorp.Transaction
	if migration.DisableTransaction {
		executor = dbMap.WithContext(ctx)
	} else {
		t, err := dbMap.Begin()
		if err != nil {
			return newTxError(migration, err)
		}
		tx = t
		executor = tx.WithContext(ctx)
	}
	fail := func(err error) error {
		if tx != nil {
			_ = tx.Rollback()
		}
		return newTxError(migration, err)
	}

	for _, stmt := range migration.Queries {
		stmt = strings.TrimSuffix(stmt, "\n")
		stmt = strings.TrimSuffix(stmt, " ")
		stmt = strings.TrimSuffix(stmt, ";")
		if _, err := executor.Exec(stmt); err != nil {
			return fail(err)
		}
	}

	switch dir {
	case migrate.Up:
		if err := executor.Insert(&migrate.MigrationRecord{
			Id:        migration.Id,
			AppliedAt: time.Now(),
		}); err != nil {
			return fail(err)
		}
	case migrate.Down:
		if _, err := executor.Delete(&migrate.MigrationRecord{
			Id: migration.Id,
		}); err != nil {
			return fail(err)
		}
	default:
		return fail(fmt.Errorf("unsupported migration direction %d", dir))
	}

	if tx != nil {
		if err := tx.Commit(); err != nil {
			return newTxError(migration, err)
		}
	}
	return nil
}

// openDB opens a database/sql handle for sql-migrate. Every connection of
// the handle carries the configured lock_timeout.
func (m *migrationManager) openDB() *sql.DB {
	connConfig := m.pool.Config().ConnConfig.Copy()
	if connConfig.RuntimeParams == nil {
		connConfig.RuntimeParams = map[string]string{}
	}
	lockTimeout := int64(0)
	if m.lockTimeout > 0 {
		lockTimeout = max(m.lockTimeout.Milliseconds(), 1)
	}
	connConfig.RuntimeParams["lock_timeout"] = strconv.FormatInt(lockTimeout, 10)
	return stdlib.OpenDB(*connConfig)
}

// acquireLock takes the migration advisory lock on a dedicated session and
// returns a function that releases it. The wait is bounded by ctx only:
// another process may legitimately hold the lock for a long migration.
func (m *migrationManager) acquireLock(ctx context.Context, db *sql.DB) (func(), error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, serrors.Wrap(opMigrationsLock, err)
	}
	release := func() { _ = conn.Close() }

	if _, err := conn.ExecContext(ctx, "SET lock_timeout = 0"); err != nil {
		release()
		return nil, serrors.Wrap(opMigrationsLock, err)
	}
	var locked bool
	if err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", m.advisoryLockKey).Scan(&locked); err != nil {
		release()
		return nil, serrors.Wrap(opMigrationsLock, err)
	}
	if !locked {
		m.logger.Infof("Waiting for another process to finish migrations (advisory lock %d)", m.advisoryLockKey)
		if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", m.advisoryLockKey); err != nil {
			release()
			return nil, serrors.Wrap(opMigrationsLock, err)
		}
	}

	return func() {
		unlockCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		if _, err := conn.ExecContext(unlockCtx, "SELECT pg_advisory_unlock($1)", m.advisoryLockKey); err != nil {
			// Closing the handle ends the session, which releases the lock.
			m.logger.Warnf("Failed to release migration advisory lock: %v", err)
		}
		release()
	}, nil
}

// migrate plans and applies at most limit migrations in dir (0 means all)
// while holding the migration advisory lock.
func (m *migrationManager) migrate(ctx context.Context, dir migrate.MigrationDirection, limit int) (int, error) {
	if m.pool == nil {
		return 0, serrors.New(serrors.Invalid, "migration manager requires a database pool")
	}
	db := m.openDB()
	defer func() { _ = db.Close() }()

	unlock, err := m.acquireLock(ctx, db)
	if err != nil {
		return 0, err
	}
	defer unlock()

	migrationSource := &migrate.FileMigrationSource{
		Dir: m.migrationsDir,
	}
	ms := migrate.MigrationSet{}
	plannedMigrations, dbMap, err := ms.PlanMigration(db, "postgres", migrationSource, dir, limit)
	if err != nil {
		return 0, err
	}
	return m.applyMigrations(ctx, dir, plannedMigrations, dbMap)
}

func (m *migrationManager) Run(ctx context.Context) error {
	applied, err := m.migrate(ctx, migrate.Up, 0)
	if err != nil {
		return serrors.Wrap(opMigrationsRun, err)
	}
	m.logger.Infof("Applied %d migrations", applied)
	return nil
}

func (m *migrationManager) Rollback(ctx context.Context, steps int) error {
	if steps < 1 {
		return serrors.New(serrors.Invalid, fmt.Sprintf("rollback steps must be at least 1, got %d; use RollbackAll to roll back every migration", steps)).WithOp(opMigrationsRollback)
	}
	rolledBack, err := m.migrate(ctx, migrate.Down, steps)
	if err != nil {
		return serrors.Wrap(opMigrationsRollback, err)
	}
	m.logger.Infof("Rolled back %d migrations", rolledBack)
	return nil
}

func (m *migrationManager) RollbackAll(ctx context.Context) error {
	rolledBack, err := m.migrate(ctx, migrate.Down, 0)
	if err != nil {
		return serrors.Wrap(opMigrationsRollbackAll, err)
	}
	m.logger.Infof("Rolled back %d migrations", rolledBack)
	return nil
}

// Status returns the status of all migrations (applied and pending)
func (m *migrationManager) Status(ctx context.Context) ([]MigrationStatus, error) {
	db := stdlib.OpenDB(*m.pool.Config().ConnConfig)
	defer func() { _ = db.Close() }()
	migrationSource := &migrate.FileMigrationSource{
		Dir: m.migrationsDir,
	}

	// Get all migrations from source
	migrations, err := migrationSource.FindMigrations()
	if err != nil {
		return nil, fmt.Errorf("failed to find migrations: %w", err)
	}

	// Status must not race the migration runner by creating its tracking table.
	ms := migrate.MigrationSet{DisableCreateTable: true}
	records, err := ms.GetMigrationRecords(db, "postgres")
	if err != nil {
		return nil, fmt.Errorf("failed to get migration records: %w", err)
	}

	// Create a map of applied migrations for quick lookup
	appliedMap := make(map[string]time.Time)
	for _, record := range records {
		appliedMap[record.Id] = record.AppliedAt
	}

	// Build status list
	statuses := make([]MigrationStatus, 0, len(migrations))
	for _, migration := range migrations {
		status := MigrationStatus{
			ID: migration.Id,
		}
		if appliedAt, found := appliedMap[migration.Id]; found {
			status.Applied = true
			status.AppliedAt = &appliedAt
		} else {
			status.Applied = false
			status.AppliedAt = nil
		}
		statuses = append(statuses, status)
	}

	return statuses, nil
}

// noopMigrationManager is a no-op implementation used when no DB config is provided.
type noopMigrationManager struct{}

func (n *noopMigrationManager) Run(_ context.Context) error             { return nil }
func (n *noopMigrationManager) Rollback(_ context.Context, _ int) error { return nil }
func (n *noopMigrationManager) RollbackAll(_ context.Context) error     { return nil }
func (n *noopMigrationManager) Status(_ context.Context) ([]MigrationStatus, error) {
	return nil, nil
}
