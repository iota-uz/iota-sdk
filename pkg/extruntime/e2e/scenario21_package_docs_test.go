package e2e_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// Scenario 21: a package ships its own end-user documentation. The Help Center
// lists it under "apps" for the people who can use the package, a document
// behind a permission only for its holders, and activating a new version
// replaces the content (documents the new version dropped disappear).
func TestScenario21PackageDocumentationInTheHelpCenter(t *testing.T) {
	e := newEnv(t, withHelpCenter())
	const ns = "e2e.guide"
	admin := e.admin
	user := e.newActor("guide-user", "Client.Read")
	auditor := e.newActor("guide-auditor", "Client.Read", "Client.Update")
	outsider := e.newActor("outsider")
	doc := func(p string) string { return "/help/doc/apps/" + ns + "/" + p }

	require.Equal(t, http.StatusNotFound, e.get(admin, doc("getting-started")).Status, "no documentation before activation")

	tr := loadTree(t, "guide")
	e.deploy(admin, ns, "1.0.0", tr)

	// The user of the package reads the open documents, in the end-user area.
	start := e.get(user, doc("getting-started"))
	require.Equal(t, http.StatusOK, start.Status, start.String())
	require.Contains(t, start.String(), "Version one text.")
	require.Contains(t, start.String(), "Getting started")
	require.Contains(t, start.String(), `href="`+doc("limits")+`"`, "a relative link to another document is resolved against the document")
	require.NotContains(t, start.String(), "limits.md", "links lose the source file extension")
	require.NotContains(t, start.String(), "setup.md")
	limits := e.get(user, doc("limits"))
	require.Equal(t, http.StatusOK, limits.Status, limits.String())
	require.Contains(t, limits.String(), "Limits of version one.")
	require.Contains(t, limits.String(), doc("getting-started"), "the sidebar lists the package's documents")
	require.NotContains(t, limits.String(), doc("admin/setup"), "a document behind a permission is not listed to readers without it")
	require.NotContains(t, limits.String(), "/help/doc/extensions/guides", "the developer area stays hidden from end users")

	// The document behind a permission: forbidden is absent.
	require.Equal(t, http.StatusNotFound, e.get(user, doc("admin/setup")).Status)
	setup := e.get(auditor, doc("admin/setup"))
	require.Equal(t, http.StatusOK, setup.Status, setup.String())
	require.Contains(t, setup.String(), "Setup, for administrators only.")
	require.Contains(t, e.get(auditor, doc("limits")).String(), doc("admin/setup"), "listed for its holders")

	// Someone who cannot use the package sees none of it.
	for _, p := range []string{"getting-started", "limits", "admin/setup"} {
		require.Equal(t, http.StatusNotFound, e.get(outsider, doc(p)).Status, p)
	}
	// An administrator of extensions sees everything, and the developer area is still theirs.
	require.Equal(t, http.StatusOK, e.get(admin, doc("admin/setup")).Status)
	require.Contains(t, e.get(admin, doc("limits")).String(), "/help/doc/extensions/guides/authoring")
	// A document that does not exist is not found, and the mount holds no
	// file of the Help Center's own tree.
	require.Equal(t, http.StatusNotFound, e.get(user, doc("nope")).Status)
	require.Equal(t, http.StatusNotFound, e.get(user, "/help/doc/apps/nope.nope/getting-started").Status)

	// Version 2 is activated: new text, a new document, and the one it dropped is gone.
	// (The build refuses a version whose documents still link the one it drops.)
	v2 := tr.with("docs/en/getting-started.md", "Version one text.", "Version two text.").
		with("docs/en/getting-started.md", ", or the [setup](admin/setup.md) if you administer it", "")
	v2["docs/en/news.md"] = "---\ntitle: What is new\n---\n\nNews of version two.\n"
	delete(v2, "docs/en/admin/setup.md")
	e.deploy(admin, ns, "2.0.0", v2)
	require.Contains(t, e.get(user, doc("getting-started")).String(), "Version two text.")
	require.NotContains(t, e.get(user, doc("getting-started")).String(), "Version one text.")
	require.Equal(t, http.StatusOK, e.get(user, doc("news")).Status)
	require.Equal(t, http.StatusNotFound, e.get(auditor, doc("admin/setup")).Status, "the active version decides what exists")
	require.Contains(t, e.get(user, doc("limits")).String(), doc("news"), "the sidebar follows the activation")

	// Deactivating the package takes its documentation with it.
	e.mgmt(admin, http.MethodPost, "/packages/"+ns+"/deactivate", nil, http.StatusNoContent, nil)
	require.Equal(t, http.StatusNotFound, e.get(user, doc("getting-started")).Status)
}
