package artifact_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/artifact"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/build"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

func TestMain(m *testing.M) {
	if err := os.Chdir("../../.."); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func pool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	h := itf.NewHarness(t, itf.HarnessConfig{
		Name:      "extruntime_artifact",
		Database:  itf.DatabaseConfig{Provisioning: itf.ProvisioningPerTestDatabase},
		Isolation: itf.IsolationConfig{Mode: itf.IsolationCommitted},
	})
	return h.Scope(t).Pool
}

func stored(t *testing.T, s *artifact.Store, tree build.Tree) string {
	t.Helper()
	hash := artifact.TreeHash(tree)
	archive, err := artifact.Pack(tree)
	require.NoError(t, err)
	require.NoError(t, s.Put(context.Background(), nil, hash, artifact.KindBuild, archive))
	return hash
}

func TestStoreCacheIsPrivateToTheUser(t *testing.T) {
	db := pool(t)

	t.Run("a new cache directory is 0700", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "nested", "cache")
		_, err := artifact.NewStore(db, dir)
		require.NoError(t, err)
		st, err := os.Stat(dir)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0o700), st.Mode().Perm())
	})
	t.Run("an open directory of the user is tightened", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.Chmod(dir, 0o777))
		_, err := artifact.NewStore(db, dir)
		require.NoError(t, err)
		st, err := os.Stat(dir)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0o700), st.Mode().Perm())
	})
	t.Run("a symlink in place of the directory is refused", func(t *testing.T) {
		base := t.TempDir()
		target := filepath.Join(base, "elsewhere")
		require.NoError(t, os.Mkdir(target, 0o700))
		link := filepath.Join(base, "cache")
		require.NoError(t, os.Symlink(target, link))
		_, err := artifact.NewStore(db, link)
		// MkdirAll follows the link, so the check must be on the link itself.
		require.Error(t, err)
	})
	t.Run("a directory of another user is refused", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root owns everything")
		}
		_, err := artifact.NewStore(db, "/usr")
		require.Error(t, err)
	})
}

func TestDirDoesNotTrustAnExistingDirectory(t *testing.T) {
	ctx := context.Background()
	db := pool(t)
	s, err := artifact.NewStore(db, t.TempDir())
	require.NoError(t, err)
	tree := build.Tree{"server.mjs": []byte("export const ok = true\n"), "dist/chunk.js": []byte("export {}\n")}
	hash := stored(t, s, tree)

	// Someone put a directory under the name of the hash before the store did.
	planted := filepath.Join(s.CacheDir(), hash)
	require.NoError(t, os.MkdirAll(planted, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(planted, "server.mjs"), []byte("process.exit(66)\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(planted, "extra.mjs"), []byte("evil\n"), 0o644))

	dir, err := s.Dir(ctx, hash)
	require.NoError(t, err)
	got, err := os.ReadFile(filepath.Join(dir, "server.mjs"))
	require.NoError(t, err)
	require.Equal(t, "export const ok = true\n", string(got), "the planted file was replaced by the stored content")
	require.NoFileExists(t, filepath.Join(dir, "extra.mjs"))
	require.FileExists(t, filepath.Join(dir, "dist", "chunk.js"))

	t.Run("a symlink planted in a materialized directory is not followed", func(t *testing.T) {
		s2, err := artifact.NewStore(db, t.TempDir())
		require.NoError(t, err)
		hash := stored(t, s2, tree)
		d := filepath.Join(s2.CacheDir(), hash)
		require.NoError(t, os.MkdirAll(filepath.Join(d, "dist"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(d, "server.mjs"), tree["server.mjs"], 0o644))
		require.NoError(t, os.Symlink("/etc/passwd", filepath.Join(d, "dist", "chunk.js")))
		dir, err := s2.Dir(ctx, hash)
		require.NoError(t, err)
		st, err := os.Lstat(filepath.Join(dir, "dist", "chunk.js"))
		require.NoError(t, err)
		require.True(t, st.Mode().IsRegular(), "the symlink was replaced")
	})

	t.Run("an intact leftover is reused", func(t *testing.T) {
		s3, err := artifact.NewStore(db, t.TempDir())
		require.NoError(t, err)
		hash := stored(t, s3, tree)
		first, err := s3.Dir(ctx, hash)
		require.NoError(t, err)
		marker := filepath.Join(first, "server.mjs")
		before, err := os.Stat(marker)
		require.NoError(t, err)
		again, err := artifact.NewStore(db, s3.CacheDir())
		require.NoError(t, err)
		second, err := again.Dir(ctx, hash)
		require.NoError(t, err)
		after, err := os.Stat(filepath.Join(second, "server.mjs"))
		require.NoError(t, err)
		require.True(t, os.SameFile(before, after), "a verified directory is kept, not rewritten")
	})
}
