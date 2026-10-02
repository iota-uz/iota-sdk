// testenv-baseline builds a uniquely named sealed SDK ERP baseline.
package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/commands/e2e"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/dbconfig"
	"github.com/iota-uz/iota-sdk/pkg/dbctl/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
)

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	name := flag.String("name", "", "fresh baseline database name")
	revision := flag.String("revision", "", "SDK build revision")
	migrations := flag.String("migrations", "migrations", "migration directory")
	seedVersion := flag.String("seed-version", "sdk-erp-v1", "baseline seed contract version")
	flag.Parse()
	if *name == "" || *revision == "" || *seedVersion == "" {
		return fmt.Errorf("name, revision and seed-version required")
	}
	hash := sha256.New()
	if err := filepath.WalkDir(*migrations, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".sql" {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(*migrations, path)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(hash, "%s\x00", relative); err != nil {
			return err
		}
		hash.Write(b)
		return nil
	}); err != nil {
		return err
	}
	schema := hex.EncodeToString(hash.Sum(nil))
	baselineHash := sha256.Sum256([]byte(schema + "\x00" + *revision + "\x00" + *seedVersion))
	manifest := testdb.Manifest{SchemaFingerprint: schema, BaselineFingerprint: hex.EncodeToString(baselineHash[:]), BuildRevision: *revision}
	db := dbconfig.Config{Name: *name, Host: env("DB_HOST", "localhost"), Port: env("DB_PORT", "5432"), User: env("DB_USER", "postgres"), Password: env("DB_PASSWORD", "postgres"), MigrationsDir: *migrations}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := testdb.Create(ctx, *name, db); err != nil {
		return err
	}
	ready := false
	defer func() {
		if !ready {
			cleanup, done := context.WithTimeout(context.Background(), 10*time.Second)
			defer done()
			_ = testdb.Drop(cleanup, *name, db)
		}
	}()
	pool, err := pgxpool.New(ctx, testdb.ConnectionString(*name, db))
	if err != nil {
		return err
	}
	defer pool.Close()
	logger := logrus.New()
	logger.SetOutput(os.Stderr)
	if err := application.NewMigrationManager(pool, db, logger).Run(); err != nil {
		return err
	}
	if err := e2e.SeedPool(ctx, pool, logger); err != nil {
		return err
	}
	pool.Close()
	conn, err := sql.Open("postgres", testdb.ConnectionString(*name, db))
	if err != nil {
		return err
	}
	if err := testdb.WriteManifest(ctx, conn, manifest); err != nil {
		_ = conn.Close()
		return err
	}
	if err := conn.Close(); err != nil {
		return err
	}
	if err := testdb.Seal(ctx, *name, db); err != nil {
		return err
	}
	ready = true
	return json.NewEncoder(os.Stdout).Encode(struct {
		Database string          `json:"database"`
		Manifest testdb.Manifest `json:"manifest"`
	}{*name, manifest})
}
