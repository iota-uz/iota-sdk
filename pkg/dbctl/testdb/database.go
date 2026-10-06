// Package testdb owns PostgreSQL database lifecycle shared by ITF and browser environments.
package testdb

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/dbconfig"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	_ "github.com/lib/pq"
	"log"
	"regexp"
	"strings"
	"time"
)

const createDatabaseAdvisoryLockKey int64 = 6_073_120_419_784_512_302
const migrationAdvisoryLockKey int64 = 6_073_120_419_784_512_301
const TemplateDBEnv = "ITF_TEMPLATE_DB"
const opCreateDBE = serrors.Op("testdb.CreateDB")
const opDropDBE = serrors.Op("testdb.DropDB")
const (
	// PostgreSQL database name maximum length is 63 characters
	maxDBNameLength = 63
	// Reserve space for hash suffix when truncating (8 chars + underscore)
	hashSuffixLength = 9
)

var nonIdentifierChars = regexp.MustCompile(`[^a-z0-9_]`)

// Name replaces special characters in database names with underscores
// and ensures the name doesn't exceed PostgreSQL's 63-character limit
func Name(name string) string {
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
	CreateDBFromTemplate(name, "", db)
}

// CreateDBFromTemplate creates a test database, cloning it from template when
// template is non-empty. Cloning replaces a migration replay (~1s) with a
// catalog-level copy (~10ms); see [MigrationConfig.TemplateDB] for how the
// harness picks the template up.
//
// The template database must exist and must have no open connections: Postgres
// refuses CREATE DATABASE ... TEMPLATE while anything is connected to the
// source. The harness never opens a pool against the template, so the only
// realistic contender is another clone in flight, which the advisory lock below
// serializes.
func CreateDBFromTemplate(name, template string, db dbconfig.Config) {
	if err := createDatabase(context.Background(), name, template, db, true, true); err != nil {
		panic(err)
	}
}
func createDatabase(ctx context.Context, name, template string, db dbconfig.Config, replace, evict bool) error {
	sanitizedName := Name(name)
	adminConnStr := AdminConnectionString(db)
	conn, err := sql.Open("postgres", adminConnStr)
	if err != nil {
		return err
	}
	defer func() {
		if err := conn.Close(); err != nil {
			log.Printf("[WARNING] Error closing CreateDB connection: %v", err)
		}
	}()

	create := fmt.Sprintf(`CREATE DATABASE "%s"`, sanitizedName)
	var sanitizedTemplate string
	if template != "" {
		sanitizedTemplate = Name(template)
		if err := assertTemplateExists(ctx, conn, sanitizedTemplate); err != nil {
			return err
		}
		create = fmt.Sprintf(`CREATE DATABASE "%s" TEMPLATE "%s"`, sanitizedName, sanitizedTemplate)
	}

	// DROP + CREATE is not atomic, and shared-per-package harnesses in sibling
	// test binaries derive the SAME database name from the same config. Without
	// this lock they interleave into
	// `duplicate key value violates unique constraint "pg_database_datname_index"`.
	// The lock is held for a single catalog operation (milliseconds), unlike
	// the migration lock, which spans a whole migration run.
	//
	// The retry wraps the lock rather than sitting inside it. Sleeping while
	// holding a cluster-wide lock turns one straggler into a convoy: every
	// sibling harness queues behind the sleeper, and with `go test -p 8` across
	// four CI shards the waits ran past the acquisition budget and came back as
	// "canceling statement due to user request" - a lock bottleneck wearing the
	// costume of a timeout.
	err = retryWhileTemplateBusy(ctx, func() error {
		return WithAdvisoryLockContext(ctx, db, createDatabaseAdvisoryLockKey, "create database", lockRequired, func() error {
			if sanitizedTemplate != "" && evict {
				// Postgres refuses to clone a database that anything is
				// connected to, and "anything" includes an autovacuum worker
				// that picked this moment to visit the template. Evicting them
				// is what makes the clone deterministic; the retry below is
				// only for the backend that connects between this statement
				// and the next.
				if err := terminateConnections(ctx, conn, sanitizedTemplate); err != nil {
					return err
				}
			}
			if replace {
				if _, err := conn.ExecContext(ctx, fmt.Sprintf(`DROP DATABASE IF EXISTS "%s"`, sanitizedName)); err != nil {
					return err
				}
			}
			_, err := conn.ExecContext(ctx, create)
			return err
		})
	})
	if err != nil {
		return err
	}
	return nil
}

// CreateDBE creates a test database and returns an error instead of panicking.
func CreateDBE(name string, db dbconfig.Config) error {
	return CreateDBFromTemplateE(name, "", db)
}

// CreateDBFromTemplateE is [CreateDBFromTemplate] returning an error instead of
// panicking.
func CreateDBFromTemplateE(name, template string, db dbconfig.Config) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = serrors.Wrap(opCreateDBE, fmt.Errorf("failed to create test database %q: %v", Name(name), r))
		}
	}()
	CreateDBFromTemplate(name, template, db)
	return nil
}

func AdminConnectionString(db dbconfig.Config) string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s dbname=postgres password=%s sslmode=disable",
		db.Host, db.Port, db.User, db.Password,
	)
}

// assertTemplateExists fails loudly rather than silently producing an empty
// database: a misspelled or not-yet-built template would otherwise surface much
// later as "schema not ready", or as hundreds of "relation does not exist".
func assertTemplateExists(ctx context.Context, conn *sql.DB, template string) error {
	var exists bool
	err := conn.QueryRowContext(
		ctx,
		"SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)",
		template,
	).Scan(&exists)
	if err != nil {
		return fmt.Errorf("probe template database %q: %w", template, err)
	}
	if !exists {
		return fmt.Errorf(
			"template database %q does not exist; build it before running tests or unset %s",
			template, TemplateDBEnv,
		)
	}
	return nil
}

// retryWhileTemplateBusy retries "source database is being accessed by other
// users", which Postgres raises when a clone starts while any backend still
// holds a connection to the template.
//
// Terminating the template's backends removes the cause; this covers the one
// case termination cannot, a backend that connects in the window between the
// eviction and the CREATE. fn must take and release the lock itself, so the
// backoff below is served without holding it.
//
// The durable fix belongs to whoever builds the template: `datallowconn =
// false`, the way template0 does it, makes the race impossible instead of rare.
func retryWhileTemplateBusy(ctx context.Context, fn func() error) error {
	const attempts = 8
	var err error
	for attempt := range attempts {
		err = fn()
		if err == nil || !isSourceDatabaseBusy(err) {
			return err
		}
		timer := time.NewTimer(time.Duration(attempt+1) * 100 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return err
}

// terminateConnections evicts every backend connected to dbName except this
// one. Errors are returned rather than swallowed: a clone that proceeds against
// a still-occupied template fails anyway, and it fails less legibly.
func terminateConnections(ctx context.Context, conn *sql.DB, dbName string) error {
	const terminateSQL = `
		SELECT pg_terminate_backend(pg_stat_activity.pid)
		FROM pg_stat_activity
		WHERE pg_stat_activity.datname = $1
		AND pid <> pg_backend_pid()
	`
	_, err := conn.ExecContext(ctx, terminateSQL, dbName)
	return err
}

func isSourceDatabaseBusy(err error) bool {
	return err != nil && strings.Contains(err.Error(), "is being accessed by other users")
}

// DropDB drops a test database. Used for cleanup after tests to free disk space.
func DropDB(name string, db dbconfig.Config) error {
	if err := dropDBContext(context.Background(), name, db); err != nil {
		log.Printf("[WARNING] DropDB failed: %v", err)
		return err
	}
	return nil
}

func dropDBContext(ctx context.Context, name string, db dbconfig.Config) error {
	sanitizedName := Name(name)
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
	_, _ = conn.ExecContext(ctx, terminateSQL, sanitizedName)
	_, err = conn.ExecContext(ctx, fmt.Sprintf(`DROP DATABASE IF EXISTS "%s"`, sanitizedName))
	return err
}

// WithMigrationAdvisoryLock runs fn while holding a session-level Postgres
// advisory lock on the shared "postgres" maintenance database. Because advisory
// locks are database-local, taking the lock on the admin DB that every parallel
// harness connects to is what actually serializes concurrent migration runs
// (which touch cluster-global catalog objects such as roles/RLS). The lock is
// acquired and released on a single pinned connection, as a session lock
// requires. If the lock cannot be acquired, fn is still run unlocked (best
// effort) so a transient lock hiccup degrades to prior behavior rather than
// failing the whole test run.
func WithMigrationAdvisoryLock(db dbconfig.Config, fn func() error) error {
	return WithAdvisoryLock(db, migrationAdvisoryLockKey, "migration", lockBestEffort, fn)
}

// WithMigrationAdvisoryLockContext serializes migrations on the maintenance
// database using the same lock as legacy harnesses. Lock acquisition is required:
// cancellation or connection failure returns an error without running fn.
// The callback must observe ctx itself; the lock remains held until it returns.
func WithMigrationAdvisoryLockContext(ctx context.Context, db dbconfig.Config, fn func() error) error {
	return WithAdvisoryLockContext(ctx, db, migrationAdvisoryLockKey, "migration", lockRequired, func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		return fn()
	})
}

// Whether a caller can survive running fn without the lock. Migrations are
// idempotent under the migrator's own bookkeeping, so an unlocked run degrades
// to the pre-lock behaviour - noisy, occasionally racy, but not destructive.
// DROP DATABASE + CREATE DATABASE is neither: two harnesses interleaving there
// replace each other's database out from under a running test. Anything that
// rewrites the catalog must fail rather than proceed unserialized.
const (
	lockBestEffort = false
	lockRequired   = true
)

// advisoryLockAcquireTimeout bounds only the WAIT for the lock, never the work
// done under it. Without it a lock leaked by a crashed sibling turns into a
// test binary that hangs until the go test timeout kills it with no
// explanation. The bound is generous because the migration path legitimately
// holds the lock for a full migration replay per waiting harness.
const advisoryLockAcquireTimeout = 5 * time.Minute

// WithAdvisoryLock runs fn while holding a session-level Postgres advisory lock
// with the given key. label appears in the warnings emitted on the degraded
// paths. See [WithMigrationAdvisoryLock] for why the lock is taken on the
// shared admin database rather than the caller's own.
//
// required decides what happens when the lock cannot be taken: with
// lockBestEffort fn runs unlocked and the failure is only logged; with
// lockRequired the error is returned and fn never runs.
func WithAdvisoryLock(db dbconfig.Config, key int64, label string, required bool, fn func() error) error {
	return WithAdvisoryLockContext(context.Background(), db, key, label, required, fn)
}
func WithAdvisoryLockContext(ctx context.Context, db dbconfig.Config, key int64, label string, required bool, fn func() error) error {
	degrade := func(stage string, cause error) error {
		if required {
			return serrors.Wrap(serrors.Op("itf.WithAdvisoryLock"),
				fmt.Errorf("%s advisory lock: %s: %w", label, stage, cause))
		}
		log.Printf("[WARNING] %s advisory lock: %s, running unlocked: %v", label, stage, cause)
		return fn()
	}

	sqlDB, err := sql.Open("postgres", AdminConnectionString(db))
	if err != nil {
		return degrade("open admin connection failed", err)
	}
	defer func() {
		if cerr := sqlDB.Close(); cerr != nil {
			log.Printf("[WARNING] %s advisory lock: closing admin pool: %v", label, cerr)
		}
	}()

	// Pin a single connection: a session advisory lock must be acquired and
	// released on the SAME connection, which a pool would not guarantee.
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return degrade("pin connection failed", err)
	}
	defer func() {
		if cerr := conn.Close(); cerr != nil {
			log.Printf("[WARNING] %s advisory lock: closing pinned connection: %v", label, cerr)
		}
	}()

	acquireCtx, cancel := context.WithTimeout(ctx, advisoryLockAcquireTimeout)
	defer cancel()
	if _, err := conn.ExecContext(acquireCtx, "SELECT pg_advisory_lock($1)", key); err != nil {
		return degrade("acquire failed", err)
	}
	defer func() {
		if _, uerr := conn.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", key); uerr != nil {
			log.Printf("[WARNING] %s advisory lock: release failed: %v", label, uerr)
		}
	}()

	return fn()
}

// DropDBE drops a test database and returns an error instead of panicking.
func DropDBE(name string, db dbconfig.Config) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = serrors.Wrap(opDropDBE, fmt.Errorf("failed to drop test database %q: %v", Name(name), r))
		}
	}()
	err = dropDBContext(context.Background(), name, db)
	return err
}

// DBOpts returns a libpq-style connection string for the named database
// using an explicit dbconfig.Config.
func DBOpts(name string, db dbconfig.Config) string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s dbname=%s password=%s sslmode=disable",
		db.Host, db.Port, db.User, strings.ToLower(Name(name)), db.Password,
	)
}
