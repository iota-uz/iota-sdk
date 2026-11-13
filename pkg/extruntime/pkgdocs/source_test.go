package pkgdocs_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/catalog"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pkgdocs"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/reference"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/helpdocs"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// world is a catalog and an artifact store the test can change between
// requests, the way activation does.
type world struct {
	mu     sync.Mutex
	active []catalog.ActiveVersion
	files  map[string]string // artifactHash + "/" + path -> text
	reads  int
}

func (w *world) set(active ...catalog.ActiveVersion) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.active = active
}

func (w *world) VisiblePackages(_ context.Context, sub spi.Subject) ([]catalog.ActiveVersion, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	var out []catalog.ActiveVersion
	for _, av := range w.active {
		if reference.PackageVisible(sub, av.Manifest, func(string) (manifest.Resource, bool) { return manifest.Resource{}, false }) {
			out = append(out, av)
		}
	}
	return out, nil
}

func (w *world) ReadFile(_ context.Context, hash, path string) ([]byte, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.reads++
	text, ok := w.files[hash+"/"+path]
	if !ok {
		return nil, serrors.New(serrors.NotFound, "no such file")
	}
	return []byte(text), nil
}

func openResource(ns string) manifest.Resource {
	return manifest.Resource{ID: ns + ".guide", Kind: manifest.ResPermission}
}

func loyalty(version, hash string) catalog.ActiveVersion {
	return catalog.ActiveVersion{
		VersionID: version, Namespace: "acme.loyalty", Version: version, ArtifactHash: hash,
		Manifest: manifest.Manifest{
			Schema: manifest.SchemaVersion, Namespace: "acme.loyalty",
			Resources: []manifest.Resource{
				openResource("acme.loyalty"),
				{ID: "acme.loyalty.manage", Kind: manifest.ResPermission},
				{ID: "acme.loyalty.cards", Kind: manifest.ResPage, Path: "/loyalty/cards"},
				{ID: "acme.loyalty.byId", Kind: manifest.ResRoute, Path: "/loyalty/cards/:id"},
			},
			Docs: []manifest.Doc{
				{Path: "getting-started", Locale: "en", Title: "Getting started", Order: 2},
				{Path: "getting-started", Locale: "ru", Title: "Начало работы", Order: 2},
				{Path: "admin/setup", Locale: "en", Title: "Setup", Order: 1, Audience: "Managers", Permission: "acme.loyalty.manage"},
				{Path: "faq", Locale: "en", Title: "FAQ", Order: 2},
			},
		},
	}
}

func vault() catalog.ActiveVersion {
	return catalog.ActiveVersion{
		VersionID: "v", Namespace: "acme.vault", Version: "1.0.0", ArtifactHash: "vault",
		Manifest: manifest.Manifest{
			Schema: manifest.SchemaVersion, Namespace: "acme.vault",
			Resources: []manifest.Resource{{ID: "acme.vault.open", Kind: manifest.ResCommand, Permission: "Vault.Use"}},
			Docs:      []manifest.Doc{{Path: "intro", Locale: "en", Title: "Vault intro"}},
		},
	}
}

func newWorld() *world {
	return &world{files: map[string]string{
		"h1/docs/en/getting-started.md": "# Start\n\nSee [setup](admin/setup.md#limits), [faq](./faq.md), [cards](route:acme.loyalty.cards), [by id](route:acme.loyalty.byId), [web](https://example.com).\n",
		"h1/docs/ru/getting-started.md": "# Начало\n",
		"h1/docs/en/admin/setup.md":     "Setup text\n",
		"h1/docs/en/faq.md":             "FAQ v1\n",
		"h2/docs/en/getting-started.md": "# Start v2\n",
		"h2/docs/en/faq.md":             "FAQ v2\n",
		"vault/docs/en/intro.md":        "Vault\n",
	}}
}

func source(t *testing.T, w *world, who *spi.Subject) helpdocs.Source {
	t.Helper()
	src, err := pkgdocs.NewSource(pkgdocs.Config{Packages: w, Files: w, Subject: func(context.Context) (spi.Subject, error) {
		if who == nil {
			return spi.Subject{}, errors.New("no user")
		}
		return *who, nil
	}})
	require.NoError(t, err)
	return src
}

func reader(perms ...string) *spi.Subject {
	return &spi.Subject{UserID: 7, Permissions: perms, Locale: "en"}
}

func leaves(nodes []helpdocs.Node) []string {
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		if len(n.Children) > 0 {
			out = append(out, leaves(n.Children)...)
			continue
		}
		out = append(out, n.Path+"="+n.Title)
	}
	return out
}

func TestMountIsTheEndUserArea(t *testing.T) {
	t.Parallel()
	src := source(t, newWorld(), reader())
	require.Equal(t, "apps", src.Mount())
	require.NotEqual(t, reference.Mount, src.Mount())
	require.Equal(t, "Applications", src.Title("en"))
	require.Equal(t, "Приложения", src.Title("ru-RU"))
	require.Equal(t, "Ilovalar", src.Title("uz"))
	require.Equal(t, "Applications", src.Title("fr"))
}

func TestTreeIsOneCategoryPerVisiblePackageInOrder(t *testing.T) {
	t.Parallel()
	w := newWorld()
	w.set(loyalty("1.0.0", "h1"), vault())
	ctx := context.Background()

	tree, err := source(t, w, reader()).Tree(ctx, "en")
	require.NoError(t, err)
	require.Len(t, tree, 1, "the vault has no resource this reader sees: no category")
	require.Equal(t, "acme.loyalty", tree[0].Title)
	require.Equal(t, "acme.loyalty", tree[0].Path)
	require.Equal(t, []string{"acme.loyalty/faq=FAQ", "acme.loyalty/getting-started=Getting started"}, leaves(tree), "the permission-gated document is not listed; equal orders sort by title")

	// A holder of the document's permission gets it, in order.
	tree, err = source(t, w, reader("acme.loyalty.manage")).Tree(ctx, "en")
	require.NoError(t, err)
	require.Equal(t, []string{"acme.loyalty/admin/setup=Setup", "acme.loyalty/faq=FAQ", "acme.loyalty/getting-started=Getting started"}, leaves(tree))
	require.Equal(t, 1, tree[0].Children[0].Order)

	// Extensions.Read sees every package and every document.
	tree, err = source(t, w, reader(reference.PermRead)).Tree(ctx, "en")
	require.NoError(t, err)
	require.Len(t, tree, 2)
	require.ElementsMatch(t, []string{
		"acme.loyalty/admin/setup=Setup", "acme.loyalty/getting-started=Getting started", "acme.loyalty/faq=FAQ", "acme.vault/intro=Vault intro",
	}, leaves(tree))

	// Using the vault shows its category.
	tree, err = source(t, w, reader("Vault.Use")).Tree(ctx, "en")
	require.NoError(t, err)
	require.Contains(t, leaves(tree), "acme.vault/intro=Vault intro")
}

func TestUnknownReaderSeesNothing(t *testing.T) {
	t.Parallel()
	w := newWorld()
	w.set(loyalty("1.0.0", "h1"))
	src := source(t, w, nil)
	tree, err := src.Tree(context.Background(), "en")
	require.NoError(t, err)
	require.Empty(t, tree)
	_, err = src.Get(context.Background(), "en", "acme.loyalty/faq")
	require.ErrorIs(t, err, helpdocs.ErrNotFound)
}

func TestGetServesTheDocumentAndHidesWhatTheReaderMayNotRead(t *testing.T) {
	t.Parallel()
	w := newWorld()
	w.set(loyalty("1.0.0", "h1"), vault())
	ctx := context.Background()
	user := source(t, w, reader())

	doc, err := user.Get(ctx, "en", "acme.loyalty/getting-started")
	require.NoError(t, err)
	require.Equal(t, "Getting started", doc.Title)
	require.Equal(t, "acme.loyalty/getting-started", doc.Path)
	text := string(doc.Markdown)
	require.Contains(t, text, "[setup](admin/setup#limits)", "document links lose .md, the fragment stays")
	require.Contains(t, text, "[faq](./faq)")
	require.Contains(t, text, "[cards](/loyalty/cards)", "route: becomes the route's path")
	require.Contains(t, text, "[by id](#)", "a route with parameters has no address")
	require.Contains(t, text, "[web](https://example.com)")

	// Forbidden is absent: a document behind a permission, a package the
	// reader cannot see, a document that does not exist and malformed paths
	// answer alike.
	for _, p := range []string{
		"acme.loyalty/admin/setup", "acme.vault/intro", "acme.loyalty/missing", "acme.loyalty", "acme.loyalty/", "acme.loyalty/../x",
		"acme.loyalty/faq.md", "acme.loyalty/FAQ", "nope.nope/faq", "/", "",
	} {
		_, err := user.Get(ctx, "en", p)
		require.ErrorIs(t, err, helpdocs.ErrNotFound, p)
	}

	doc, err = source(t, w, reader("acme.loyalty.manage")).Get(ctx, "en", "acme.loyalty/admin/setup")
	require.NoError(t, err)
	require.Equal(t, "Managers", doc.Audience)
	require.Equal(t, "Setup text\n", string(doc.Markdown))

	// Extensions.Read opens everything.
	doc, err = source(t, w, reader(reference.PermRead)).Get(ctx, "en", "acme.vault/intro")
	require.NoError(t, err)
	require.Equal(t, "Vault\n", string(doc.Markdown))
}

func TestLocaleFallsBackToTheDefaultLocale(t *testing.T) {
	t.Parallel()
	w := newWorld()
	w.set(loyalty("1.0.0", "h1"))
	src := source(t, w, reader())
	ctx := context.Background()
	for locale, want := range map[string]string{
		"en": "Getting started", "ru": "Начало работы", "RU": "Начало работы", "ru-RU": "Начало работы", "ru_RU": "Начало работы",
		"uz": "Getting started", "": "Getting started", "pt-BR": "Getting started",
	} {
		doc, err := src.Get(ctx, locale, "acme.loyalty/getting-started")
		require.NoError(t, err, locale)
		require.Equal(t, want, doc.Title, locale)
	}
	// A document with no Russian version is the English one in Russian.
	doc, err := src.Get(ctx, "ru", "acme.loyalty/faq")
	require.NoError(t, err)
	require.Equal(t, "FAQ", doc.Title)
	require.Equal(t, "FAQ v1\n", string(doc.Markdown))
	tree, err := src.Tree(ctx, "ru")
	require.NoError(t, err)
	require.Equal(t, []string{"acme.loyalty/faq=FAQ", "acme.loyalty/getting-started=Начало работы"}, leaves(tree))
}

func TestOnlyTheActiveVersionIsServedAndItChangesWithActivation(t *testing.T) {
	t.Parallel()
	w := newWorld()
	src := source(t, w, reader())
	ctx := context.Background()

	_, err := src.Get(ctx, "en", "acme.loyalty/faq")
	require.ErrorIs(t, err, helpdocs.ErrNotFound, "no active version, no documents")
	tree, err := src.Tree(ctx, "en")
	require.NoError(t, err)
	require.Empty(t, tree)

	w.set(loyalty("1.0.0", "h1"))
	doc, err := src.Get(ctx, "en", "acme.loyalty/faq")
	require.NoError(t, err)
	require.Equal(t, "FAQ v1\n", string(doc.Markdown))

	// Version 2 is activated: new text, and the documents it dropped are gone.
	v2 := loyalty("2.0.0", "h2")
	v2.Manifest.Docs = []manifest.Doc{
		{Path: "getting-started", Locale: "en", Title: "Start here"},
		{Path: "faq", Locale: "en", Title: "Questions"},
	}
	w.set(v2)
	doc, err = src.Get(ctx, "en", "acme.loyalty/faq")
	require.NoError(t, err)
	require.Equal(t, "FAQ v2\n", string(doc.Markdown))
	require.Equal(t, "Questions", doc.Title)
	_, err = src.Get(ctx, "en", "acme.loyalty/admin/setup")
	require.ErrorIs(t, err, helpdocs.ErrNotFound)
	tree, err = src.Tree(ctx, "en")
	require.NoError(t, err)
	require.Equal(t, []string{"acme.loyalty/faq=Questions", "acme.loyalty/getting-started=Start here"}, leaves(tree))

	// Deactivation removes the documentation.
	w.set()
	_, err = src.Get(ctx, "en", "acme.loyalty/faq")
	require.ErrorIs(t, err, helpdocs.ErrNotFound)
}

func TestTitlesStayPlainText(t *testing.T) {
	t.Parallel()
	w := newWorld()
	av := loyalty("1.0.0", "h1")
	// A manifest that bypassed validation.
	av.Manifest.Docs = []manifest.Doc{{Path: "faq", Locale: "en", Title: "<img src=x onerror=alert(1)>\nFAQ", Audience: "<b>all</b>"}}
	w.set(av)
	src := source(t, w, reader())
	tree, err := src.Tree(context.Background(), "en")
	require.NoError(t, err)
	require.Equal(t, []string{"acme.loyalty/faq=img src=x onerror=alert(1) FAQ"}, leaves(tree))
	doc, err := src.Get(context.Background(), "en", "acme.loyalty/faq")
	require.NoError(t, err)
	require.NotContains(t, doc.Title+doc.Audience, "<")
	require.Equal(t, "ball/b", doc.Audience)
}

func TestAMissingArtifactFileIsNotFound(t *testing.T) {
	t.Parallel()
	w := newWorld()
	delete(w.files, "h1/docs/en/faq.md")
	w.set(loyalty("1.0.0", "h1"))
	_, err := source(t, w, reader()).Get(context.Background(), "en", "acme.loyalty/faq")
	require.ErrorIs(t, err, helpdocs.ErrNotFound)
}

func TestTextsAreReadOnce(t *testing.T) {
	t.Parallel()
	w := newWorld()
	w.set(loyalty("1.0.0", "h1"))
	src := source(t, w, reader())
	for range 3 {
		_, err := src.Get(context.Background(), "en", "acme.loyalty/faq")
		require.NoError(t, err)
	}
	require.Equal(t, 1, w.reads, "artifacts are immutable: the text is cached by hash and file")
}

func TestLibraryListsForTools(t *testing.T) {
	t.Parallel()
	w := newWorld()
	w.set(loyalty("1.0.0", "h1"), vault())
	lib, err := pkgdocs.NewLibrary(w, w)
	require.NoError(t, err)
	got, err := lib.List(context.Background(), *reader("acme.loyalty.manage"), "en")
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "acme.loyalty", got[0].Namespace)
	paths := make([]string, 0, len(got[0].Docs))
	for _, d := range got[0].Docs {
		paths = append(paths, d.Path)
	}
	require.Equal(t, []string{"admin/setup", "faq", "getting-started"}, paths, "ordered by order, then title")
	page, err := lib.Read(context.Background(), *reader("acme.loyalty.manage"), "en", "acme.loyalty", "admin/setup")
	require.NoError(t, err)
	require.Equal(t, "Setup", page.Title)
	_, err = lib.Read(context.Background(), *reader(), "en", "acme.loyalty", "admin/setup")
	require.ErrorIs(t, err, helpdocs.ErrNotFound)
	_, err = pkgdocs.NewLibrary(nil, w)
	require.Error(t, err)
}

func TestRewriteLinksLeavesCodeAndOtherLinksAlone(t *testing.T) {
	t.Parallel()
	m := manifest.Manifest{Resources: []manifest.Resource{{ID: "a.b.page", Kind: manifest.ResPage, Path: "/a/b"}, {ID: "a.b.evil", Kind: manifest.ResRoute, Path: "javascript:alert(1)"}}}
	in := strings.Join([]string{
		"[a](x.md) ![img](y.md) [abs](/x.md) [anchor](#x.md) [url](https://e.com/x.md) [sub](../up/z.md#h) [angle](<w v.md>)",
		"`[code](x.md)` and [real](x.md)",
		"```",
		"[fenced](x.md)",
		"```",
		"[ref]: other.md",
		"[evil](route:a.b.evil) [unknown](route:a.b.none) [ok](route:a.b.page#top) [js](javascript:alert(1))",
		"",
	}, "\n")
	want := strings.Join([]string{
		"[a](x) ![img](y.md) [abs](/x.md) [anchor](#x.md) [url](https://e.com/x.md) [sub](../up/z#h) [angle](<w v>)",
		"`[code](x.md)` and [real](x)",
		"```",
		"[fenced](x.md)",
		"```",
		"[ref]: other",
		"[evil](#) [unknown](#) [ok](/a/b#top) [js](javascript:alert(1))",
		"",
	}, "\n")
	require.Equal(t, want, string(pkgdocs.RewriteLinks([]byte(in), m)))
}
