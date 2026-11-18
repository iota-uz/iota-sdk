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

	// The guides and the HTTP API document are developer documentation: a
	// reader without Extensions.Read is not shown them, neither in the sidebar
	// nor by address.
	guidePath, apiPath := "/help/doc/extensions/guides/authoring", "/help/doc/extensions/api"
	require.Equal(t, http.StatusOK, e.get(admin, apiPath).Status)
	for name, who := range map[string]*actor{"a writer": member, "a reader without rights": nobody} {
		require.Equal(t, http.StatusNotFound, e.get(who, guidePath).Status, "guides are hidden from %s", name)
		require.Equal(t, http.StatusNotFound, e.get(who, "/help/doc/extensions/guides/consumer").Status, name)
		require.Equal(t, http.StatusNotFound, e.get(who, apiPath).Status, "the HTTP API document is hidden from %s", name)
	}

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
	require.NotContains(t, writer.String(), guidePath, "the writer's sidebar lists the reference only")
	require.NotContains(t, plain.String(), guidePath)
	require.Contains(t, full.String(), guidePath, "the administrator's sidebar lists the guides")
	require.NotContains(t, plain.String(), "/_ext/openapi.json", "the overview does not point a reader at documents they cannot open")

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

// A field the reader may not read is absent from the whole reference: the
// field list, the record's signature, schema and example, and the OpenAPI and
// management documents.
func TestScenario20HiddenFieldsAreAbsentFromTheReference(t *testing.T) {
	e := newEnv(t, withHelpCenter())
	const ns = "e2e.clientextra"
	admin := e.admin
	e.deploy(admin, ns, "1.0.0", loadTree(t, "clientextra"))
	refPath := "/help/doc/extensions/reference/" + ns

	// The overlay is visible to whoever may see crm.client; its internal note
	// needs one more permission.
	plain := e.newActor("client-reader", "Client.Read")
	auditor := e.newActor("auditor", "Client.Read", permViewIntern)

	seen := e.get(plain, refPath)
	require.Equal(t, http.StatusOK, seen.Status, seen.String())
	require.Contains(t, seen.String(), "tier", "the overlay's open field is documented")
	require.NotContains(t, seen.String(), "internalNote", "a hidden field is absent from the reader's reference, columns included")

	both := e.get(auditor, refPath)
	require.Equal(t, http.StatusOK, both.Status, both.String())
	require.Contains(t, both.String(), "internalNote", "with the permission the field is documented")
	require.Contains(t, e.get(admin, refPath).String(), "internalNote")

	// The configurator's model (Extensions.Read only) is the full one.
	ref := e.get(plain, "/_ext/manage/reference")
	require.Equal(t, http.StatusForbidden, ref.Status, "the configurator's reference needs Extensions.Read")
	require.NotContains(t, ref.String(), "internalNote")
	full := e.get(admin, "/_ext/manage/reference")
	require.Equal(t, http.StatusOK, full.Status, full.String())
	require.Contains(t, full.String(), "internalNote")
}
