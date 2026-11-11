package runtime_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/runtime"
	"github.com/iota-uz/iota-sdk/pkg/sdkidentity"
)

const commit = "0123456789abcdef0123456789abcdef01234567"

// fakeDist writes an @iota-uz/sdk package layout under root/node_modules.
func fakeDist(t *testing.T, root, version string, identity sdkidentity.Identity, workers bool) string {
	t.Helper()
	dir := filepath.Join(root, "node_modules", "@iota-uz", "sdk")
	write := func(rel string, v any) {
		p := filepath.Join(dir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		b, err := json.Marshal(v)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(p, b, 0o644))
	}
	write("package.json", map[string]string{"name": "@iota-uz/sdk", "version": version})
	write("dist/sdk-identity.json", identity)
	if workers {
		write("dist/runtime/host/worker.mjs", "")
		write("dist/runtime/build/worker.mjs", "")
	}
	return dir
}

func goodIdentity() sdkidentity.Identity {
	return sdkidentity.Identity{ReleaseVersion: sdkidentity.ReleaseVersion, SourceCommit: commit, ProtocolVersion: sdkidentity.ProtocolVersion}
}

func TestResolveDistFindsTheWorkersOfTheMatchingRelease(t *testing.T) {
	root := t.TempDir()
	dir := fakeDist(t, root, sdkidentity.ReleaseVersion, goodIdentity(), true)
	d, err := runtime.ResolveDist(runtime.DistConfig{AppRoot: root})
	require.NoError(t, err)
	require.Equal(t, dir, d.Dir)
	require.Equal(t, filepath.Join(dir, "dist", "runtime", "host", "worker.mjs"), d.HostWorker)
	require.Equal(t, filepath.Join(dir, "dist", "runtime", "build", "worker.mjs"), d.BuildWorker)
	require.Equal(t, commit, d.Identity.SourceCommit)

	t.Setenv(runtime.DistEnv, dir)
	d, err = runtime.ResolveDist(runtime.DistConfig{AppRoot: t.TempDir()})
	require.NoError(t, err, "IOTA_SDK_DIST wins over the application root")
	require.Equal(t, dir, d.Dir)
}

func TestResolveDistNamesWhatIsWrong(t *testing.T) {
	t.Setenv(runtime.DistEnv, "")
	_, err := runtime.ResolveDist(runtime.DistConfig{AppRoot: t.TempDir()})
	require.ErrorContains(t, err, "package.json does not exist")
	require.ErrorContains(t, err, "install @iota-uz/sdk@"+sdkidentity.ReleaseVersion)

	root := t.TempDir()
	fakeDist(t, root, "0.0.1", goodIdentity(), true)
	_, err = runtime.ResolveDist(runtime.DistConfig{AppRoot: root})
	require.ErrorContains(t, err, "version 0.0.1 does not match the Go SDK release "+sdkidentity.ReleaseVersion)

	root = t.TempDir()
	bad := goodIdentity()
	bad.ProtocolVersion = "99.0.0"
	fakeDist(t, root, sdkidentity.ReleaseVersion, bad, true)
	_, err = runtime.ResolveDist(runtime.DistConfig{AppRoot: root})
	require.ErrorContains(t, err, "dist/sdk-identity.json")
	require.ErrorContains(t, err, "protocol")

	root = t.TempDir()
	fakeDist(t, root, sdkidentity.ReleaseVersion, goodIdentity(), false)
	_, err = runtime.ResolveDist(runtime.DistConfig{AppRoot: root})
	require.ErrorContains(t, err, "worker.mjs is missing")
}

func TestNewRequiresNativesAndResolvesMissingWorkersFromTheDistribution(t *testing.T) {
	t.Setenv(runtime.DistEnv, "")
	// The pool connects lazily; New fails on configuration before touching it.
	db, err := pgxpool.New(context.Background(), "postgres://nobody:none@127.0.0.1:1/none")
	require.NoError(t, err)
	t.Cleanup(db.Close)

	_, err = runtime.New(runtime.Config{CacheDir: t.TempDir()}, runtime.Deps{DB: db})
	require.ErrorContains(t, err, "Natives is required")

	_, err = runtime.New(runtime.Config{CacheDir: t.TempDir(), Natives: native.NewSet(), Dist: runtime.DistConfig{AppRoot: t.TempDir()}}, runtime.Deps{DB: db})
	require.ErrorContains(t, err, "@iota-uz/sdk package", "without WorkerScript and Builder the workers come from the distribution")
}
