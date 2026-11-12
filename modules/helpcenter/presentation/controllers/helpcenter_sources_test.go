package controllers

import (
	"context"
	"encoding/json"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/components/sidebar"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/helpcenter/internal/fakesource"
	"github.com/iota-uz/iota-sdk/modules/helpcenter/services"
	"github.com/iota-uz/iota-sdk/pkg/bichat/kb"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/helpdocs"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/markdown"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

const commandsDoc = `# Commands

Run [actions](actions), the [overview](../overview), the [overview tab](../overview?tab=1#x),
the [file doc](/help/doc/modules/crm.md), [site](https://example.test/x), [mail](mailto:a@example.test),
[proto](//cdn.example.test/x) and [here](#section).

<script>alert(1)</script>
`

func newSource() *fakesource.Source {
	return &fakesource.Source{
		MountPath: "extensions/reference",
		Names:     map[string]string{"": "Extension reference", "ru": "Справочник расширений"},
		Position:  10,
		Layout: []helpdocs.Node{
			{Title: "Overview", Path: "overview", Order: 1},
			{Title: "CRM", Path: "crm", Order: 2, Children: []helpdocs.Node{
				{Title: "Actions", Path: "crm/actions", Order: 1},
				{Title: "Commands", Path: "crm/commands", Order: 2},
			}},
			{Title: "Signing keys", Path: "keys", Order: 3},
		},
		Docs: map[string]fakesource.Doc{
			"overview":     {Title: "Overview", Markdown: "# Overview\n\nHello."},
			"crm/actions":  {Title: "Actions", Markdown: "# Actions\n\nAct."},
			"crm/commands": {Title: "Commands", Markdown: commandsDoc, Audience: "Developers", Updated: time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)},
			"keys":         {Title: "Signing keys", Markdown: "# Signing keys\n\nSecret.", Restricted: true},
		},
	}
}

// newSourceController builds the controller the way the component does: the
// content service derives from the file content plus the contributed sources.
func newSourceController(t *testing.T, searcher kb.KBSearcher, sources ...helpdocs.Source) *HelpCenterController {
	t.Helper()
	content, err := services.NewContentService(services.ContentConfig{
		FS: fstest.MapFS{
			"content/en/modules/crm.md":  &fstest.MapFile{Data: []byte("# CRM\n\nSee [other](other.md).")},
			"content/en/modules/keys.md": &fstest.MapFile{Data: []byte("# Keys and tokens\n\nFile doc.")},
		},
		Root:          "content",
		Locales:       []string{"en"},
		DefaultLocale: "en",
	}).WithSources(sources)
	require.NoError(t, err)
	controller, ok := NewHelpCenterController(HelpCenterControllerConfig{
		BasePath:       "/help",
		ContentService: content,
		Renderer:       markdown.NewRenderer(),
		Searcher:       searcher,
	}).(*HelpCenterController)
	require.True(t, ok)
	return controller
}

func person(t *testing.T, perms ...permission.Permission) user.User {
	t.Helper()
	email, err := internet.NewEmail("reader@example.com")
	require.NoError(t, err)
	return user.New("Reader", "", email, user.UILanguageEN, user.WithPermissions(perms))
}

// requestCtx fills the request context the way the application's middleware
// does for a signed-in reader with the given locale.
func requestCtx(t *testing.T, r *http.Request, reader user.User, locale language.Tag) context.Context {
	t.Helper()
	bundle := i18n.NewBundle(locale)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	_, err := bundle.LoadMessageFile("../locales/" + locale.String() + ".json")
	require.NoError(t, err)
	_, err = bundle.LoadMessageFile("../../../core/presentation/locales/" + locale.String() + ".json")
	require.NoError(t, err)
	localizer := i18n.NewLocalizer(bundle, locale.String())
	ctx := intl.WithLocalizer(intl.WithLocale(r.Context(), locale), localizer)
	ctx = composables.WithPageCtx(ctx, types.NewPageContext(locale, r.URL, localizer))
	ctx = context.WithValue(ctx, constants.SidebarPropsKey, sidebar.Props{})
	ctx = context.WithValue(ctx, constants.LogoKey, templ.NopComponent)
	ctx = context.WithValue(ctx, constants.HeadKey, templ.NopComponent)
	return composables.WithUser(ctx, reader)
}

// get serves target through the Help Center's router (route matching and path
// variables included) as reader, with the application's request context.
func get(t *testing.T, controller *HelpCenterController, reader user.User, locale language.Tag, target string, htmx bool) *httptest.ResponseRecorder {
	t.Helper()
	router := mux.NewRouter()
	sub := router.PathPrefix("/help").Subrouter()
	sub.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(requestCtx(t, r, reader, locale)))
		})
	})
	controller.routes(sub)
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if htmx {
		req.Header.Set("Hx-Request", "true")
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

var hrefPattern = regexp.MustCompile(`<a [^>]*href="([^"]*)"`)

func hrefs(body string) []string {
	matches := hrefPattern.FindAllStringSubmatch(body, -1)
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		out = append(out, match[1])
	}
	return out
}

// False green: calling controller.doc with hand-set mux vars, which skips the
// router's matching of /doc/{path:.*} for an extension-less, nested path.
func TestSources_DocRendersThroughRouterWithNavigation(t *testing.T) {
	controller := newSourceController(t, nil, newSource())

	rec := get(t, controller, person(t), language.English, "/help/doc/extensions/reference/crm/commands", true)

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	require.Contains(t, body, "<h1")
	require.Contains(t, body, "Commands</h1>")
	require.NotContains(t, body, "<script", "source documents go through the same sanitizer as file documents")
	meta := body[strings.Index(body, `class="help-article-meta"`):]
	meta = meta[:strings.Index(meta, "</div>")]
	require.Contains(t, meta, "<span>Developers</span>")
	require.Contains(t, meta, "<span>Updated November 4, 2026</span>")
	require.Contains(t, meta, "1 minute read")
	kicker := body[strings.Index(body, `class="help-article-kicker"`):]
	kicker = kicker[:strings.Index(kicker, "</nav>")]
	require.Contains(t, kicker, "<span>Extensions</span>")
	require.Contains(t, kicker, "<span>Extension reference</span>")
	require.Contains(t, kicker, "<span>CRM</span>")
	pager := body[strings.Index(body, `class="help-pager"`):]
	require.Contains(t, pager, `class="help-pager-prev"`)
	require.Contains(t, pager, `href="/help/doc/extensions/reference/crm/actions"`)
	require.NotContains(t, pager, "help-pager-next", "Commands is the last document of its category")
	require.Contains(t, body, "<span>extensions/reference/crm/commands</span>")
}

func TestSources_PagerLinksNeighboursOfTheSameCategory(t *testing.T) {
	controller := newSourceController(t, nil, newSource())

	rec := get(t, controller, person(t, permissions.UserRead), language.English, "/help/doc/extensions/reference/overview", true)

	require.Equal(t, http.StatusOK, rec.Code)
	pager := rec.Body.String()[strings.Index(rec.Body.String(), `class="help-pager"`):]
	require.Contains(t, pager, `class="help-pager-next"`)
	require.Contains(t, pager, `href="/help/doc/extensions/reference/keys"`)
	require.NotContains(t, pager, "help-pager-prev")
}

// False green: a fixture where every link is already absolute, and file
// documents rewritten too (their relative links must stay as authored).
func TestSources_RelativeLinksResolveAgainstTheSourceDocument(t *testing.T) {
	controller := newSourceController(t, nil, newSource())

	source := get(t, controller, person(t), language.English, "/help/doc/extensions/reference/crm/commands", true)
	file := get(t, controller, person(t), language.English, "/help/doc/modules/crm.md", true)

	require.Equal(t, http.StatusOK, source.Code)
	links := hrefs(source.Body.String())
	for _, want := range []string{
		"/help/doc/extensions/reference/crm/actions",
		"/help/doc/extensions/reference/overview",
		"/help/doc/extensions/reference/overview?tab=1#x",
		"/help/doc/modules/crm.md",
		"https://example.test/x",
		"mailto:a@example.test",
		"//cdn.example.test/x",
		"#section",
	} {
		require.Contains(t, links, want)
	}
	require.NotContains(t, links, "actions")
	require.NotContains(t, links, "../overview")
	require.Contains(t, hrefs(file.Body.String()), "other.md")
}

// False green: asserting only the status code. The error page must be the
// one a missing file document gets, and a forbidden document must be
// indistinguishable from a missing one.
func TestSources_NotFoundAndForbiddenDocumentsAre404(t *testing.T) {
	controller := newSourceController(t, nil, newSource())
	reader := person(t)

	missing := get(t, controller, reader, language.English, "/help/doc/extensions/reference/missing", true)
	forbidden := get(t, controller, reader, language.English, "/help/doc/extensions/reference/keys", true)
	allowed := get(t, controller, person(t, permissions.UserRead), language.English, "/help/doc/extensions/reference/keys", true)
	fileMissing := get(t, controller, reader, language.English, "/help/doc/modules/missing.md", true)
	badPath := get(t, controller, reader, language.English, "/help/doc/extensions/reference/overview/", true)

	require.Equal(t, http.StatusNotFound, missing.Code)
	require.Equal(t, http.StatusNotFound, forbidden.Code)
	require.Equal(t, missing.Body.String(), forbidden.Body.String())
	require.Equal(t, fileMissing.Body.String(), missing.Body.String(), "same 404 page as a missing file document")
	require.Equal(t, http.StatusNotFound, badPath.Code)
	require.Equal(t, http.StatusOK, allowed.Code)
}

// False green: one reader, or a sidebar rendered from a tree cached at
// construction; both readers hit the same source instance.
func TestSources_ReadersGetDifferentSidebars(t *testing.T) {
	controller := newSourceController(t, nil, newSource())
	keys := "/help/doc/extensions/reference/keys"

	admin := get(t, controller, person(t, permissions.UserRead), language.English, "/help/doc/extensions/reference/overview", false)
	plain := get(t, controller, person(t), language.English, "/help/doc/extensions/reference/overview", false)

	require.Equal(t, http.StatusOK, admin.Code)
	require.Equal(t, http.StatusOK, plain.Code)
	require.Contains(t, hrefs(admin.Body.String()), keys)
	require.NotContains(t, hrefs(plain.Body.String()), keys)
	require.Contains(t, hrefs(plain.Body.String()), "/help/doc/extensions/reference/overview")
	require.Contains(t, hrefs(plain.Body.String()), "/help/doc/modules/crm.md", "file documents are in both sidebars")
}

func TestSources_RequestLocaleReachesSource(t *testing.T) {
	source := newSource()
	controller := newSourceController(t, nil, source)

	rec := get(t, controller, person(t), language.Russian, "/help/doc/extensions/reference/overview", false)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "Справочник расширений")
	locales := source.Locales()
	require.NotEmpty(t, locales)
	for _, locale := range locales {
		require.Equal(t, "ru", locale)
	}
}

// False green: a searcher that answers every query, where hiding source hits
// from a reader who may not see them is never exercised.
func TestSources_SearchMatchesTitlesPerReader(t *testing.T) {
	controller := newSourceController(t, &stubSearcher{results: []kb.SearchResult{
		searchHit("en/modules/keys.md", "Keys and tokens", "file <mark>keys</mark>"),
	}}, newSource())
	keys := "/help/doc/extensions/reference/keys"

	allowed := get(t, controller, person(t, permissions.UserRead), language.English, "/help/search?q="+url.QueryEscape("signing KEYS"), false)
	forbidden := get(t, controller, person(t), language.English, "/help/search?q="+url.QueryEscape("signing keys"), false)
	viaCategory := get(t, controller, person(t), language.English, "/help/search?q=crm", false)
	none := get(t, controller, person(t), language.English, "/help/search?q=zzzz", false)

	require.Equal(t, http.StatusOK, allowed.Code)
	require.Contains(t, hrefs(allowed.Body.String()), keys)
	require.Contains(t, allowed.Body.String(), "Signing keys")
	require.Contains(t, allowed.Body.String(), "Extension reference", "hit names its sidebar category")
	require.NotContains(t, forbidden.Body.String(), "Signing keys", "a reader without access gets no title leak")
	require.NotContains(t, hrefs(forbidden.Body.String()), keys)
	require.Contains(t, hrefs(forbidden.Body.String()), "/help/doc/modules/keys.md", "the index still answers for file documents")
	require.Contains(t, hrefs(viaCategory.Body.String()), "/help/doc/extensions/reference/crm/commands", "a category title matches the documents below it")
	require.Contains(t, hrefs(viaCategory.Body.String()), "/help/doc/extensions/reference/crm/actions")
	require.NotContains(t, none.Body.String(), "/help/doc/extensions/")
}

func TestSources_SearchRanksTitleMatchesAndLeavesRoomForTheIndex(t *testing.T) {
	source := newSource()
	source.Layout = nil
	source.Docs = map[string]fakesource.Doc{}
	for _, name := range []string{"a", "b", "c", "d", "e", "f"} {
		source.Layout = append(source.Layout, helpdocs.Node{Title: "Deploy " + name, Path: name})
		source.Docs[name] = fakesource.Doc{Title: "Deploy " + name, Markdown: "# Deploy"}
	}
	source.Layout = append(source.Layout, helpdocs.Node{Title: "Setup", Path: "setup"})
	source.Docs["setup"] = fakesource.Doc{Title: "Setup", Markdown: "# Setup"}
	names := []string{"one", "two", "three", "four", "five", "six"}
	indexed := make([]kb.SearchResult, 0, len(names))
	for _, name := range names {
		indexed = append(indexed, searchHit("en/guides/"+name+".md", "Guide "+name, ""))
	}
	controller := newSourceController(t, &stubSearcher{results: indexed}, source)

	rec := get(t, controller, person(t), language.English, "/help/search?q=deploy", false)

	links := hrefs(rec.Body.String())
	require.Len(t, links, searchResultLimit)
	require.Equal(t, "/help/doc/extensions/reference/a", links[0])
	require.Equal(t, sourceHitLimit, countPrefix(links, "/help/doc/extensions/reference/"), "source hits are capped")
	require.Equal(t, searchResultLimit-sourceHitLimit, countPrefix(links, "/help/doc/guides/"), "full-text hits keep the rest")
}

// False green: a controller that always has a searcher; without BiChat the
// sources are the only searchable content, and without sources nothing is.
func TestSources_SearchWithoutIndex(t *testing.T) {
	withSources := newSourceController(t, nil, newSource())
	without := newSourceController(t, nil)

	found := get(t, withSources, person(t), language.English, "/help/search?q=overview", false)
	unavailable := get(t, without, person(t), language.English, "/help/search?q=overview", false)

	require.Contains(t, hrefs(found.Body.String()), "/help/doc/extensions/reference/overview")
	require.True(t, withSources.searchAvailable())
	require.False(t, without.searchAvailable())
	require.NotContains(t, unavailable.Body.String(), "/help/doc/")
}

func TestSources_FileSearchIsUnchangedWithoutSources(t *testing.T) {
	controller := newSourceController(t, &stubSearcher{results: []kb.SearchResult{
		searchHit("en/modules/crm.md", "CRM", "about <mark>crm</mark>"),
	}})

	rec := get(t, controller, person(t), language.English, "/help/search?q=crm", false)

	require.Equal(t, []string{"/help/doc/modules/crm.md"}, hrefs(rec.Body.String()))
}

func countPrefix(values []string, prefix string) int {
	count := 0
	for _, value := range values {
		if strings.HasPrefix(value, prefix) {
			count++
		}
	}
	return count
}

func TestResolveRelativeLinks(t *testing.T) {
	const docURL = "/help/doc/ext/guides/intro"
	tests := map[string]string{
		`<a href="next">n</a>`:                          `<a href="/help/doc/ext/guides/next">n</a>`,
		`<a href="./next">n</a>`:                        `<a href="/help/doc/ext/guides/next">n</a>`,
		`<a href="../reference/crm">n</a>`:              `<a href="/help/doc/ext/reference/crm">n</a>`,
		`<a href="../../../../x">n</a>`:                 `<a href="/x">n</a>`,
		`<a href="next?a=1&amp;b=2#frag">n</a>`:         `<a href="/help/doc/ext/guides/next?a=1&amp;b=2#frag">n</a>`,
		`<a href="/abs/path">n</a>`:                     `<a href="/abs/path">n</a>`,
		`<a href="#frag">n</a>`:                         `<a href="#frag">n</a>`,
		`<a href="https://example.test/a">n</a>`:        `<a href="https://example.test/a">n</a>`,
		`<a href="mailto:a@example.test">n</a>`:         `<a href="mailto:a@example.test">n</a>`,
		`<a href="//cdn.example.test/a">n</a>`:          `<a href="//cdn.example.test/a">n</a>`,
		`<a title="t" href="next" rel="nofollow">n</a>`: `<a title="t" href="/help/doc/ext/guides/next" rel="nofollow">n</a>`,
		`<img src="pic.png"/>`:                          `<img src="pic.png"/>`,
		`&lt;a href="next"&gt;`:                         `&lt;a href="next"&gt;`,
	}
	for in, want := range tests {
		t.Run(in, func(t *testing.T) {
			require.Equal(t, want, string(resolveRelativeLinks(template.HTML(in), docURL)))
		})
	}
}
