package runtime_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/runtime"
)

// A package delivery records its invocation on a second connection while its
// transaction holds one: the runtime refuses a one-connection pool at startup
// instead of hanging on the first delivery (the Go event core alone accepts it).
func TestNewRefusesAPoolTooSmallForPackageDeliveries(t *testing.T) {
	cfg, err := pgxpool.ParseConfig("postgres://nobody@127.0.0.1:1/none?pool_max_conns=1")
	require.NoError(t, err)
	db, err := pgxpool.NewWithConfig(context.Background(), cfg) // lazy: never connects
	require.NoError(t, err)
	t.Cleanup(db.Close)
	natives, err := native.Compose()
	require.NoError(t, err)
	_, err = runtime.New(runtime.Config{Natives: natives, CacheDir: t.TempDir()}, runtime.Deps{DB: db})
	require.ErrorContains(t, err, "at least 2 connections")
}
