package controllers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/helpcenter/services"
	"github.com/iota-uz/iota-sdk/pkg/bichat/kb"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/microcosm-cc/bluemonday"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

// stubSearcher answers like a real index: in its own order, cut to TopK.
type stubSearcher struct {
	kb.KBSearcher
	results []kb.SearchResult
	topK    int
}

func (s *stubSearcher) Search(_ context.Context, _ string, opts kb.SearchOptions) ([]kb.SearchResult, error) {
	s.topK = opts.TopK
	return s.results[:min(len(s.results), opts.TopK)], nil
}

func (s *stubSearcher) IsAvailable() bool { return true }

func searchHit(docPath, title, excerpt string) kb.SearchResult {
	return kb.SearchResult{Document: kb.Document{Path: docPath, Title: title}, Excerpt: excerpt}
}

// The shared KB index ranks English and Ali-only documents above the reader's
// Russian articles here. False green: a stub that ignores TopK (the cut is
// the bug), or hidden hits placed after the visible ones, where cutting first
// and filtering second still looks right.
func TestHelpCenterController_SearchFiltersLocaleAndHiddenBeforeLimit(t *testing.T) {
	searcher := &stubSearcher{}
	for i := 0; i < searchResultLimit; i++ {
		searcher.results = append(searcher.results, searchHit("/kb/en/insurance/osago.md", "Issue an OSAGO policy", "<mark>OSAGO</mark>"))
	}
	searcher.results = append(searcher.results,
		searchHit("/kb/ru/insurance/osago-sql.md", "SQL по ОСАГО", "<mark>ОСАГО</mark> SELECT"),
		searchHit("/kb/ru/internal/runbook.md", "Ранбук", "<mark>ОСАГО</mark>"),
		searchHit("/kb/ru/insurance/osago.md", "Выпуск полиса ОСАГО", "…полиса ## Цель Оформить <mark>ОСАГО</mark> → [новый договор](/portfolio/contracts/osago/new) и **оплату**"),
		searchHit("/kb/ru/insurance/osago.md", "Выпуск полиса ОСАГО", "<mark>ОСАГО</mark> повтор"),
		searchHit("/kb/ru/insurance/benefits.md", "Льготы ОСАГО", "E --&gt; F[&#34;<mark>ОСАГО</mark>&#34;] click A href &#34;/x&#34;"),
	)
	controller := newSearchTestController(searcher)

	body := renderSearch(t, controller, language.Russian, "ОСАГО")

	require.Greater(t, searcher.topK, searchResultLimit, "the index must be over-fetched before filtering")
	require.Equal(t, 1, strings.Count(body, `href="/help/doc/insurance/osago.md"`), "one row per article")
	require.Contains(t, body, `href="/help/doc/insurance/benefits.md"`)
	require.NotContains(t, body, "osago-sql.md")
	require.NotContains(t, body, "internal/runbook.md")
	require.NotContains(t, body, "Issue an OSAGO policy")
	require.Contains(t, body, "Оформить <mark>ОСАГО</mark> → новый договор и оплату")
	require.Contains(t, body, ">Портфель</div>", "the row names the article's section, without its numeric prefix")
	require.NotContains(t, body, ">insurance/osago.md<", "the row no longer shows the file path")
	for _, markup := range []string{"## ", "](", "**", "--&gt;", "click A"} {
		require.NotContains(t, body, markup)
	}

	enBody := renderSearch(t, controller, language.English, "OSAGO")
	require.Contains(t, enBody, `href="/help/doc/insurance/osago.md"`)
	require.NotContains(t, enBody, "Выпуск полиса")
}

func TestPlainExcerptDropsMarkdownKeepsHighlight(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ in, want string }{
		{
			in:   "…e an <mark>OSAGO</mark> Policy ## Goal Create an <mark>OSAGO</mark> contract",
			want: "…e an <mark>OSAGO</mark> Policy Goal Create an <mark>OSAGO</mark> contract",
		},
		{
			in:   "…](/help/doc/04-website/non-<mark>osago</mark>-quote-to-payment.md) - [Manage the policy",
			want: "… Manage the policy",
		},
		{
			in:   "> [!IMPORTANT]\n> Откройте [новый договор <mark>ОСАГО</mark>](/portfolio/contracts/osago/new).",
			want: "Откройте новый договор <mark>ОСАГО</mark>.",
		},
		{
			in:   "…о изменилось?\"} E --> F[\"Переоформление\"] E --> G[\"<mark>Расторжение</mark>\"] click A href",
			want: "…о изменилось? Переоформление <mark>Расторжение</mark>",
		},
		{
			in:   "…жизненного цикла E --&gt; F[&#34;Переоформление&#34;] E --&gt; G[&#34;<mark>Расторжение</mark>&#34;] click A href &#34;/portfolio/contracts&#34; &#34;Открыть&#34;",
			want: "…жизненного цикла Переоформление <mark>Расторжение</mark>",
		},
		{
			in:   "…ransfer.md) Изменение жизненного цикла",
			want: "… Изменение жизненного цикла",
		},
		{
			in:   "| Признак | **Действие** |\n| --- | --- |",
			want: "Признак Действие",
		},
		{
			in:   "Ситуация Что происходит --- :---: Общая скидка по <mark>ОСАГО</mark> — 10-20 %",
			want: "Ситуация Что происходит Общая скидка по <mark>ОСАГО</mark> — 10-20 %",
		},
	} {
		require.Equal(t, tc.want, plainExcerpt(tc.in))
	}
}

// Article HTML is indexed escaped. False green: losing the highlighted word
// together with the tags, leaving a partial tag at an excerpt boundary, or
// unescaping text that only looks like a tag.
func TestPlainExcerptDropsEscapedArticleHTMLKeepsHighlight(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ in, want string }{
		{in: "&lt;p incomplete <mark>coverage</mark> stays", want: "<mark>coverage</mark> stays"},
		{in: "&lt;a incomplete <mark>coverage</mark> stays", want: "<mark>coverage</mark> stays"},
		{in: "Amount &lt;limit and <mark>coverage</mark> stays", want: "Amount &lt;limit and <mark>coverage</mark> stays"},
		{in: "Amount &lt;summaryLimit and <mark>coverage</mark> stays", want: "Amount &lt;summaryLimit and <mark>coverage</mark> stays"},
		{
			in:   "&lt;details&gt; &lt;summary&gt;<mark>LRC</mark>: remaining coverage&lt;/summary&gt; &lt;p&gt;&lt;strong&gt;Principle.&lt;/strong&gt; Services remain.&lt;/p&gt;",
			want: "<mark>LRC</mark>: remaining coverage Principle. Services remain.",
		},
		{
			in:   "&lt;p&gt;Откройте &lt;a href=&#34;/accounting/ifrs17/calc&#34;&gt;<mark>Результаты</mark>&lt;/a&gt;.&lt;/p&gt;",
			want: "Откройте <mark>Результаты</mark> .",
		},
		{
			in:   "<mark>CSM</mark> release. &lt;a href=&#34;/accounting/ifrs17/",
			want: "<mark>CSM</mark> release.",
		},
		{
			in:   "&lt;div class=&#34;help-task-card&#34;&gt;&lt;div class=&#34;help-task-meta&#34;&gt;&lt;span&gt;Глава 02&lt;/span&gt;&lt;/div&gt;&lt;div class=&#34;help-task-title&#34;&gt;<mark>LRC</mark>, LIC&lt;/div&gt;&lt;small&gt;Шаг 1&lt;/small&gt;&lt;b&gt;5 глав&lt;/b&gt;",
			want: "Глава 02 <mark>LRC</mark>, LIC Шаг 1 5 глав",
		},
	} {
		require.Equal(t, tc.want, plainExcerpt(tc.in))
	}
}

// False green: a fallback that slices bytes, which passes on ASCII and breaks
// a Cyrillic letter in half.
func TestSanitizeSearchExcerptFallsBackToPlainContent(t *testing.T) {
	t.Parallel()

	controller := &HelpCenterController{snippetPolicy: bluemonday.StrictPolicy().AllowElements("mark")}
	content := "## Цель\n\n<script>x</script> " + strings.Repeat("я", searchExcerptRunes)

	got := string(controller.sanitizeSearchExcerpt(kb.SearchResult{Document: kb.Document{Content: content}}))

	require.True(t, strings.HasPrefix(got, "Цель "), got)
	require.NotContains(t, got, "<script>")
	require.True(t, strings.HasSuffix(got, "я…"), got)
	require.NotContains(t, got, "�")
}

func newSearchTestController(searcher kb.KBSearcher) *HelpCenterController {
	doc := func(title string) *fstest.MapFile {
		return &fstest.MapFile{Data: []byte("# " + title + "\n\nТекст.")}
	}
	return &HelpCenterController{
		basePath:      "/help",
		searcher:      searcher,
		snippetPolicy: bluemonday.StrictPolicy().AllowElements("mark"),
		contentService: services.NewContentService(services.ContentConfig{
			FS: fstest.MapFS{
				"content/ru/insurance/osago.md":     doc("Выпуск полиса ОСАГО"),
				"content/ru/insurance/benefits.md":  doc("Льготы ОСАГО"),
				"content/ru/insurance/osago-sql.md": doc("SQL по ОСАГО"),
				"content/ru/internal/runbook.md":    doc("Ранбук"),
				"content/en/insurance/osago.md":     doc("Issue an OSAGO policy"),
			},
			Root:           "content",
			Locales:        []string{"en", "ru"},
			DefaultLocale:  "en",
			HiddenPaths:    []string{"internal", "insurance/osago-sql.md"},
			CategoryTitles: map[string]map[string]string{"ru": {"insurance": "01 Портфель"}},
		}),
	}
}

func renderSearch(t *testing.T, controller *HelpCenterController, locale language.Tag, query string) string {
	t.Helper()
	bundle := i18n.NewBundle(locale)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	_, err := bundle.LoadMessageFile("../locales/" + locale.String() + ".json")
	require.NoError(t, err)
	localizer := i18n.NewLocalizer(bundle, locale.String())

	req := httptest.NewRequest(http.MethodGet, "/help/search?q="+url.QueryEscape(query), nil)
	ctx := intl.WithLocalizer(intl.WithLocale(context.Background(), locale), localizer)
	ctx = composables.WithPageCtx(ctx, types.NewPageContext(locale, req.URL, localizer))
	rec := httptest.NewRecorder()

	controller.search(rec, req.WithContext(ctx))

	require.Equal(t, http.StatusOK, rec.Code)
	return rec.Body.String()
}
