package services

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/modules/helpcenter/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

func TestContentService_TreeBuildsNestedCategories(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, "en/guides/setup/install.md", "# Install\n\nUse it.")
	writeDoc(t, root, "en/intro.md", "# Intro\n\nStart here.")
	service := NewContentService(ContentConfig{Root: root, Locales: []string{"en"}, DefaultLocale: "en"})

	tree, err := service.Tree(context.Background())

	require.NoError(t, err)
	require.Len(t, tree, 2)
	require.Equal(t, "Guides", tree[0].Title)
	require.Equal(t, "Setup", tree[0].Children[0].Title)
	require.Equal(t, "Install", tree[0].Children[0].Children[0].Title)
	require.Equal(t, "guides/setup/install.md", tree[0].Children[0].Children[0].Path)
}

func TestContentService_TreeUsesLocalizedCategoryTitles(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, "ru/guides/setup/install.md", "# Установка\n\nИспользуйте.")
	writeDoc(t, root, "ru/manuals/overview.md", "# Обзор\n\nПрочитайте.")
	service := NewContentService(ContentConfig{
		Root:          root,
		Locales:       []string{"en", "ru"},
		DefaultLocale: "en",
		CategoryTitles: map[string]map[string]string{
			"ru": {
				"guides":       "Руководства",
				"guides/setup": "Настройка",
				"manuals":      "Руководства",
			},
		},
	})
	ctx := intl.WithLocale(context.Background(), language.Russian)

	tree, err := service.Tree(ctx)

	require.NoError(t, err)
	require.Len(t, tree, 2)
	var setup, manuals *viewmodels.CategoryNode
	for i := range tree {
		switch tree[i].Children[0].Title {
		case "Настройка":
			setup = &tree[i]
		case "Обзор":
			manuals = &tree[i]
		}
	}
	require.NotNil(t, setup)
	require.Equal(t, "Руководства", setup.Title)
	require.Equal(t, "Установка", setup.Children[0].Children[0].Title)
	require.NotNil(t, manuals)
	require.Equal(t, "Руководства", manuals.Title)
}

func TestContentService_TreeUsesVirtualCategoryPaths(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, "ru/insurance/policies/issue.md", "# Выпуск полиса\n\nОформите полис.")
	writeDoc(t, root, "ru/crm/clients/create.md", "# Создание клиента\n\nСоздайте клиента.")
	service := NewContentService(ContentConfig{
		Root:          root,
		Locales:       []string{"ru"},
		DefaultLocale: "ru",
		CategoryPaths: map[string]string{
			"insurance": "01-erp/01-portfolio",
			"crm":       "02-crm",
		},
		CategoryTitles: map[string]map[string]string{
			"ru": {
				"01-erp":              "01 ERP",
				"01-erp/01-portfolio": "01 Портфель",
				"02-crm":              "02 CRM",
			},
		},
	})

	tree, err := service.Tree(context.Background())

	require.NoError(t, err)
	require.Len(t, tree, 2)
	require.Equal(t, "01 ERP", tree[0].Title)
	require.Equal(t, "01 Портфель", tree[0].Children[0].Title)
	require.Equal(t, "Policies", tree[0].Children[0].Children[0].Title)
	require.Equal(t, "insurance/policies/issue.md", tree[0].Children[0].Children[0].Children[0].Path)
	require.Equal(t, "02 CRM", tree[1].Title)
	require.Equal(t, "Clients", tree[1].Children[0].Title)
	require.Equal(t, "crm/clients/create.md", tree[1].Children[0].Children[0].Path)
}

func TestContentService_HiddenPathsAreNotReaderFacing(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, "en/guides/start.md", "# Start\n\nBegin here.")
	writeDoc(t, root, "en/internal/runbook.md", "# Runbook\n\nOperators only.")
	writeDoc(t, root, "en/technical.md", "# Technical\n\nImplementation details.")
	service := NewContentService(ContentConfig{
		Root:          root,
		Locales:       []string{"en"},
		DefaultLocale: "en",
		HiddenPaths:   []string{"internal", "technical.md"},
	})

	tree, err := service.Tree(context.Background())

	require.NoError(t, err)
	require.Len(t, tree, 1)
	require.Equal(t, "Guides", tree[0].Title)

	_, err = service.Get(context.Background(), "internal/runbook.md")
	require.ErrorIs(t, err, ErrDocumentNotFound)
	_, err = service.Get(context.Background(), "technical.md")
	require.ErrorIs(t, err, ErrDocumentNotFound)
}

func TestContentService_GetUsesLocaleAndDefaultFallback(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, "en/intro.md", "# English\n\nHello.")
	service := NewContentService(ContentConfig{Root: root, Locales: []string{"en", "ru"}, DefaultLocale: "en"})
	ctx := intl.WithLocale(context.Background(), language.Russian)

	doc, err := service.Get(ctx, "intro.md")

	require.NoError(t, err)
	require.Equal(t, "English", doc.Title)
}

func TestContentService_GetStripsFrontMatter(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, "en/intro.md", "---\ntitle: Getting Started\nkeywords:\n  - help\n---\n\n# Intro\n\nStart here.")
	service := NewContentService(ContentConfig{Root: root, Locales: []string{"en"}, DefaultLocale: "en"})

	doc, err := service.Get(context.Background(), "intro.md")

	require.NoError(t, err)
	require.Equal(t, "Getting Started", doc.Title)
	require.Equal(t, "# Intro\n\nStart here.", string(doc.Content))
}

func TestContentService_GetPreservesMalformedFrontMatter(t *testing.T) {
	root := t.TempDir()
	source := "---\ntitle: [not valid\n---\n\n# Intro\n\nStart here."
	writeDoc(t, root, "en/intro.md", source)
	service := NewContentService(ContentConfig{Root: root, Locales: []string{"en"}, DefaultLocale: "en"})

	doc, err := service.Get(context.Background(), "intro.md")

	require.NoError(t, err)
	require.Equal(t, source, string(doc.Content))
	require.Equal(t, "Intro", doc.Title)
}

func TestContentService_GetHidesConfiguredSections(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, "en/intro.md", `# Intro

Start here.

## Keywords

internal, search, terms

## Related docs

- A useful guide

## UI routes

/internal/route`)
	service := NewContentService(ContentConfig{
		Root:           root,
		Locales:        []string{"en"},
		DefaultLocale:  "en",
		HiddenSections: []string{"Keywords", "UI routes"},
	})

	doc, err := service.Get(context.Background(), "intro.md")

	require.NoError(t, err)
	require.Equal(t, "# Intro\n\nStart here.\n\n## Related docs\n\n- A useful guide", string(doc.Content))
}

func TestContentService_GetDoesNotHideHeadingsInsideCodeFences(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, "en/intro.md", "# Intro\n\n```markdown\n## Keywords\nvisible example\n```\n")
	service := NewContentService(ContentConfig{
		Root:           root,
		Locales:        []string{"en"},
		DefaultLocale:  "en",
		HiddenSections: []string{"Keywords"},
	})

	doc, err := service.Get(context.Background(), "intro.md")

	require.NoError(t, err)
	require.Contains(t, string(doc.Content), "## Keywords")
	require.Contains(t, string(doc.Content), "visible example")
}

func TestContentService_GetRespectsLongCodeFenceDelimiter(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, "en/intro.md", "# Intro\n\n````markdown\n```\n## Keywords\nvisible example\n````\n")
	service := NewContentService(ContentConfig{
		Root:           root,
		Locales:        []string{"en"},
		DefaultLocale:  "en",
		HiddenSections: []string{"Keywords"},
	})

	doc, err := service.Get(context.Background(), "intro.md")

	require.NoError(t, err)
	require.Contains(t, string(doc.Content), "## Keywords")
	require.Contains(t, string(doc.Content), "visible example")
}

func TestContentService_GetRejectsTraversal(t *testing.T) {
	service := NewContentService(ContentConfig{Root: t.TempDir(), Locales: []string{"en"}, DefaultLocale: "en"})

	_, err := service.Get(context.Background(), "../secret.md")

	require.ErrorIs(t, err, ErrInvalidPath)
}

func TestContentService_TasksUsesLocaleAndDefaultFallback(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, "en/intro.md", "# Intro")
	writeDoc(t, root, "ru/intro.md", "# Введение")
	service := NewContentService(ContentConfig{
		Root:          root,
		Locales:       []string{"en", "ru", "uz"},
		DefaultLocale: "en",
		HomeTasks: map[string][]viewmodels.TaskGroup{
			"en": {
				{Title: "Daily work", Tasks: []viewmodels.HelpTask{{Title: "Find a client"}}},
			},
			"ru": {
				{Title: "Ежедневная работа", Tasks: []viewmodels.HelpTask{{Title: "Найти клиента"}}},
			},
		},
	})

	ruTasks := service.Tasks(intl.WithLocale(context.Background(), language.Russian))
	require.Equal(t, "Ежедневная работа", ruTasks[0].Title)
	require.Equal(t, "Найти клиента", ruTasks[0].Tasks[0].Title)

	uzTasks := service.Tasks(intl.WithLocale(context.Background(), language.Uzbek))
	require.Equal(t, "Daily work", uzTasks[0].Title)
}

func TestContentService_MediaUsesLocaleAndDefaultFallback(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "en/images/lifecycle.svg", "<svg>english</svg>")
	writeFile(t, root, "ru/images/overview.svg", "<svg>russian</svg>")
	service := NewContentService(ContentConfig{Root: root, Locales: []string{"en", "ru"}, DefaultLocale: "en"})
	ctx := intl.WithLocale(context.Background(), language.Russian)

	localized, err := service.Media(ctx, "images/overview.svg")
	require.NoError(t, err)
	require.Equal(t, "<svg>russian</svg>", string(localized.Content))

	fallback, err := service.Media(ctx, "images/lifecycle.svg")
	require.NoError(t, err)
	require.Equal(t, "<svg>english</svg>", string(fallback.Content))
}

func TestContentService_MediaRejectsTraversal(t *testing.T) {
	service := NewContentService(ContentConfig{Root: t.TempDir(), Locales: []string{"en"}, DefaultLocale: "en"})

	for _, mediaPath := range []string{"../secret.png", "images/../secret.png", "/etc/passwd", `images\\secret.png`} {
		t.Run(mediaPath, func(t *testing.T) {
			_, err := service.Media(context.Background(), mediaPath)
			require.ErrorIs(t, err, ErrInvalidPath)
		})
	}
}

func TestContentService_MediaRejectsNonImageFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "en/internal/source.md", "# Hidden implementation notes")
	service := NewContentService(ContentConfig{Root: root, Locales: []string{"en"}, DefaultLocale: "en"})

	_, err := service.Media(context.Background(), "internal/source.md")

	require.ErrorIs(t, err, ErrInvalidPath)
}

func TestContentService_MediaRejectsHiddenPaths(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "en/internal/diagram.png", "hidden image")
	service := NewContentService(ContentConfig{
		Root:          root,
		Locales:       []string{"en"},
		DefaultLocale: "en",
		HiddenPaths:   []string{"internal"},
	})

	_, err := service.Media(context.Background(), "internal/diagram.png")

	require.ErrorIs(t, err, ErrMediaNotFound)
}

func TestContentService_MediaDoesNotServeDirectories(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "en/images.svg/diagram.png", "image")
	service := NewContentService(ContentConfig{Root: root, Locales: []string{"en"}, DefaultLocale: "en"})

	_, err := service.Media(context.Background(), "images.svg")

	require.ErrorIs(t, err, ErrMediaNotFound)
}

func writeDoc(t *testing.T, root, path, content string) {
	t.Helper()
	writeFile(t, root, path, content)
}

func writeFile(t *testing.T, root, path, content string) {
	t.Helper()
	fullPath := filepath.Join(root, filepath.FromSlash(path))
	require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0o755))
	require.NoError(t, os.WriteFile(fullPath, []byte(content), 0o644))
}

// False green: reading the fields back from a hand-built Document, or a date
// layout that also accepts the zero value.
func TestContentService_GetReadsReaderMetadataFromFrontMatter(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, "en/course/numbers.md", "---\ntitle: Numbers\norder: 3\npart: \"**Chapter 2** of 5\"\naudience: For accountants\nupdated: 2026-11-04\nreading_minutes: 12\n---\n\n# Contract in numbers\n\nLead.")
	writeDoc(t, root, "en/course/plain.md", "---\nupdated: not-a-date\n---\n\n# Plain\n\nBody.")
	service := NewContentService(ContentConfig{Root: root, Locales: []string{"en"}, DefaultLocale: "en"})

	doc, err := service.Get(context.Background(), "course/numbers.md")
	require.NoError(t, err)
	require.Equal(t, "Numbers", doc.Title)
	require.Equal(t, DocumentMeta{
		Order:          3,
		Part:           "**Chapter 2** of 5",
		Audience:       "For accountants",
		Updated:        time.Date(2026, time.November, 4, 0, 0, 0, 0, time.UTC),
		ReadingMinutes: 12,
	}, doc.Meta)

	plain, err := service.Get(context.Background(), "course/plain.md")
	require.NoError(t, err)
	require.True(t, plain.Meta.Updated.IsZero())
	require.Equal(t, 1, plain.Meta.ReadingMinutes)
}

// False green: a fixture whose titles already sort in the declared order.
func TestContentService_TreeOrdersDocumentsByFrontMatterOrder(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, "en/course/a.md", "---\ntitle: Zeta route\norder: 1\n---\n\n# Z")
	writeDoc(t, root, "en/course/b.md", "---\ntitle: Alpha chapter\norder: 2\n---\n\n# A")
	writeDoc(t, root, "en/guides/b.md", "# Beta")
	writeDoc(t, root, "en/guides/a.md", "# Alpha")
	service := NewContentService(ContentConfig{Root: root, Locales: []string{"en"}, DefaultLocale: "en"})

	tree, err := service.Tree(context.Background())

	require.NoError(t, err)
	require.Equal(t, []string{"Zeta route", "Alpha chapter"}, nodeTitles(tree[0].Children))
	require.Equal(t, []string{"Alpha", "Beta"}, nodeTitles(tree[1].Children))
}

// False green: neighbours taken from the flattened tree, which would page
// into a nested category or across sections.
func TestContentService_NavigationLocatesCategoriesAndNeighbours(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, "en/finance/course/start.md", "---\ntitle: Route\norder: 1\n---\n\n# Route")
	writeDoc(t, root, "en/finance/course/numbers.md", "---\ntitle: Numbers\norder: 2\n---\n\n# Numbers")
	writeDoc(t, root, "en/finance/course/deep/extra.md", "---\norder: 3\n---\n\n# Extra")
	writeDoc(t, root, "en/finance/course/close.md", "---\ntitle: Close\norder: 4\n---\n\n# Close")
	service := NewContentService(ContentConfig{
		Root: root, Locales: []string{"en"}, DefaultLocale: "en",
		CategoryTitles: map[string]map[string]string{"en": {"finance": "05 Finance", "finance/course": "00 Course"}},
	})

	tree, err := service.Tree(context.Background())
	require.NoError(t, err)

	nav, found := Locate(tree, "finance/course/numbers.md")
	require.True(t, found)
	require.Equal(t, []string{"05 Finance", "00 Course"}, nav.Categories)
	require.Equal(t, "finance/course/start.md", nav.Previous.Path)
	require.Equal(t, "finance/course/close.md", nav.Next.Path)

	first, _ := Locate(tree, "finance/course/start.md")
	require.Nil(t, first.Previous)
	require.Equal(t, "Numbers", first.Next.Title)
}

func TestEstimateReadingMinutesCountsWordsOutsideMarkup(t *testing.T) {
	require.Equal(t, 1, estimateReadingMinutes(nil))
	words := strings.Repeat("слово ", readingWordsPerMinute)
	require.Equal(t, 1, estimateReadingMinutes([]byte(words)))
	require.Equal(t, 2, estimateReadingMinutes([]byte(words+"ещё")))
	require.Equal(t, 1, estimateReadingMinutes([]byte(words+`<div class="help-task-card"> | --- | </div>`)))
}

func TestContentService_FeedbackURLEscapesDocumentPath(t *testing.T) {
	service := NewContentService(ContentConfig{FeedbackURL: "https://example.test/new?doc={path}"})

	require.Equal(t, "https://example.test/new?doc=course%2Fnumbers.md", service.FeedbackURL("course/numbers.md"))
	require.Equal(t, "https://example.test/new?doc=", service.FeedbackURL(""))
	require.Empty(t, NewContentService(ContentConfig{}).FeedbackURL("course/numbers.md"))
}

func nodeTitles(nodes []viewmodels.CategoryNode) []string {
	titles := make([]string, 0, len(nodes))
	for _, node := range nodes {
		titles = append(titles, node.Title)
	}
	return titles
}
