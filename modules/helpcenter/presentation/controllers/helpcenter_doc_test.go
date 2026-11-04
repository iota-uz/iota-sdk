package controllers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gorilla/mux"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/helpcenter/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/markdown"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

// False green: rendering the template with hand-built view models, which
// would skip the controller's navigation lookup, table wrapping and the
// translated plural and date labels.
func TestHelpCenterController_DocRendersReaderNavigation(t *testing.T) {
	controller := newDocTestController(t)

	body := renderDocFragment(t, controller, "course/numbers.md")

	require.Contains(t, body, `<a href="/help">Центр помощи</a>`)
	require.Contains(t, body, `<span>Курс</span>`)
	require.NotContains(t, body, "01 Курс")
	require.Contains(t, body, "<b>Глава 1</b> из 2")
	require.Contains(t, body, "1 минута чтения")
	require.Contains(t, body, "Для бухгалтера")
	require.Contains(t, body, "Обновлено 4 ноября 2026")
	lead, meta, section := strings.Index(body, "<p>Лид.</p>"), strings.Index(body, `class="help-article-meta"`), strings.Index(body, "<h2")
	require.True(t, lead >= 0 && lead < meta && meta < section, "meta line must follow the lead paragraph")
	require.Contains(t, body, `<div class="help-table-wrap"><table>`)
	require.Contains(t, body, `href="/help/doc/course/start.md"`)
	require.Contains(t, body, `href="/help/doc/course/close.md"`)
	require.Contains(t, body, `href="https://example.test/feedback?doc=course%2Fnumbers.md"`)
	require.Contains(t, body, "course/numbers.md</span>")
}

func TestHelpCenterController_DocWithoutNeighboursHasNoPager(t *testing.T) {
	controller := newDocTestController(t)

	body := renderDocFragment(t, controller, "single.md")

	require.NotContains(t, body, `class="help-pager"`)
	require.Contains(t, body, "1 минута чтения")
	require.NotContains(t, body, "Обновлено")
}

func newDocTestController(t *testing.T) *HelpCenterController {
	t.Helper()
	contentFS := fstest.MapFS{
		"content/ru/course/start.md":   &fstest.MapFile{Data: []byte("---\ntitle: Начало\norder: 1\n---\n\n# Начало\n\nВход.")},
		"content/ru/course/numbers.md": &fstest.MapFile{Data: []byte("---\ntitle: Числа\norder: 2\npart: \"**Глава 1** из 2\"\naudience: Для бухгалтера\nupdated: 2026-11-04\n---\n\n# Договор в цифрах\n\nЛид.\n\n## Раздел\n\n| a | b |\n| --- | --- |\n| 1 | 2 |\n")},
		"content/ru/course/close.md":   &fstest.MapFile{Data: []byte("---\ntitle: Закрытие\norder: 3\n---\n\n# Закрытие\n\nКонец.")},
		"content/ru/single.md":         &fstest.MapFile{Data: []byte("# Одна\n\nТекст.")},
	}
	return &HelpCenterController{
		basePath: "/help",
		renderer: markdown.NewRenderer(),
		contentService: services.NewContentService(services.ContentConfig{
			FS:             contentFS,
			Root:           "content",
			Locales:        []string{"ru"},
			DefaultLocale:  "ru",
			CategoryTitles: map[string]map[string]string{"ru": {"course": "01 Курс"}},
			FeedbackURL:    "https://example.test/feedback?doc={path}",
		}),
	}
}

func renderDocFragment(t *testing.T, controller *HelpCenterController, docPath string) string {
	t.Helper()
	bundle := i18n.NewBundle(language.Russian)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	_, err := bundle.LoadMessageFile("../locales/ru.json")
	require.NoError(t, err)
	localizer := i18n.NewLocalizer(bundle, "ru")

	req := httptest.NewRequest(http.MethodGet, "/help/doc/"+docPath, nil)
	req.Header.Set("Hx-Request", "true")
	ctx := intl.WithLocalizer(intl.WithLocale(context.Background(), language.Russian), localizer)
	ctx = composables.WithPageCtx(ctx, types.NewPageContext(language.Russian, req.URL, localizer))
	req = mux.SetURLVars(req.WithContext(ctx), map[string]string{"path": docPath})
	rec := httptest.NewRecorder()

	controller.doc(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	return rec.Body.String()
}
