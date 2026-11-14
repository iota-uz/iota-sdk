package mcptools_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/catalog"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pkgdocs"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/reference"
	"github.com/iota-uz/iota-sdk/pkg/mcp"
)

type noFiles struct{}

func (noFiles) ReadFile(context.Context, string, string) ([]byte, error) { return nil, nil }

// docsLibrary is a library over an empty catalog, for tests that only list tools.
func docsLibrary(t *testing.T) *pkgdocs.Library {
	t.Helper()
	lib, err := pkgdocs.NewLibrary(reference.New(catalog.New(nil, catalog.NewNativeSet(data.DefaultNatives()))), noFiles{})
	require.NoError(t, err)
	return lib
}

// ext_docs serves the documents a package ships, to the users who may read
// them, exactly as the Help Center does: a document behind a permission, or of
// a package the caller does not see, does not exist for them.
func TestDocsToolFollowsTheCallersVisibility(t *testing.T) {
	e := newEnv(t)
	const ns = "acme.userdocs"
	m := manifestOf(ns)
	// Without a permission the open entity opens the package to anyone; the
	// setup document asks for a native permission.
	m.Docs = []manifest.Doc{
		{Path: "getting-started", Locale: "en", Title: "Getting started", Order: 1},
		{Path: "getting-started", Locale: "ru", Title: "Начало работы", Order: 1},
		{Path: "admin/setup", Locale: "en", Title: "Setup", Order: 2, Permission: "Policy.Renew"},
	}
	raw, err := json.Marshal(m)
	require.NoError(t, err)

	e.ok(adminID, "ext_package_create", map[string]string{"namespace": ns}, nil)
	for p, c := range map[string]string{
		"package.json":               `{"name":"@acme/docs","version":"1.0.0","iota":{"namespace":"` + ns + `"}}`,
		"manifest.json":              string(raw),
		"server.mjs":                 "export const renew = async () => ({ ok: true });\n",
		"contracts.mjs":              "export {}\n",
		"docs/en/getting-started.md": "# Getting started\n\nSee [setup](admin/setup.md).\n",
		"docs/ru/getting-started.md": "# Начало работы\n",
		"docs/en/admin/setup.md":     "Setup text\n",
	} {
		e.ok(adminID, "ext_file_write", map[string]string{"namespace": ns, "path": p, "content": c}, nil)
	}
	e.ok(adminID, "ext_publish", map[string]string{"namespace": ns, "version": "1.0.0"}, nil)
	e.ok(adminID, "ext_review", map[string]any{"namespace": ns, "version": "1.0.0", "approve": true}, nil)
	e.ok(adminID, "ext_activate", map[string]string{"namespace": ns, "version": "1.0.0"}, nil)

	const strangerID = 60
	e.users[strangerID] = account(t, e.tenant, strangerID)

	type result struct {
		Namespace string
		Path      string
		Title     string
		Markdown  string
		Packages  []pkgdocs.Listing
	}
	paths := func(r result) []string {
		out := []string{}
		for _, l := range r.Packages {
			for _, d := range l.Docs {
				out = append(out, l.Namespace+"/"+d.Path)
			}
		}
		return out
	}

	// Extensions.Read: everything.
	var out result
	e.ok(readerID, "ext_docs", map[string]string{}, &out)
	require.Equal(t, []string{ns + "/getting-started", ns + "/admin/setup"}, paths(out))
	require.Contains(t, out.Markdown, "`getting-started`")
	e.ok(readerID, "ext_docs", map[string]string{"namespace": ns, "path": "admin/setup"}, &out)
	require.Equal(t, "Setup", out.Title)
	require.Equal(t, "Setup text\n", out.Markdown)

	// A user who may use the package, without the setup permission.
	e.ok(strangerID, "ext_docs", map[string]string{"namespace": ns}, &out)
	require.Equal(t, []string{ns + "/getting-started"}, paths(out))
	e.ok(strangerID, "ext_docs", map[string]string{"namespace": ns, "path": "getting-started", "locale": "ru"}, &out)
	require.Equal(t, "Начало работы", out.Title)
	e.ok(strangerID, "ext_docs", map[string]string{"namespace": ns, "path": "getting-started"}, &out)
	require.Contains(t, out.Markdown, "[setup](admin/setup)", "links are resolved for the Help Center")
	e.failed(strangerID, "ext_docs", map[string]string{"namespace": ns, "path": "admin/setup"}, mcp.CodeNotFound)
	e.failed(strangerID, "ext_docs", map[string]string{"namespace": "nope.nope", "path": "x"}, mcp.CodeNotFound)
	e.failed(strangerID, "ext_docs", map[string]string{"namespace": "nope.nope"}, mcp.CodeNotFound)
	e.failed(strangerID, "ext_docs", map[string]string{"path": "getting-started"}, mcp.CodeInvalidInput)
}
