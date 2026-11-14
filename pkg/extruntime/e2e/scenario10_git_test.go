package e2e_test

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=Author", "-c", "user.email=author@example.com", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
	return strings.TrimSpace(string(out))
}

func fileHashes(files []manage.File) map[string]string {
	out := map[string]string{}
	for _, f := range files {
		out[f.Path] = f.Hash
	}
	return out
}

// Scenario 10: a commit of a local bare repository (file transport enabled by
// the operator) is imported as a package draft and goes through the same
// check, build, publish, review and activation as a runtime-authored draft;
// exporting and re-importing keeps every file hash, and a rebuild of the
// round-tripped tree reproduces the source and artifact hashes.
func TestScenario10GitImportAndExportRoundTrip(t *testing.T) {
	e := newEnv(t)
	admin := e.admin
	const ns = "e2e.notes"

	// A monorepo with the package in a subdirectory, pushed to a bare repository.
	work, bare := t.TempDir(), filepath.Join(t.TempDir(), "notes.git")
	git(t, work, "init", "-q", "-b", "main")
	for p, c := range loadTree(t, "notes") {
		full := filepath.Join(work, "packages", "notes", p)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(c), 0o644))
	}
	require.NoError(t, os.WriteFile(filepath.Join(work, "README.md"), []byte("monorepo"), 0o644))
	git(t, work, "add", "-A")
	git(t, work, "commit", "-q", "-m", "notes package")
	commit := git(t, work, "rev-parse", "HEAD")
	git(t, work, "clone", "-q", "--bare", ".", bare)

	// Only explicit full commits are imported, and only from allowed locations.
	e.mgmt(admin, http.MethodPost, "/import/git", map[string]string{"repoUrl": bare, "commit": "main", "subdir": "packages/notes"}, http.StatusBadRequest, nil)
	e.mgmt(admin, http.MethodPost, "/import/git", map[string]string{"repoUrl": bare, "commit": strings.Repeat("0", 40), "subdir": "packages/notes"}, http.StatusNotFound, nil)
	e.mgmt(admin, http.MethodPost, "/import/git", map[string]string{"repoUrl": "https://example.invalid/x.git", "commit": commit}, http.StatusBadRequest, nil)
	reader := e.newActor("reader", "Extensions.Read")
	e.mgmt(reader, http.MethodPost, "/import/git", map[string]string{"repoUrl": bare, "commit": commit, "subdir": "packages/notes"}, http.StatusForbidden, nil)

	var pkg manage.Package
	e.mgmt(admin, http.MethodPost, "/import/git", map[string]string{"repoUrl": bare, "commit": commit, "subdir": "packages/notes"}, http.StatusOK, &pkg)
	require.Equal(t, ns, pkg.Namespace)
	require.Equal(t, manage.ProvenanceGit, pkg.Provenance)
	require.Equal(t, commit, pkg.RepoCommit)

	// The draft is the commit's tree, byte for byte.
	var files []manage.File
	e.mgmt(admin, http.MethodGet, "/packages/"+ns+"/files", nil, http.StatusOK, &files)
	imported := fileHashes(files)
	want := loadTree(t, "notes")
	require.Len(t, imported, len(want))
	for p := range want {
		require.Contains(t, imported, p)
	}
	require.NotContains(t, imported, "README.md", "only the subdirectory is the package")

	// Same build and activation path as a runtime draft.
	act := e.release(admin, ns, "0.1.0")
	require.NotEmpty(t, act.Bound)
	var vd manage.VersionDetail
	e.mgmt(admin, http.MethodGet, "/packages/"+ns+"/versions/0.1.0", nil, http.StatusOK, &vd)
	require.Equal(t, manage.ProvenanceGit, vd.Provenance)
	require.Equal(t, commit, vd.RepoCommit)
	require.True(t, vd.Active && vd.Approved)
	member := e.newActor("member", permNotesWrite)
	var added struct {
		Note map[string]string `json:"note"`
	}
	e.mustRPC(member, "command", "e2e.notes.addNote", map[string]any{"title": "from git"}, &added)
	require.Equal(t, http.StatusOK, e.get(member, "/notes").Status)

	// Export the published version, import the archive again, export the draft:
	// every identity is preserved.
	r := e.mgmt(admin, http.MethodGet, "/packages/"+ns+"/export?format=tar&version=0.1.0", nil, http.StatusOK, nil)
	treeHash := r.Header.Get("X-Tree-Hash")
	require.NotEmpty(t, treeHash)
	require.Equal(t, vd.TreeHash, treeHash, "the export carries the published tree identity")
	var back manage.Package
	rr := e.do(admin, http.MethodPost, "/_ext/manage/import/tree", strings.NewReader(string(r.Body)))
	require.Equal(t, http.StatusOK, rr.Status, rr.String())
	require.NoError(t, jsonUnmarshal(rr.Body, &back))
	require.Equal(t, ns, back.Namespace)
	e.mgmt(admin, http.MethodGet, "/packages/"+ns+"/files", nil, http.StatusOK, &files)
	require.Equal(t, imported, fileHashes(files), "file hashes survive the archive round trip")
	r2 := e.mgmt(admin, http.MethodGet, "/packages/"+ns+"/export?format=zip", nil, http.StatusOK, nil)
	require.Equal(t, treeHash, r2.Header.Get("X-Tree-Hash"), "tar and zip describe the same tree")

	// Rebuilding the round-tripped tree reproduces the hashes and resource IDs.
	rep := e.buildPkg(admin, ns)
	require.False(t, rep.HasErrors)
	require.Equal(t, vd.SourceHash, rep.SourceHash)
	require.Equal(t, vd.ArtifactHash, rep.ArtifactHash)
	ids := make([]string, 0, len(rep.Manifest.Resources))
	for _, res := range rep.Manifest.Resources {
		ids = append(ids, res.ID)
	}
	before := make([]string, 0, len(vd.Manifest.Resources))
	for _, res := range vd.Manifest.Resources {
		before = append(before, res.ID)
	}
	sort.Strings(ids)
	sort.Strings(before)
	require.Equal(t, before, ids)
	v2 := e.publish(admin, ns, "0.1.1")
	require.Equal(t, vd.ArtifactHash, v2.ArtifactHash)
	require.Equal(t, vd.SourceHash, v2.SourceHash)
	require.Equal(t, vd.TreeHash, v2.TreeHash)
	require.Equal(t, manage.ProvenanceRuntime, v2.Provenance, "after the archive round trip the provenance is the runtime's")
}
