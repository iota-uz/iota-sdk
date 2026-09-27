package services

import (
	"context"
	"os"
	"path/filepath"
	"testing"

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
