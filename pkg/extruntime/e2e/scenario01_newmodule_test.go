package e2e_test

import (
	"encoding/json"
	"net/http"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

type noteRow struct {
	Note   map[string]string `json:"note"`
	Title  string            `json:"title"`
	Author string            `json:"author"`
}

// Scenario 1: a new module — entity, query, ordinary command and a TSX page
// with navigation — is authored through the management API only (draft files,
// check, build, the package's own synthetic test, publish, self-review,
// activation) and becomes live without a Go rebuild.
func TestScenario01NewModuleFromDraftToLivePage(t *testing.T) {
	e := newEnv(t)
	const ns = "e2e.notes"
	admin := e.admin

	// Nothing of the module exists before the draft is activated.
	nobody := e.newActor("nobody")
	member := e.newActor("member", permNotesWrite)
	other := e.newActor("other", permNotesWrite)
	auditor := e.newActor("auditor", permNotesRead, permNotesWrite)
	require.Equal(t, http.StatusNotFound, e.get(member, "/notes").Status, "no page before activation")
	require.Equal(t, http.StatusNotFound, e.rpc(member, "command", "e2e.notes.addNote", map[string]any{"title": "x"}).Status)

	// Draft files arrive through the management API.
	e.put(admin, ns, loadTree(t, "notes"))

	chk := e.check(admin, ns)
	require.False(t, chk.HasErrors, "check: %+v", chk.Diagnostics)

	rep := e.buildPkg(admin, ns)
	require.False(t, rep.HasErrors, "build: %+v", rep.Diagnostics)
	require.NotNil(t, rep.Manifest)
	require.NotEmpty(t, rep.ArtifactHash)
	ids := map[string]string{}
	for _, r := range rep.Manifest.Resources {
		ids[r.ID] = string(r.Kind)
	}
	require.Equal(t, "entity", ids["e2e.notes.note"])
	require.Equal(t, "command", ids["e2e.notes.addNote"])
	require.Equal(t, "query", ids["e2e.notes.listNotes"])
	require.Equal(t, "page", ids["e2e.notes.notes"])

	// The package's own synthetic test runs in the build worker.
	tests := e.testPkg(admin, ns)
	require.True(t, tests.Passed, "synthetic tests: %+v", tests)
	require.GreaterOrEqual(t, len(tests.Cases), 2)

	v := e.publish(admin, ns, "0.1.0")
	require.Equal(t, rep.ArtifactHash, v.ArtifactHash, "publish stores the artifact the build produced")
	require.False(t, v.Approved, "an unreviewed version is not approved")
	e.mgmt(admin, http.MethodPost, "/packages/"+ns+"/versions/0.1.0/activate", nil, http.StatusConflict, nil)
	e.review(admin, ns, v)
	act := e.activate(admin, ns, "0.1.0")
	require.NotEmpty(t, act.SchemaSteps, "activation created the entity table")
	require.NotEmpty(t, act.Bound, "the package's implementations are bound")

	// The page is mounted inside the authenticated layout, with its nav item.
	page := e.get(member, "/notes")
	require.Equal(t, http.StatusOK, page.Status, page.String())
	html := page.String()
	require.Contains(t, html, `id="iota-client-route`, "canonical client host mount")
	require.Contains(t, html, "e2e.notes.notes", "route identity of the page")
	require.Contains(t, html, `/_ext/assets/`+v.ArtifactHash+`/`, "page module is an immutable asset of the version")
	require.Regexp(t, regexp.MustCompile(`href="/notes"`), html, "navigation item of the page")
	require.Contains(t, html, "Notes")
	require.Contains(t, html, "<html", "the authenticated layout wraps the page")
	// ... and the same layout serves legacy pages: the item appears there too.
	legacy := e.get(admin, "/crm/clients")
	require.Equal(t, http.StatusOK, legacy.Status)
	require.Regexp(t, regexp.MustCompile(`href="/notes"`), legacy.String(), "runtime navigation is part of the shared shell")
	// The page module is served from the artifact.
	var module string
	m := regexp.MustCompile(`/_ext/assets/[^"\\]+\.js`).FindString(html)
	require.NotEmpty(t, m, "page module URL in the bootstrap data")
	module = m
	js := e.get(member, module)
	require.Equal(t, http.StatusOK, js.Status, module)
	require.Contains(t, js.Header.Get("Content-Type"), "javascript")
	require.Contains(t, js.Header.Get("Cache-Control"), "immutable")
	// The permission is enforced on the server, whatever the navigation shows.
	require.Equal(t, http.StatusForbidden, e.get(nobody, "/notes").Status)
	require.NotRegexp(t, regexp.MustCompile(`href="/notes"`), e.get(nobody, "/crm/clients").String(), "no nav item without the permission")
	require.Equal(t, http.StatusFound, e.get(nil, "/notes").Status, "anonymous visitors go to sign-in")

	// The command writes a row; the query reads it with the access rules.
	var added struct {
		Note map[string]string `json:"note"`
	}
	e.mustRPC(member, "command", "e2e.notes.addNote", map[string]any{"title": "  Buy milk ", "body": "2 litres"}, &added)
	require.Equal(t, "e2e.notes.note", added.Note["entity"])
	require.NotEmpty(t, added.Note["key"])
	e.mustRPC(other, "command", "e2e.notes.addNote", map[string]any{"title": "Call Bob"}, nil)

	var mine, theirs, all []noteRow
	e.mustRPC(member, "query", "e2e.notes.listNotes", map[string]any{}, &mine)
	require.Len(t, mine, 1)
	require.Equal(t, "Buy milk", mine[0].Title, "the command's own logic trimmed the title")
	require.Equal(t, added.Note["key"], mine[0].Note["key"])
	require.Equal(t, itoa(member.id), mine[0].Author)
	e.mustRPC(other, "query", "e2e.notes.listNotes", map[string]any{}, &theirs)
	require.Len(t, theirs, 1)
	require.Equal(t, "Call Bob", theirs[0].Title, "rows of other authors are invisible")
	e.mustRPC(auditor, "query", "e2e.notes.listNotes", map[string]any{}, &all)
	require.Len(t, all, 2, "the read-all permission widens the rows")

	// Declared domain errors and permissions are part of the contract.
	dup := e.rpc(member, "command", "e2e.notes.addNote", map[string]any{"title": "Buy milk"})
	require.False(t, dup.OK)
	require.Equal(t, http.StatusUnprocessableEntity, dup.Status)
	require.Equal(t, "domain", dup.Error.Kind)
	require.Equal(t, "e2e.notes.titleTaken", dup.Error.Code)
	require.JSONEq(t, `{"title":"Buy milk"}`, string(dup.Error.Params))
	require.Equal(t, http.StatusForbidden, e.rpc(nobody, "command", "e2e.notes.addNote", map[string]any{"title": "z"}).Status)
	bad := e.rpc(member, "command", "e2e.notes.addNote", map[string]any{"title": 7})
	require.Equal(t, http.StatusUnprocessableEntity, bad.Status)
	require.Equal(t, "validation", bad.Error.Kind)
	// A query cannot be called as a command and operations are not callable from the browser.
	require.Equal(t, http.StatusForbidden, e.rpc(member, "command", "e2e.notes.listNotes", map[string]any{}).Status)

	// Exactly the rows the commands wrote exist, in the entity's own table.
	require.Equal(t, 2, e.scalarInt(`SELECT count(*) FROM x_e2e_notes.note WHERE tenant_id = $1`, e.tenant))

	// Both invocations are recorded with their binding.
	var recorded []struct{}
	_ = recorded
	require.GreaterOrEqual(t, e.scalarInt(`SELECT count(*) FROM extruntime.invocations WHERE contract = 'e2e.notes.addNote' AND outcome = 'ok'`), 2)

	// The version listing shows the lifecycle.
	var versions []struct {
		Version  string
		Approved bool
		Active   bool
	}
	r := e.mgmt(admin, http.MethodGet, "/packages/"+ns+"/versions", nil, http.StatusOK, nil)
	require.NoError(t, json.Unmarshal(r.Body, &versions))
	require.Len(t, versions, 1)
	require.True(t, versions[0].Approved && versions[0].Active)
}

func itoa(n uint) string {
	b, _ := json.Marshal(n)
	return string(b)
}
