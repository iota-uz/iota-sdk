package services

import (
	"context"
	"os"
	"path/filepath"
	"testing"

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
	require.Equal(t, "Руководства", tree[0].Title)
	require.Equal(t, "Настройка", tree[0].Children[0].Title)
	require.Equal(t, "Установка", tree[0].Children[0].Children[0].Title)
	require.Equal(t, "Руководства", tree[1].Title)
	require.Equal(t, "Обзор", tree[1].Children[0].Title)
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

func TestContentService_GetRejectsTraversal(t *testing.T) {
	service := NewContentService(ContentConfig{Root: t.TempDir(), Locales: []string{"en"}, DefaultLocale: "en"})

	_, err := service.Get(context.Background(), "../secret.md")

	require.ErrorIs(t, err, ErrInvalidPath)
}

func writeDoc(t *testing.T, root, path, content string) {
	t.Helper()
	fullPath := filepath.Join(root, filepath.FromSlash(path))
	require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0o755))
	require.NoError(t, os.WriteFile(fullPath, []byte(content), 0o644))
}
