package e2e_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Scenario 20: the documentation of the platform lives in the product. After a
// package is activated, the Help Center lists its live reference next to the
// SDK guides, filtered by what each reader may use, and the OpenAPI document
// describes the same contracts.
func TestScenario20ReferenceInTheHelpCenter(t *testing.T) {
	e := newEnv(t, withHelpCenter())
	const ns = "e2e.notes"
	admin := e.admin
	member := e.newActor("member", permNotesWrite)
	nobody := e.newActor("nobody")
	refPath := "/help/doc/extensions/reference/" + ns

	require.Equal(t, http.StatusNotFound, e.get(admin, refPath).Status, "no reference before activation")

	e.deploy(admin, ns, "0.1.0", loadTree(t, "notes"))

	// The Help Center tree lists the guides and the package's reference.
	index := e.get(admin, "/help/doc/extensions/guides/authoring")
	require.Equal(t, http.StatusOK, index.Status, index.String())
	require.Contains(t, index.String(), refPath, "sidebar links the package reference")

	// An administrator of extensions sees every contract of the package.
	full := e.get(admin, refPath)
	require.Equal(t, http.StatusOK, full.Status, full.String())
	for _, id := range []string{"e2e.notes.addNote", "e2e.notes.listNotes", "e2e.notes.note"} {
		require.Contains(t, full.String(), id)
	}

	// A writer sees the command they may run; a reader without rights only
	// what has no permission (the query and the row-filtered entity).
	writer := e.get(member, refPath)
	require.Equal(t, http.StatusOK, writer.Status, writer.String())
	require.Contains(t, writer.String(), "e2e.notes.addNote")
	plain := e.get(nobody, refPath)
	require.Equal(t, http.StatusOK, plain.Status, plain.String())
	require.Contains(t, plain.String(), "e2e.notes.listNotes")
	require.NotContains(t, plain.String(), "e2e.notes.addNote", "a command the reader may not run is not documented to them")

	// The OpenAPI document is filtered the same way and carries the contract's input schema.
	type openAPI struct {
		Paths map[string]json.RawMessage `json:"paths"`
	}
	var adminDoc, plainDoc openAPI
	a := e.get(admin, "/_ext/openapi.json")
	require.Equal(t, http.StatusOK, a.Status, a.String())
	require.NoError(t, json.Unmarshal(a.Body, &adminDoc))
	require.Contains(t, adminDoc.Paths, "/_ext/rpc")
	require.Contains(t, a.String(), `"e2e.notes.addNote"`, "the rpc request variants include the command")
	hasManage := false
	for p := range adminDoc.Paths {
		hasManage = hasManage || strings.HasPrefix(p, "/_ext/manage/")
	}
	require.True(t, hasManage, "the management API is documented to its readers")

	p := e.get(nobody, "/_ext/openapi.json")
	require.Equal(t, http.StatusOK, p.Status, p.String())
	require.NoError(t, json.Unmarshal(p.Body, &plainDoc))
	require.Contains(t, p.String(), `"e2e.notes.listNotes"`)
	require.NotContains(t, p.String(), `"e2e.notes.addNote"`, "no variant for a command the reader may not run")
	for path := range plainDoc.Paths {
		require.False(t, strings.HasPrefix(path, "/_ext/manage/"), "management API hidden from %s", path)
	}
}
