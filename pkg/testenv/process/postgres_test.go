package process

import (
	"context"
	"database/sql"
	"os"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/dbconfig"
	"github.com/iota-uz/iota-sdk/pkg/dbctl/testdb"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func TestPostgresRejectsManifestDriftAndDisposesClone(t *testing.T) {
	// Falsely green if requested fingerprints are echoed without reading the cloned database.
	dsn := os.Getenv("TESTENV_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set TESTENV_POSTGRES_DSN to an isolated PostgreSQL cluster")
	}
	cfg, err := pgx.ParseConfig(dsn)
	require.NoError(t, err)
	db := dbconfig.Config{Host: cfg.Host, Port: strconv.Itoa(int(cfg.Port)), User: cfg.User, Password: cfg.Password}
	ctx := context.Background()
	base := "testenv_manifest_" + testdb.Name(uuid.NewString())
	require.NoError(t, testdb.Create(ctx, base, db))
	t.Cleanup(func() { require.NoError(t, testdb.Drop(ctx, base, db)) })
	seed, err := sql.Open("postgres", testdb.ConnectionString(base, db))
	require.NoError(t, err)
	m := testdb.Manifest{SchemaFingerprint: "schema", BaselineFingerprint: "baseline", BuildRevision: "revision"}
	require.NoError(t, testdb.WriteManifest(ctx, seed, m))
	require.NoError(t, seed.Close())
	require.NoError(t, testdb.Seal(ctx, base, db))
	p := &Postgres{Config: db, Template: base, Manifest: m}
	id := uuid.NewString()
	_, err = p.Prepare(ctx, id)
	require.NoError(t, err)
	require.NoError(t, p.Dispose(ctx, id))
	require.NoError(t, p.Dispose(ctx, id))
	p.Manifest.BaselineFingerprint = "drift"
	id = uuid.NewString()
	_, err = p.Prepare(ctx, id)
	require.Error(t, err)
	require.Contains(t, p.owned, id)
	require.NoError(t, p.Dispose(ctx, id))
	require.Empty(t, p.owned)
}
