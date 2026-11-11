package manage_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/artifact"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/build"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/catalog"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=T", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	return strings.TrimSpace(string(out))
}

// bareRepo commits files (plus a symlink and a node_modules file that must
// not become part of the package tree) and returns a bare clone and the SHA.
func bareRepo(t *testing.T, files map[string]string) (repo, sha string) {
	t.Helper()
	work := t.TempDir()
	git(t, work, "init", "-q")
	for p, c := range files {
		full := filepath.Join(work, filepath.FromSlash(p))
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(c), 0o644))
	}
	require.NoError(t, os.Symlink("/etc/passwd", filepath.Join(work, "link")))
	require.NoError(t, os.MkdirAll(filepath.Join(work, "node_modules"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(work, "node_modules", "x.js"), []byte("x"), 0o644))
	git(t, work, "add", "-A", "-f")
	git(t, work, "commit", "-q", "-m", "package")
	sha = git(t, work, "rev-parse", "HEAD")
	repo = filepath.Join(t.TempDir(), "pkg.git")
	git(t, work, "clone", "-q", "--bare", work, repo)
	return repo, sha
}

func allowFile(c *manage.Config) { c.Git.AllowedSchemes = []string{"file"} }

func packageFiles(t *testing.T, version string) map[string]string {
	t.Helper()
	m := insuranceManifest(tm(manifest.KindString))
	return map[string]string{
		"package.json":     pkgJSON(ns, version),
		"manifest.json":    string(mustJSON(t, m)),
		"server.mjs":       "export const renew = async () => ({});\n",
		"contracts.mjs":    "export const policy = {};\n",
		"src/contracts.ts": "export {}\n",
		"README.md":        "# insurance\n\nunicode: тест\n",
	}
}

func TestGitImportExportRoundTripPreservesHashes(t *testing.T) {
	e := newEnv(t, allowFile)
	ctx := context.Background()
	files := packageFiles(t, "1.0.0")
	repo, sha := bareRepo(t, files)

	want := build.Tree{}
	for p, c := range files {
		want[p] = []byte(c)
	}

	// a default service refuses local repositories; transports are allow-listed
	strict := newEnv(t)
	_, err := strict.svc.ImportGit(ctx, strict.author, repo, sha, "")
	require.Equal(t, serrors.Invalid, serrors.CodeOf(err))
	require.Contains(t, strict.auditActions(manage.OutcomeFailed), manage.ActionImportGit)

	// only a full object id is accepted; nothing is resolved from names
	for _, bad := range []string{"main", sha[:12], strings.ToUpper(sha)} {
		_, err = e.svc.ImportGit(ctx, e.author, repo, bad, "")
		require.Equal(t, serrors.Invalid, serrors.CodeOf(err), bad)
	}
	_, err = e.svc.ImportGit(ctx, e.author, repo, strings.Repeat("0", 40), "")
	require.Error(t, err, "an unknown commit is refused")
	_, err = e.svc.ImportGit(ctx, e.author, "--upload-pack=touch /tmp/pwned", sha, "")
	require.Equal(t, serrors.Invalid, serrors.CodeOf(err))
	_, err = e.svc.ImportGit(ctx, e.reader, repo, sha, "")
	require.Equal(t, serrors.PermissionDenied, serrors.CodeOf(err))

	pk, err := e.svc.ImportGit(ctx, e.author, repo, sha, "")
	require.NoError(t, err)
	require.Equal(t, ns, pk.Namespace)
	require.Equal(t, manage.ProvenanceGit, pk.Provenance)
	require.Equal(t, sha, pk.RepoCommit)
	require.Equal(t, repo, pk.RepoURL)

	files2, err := e.svc.ListFiles(ctx, e.reader, ns)
	require.NoError(t, err)
	require.Len(t, files2, len(files), "symlinks and node_modules are not part of the tree")
	for _, f := range files2 {
		require.Equal(t, artifact.HashBytes(want[f.Path]), f.Hash, f.Path)
	}

	// export tar and zip: same tree identity as the repository content
	tarball, err := e.svc.ExportDraft(ctx, e.reader, ns, manage.FormatTar)
	require.NoError(t, err)
	require.Equal(t, artifact.TreeHash(want), tarball.TreeHash)
	tree, err := manage.UnpackArchive(tarball.Data)
	require.NoError(t, err)
	require.Equal(t, want, tree)
	zipped, err := e.svc.ExportDraft(ctx, e.reader, ns, manage.FormatZip)
	require.NoError(t, err)
	tree, err = manage.UnpackArchive(zipped.Data)
	require.NoError(t, err)
	require.Equal(t, want, tree)
	again, err := e.svc.ExportDraft(ctx, e.reader, ns, manage.FormatTar)
	require.NoError(t, err)
	require.Equal(t, tarball.Data, again.Data, "exports are deterministic")

	// uploaded tree = same model with runtime provenance; ids/hashes survive
	pk, err = e.svc.ImportTree(ctx, e.author, tree, manage.ImportMeta{Provenance: manage.ProvenanceRuntime})
	require.NoError(t, err)
	require.Equal(t, manage.ProvenanceRuntime, pk.Provenance)
	require.Empty(t, pk.RepoCommit)
	back, err := e.svc.ExportDraft(ctx, e.reader, ns, manage.FormatTar)
	require.NoError(t, err)
	require.Equal(t, tarball.TreeHash, back.TreeHash)

	// a git draft publishes with git provenance until it is edited
	_, err = e.svc.ImportGit(ctx, e.author, repo, sha, "")
	require.NoError(t, err)
	v, err := e.svc.Publish(ctx, e.admin, ns, "")
	require.NoError(t, err)
	require.Equal(t, manage.ProvenanceGit, v.Provenance)
	require.Equal(t, sha, v.RepoCommit)
	require.Equal(t, repo, v.RepoURL)
	require.Equal(t, tarball.TreeHash, v.TreeHash)
	exp, err := e.svc.ExportVersion(ctx, e.reader, ns, "1.0.0", manage.FormatTar)
	require.NoError(t, err)
	require.Equal(t, tarball.Data, exp.Data, "the version exports exactly the imported source")

	_, err = e.svc.PutFile(ctx, e.author, ns, "README.md", []byte("edited\n"), nil)
	require.NoError(t, err)
	v2, err := e.svc.Publish(ctx, e.admin, ns, "1.0.1")
	require.NoError(t, err)
	require.Equal(t, manage.ProvenanceRuntime, v2.Provenance, "an edited draft no longer matches the commit")
	require.Empty(t, v2.RepoCommit)
}

func TestUntrustedArchivesAreRejected(t *testing.T) {
	mk := func(h *tar.Header, body string) []byte {
		var buf bytes.Buffer
		w := tar.NewWriter(&buf)
		h.Size = int64(len(body))
		require.NoError(t, w.WriteHeader(h))
		_, err := w.Write([]byte(body))
		require.NoError(t, err)
		require.NoError(t, w.Close())
		return buf.Bytes()
	}
	for name, data := range map[string][]byte{
		"parent":   mk(&tar.Header{Name: "../x", Typeflag: tar.TypeReg, Mode: 0o644}, "x"),
		"absolute": mk(&tar.Header{Name: "/etc/x", Typeflag: tar.TypeReg, Mode: 0o644}, "x"),
		"symlink":  mk(&tar.Header{Name: "l", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"}, ""),
		"git":      mk(&tar.Header{Name: ".git/config", Typeflag: tar.TypeReg, Mode: 0o644}, "x"),
	} {
		_, err := manage.UnpackArchive(data)
		require.Error(t, err, name)
	}
	e := newEnv(t)
	_, err := e.svc.ImportTree(context.Background(), e.author, build.Tree{"package.json": []byte(`{"iota":{"namespace":"core.evil"}}`)}, manage.ImportMeta{})
	require.Error(t, err, "native namespaces are reserved")
	_, err = e.svc.ImportTree(context.Background(), e.author, build.Tree{"package.json": []byte(`{"iota":{}}`)}, manage.ImportMeta{})
	require.Error(t, err, "a tree must declare its namespace")
}

func TestKitExportHasRefsAndTypesButNoHandlers(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()

	k0, err := e.svc.Kit(ctx)
	require.NoError(t, err)
	require.Contains(t, k0.Modules, "@iota-kit/core")
	require.Contains(t, k0.Modules, "@iota-kit/crm")
	require.Contains(t, string(k0.Modules["@iota-kit/core"]["index.d.ts"]), "core_commands_RejectIncoming")
	_, err = e.svc.Kit(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, e.kits.Calls, "an unchanged catalog does not regenerate the kit")

	e.draft(ns, "1.0.0", insuranceManifest(tm(manifest.KindString)), nil)
	v1 := e.release(ns, "1.0.0")
	_ = v1
	k1, err := e.svc.Kit(ctx)
	require.NoError(t, err)
	require.NotEqual(t, k0.Hash, k1.Hash)
	mod := k1.Modules["@iota-kit/"+ns]
	require.NotNil(t, mod)
	require.Contains(t, string(mod["index.js"]), "export const policy", "pure contract module of the active version")
	require.Contains(t, string(mod["index.d.ts"]), "acme_insurance_policy")
	require.Contains(t, string(mod["index.d.ts"]), "acme_insurance_renewPolicy")

	// what the generator received: contract kinds only, never the server graph
	var src build.KitSource
	for _, s := range e.kits.Sources {
		if s.Namespace == ns {
			src = s
		}
	}
	for _, r := range src.Resources {
		require.NotEqual(t, manifest.ResPage, r.Kind, "client values are not kit contracts")
		require.NotEmpty(t, r.ContractHash)
	}
	for _, m := range k1.Modules {
		for p, c := range m {
			require.NotContains(t, string(c), "HANDLER_SECRET", p)
		}
	}

	// the published version records the snapshot it was built against
	e.draft(ns, "2.0.0", insuranceManifest(tm(manifest.KindString)), nil)
	pub, err := e.svc.Publish(ctx, e.admin, ns, "2.0.0")
	require.NoError(t, err)
	require.Equal(t, k1.Hash, pub.KitHash)
	snap, err := e.svc.KitSnapshot(ctx, e.reader, k1.Hash)
	require.NoError(t, err)
	require.Equal(t, k1.Modules["@iota-kit/"+ns], snap.Modules["@iota-kit/"+ns])
	old, err := e.svc.GetVersion(ctx, e.reader, ns, "1.0.0")
	require.NoError(t, err)
	require.Equal(t, k0.Hash, old.KitHash, "v1 was built before its own contracts were in the kit")

	// npm-installable tarball per kit module
	arc, err := e.svc.KitTarball(ctx, e.reader, "", "@iota-kit/"+ns)
	require.NoError(t, err)
	require.Equal(t, "iota-kit-"+ns+".tgz", arc.Filename)
	gz, err := gzip.NewReader(bytes.NewReader(arc.Data))
	require.NoError(t, err)
	tr := tar.NewReader(gz)
	got := map[string]string{}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		b, _ := io.ReadAll(tr)
		got[h.Name] = string(b)
	}
	require.Contains(t, got, "package/index.js")
	require.Contains(t, got, "package/index.d.ts")
	require.Contains(t, got["package/package.json"], `"name": "@iota-kit/`+ns+`"`)
	require.Contains(t, got["package/package.json"], `"version": "0.0.0-kit-`)
	for p, c := range got {
		require.NotContains(t, c, "HANDLER_SECRET", p)
	}
	// short names work; an older snapshot yields the older content
	arc2, err := e.svc.KitTarball(ctx, e.reader, k0.Hash, "core")
	require.NoError(t, err)
	require.Equal(t, "iota-kit-core.tgz", arc2.Filename)
	_, err = e.svc.KitTarball(ctx, e.reader, k0.Hash, ns)
	require.Equal(t, serrors.NotFound, serrors.CodeOf(err), "v1 contracts were not part of the earlier snapshot")
	_, err = e.svc.KitTarball(ctx, spiAnon(), "", "core")
	require.Equal(t, serrors.PermissionDenied, serrors.CodeOf(err))
}

func TestCatalogInvalidatesOnActivationAcrossReplicas(t *testing.T) {
	e := newEnv(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	replica := catalog.New(e.db, e.natives)
	done := make(chan struct{})
	go func() { _ = replica.Listen(ctx); close(done) }()
	defer func() { cancel(); <-done }()

	e.draft(ns, "1.0.0", insuranceManifest(tm(manifest.KindString)), nil)
	_, ok, err := e.cat.Resolve(ctx, ns+".policy")
	require.NoError(t, err)
	require.False(t, ok)
	_, ok, err = replica.Resolve(ctx, ns+".policy") // primes the replica's cache
	require.NoError(t, err)
	require.False(t, ok)

	e.release(ns, "1.0.0")
	_, ok, _ = e.cat.Resolve(ctx, ns+".policy")
	require.True(t, ok, "the activating node sees the change at once")
	require.Eventually(t, func() bool {
		_, ok, _ := replica.Resolve(ctx, ns+".policy")
		return ok
	}, 5*time.Second, 50*time.Millisecond, "other replicas learn through NOTIFY")

	ents, err := e.cat.Entities(ctx)
	require.NoError(t, err)
	var ids []string
	for _, r := range ents {
		ids = append(ids, r.Resource.ID)
	}
	require.ElementsMatch(t, []string{"core.user", "crm.client", ns + ".policy"}, ids)

	require.NoError(t, e.svc.Deactivate(ctx, e.admin, ns))
	_, ok, _ = e.cat.Resolve(ctx, ns+".policy")
	require.False(t, ok)
	require.Eventually(t, func() bool {
		_, ok, _ := replica.Resolve(ctx, ns+".policy")
		return !ok
	}, 5*time.Second, 50*time.Millisecond)
}

func TestCatalogGuardsAndNativeRegistration(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	m := insuranceManifest(tm(manifest.KindString))
	m.Guards = []manifest.Guard{
		{ID: ns + ".zGuard", Export: "g2", Target: ns + ".activatePolicy", Phase: manifest.GuardOperation},
		{ID: ns + ".aGuard", Export: "g1", Target: ns + ".activatePolicy", Phase: manifest.GuardOperation},
		{ID: ns + ".other", Export: "g3", Target: ns + ".renewPolicy", Phase: manifest.GuardInput},
	}
	e.draft(ns, "1.0.0", m, nil)
	e.release(ns, "1.0.0")
	gs, err := e.cat.Guards(ctx, ns+".activatePolicy", manifest.GuardOperation)
	require.NoError(t, err)
	require.Len(t, gs, 2)
	require.Equal(t, []string{ns + ".aGuard", ns + ".zGuard"}, []string{gs[0].Guard.ID, gs[1].Guard.ID}, "stable ID order")
	require.Equal(t, "1.0.0", gs[0].Version)
	r, ok, err := e.cat.Resolve(ctx, "core.commands.RejectIncoming")
	require.NoError(t, err)
	require.True(t, ok)
	require.True(t, r.Native)

	// packages cannot declare resources of another owner
	clash := insuranceManifest(tm(manifest.KindString))
	clash.Namespace = "acme.insurance.sub"
	clash.Resources = []manifest.Resource{{ID: ns + ".policy", Kind: manifest.ResQuery, Input: emptyObj(), Output: emptyObj()}}
	clash.Implementations, clash.Subscriptions = nil, nil
	e.draft("acme.insurance.sub", "1.0.0", clash, nil)
	_, err = e.svc.Publish(ctx, e.admin, "acme.insurance.sub", "1.0.0")
	require.Error(t, err, "a manifest validates resource ownership at publish")
}
