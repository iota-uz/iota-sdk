package services

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/helpcenter/internal/fakesource"
	"github.com/iota-uz/iota-sdk/modules/helpcenter/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/helpdocs"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

func fileContent() fstest.MapFS {
	return fstest.MapFS{
		"content/en/getting-started/welcome.md": &fstest.MapFile{Data: []byte("# Welcome\n\nHello.")},
		"content/en/modules/crm.md":             &fstest.MapFile{Data: []byte("# CRM\n\nLeads.")},
	}
}

func fileConfig() ContentConfig {
	return ContentConfig{
		FS:            fileContent(),
		Root:          "content",
		Locales:       []string{"en"},
		DefaultLocale: "en",
		CategoryOrder: map[string]int{"getting-started": 1, "modules": 2},
	}
}

func referenceSource() *fakesource.Source {
	return &fakesource.Source{
		MountPath: "extensions/reference",
		Names:     map[string]string{"": "Extension reference", "ru": "Справочник расширений"},
		Position:  10,
		Layout: []helpdocs.Node{
			{Title: "Overview", Path: "overview", Order: 1},
			{Title: "CRM", Path: "crm", Order: 2, Children: []helpdocs.Node{
				{Title: "Commands", Path: "crm/commands", Order: 2},
				{Title: "Actions", Path: "crm/actions", Order: 1},
			}},
			{Title: "Secrets", Path: "secrets", Order: 3},
		},
		Docs: map[string]fakesource.Doc{
			"overview":      {Title: "Overview", Markdown: "# Overview\n\nHello."},
			"crm/commands":  {Title: "Commands", Markdown: "# Commands\n\nRun."},
			"crm/actions":   {Title: "Actions", Markdown: "# Actions\n\nAct."},
			"secrets":       {Title: "Secrets", Markdown: "# Secrets\n\nHidden.", Restricted: true},
			"crm/not-in-ui": {Title: "Unlisted", Markdown: "# Unlisted"},
		},
	}
}

func guidesSource() *fakesource.Source {
	return &fakesource.Source{
		MountPath: "extensions/guides",
		Names:     map[string]string{"": "Extension guides"},
		Position:  -5,
		Layout:    []helpdocs.Node{{Title: "Start", Path: "start"}},
		Docs:      map[string]fakesource.Doc{"start": {Title: "Start", Markdown: "# Start"}},
	}
}

func withSources(t *testing.T, config ContentConfig, sources ...helpdocs.Source) *ContentService {
	t.Helper()
	service, err := NewContentService(config).WithSources(sources)
	require.NoError(t, err)
	return service
}

func reader(t *testing.T, perms ...permission.Permission) context.Context {
	t.Helper()
	email, err := internet.NewEmail("reader@example.com")
	require.NoError(t, err)
	return composables.WithUser(context.Background(), user.New("Reader", "", email, user.UILanguageEN, user.WithPermissions(perms)))
}

// outline flattens a tree into "Category > Document (path)" lines in display
// order.
func outline(nodes []viewmodels.CategoryNode) []string {
	var lines []string
	var walk func(nodes []viewmodels.CategoryNode, prefix string)
	walk = func(nodes []viewmodels.CategoryNode, prefix string) {
		for _, node := range nodes {
			label := prefix + node.Title
			if node.Path != "" {
				lines = append(lines, label+" ("+node.Path+")")
				continue
			}
			lines = append(lines, label+"/")
			walk(node.Children, label+" > ")
		}
	}
	walk(nodes, "")
	return lines
}

// False green: asserting only that a source category exists, which passes
// when sources are appended after the file tree instead of obeying the
// order rules, or when the mount's parent categories are missing.
func TestWithSources_MergesIntoTreeByOrder(t *testing.T) {
	config := fileConfig()
	config.CategoryOrder["extensions"] = 3 // the shared parent of both mounts follows the category rules
	service := withSources(t, config, referenceSource(), guidesSource())

	tree, err := service.Tree(reader(t, permissions.UserRead))

	require.NoError(t, err)
	require.Equal(t, []string{
		"Getting Started/",
		"Getting Started > Welcome (getting-started/welcome.md)",
		"Modules/",
		"Modules > CRM (modules/crm.md)",
		"Extensions/",
		"Extensions > Extension guides/",
		"Extensions > Extension guides > Start (extensions/guides/start)",
		"Extensions > Extension reference/",
		"Extensions > Extension reference > Overview (extensions/reference/overview)",
		"Extensions > Extension reference > CRM/",
		"Extensions > Extension reference > CRM > Actions (extensions/reference/crm/actions)",
		"Extensions > Extension reference > CRM > Commands (extensions/reference/crm/commands)",
		"Extensions > Extension reference > Secrets (extensions/reference/secrets)",
	}, outline(tree), "the unlisted document stays out and the restricted one needs a reader who may read it")
}

func TestWithSources_SourceOrderPlacesMountAmongFileCategories(t *testing.T) {
	first := guidesSource()
	first.MountPath = "tour"
	first.Position = 0
	last := referenceSource()
	last.MountPath = "zeta"
	last.Position = 1
	last.Names = map[string]string{"": "Alpha reference"}
	service := withSources(t, fileConfig(), last, first)

	tree, err := service.Tree(context.Background())

	require.NoError(t, err)
	top := make([]string, 0, len(tree))
	for _, node := range tree {
		top = append(top, node.Title)
	}
	// Siblings sort by order, then title: the first source is 0, the second
	// ties with getting-started at 1 and wins on title, modules is 2.
	require.Equal(t, []string{"Extension guides", "Alpha reference", "Getting Started", "Modules"}, top)
}

// False green: a source with no documents for this reader that still adds an
// empty category (or the parent categories of its mount) to the sidebar.
func TestWithSources_EmptyTreeHidesMount(t *testing.T) {
	empty := guidesSource()
	empty.Layout = nil
	service := withSources(t, fileConfig(), empty)

	tree, err := service.Tree(context.Background())

	require.NoError(t, err)
	require.Equal(t, []string{"Getting Started/", "Getting Started > Welcome (getting-started/welcome.md)", "Modules/", "Modules > CRM (modules/crm.md)"}, outline(tree))
}

func TestWithSources_CategoryWithoutVisibleDocumentsIsDropped(t *testing.T) {
	source := referenceSource()
	source.Layout = []helpdocs.Node{
		{Title: "Overview", Path: "overview"},
		{Title: "Private", Path: "private", Children: []helpdocs.Node{{Title: "Secrets", Path: "secrets"}}},
	}
	service := withSources(t, fileConfig(), source)

	tree, err := service.Tree(context.Background())

	require.NoError(t, err)
	require.NotContains(t, strings.Join(outline(tree), "\n"), "Private")
	require.Contains(t, outline(tree), "Extensions > Extension reference > Overview (extensions/reference/overview)")
}

// False green: one reader in the test, where a source cached across requests
// or built once at construction still passes.
func TestWithSources_ReadersSeeTheirOwnTree(t *testing.T) {
	service := withSources(t, fileConfig(), referenceSource())
	secret := "Extensions > Extension reference > Secrets (extensions/reference/secrets)"

	admin, err := service.Tree(reader(t, permissions.UserRead))
	require.NoError(t, err)
	plain, err := service.Tree(reader(t))
	require.NoError(t, err)
	anonymous, err := service.Tree(context.Background())
	require.NoError(t, err)

	require.Contains(t, outline(admin), secret)
	require.NotContains(t, outline(plain), secret)
	require.NotContains(t, outline(anonymous), secret)

	_, err = service.Get(reader(t, permissions.UserRead), "extensions/reference/secrets")
	require.NoError(t, err)
	_, err = service.Get(reader(t), "extensions/reference/secrets")
	require.ErrorIs(t, err, ErrDocumentNotFound)
}

func TestWithSources_SourceChangesBetweenRequests(t *testing.T) {
	source := guidesSource()
	service := withSources(t, fileConfig(), source)
	before, err := service.Tree(context.Background())
	require.NoError(t, err)

	source.Layout = append(source.Layout, helpdocs.Node{Title: "Later", Path: "later"})
	source.Docs["later"] = fakesource.Doc{Title: "Later", Markdown: "# Later"}
	after, err := service.Tree(context.Background())

	require.NoError(t, err)
	require.NotContains(t, outline(before), "Extensions > Extension guides > Later (extensions/guides/later)")
	require.Contains(t, outline(after), "Extensions > Extension guides > Later (extensions/guides/later)")
}

// False green: asserting the clamped content locale. The file content has only
// "en" here; the source must still get the reader's own locale.
func TestWithSources_PassesRequestLocaleToSource(t *testing.T) {
	source := referenceSource()
	service := withSources(t, fileConfig(), source)
	ctx := intl.WithLocale(reader(t, permissions.UserRead), language.Russian)

	tree, err := service.Tree(ctx)
	require.NoError(t, err)
	_, err = service.Get(ctx, "extensions/reference/overview")
	require.NoError(t, err)
	_, err = service.Tree(context.Background())
	require.NoError(t, err)

	require.Contains(t, outline(tree), "Extensions > Справочник расширений/")
	require.Equal(t, []string{"ru", "ru", "en"}, source.Locales())
}

func TestWithSources_GetMapsSourceDocument(t *testing.T) {
	source := referenceSource()
	updated := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	source.Docs["overview"] = fakesource.Doc{Title: "Overview", Markdown: "# Overview\n\nOne two three.", Audience: " Developers ", Updated: updated}
	service := withSources(t, fileConfig(), source)

	doc, err := service.Get(context.Background(), "/extensions/reference/overview")

	require.NoError(t, err)
	require.Equal(t, "Overview", doc.Title)
	require.Equal(t, "extensions/reference/overview", doc.Path)
	require.Equal(t, "extensions/reference", doc.SourceMount)
	require.Equal(t, "# Overview\n\nOne two three.", string(doc.Content))
	require.Equal(t, "Developers", doc.Meta.Audience)
	require.Equal(t, updated, doc.Meta.Updated)
	require.Equal(t, 1, doc.Meta.ReadingMinutes)

	file, err := service.Get(context.Background(), "modules/crm.md")
	require.NoError(t, err)
	require.Empty(t, file.SourceMount, "file documents are not source documents")
}

func TestWithSources_GetErrors(t *testing.T) {
	service := withSources(t, fileConfig(), referenceSource())

	_, err := service.Get(context.Background(), "extensions/reference/missing")
	require.ErrorIs(t, err, ErrDocumentNotFound)

	for _, bad := range []string{"extensions/reference/", "extensions/reference/../reference/overview", "extensions/reference/a//b"} {
		_, err = service.Get(context.Background(), bad)
		require.Error(t, err, bad)
	}
	_, err = service.Get(context.Background(), "extensions/reference/overview/")
	require.ErrorIs(t, err, ErrInvalidPath)
}

// False green: a source whose Get fails with an unrelated error reported as
// "not found"; the reader would see a 404 for an outage.
func TestWithSources_SourceFailureIsNotNotFound(t *testing.T) {
	failing := &failingSource{Source: referenceSource()}
	service := withSources(t, fileConfig(), failing)

	_, err := service.Get(context.Background(), "extensions/reference/overview")

	require.Error(t, err)
	require.NotErrorIs(t, err, ErrDocumentNotFound)
	require.ErrorIs(t, err, errBackend)
}

var errBackend = errors.New("backend down")

type failingSource struct{ *fakesource.Source }

func (failingSource) Get(context.Context, string, string) (helpdocs.Document, error) {
	return helpdocs.Document{}, errBackend
}

func TestWithSources_FailingTreeLeavesOnlyThatSourceOut(t *testing.T) {
	broken := referenceSource()
	broken.Err = errBackend
	hook := logrustest.NewGlobal()
	t.Cleanup(func() { logrus.StandardLogger().ReplaceHooks(make(logrus.LevelHooks)) })
	service := withSources(t, fileConfig(), broken, guidesSource())

	tree, err := service.Tree(context.Background())

	require.NoError(t, err)
	lines := strings.Join(outline(tree), "\n")
	require.Contains(t, lines, "Extension guides")
	require.NotContains(t, lines, "Extension reference")
	require.Equal(t, 1, countLogs(hook, "help source tree unavailable"))
}

func countLogs(hook *logrustest.Hook, message string) int {
	count := 0
	for _, entry := range hook.AllEntries() {
		if entry.Message == message {
			count++
		}
	}
	return count
}

func TestWithSources_NoSourcesKeepsService(t *testing.T) {
	service := NewContentService(fileConfig())

	same, err := service.WithSources(nil)

	require.NoError(t, err)
	require.Same(t, service, same)
	require.False(t, same.HasSources())
	require.False(t, same.IsSourcePath("extensions/reference/overview"))
}

func TestWithSources_DefaultDocumentComesFromFiles(t *testing.T) {
	first := guidesSource()
	first.Position = -100
	service := withSources(t, fileConfig(), first)

	doc, err := service.DefaultDocument(context.Background())

	require.NoError(t, err)
	require.Equal(t, "getting-started/welcome.md", doc.Path)
}

// False green: checking only two sources that overlap each other, where a
// mount that sits on the file tree is never compared.
func TestWithSources_MountCollisionsFailAtConstruction(t *testing.T) {
	mount := func(path string) *fakesource.Source {
		source := guidesSource()
		source.MountPath = path
		return source
	}
	tests := []struct {
		name    string
		config  ContentConfig
		sources []helpdocs.Source
		message string
	}{
		{"same mount twice", fileConfig(), []helpdocs.Source{mount("ext/a"), mount("ext/a")}, `"ext/a" and "ext/a" overlap`},
		{"nested mount", fileConfig(), []helpdocs.Source{mount("ext"), mount("ext/a")}, `"ext" and "ext/a" overlap`},
		{"mount is a file category", fileConfig(), []helpdocs.Source{mount("modules")}, `mount "modules" collides with "modules"`},
		{"file documents live below the mount", fileConfig(), []helpdocs.Source{mount("getting-started")}, `mount "getting-started" collides with`},
		{"empty mount", fileConfig(), []helpdocs.Source{mount("")}, "clean slash-separated path"},
		{"leading slash", fileConfig(), []helpdocs.Source{mount("/ext")}, "clean slash-separated path"},
		{"trailing slash", fileConfig(), []helpdocs.Source{mount("ext/")}, "clean slash-separated path"},
		{"traversal", fileConfig(), []helpdocs.Source{mount("ext/../x")}, "clean slash-separated path"},
		{"nil source", fileConfig(), []helpdocs.Source{nil}, "nil"},
		{
			"mount is a virtual category",
			ContentConfig{FS: fileContent(), Root: "content", Locales: []string{"en"}, DefaultLocale: "en", CategoryPaths: map[string]string{"modules": "guides"}},
			[]helpdocs.Source{mount("guides")},
			`mount "guides" collides`,
		},
		{
			"mount collides in another content locale",
			ContentConfig{
				FS:   fstest.MapFS{"content/en/a.md": &fstest.MapFile{Data: []byte("# A")}, "content/ru/ext/b.md": &fstest.MapFile{Data: []byte("# B")}},
				Root: "content", Locales: []string{"en", "ru"}, DefaultLocale: "en",
			},
			[]helpdocs.Source{mount("ext")},
			"of the ru help content",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewContentService(tt.config).WithSources(tt.sources)

			require.Error(t, err)
			require.Contains(t, err.Error(), tt.message)
		})
	}
}

func TestWithSources_MountNestedUnderFileCategoryIsAllowedAndShared(t *testing.T) {
	source := guidesSource()
	source.MountPath = "modules/extra"
	config := fileConfig()
	config.CategoryTitles = map[string]map[string]string{"en": {"modules": "Product modules"}}
	service := withSources(t, config, source)

	tree, err := service.Tree(context.Background())

	require.NoError(t, err)
	require.Contains(t, outline(tree), "Product modules > Extension guides > Start (modules/extra/start)")
	require.Contains(t, outline(tree), "Product modules > CRM (modules/crm.md)")
	doc, err := service.Get(context.Background(), "modules/extra/start")
	require.NoError(t, err)
	require.Equal(t, "Start", doc.Title)
	crm, err := service.Get(context.Background(), "modules/crm.md")
	require.NoError(t, err)
	require.Equal(t, "CRM", crm.Title)
}

// False green: a disk tree that never changes after construction. Disk
// content may gain a category later; the source then gives way, once-logged.
func TestWithSources_CollisionAfterConstructionFilesWinAndLogOnce(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "en", "modules"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "en", "modules", "crm.md"), []byte("# CRM"), 0o600))
	source := guidesSource()
	source.MountPath = "ext"
	service := withSources(t, ContentConfig{Root: root, Locales: []string{"en"}, DefaultLocale: "en"}, source)
	before, err := service.Tree(context.Background())
	require.NoError(t, err)
	require.Contains(t, outline(before), "Extension guides > Start (ext/start)")

	require.NoError(t, os.MkdirAll(filepath.Join(root, "en", "ext"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "en", "ext", "own.md"), []byte("# Own"), 0o600))
	hook := logrustest.NewGlobal()
	t.Cleanup(func() { logrus.StandardLogger().ReplaceHooks(make(logrus.LevelHooks)) })
	for range 3 {
		tree, err := service.Tree(context.Background())
		require.NoError(t, err)
		lines := strings.Join(outline(tree), "\n")
		require.Contains(t, lines, "Ext > Own (ext/own.md)")
		require.NotContains(t, lines, "(ext/start)")
		require.NotContains(t, lines, "Extension guides")
	}

	require.Equal(t, 1, countLogs(hook, "help source mount collides with the help content; the help content wins"))
}

func TestWithSources_SkipsUnservablePaths(t *testing.T) {
	source := guidesSource()
	source.Layout = []helpdocs.Node{
		{Title: "Good", Path: "good"},
		{Title: "Absolute", Path: "/abs"},
		{Title: "Traversal", Path: "../up"},
		{Title: "Empty", Path: ""},
		{Title: "Dirty", Path: "a//b"},
	}
	source.Docs["good"] = fakesource.Doc{Title: "Good", Markdown: "# Good"}
	for _, path := range []string{"/abs", "../up", "", "a//b"} {
		source.Docs[path] = fakesource.Doc{Title: path, Markdown: "# x"}
	}
	service := withSources(t, fileConfig(), source)

	tree, err := service.Tree(context.Background())

	require.NoError(t, err)
	lines := strings.Join(outline(tree), "\n")
	require.Contains(t, lines, "Good (extensions/guides/good)")
	for _, bad := range []string{"Absolute", "Traversal", "Empty", "Dirty"} {
		require.NotContains(t, lines, bad)
	}
}
