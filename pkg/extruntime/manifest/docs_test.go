package manifest_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
)

func withDocs() manifest.Manifest {
	m := valid()
	m.Resources = append(m.Resources, manifest.Resource{ID: "acme.loyalty.manage", Kind: manifest.ResPermission})
	m.Docs = []manifest.Doc{
		{Path: "getting-started", Locale: "en", Title: "Getting started", Order: 1, Audience: "Cashiers"},
		{Path: "getting-started", Locale: "ru", Title: "Начало работы"},
		{Path: "admin/setup", Locale: "en", Title: "Setup", Permission: "acme.loyalty.manage"},
		{Path: "admin/limits", Locale: "en", Title: "Limits", Permission: "Client.Read"},
	}
	return m
}

func TestValidateAcceptsDocs(t *testing.T) {
	t.Parallel()
	m := withDocs()
	require.NoError(t, m.Validate())
	require.Len(t, m.DocsIn("en"), 3)
	require.Equal(t, "docs/ru/getting-started.md", manifest.DocFile(m.Docs[1]))
}

func TestValidateRefusesBadDocs(t *testing.T) {
	t.Parallel()
	const hostile = "<img src=x onerror=alert(1)>"
	for name, tc := range map[string]struct {
		mutate func(*manifest.Manifest)
		want   string
	}{
		"path with dots":        {func(m *manifest.Manifest) { m.Docs[0].Path = "../etc/passwd" }, "path"},
		"path with extension":   {func(m *manifest.Manifest) { m.Docs[0].Path = "start.md" }, "path"},
		"absolute path":         {func(m *manifest.Manifest) { m.Docs[0].Path = "/start" }, "path"},
		"upper case path":       {func(m *manifest.Manifest) { m.Docs[0].Path = "Start" }, "path"},
		"empty segment":         {func(m *manifest.Manifest) { m.Docs[0].Path = "a//b" }, "path"},
		"too deep":              {func(m *manifest.Manifest) { m.Docs[0].Path = "a/b/c/d/e/f" }, "path"},
		"long path":             {func(m *manifest.Manifest) { m.Docs[0].Path = strings.Repeat("a", 121) }, "path"},
		"locale":                {func(m *manifest.Manifest) { m.Docs[1].Locale = "RU" }, "locale"},
		"locale with a slash":   {func(m *manifest.Manifest) { m.Docs[1].Locale = "ru/x" }, "locale"},
		"duplicate":             {func(m *manifest.Manifest) { m.Docs = append(m.Docs, m.Docs[0]) }, "declared twice"},
		"empty title":           {func(m *manifest.Manifest) { m.Docs[0].Title = "" }, "title"},
		"markup in the title":   {func(m *manifest.Manifest) { m.Docs[0].Title = hostile }, "title"},
		"newline in the title":  {func(m *manifest.Manifest) { m.Docs[0].Title = "a\nb" }, "title"},
		"padded title":          {func(m *manifest.Manifest) { m.Docs[0].Title = " a" }, "title"},
		"long title":            {func(m *manifest.Manifest) { m.Docs[0].Title = strings.Repeat("я", 121) }, "title"},
		"markup in audience":    {func(m *manifest.Manifest) { m.Docs[0].Audience = hostile }, "audience"},
		"order":                 {func(m *manifest.Manifest) { m.Docs[0].Order = 10_001 }, "order"},
		"permission shape":      {func(m *manifest.Manifest) { m.Docs[2].Permission = hostile }, "permission"},
		"undeclared own":        {func(m *manifest.Manifest) { m.Docs[2].Permission = "acme.loyalty.nope" }, "does not declare"},
		"no default":            {func(m *manifest.Manifest) { m.Docs[0].Locale = "uz" }, "no en version"},
		"permission per locale": {func(m *manifest.Manifest) { m.Docs[1].Permission = "Client.Read" }, "different permissions"},
		"too many": {func(m *manifest.Manifest) {
			for i := range manifest.MaxDocs {
				m.Docs = append(m.Docs, manifest.Doc{Path: "p" + strings.Repeat("a", i%50) + string(rune('a'+i%26)) + strings.Repeat("b", i/50), Locale: "en", Title: "t"})
			}
		}, "documents, the limit"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			m := withDocs()
			tc.mutate(&m)
			require.ErrorContains(t, m.Validate(), tc.want)
		})
	}
}

func TestValidateDocFiles(t *testing.T) {
	t.Parallel()
	m := withDocs()
	files := map[string][]byte{
		"server.mjs":                 []byte("//"),
		"docs/en/getting-started.md": []byte("# Hi\n"),
		"docs/ru/getting-started.md": []byte("# Привет\n"),
		"docs/en/admin/setup.md":     []byte("Setup\n"),
		"docs/en/admin/limits.md":    []byte("Limits\n"),
		"client/manifest.json":       []byte("{}"),
	}
	require.NoError(t, manifest.ValidateDocFiles(m, files))

	bad := func(mutate func(map[string][]byte), want string) {
		t.Helper()
		f := map[string][]byte{}
		for k, v := range files {
			f[k] = v
		}
		mutate(f)
		require.ErrorContains(t, manifest.ValidateDocFiles(m, f), want)
	}
	bad(func(f map[string][]byte) { delete(f, "docs/ru/getting-started.md") }, "artifact has no docs/ru/getting-started.md")
	bad(func(f map[string][]byte) { f["docs/en/extra.md"] = []byte("x") }, "does not list")
	bad(func(f map[string][]byte) { f["docs/en/logo.png"] = []byte("x") }, "does not list")
	bad(func(f map[string][]byte) { f["docs/en/admin/setup.md"] = make([]byte, manifest.MaxDocBytes+1) }, "limit")
	bad(func(f map[string][]byte) { f["docs/en/admin/setup.md"] = []byte{0xff, 0xfe} }, "UTF-8")
	bad(func(f map[string][]byte) { f["docs/en/admin/setup.md"] = []byte("a\x00b") }, "UTF-8")

	none := valid()
	require.NoError(t, manifest.ValidateDocFiles(none, map[string][]byte{"server.mjs": nil}))
	require.ErrorContains(t, manifest.ValidateDocFiles(none, map[string][]byte{"docs/en/a.md": []byte("x")}), "does not list")
}

func TestDocPermissionShapes(t *testing.T) {
	t.Parallel()
	for _, p := range []string{"Client.Read", "Extensions.Read", "acme.loyalty.manage", "e2e.notes.write"} {
		require.True(t, manifest.ValidDocPermission(p), p)
	}
	for _, p := range []string{"", "Client", "Client.Read.All", "a b", "x.<y>"} {
		require.False(t, manifest.ValidDocPermission(p), p)
	}
	require.True(t, manifest.NativeDocPermission("Client.Read"))
	require.False(t, manifest.NativeDocPermission("acme.loyalty.manage"))
}
