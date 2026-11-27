package itf

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/dbconfig"
	"github.com/iota-uz/iota-sdk/pkg/dbctl/testdb"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	migrate "github.com/rubenv/sql-migrate"
	"github.com/sirupsen/logrus"
)

func autoTemplateEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(AutoTemplateEnv))) {
	case "0", "false", "off", "no":
		return false
	}
	return true
}

// ensureAutoTemplate returns the migrated template the harness clones from and
// the migration ids it was built from, or "" when there is nothing to build one
// from: no database config in the source, or no migrations directory where the
// migrator would look. The harness then falls back to an empty database,
// exactly as before.
func ensureAutoTemplate(ctx context.Context, admin dbconfig.Config, src config.Source) (string, []string, error) {
	_, migrationCfg, err := resolveApplicationSource(src)
	if err != nil {
		return "", nil, err
	}
	if migrationCfg == nil || migrationCfg.MigrationsDir == "" {
		return "", nil, nil
	}
	fingerprint, err := testdb.MigrationsFingerprint(migrationCfg.MigrationsDir)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil, nil
	}
	if err != nil {
		return "", nil, err
	}
	source, err := (&migrate.FileMigrationSource{Dir: migrationCfg.MigrationsDir}).FindMigrations()
	if err != nil {
		return "", nil, err
	}
	ids := make([]string, 0, len(source))
	for _, mig := range source {
		ids = append(ids, mig.Id)
	}
	name, err := testdb.EnsureMigratedTemplate(ctx, admin, fingerprint, func(ctx context.Context, name string) error {
		pool, err := pgxpool.New(ctx, DBOpts(name, admin))
		if err != nil {
			return err
		}
		defer pool.Close()
		logger := logrus.New()
		logger.SetOutput(os.Stderr)
		logger.SetLevel(logrus.WarnLevel)
		// Migrations create cluster-global objects (roles) too, so the build
		// takes the same lock as a harness replaying them directly.
		return testdb.WithMigrationAdvisoryLockContext(ctx, admin, func() error {
			return application.NewMigrationManager(pool, *migrationCfg, logger).Run(ctx)
		})
	})
	if err != nil {
		return "", nil, err
	}
	return name, ids, nil
}

// verifyAutoClone checks, without running the migrator, that a clone of the
// auto template carries the history it was built from: some gorp_migrations
// table lists every id. The table is looked up in every schema, not through
// search_path: the migrator creates it in the first existing schema of the
// building session's search_path, which need not be public, and the clone's
// sessions may resolve the unqualified name elsewhere (a migration can change
// the database's search_path).
func verifyAutoClone(ctx context.Context, pool *pgxpool.Pool, ids []string) error {
	rows, err := pool.Query(ctx, `
		SELECT format('%I.%I', n.nspname, c.relname)
		FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relname = 'gorp_migrations' AND c.relkind IN ('r', 'p')`)
	if err != nil {
		return serrors.WrapContext(opSchemaReadiness, err, "find migration history")
	}
	ledgers, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return serrors.WrapContext(opSchemaReadiness, err, "find migration history")
	}
	for _, ledger := range ledgers {
		var recorded int
		if err := pool.QueryRow(ctx, `SELECT count(DISTINCT id) FROM `+ledger+` WHERE id = ANY($1)`, ids).Scan(&recorded); err != nil {
			return serrors.WrapContext(opSchemaReadiness, err, "read migration history "+ledger)
		}
		if recorded == len(ids) {
			return nil
		}
	}
	return serrors.New(serrors.Invalid, fmt.Sprintf(
		"clone of the migrated template has no complete migration history (gorp_migrations tables found: %v)", ledgers)).WithOp(opSchemaReadiness)
}
