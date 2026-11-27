// Package application provides this package.
package application

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
// Run, Rollback and RollbackAll execute on one dedicated database session that
// holds a Postgres advisory lock for the whole plan-and-apply cycle. Concurrent
// callers (for example several replicas starting at once) therefore never plan
// or run the same migration at the same time, and if that session is lost the
// run stops at its next statement instead of continuing unlocked.
//
// Migrations marked with "-- +migrate Up notransaction" or
// "-- +migrate Down notransaction" run outside a transaction, which is
// required for statements such as CREATE INDEX CONCURRENTLY. For those, the
// SQL and the tracking-table write are separate steps: an interruption between
// them leaves the schema changed but the migration still pending, and a failed
// concurrent build can leave an INVALID index. The lock serializes attempts; it
// does not make such a migration atomic. Write them to be re-runnable (for
// example CREATE INDEX CONCURRENTLY IF NOT EXISTS) and see the migrations guide
// for recovery.
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

// migrationTable is the tracking table shared with sql-migrate (used by Status).
const migrationTable = "gorp_migrations"

type migrationDirection int

const (
	migrationUp migrationDirection = iota
	migrationDown
)

func (d migrationDirection) String() string {
	if d == migrationDown {
		return "down"
	}
	return "up"
}

type plannedMigration struct {
	id            string
	queries       []string
	noTransaction bool
}

func validateRollbackSteps(steps int) error {
	if steps < 1 {
		return serrors.New(serrors.Invalid, fmt.Sprintf("rollback steps must be at least 1, got %d; use RollbackAll to roll back every migration", steps)).WithOp(opMigrationsRollback)
	}
	return nil
}

// connect opens the dedicated session for one migration run. It carries the
// configured lock_timeout as a session default.
func (m *migrationManager) connect(ctx context.Context) (*pgx.Conn, error) {
	connConfig := m.pool.Config().ConnConfig.Copy()
	if connConfig.RuntimeParams == nil {
		connConfig.RuntimeParams = map[string]string{}
	}
	lockTimeout := int64(0)
	if m.lockTimeout > 0 {
		lockTimeout = max(m.lockTimeout.Milliseconds(), 1)
	}
	connConfig.RuntimeParams["lock_timeout"] = strconv.FormatInt(lockTimeout, 10)
	return pgx.ConnectConfig(ctx, connConfig)
}

// acquireLock takes the migration advisory lock on conn. The wait is bounded
// by ctx only: another process may legitimately hold the lock for a long
// migration, so lock_timeout and statement_timeout are lifted for the wait and
// restored to the session defaults afterwards.
func (m *migrationManager) acquireLock(ctx context.Context, conn *pgx.Conn) error {
	var locked bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", m.advisoryLockKey).Scan(&locked); err != nil {
		return serrors.Wrap(opMigrationsLock, err)
	}
	if locked {
		return nil
	}
	m.logger.Infof("Waiting for another process to finish migrations (advisory lock %d)", m.advisoryLockKey)
	if _, err := conn.Exec(ctx, "SET lock_timeout = 0"); err != nil {
		return serrors.Wrap(opMigrationsLock, err)
	}
	if _, err := conn.Exec(ctx, "SET statement_timeout = 0"); err != nil {
		return serrors.Wrap(opMigrationsLock, err)
	}
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", m.advisoryLockKey); err != nil {
		return serrors.Wrap(opMigrationsLock, err)
	}
	if _, err := conn.Exec(ctx, "RESET lock_timeout"); err != nil {
		return serrors.Wrap(opMigrationsLock, err)
	}
	if _, err := conn.Exec(ctx, "RESET statement_timeout"); err != nil {
		return serrors.Wrap(opMigrationsLock, err)
	}
	return nil
}

func (m *migrationManager) releaseLock(ctx context.Context, conn *pgx.Conn) {
	unlockCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if _, err := conn.Exec(unlockCtx, "SELECT pg_advisory_unlock($1)", m.advisoryLockKey); err != nil {
		// Closing the connection ends the session, which releases the lock.
		m.logger.Warnf("Failed to release migration advisory lock: %v", err)
	}
}

// appliedMigrationIDs creates the tracking table if needed and returns the
// recorded migration ids.
func appliedMigrationIDs(ctx context.Context, conn *pgx.Conn) (map[string]struct{}, error) {
	if _, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS `+migrationTable+` (id varchar(255) NOT NULL PRIMARY KEY, applied_at timestamp with time zone)`); err != nil {
		return nil, err
	}
	rows, err := conn.Query(ctx, `SELECT id FROM `+migrationTable)
	if err != nil {
		return nil, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}
	applied := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		applied[id] = struct{}{}
	}
	return applied, nil
}

// planMigrations builds the plan from the source and the tracking table.
// Up applies every pending migration in order, including older ones added
// after newer ones were applied. Down only ever touches applied migrations,
// newest first; limit 0 means all, otherwise exactly limit are required.
func planMigrations(source []*migrate.Migration, applied map[string]struct{}, squashed map[string]struct{}, dir migrationDirection, limit int) ([]plannedMigration, error) {
	known := make(map[string]struct{}, len(source))
	for _, mig := range source {
		known[mig.Id] = struct{}{}
	}
	for id := range applied {
		_, isKnown := known[id]
		_, isSquashed := squashed[id]
		if !isKnown && !isSquashed {
			return nil, fmt.Errorf("unknown migration %s in database", id)
		}
	}

	var plan []plannedMigration
	switch dir {
	case migrationUp:
		for _, mig := range source {
			if _, ok := applied[mig.Id]; !ok {
				plan = append(plan, plannedMigration{id: mig.Id, queries: mig.Up, noTransaction: mig.DisableTransactionUp})
			}
		}
	case migrationDown:
		for i := len(source) - 1; i >= 0; i-- {
			mig := source[i]
			if _, ok := applied[mig.Id]; ok {
				plan = append(plan, plannedMigration{id: mig.Id, queries: mig.Down, noTransaction: mig.DisableTransactionDown})
			}
		}
		if limit > 0 {
			if len(plan) < limit {
				return nil, serrors.New(serrors.Invalid, fmt.Sprintf("cannot roll back %d migrations: only %d applied", limit, len(plan)))
			}
			plan = plan[:limit]
		}
	}
	return plan, nil
}

type migrationExecer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func applyMigration(ctx context.Context, conn *pgx.Conn, dir migrationDirection, mig plannedMigration) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("migration %s (%s): %w", mig.id, dir, err)
		}
	}()

	var exec migrationExecer = conn
	var tx pgx.Tx
	if !mig.noTransaction {
		if tx, err = conn.Begin(ctx); err != nil {
			return err
		}
		defer func() {
			if err != nil {
				_ = tx.Rollback(context.WithoutCancel(ctx))
			}
		}()
		exec = tx
	}

	for _, stmt := range mig.queries {
		stmt = strings.TrimSuffix(stmt, "\n")
		stmt = strings.TrimSuffix(stmt, " ")
		stmt = strings.TrimSuffix(stmt, ";")
		if _, err = exec.Exec(ctx, stmt); err != nil {
			return err
		}
	}

	if dir == migrationUp {
		_, err = exec.Exec(ctx, `INSERT INTO `+migrationTable+` (id, applied_at) VALUES ($1, $2)`, mig.id, time.Now())
	} else {
		_, err = exec.Exec(ctx, `DELETE FROM `+migrationTable+` WHERE id = $1`, mig.id)
	}
	if err != nil {
		return err
	}

	if tx != nil {
		return tx.Commit(ctx)
	}
	return nil
}

// migrate plans and applies migrations in dir (limit 0 means all) on one
// session that holds the migration advisory lock throughout.
func (m *migrationManager) migrate(ctx context.Context, dir migrationDirection, limit int) (int, error) {
	if m.pool == nil {
		return 0, serrors.New(serrors.Invalid, "migration manager requires a database pool")
	}
	source, err := (&migrate.FileMigrationSource{Dir: m.migrationsDir}).FindMigrations()
	if err != nil {
		return 0, fmt.Errorf("failed to find migrations: %w", err)
	}
	baselines, err := readSquashedBaselines(m.migrationsDir, source)
	if err != nil {
		return 0, err
	}

	conn, err := m.connect(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()

	if err := m.acquireLock(ctx, conn); err != nil {
		return 0, err
	}
	defer m.releaseLock(ctx, conn)

	applied, err := appliedMigrationIDs(ctx, conn)
	if err != nil {
		return 0, err
	}
	// Everything is decided before anything is written: a refused partial
	// history, an unknown applied id or a rollback that would cross a baseline
	// leaves the tracking table as it was.
	adoption, err := planSquashAdoption(baselines, applied)
	if err != nil {
		return 0, err
	}
	plan, err := planMigrations(source, applied, squashedIDs(baselines), dir, limit)
	if err != nil {
		return 0, err
	}
	if err := checkBaselineBoundary(plan, baselines, dir, limit); err != nil {
		return 0, err
	}
	if err := writeSquashAdoption(ctx, conn, adoption); err != nil {
		return 0, err
	}
	for _, b := range adoption.adopt {
		m.logger.Infof("Recorded baseline %s as applied: the database already has every migration it replaces", b.id)
	}
	for i, mig := range plan {
		if err := applyMigration(ctx, conn, dir, mig); err != nil {
			return i, err
		}
	}
	return len(plan), nil
}

func (m *migrationManager) Run(ctx context.Context) error {
	applied, err := m.migrate(ctx, migrationUp, 0)
	if err != nil {
		return serrors.Wrap(opMigrationsRun, err)
	}
	m.logger.Infof("Applied %d migrations", applied)
	return nil
}

func (m *migrationManager) Rollback(ctx context.Context, steps int) error {
	if err := validateRollbackSteps(steps); err != nil {
		return err
	}
	rolledBack, err := m.migrate(ctx, migrationDown, steps)
	if err != nil {
		return serrors.Wrap(opMigrationsRollback, err)
	}
	m.logger.Infof("Rolled back %d migrations", rolledBack)
	return nil
}

func (m *migrationManager) RollbackAll(ctx context.Context) error {
	rolledBack, err := m.migrate(ctx, migrationDown, 0)
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

	// A database that has not been adopted yet (no Run since the upgrade)
	// still lists the replaced migrations; its baseline is applied in effect.
	baselines, err := readSquashedBaselines(m.migrationsDir, migrations)
	if err != nil {
		return nil, err
	}
	for _, b := range baselines {
		if _, ok := appliedMap[b.id]; ok {
			continue
		}
		var latest time.Time
		complete := true
		for _, id := range b.replaces {
			appliedAt, ok := appliedMap[id]
			if !ok {
				complete = false
				break
			}
			if appliedAt.After(latest) {
				latest = appliedAt
			}
		}
		if complete {
			appliedMap[b.id] = latest
		}
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

func (n *noopMigrationManager) Run(_ context.Context) error { return nil }
func (n *noopMigrationManager) Rollback(_ context.Context, steps int) error {
	return validateRollbackSteps(steps)
}
func (n *noopMigrationManager) RollbackAll(_ context.Context) error { return nil }
func (n *noopMigrationManager) Status(_ context.Context) ([]MigrationStatus, error) {
	return nil, nil
}
