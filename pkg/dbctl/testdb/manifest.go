package testdb

import (
	"context"
	"database/sql"
	"fmt"
)

type Manifest struct {
	SchemaFingerprint   string `json:"schemaFingerprint"`
	BaselineFingerprint string `json:"baselineFingerprint"`
	BuildRevision       string `json:"buildRevision"`
}

// WriteManifest is called by the baseline builder after migrations and seed.
func WriteManifest(ctx context.Context, db *sql.DB, m Manifest) error {
	if m.SchemaFingerprint == "" || m.BaselineFingerprint == "" || m.BuildRevision == "" {
		return fmt.Errorf("baseline manifest fields must be nonempty")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, `CREATE SCHEMA IF NOT EXISTS testenv; CREATE TABLE IF NOT EXISTS testenv.baseline_manifest (singleton boolean PRIMARY KEY DEFAULT true CHECK(singleton), schema_fingerprint text NOT NULL, baseline_fingerprint text NOT NULL, build_revision text NOT NULL)`); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO testenv.baseline_manifest(singleton,schema_fingerprint,baseline_fingerprint,build_revision) VALUES(true,$1,$2,$3) ON CONFLICT(singleton) DO UPDATE SET schema_fingerprint=EXCLUDED.schema_fingerprint,baseline_fingerprint=EXCLUDED.baseline_fingerprint,build_revision=EXCLUDED.build_revision`, m.SchemaFingerprint, m.BaselineFingerprint, m.BuildRevision); err != nil {
		return err
	}
	return tx.Commit()
}
func ReadManifest(ctx context.Context, db *sql.DB) (Manifest, error) {
	var m Manifest
	err := db.QueryRowContext(ctx, `SELECT schema_fingerprint,baseline_fingerprint,build_revision FROM testenv.baseline_manifest WHERE singleton=true`).Scan(&m.SchemaFingerprint, &m.BaselineFingerprint, &m.BuildRevision)
	return m, err
}
