package build_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/build"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
)

// withDocs is the acme package plus end-user documentation.
func withDocs(t *testing.T) build.Tree {
	t.Helper()
	tree := clone(loadTree(t, "acme"))
	tree["docs/en/getting-started.md"] = []byte("---\ntitle: Getting started\norder: 1\naudience: Brokers\n---\n\n# Welcome\n\nOpen the [policies](route:acme.insurance.policies) or read [the setup](admin/setup.md#limits).\n")
	tree["docs/ru/getting-started.md"] = []byte("# Начало работы\n\nОткройте [полисы](route:acme.insurance.policies).\n")
	tree["docs/en/admin/setup.md"] = []byte("---\ntitle: Setup\npermission: acme.insurance.readPolicies\n---\nBack to [start](../getting-started.md).\n")
	return tree
}

func TestDocsAreInTheManifestAndTheArtifact(t *testing.T) {
	b := newBuilder(t)
	kit := nativeKit(t, b)
	res, err := b.Build(context.Background(), withDocs(t), kit)
	require.NoError(t, err)
	require.Empty(t, res.Diagnostics)
	require.NoError(t, res.Manifest.Validate())

	require.Equal(t, []manifest.Doc{
		{Path: "admin/setup", Locale: "en", Title: "Setup", Permission: "acme.insurance.readPolicies"},
		{Path: "getting-started", Locale: "en", Title: "Getting started", Order: 1, Audience: "Brokers"},
		{Path: "getting-started", Locale: "ru", Title: "Начало работы"},
	}, res.Manifest.Docs)
	require.NoError(t, manifest.ValidateDocFiles(res.Manifest, res.Artifacts))

	// The artifact text has no front matter; the source keeps it.
	text := string(res.Artifacts["docs/en/getting-started.md"])
	require.True(t, strings.HasPrefix(text, "# Welcome"), text)
	require.NotContains(t, text, "title:")

	// The documentation is covered by both hashes, so a reviewer approves it
	// with the code: changing a word changes the version.
	edited := withDocs(t)
	replace(t, edited, "docs/en/admin/setup.md", "Back to", "Return to")
	res2, err := b.Build(context.Background(), edited, kit)
	require.NoError(t, err)
	require.NotEqual(t, res.SourceHash, res2.SourceHash)
	require.NotEqual(t, res.ArtifactHash, res2.ArtifactHash)

	// A package without docs/ is unchanged: no docs in the manifest.
	plain, err := b.Build(context.Background(), loadTree(t, "acme"), kit)
	require.NoError(t, err)
	require.Empty(t, plain.Manifest.Docs)
	for p := range plain.Artifacts {
		require.False(t, strings.HasPrefix(p, "docs/"), p)
	}
}

func TestDocsDiagnosticsPointAtTheAuthorsFiles(t *testing.T) {
	b := newBuilder(t)
	kit := nativeKit(t, b)

	tree := withDocs(t)
	tree["docs/en/broken.md"] = []byte("---\ntitle: Broken\ncolour: red\n---\n\nIntro\n\nSee [nowhere](missing.md) and [the page](route:acme.insurance.nope).\n")
	tree["docs/en/logo.png"] = []byte("png")
	tree["docs/ru/only.md"] = []byte("Только по-русски\n")
	replace(t, tree, "docs/en/admin/setup.md", "permission: acme.insurance.readPolicies", "permission: acme.insurance.typo")
	tree["src/help.tsx"] = []byte("import { helpHref } from \"@iota-uz/sdk/runtime/client\"\nexport const link = () => helpHref(\"getting-started\")\nexport const bad = () => helpHref(\"gettin-started\")\n")
	replace(t, tree, "src/client.tsx", "export const policies", "export { link, bad } from \"./help\"\nexport const policies")

	_, err := b.Build(context.Background(), tree, kit)
	var failed *build.FailedError
	require.ErrorAs(t, err, &failed)
	got := map[string]build.Diagnostic{}
	for _, d := range failed.Diagnostics {
		got[d.Code+" "+d.File] = d
	}
	at := func(code, file string, line int) {
		t.Helper()
		d, ok := got[code+" "+file]
		require.True(t, ok, "%s %s in %+v", code, file, failed.Diagnostics)
		require.Equal(t, build.SeverityError, d.Severity)
		require.Equal(t, line, d.Line, "%s %s: %s", code, file, d.Message)
	}
	at("docs.frontMatter", "docs/en/broken.md", 3)
	at("docs.unknownPermission", "docs/en/admin/setup.md", 3)
	at("docs.unsupportedFile", "docs/en/logo.png", 0)
	at("docs.missingDefault", "docs/ru/only.md", 1)
	require.Contains(t, got["docs.unknownPermission docs/en/admin/setup.md"].Message, "acme.insurance.readPolicies", "lists what the package declares")
	// Both broken links of broken.md, on their line, with a column.
	var links []build.Diagnostic
	for _, d := range failed.Diagnostics {
		if d.File == "docs/en/broken.md" && (d.Code == "docs.brokenLink" || d.Code == "docs.unknownRoute") {
			links = append(links, d)
		}
	}
	require.Len(t, links, 2)
	for _, d := range links {
		require.Equal(t, 8, d.Line, d.Message)
		require.Positive(t, d.Column)
	}
	d := got["docs.unknownHelpHref src/help.tsx"]
	require.Equal(t, 3, d.Line)
	require.Contains(t, d.Message, "getting-started")
	require.Len(t, failed.Diagnostics, 7, "%+v", failed.Diagnostics)

	// Check reports the same without producing artifacts.
	ds, err := b.Check(context.Background(), tree, kit)
	require.NoError(t, err)
	require.Len(t, ds, len(failed.Diagnostics))
}
