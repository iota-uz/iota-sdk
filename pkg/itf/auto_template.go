package itf

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/dbconfig"
	"github.com/iota-uz/iota-sdk/pkg/dbctl/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
)

func autoTemplateEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(AutoTemplateEnv))) {
	case "0", "false", "off", "no":
		return false
	}
	return true
}

// ensureAutoTemplate returns the migrated template the harness clones from, or
// "" when there is nothing to build one from: no database config in the
// source, or no migrations directory where the migrator would look. The
// harness then falls back to an empty database, exactly as before.
func ensureAutoTemplate(ctx context.Context, admin dbconfig.Config, src config.Source) (string, error) {
	_, migrationCfg, err := resolveApplicationSource(src)
	if err != nil {
		return "", err
	}
	if migrationCfg == nil || migrationCfg.MigrationsDir == "" {
		return "", nil
	}
	fingerprint, err := testdb.MigrationsFingerprint(migrationCfg.MigrationsDir)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return testdb.EnsureMigratedTemplate(ctx, admin, fingerprint, func(ctx context.Context, name string) error {
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
}
