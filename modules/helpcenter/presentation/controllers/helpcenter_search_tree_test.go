package controllers

import (
	"io/fs"
	"net/http"
	"testing"
	"testing/fstest"

	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"

	"github.com/iota-uz/iota-sdk/modules/helpcenter/services"
	"github.com/iota-uz/iota-sdk/pkg/bichat/kb"
	"github.com/iota-uz/iota-sdk/pkg/markdown"
)

// unreadableFS lists its files but fails to open one of them: the sidebar tree
// cannot be built. It implements only Open, so fs.Sub and fs.ReadFile go
// through it.
type unreadableFS struct{ inner fs.FS }

func (u unreadableFS) Open(name string) (fs.File, error) {
	if name == "content/en/modules/keys.md" {
		return nil, fs.ErrPermission
	}
	return u.inner.Open(name)
}

// The sidebar tree supplies the titles of the documentation sources to the
// search. When it cannot be built the search still answers from the index, and
// says why it ran without the sources instead of dropping the error.
func TestSearchLogsAnUnavailableTreeAndAnswersFromTheIndex(t *testing.T) {
	content, err := services.NewContentService(services.ContentConfig{
		FS: unreadableFS{fstest.MapFS{
			"content/en/modules/keys.md": &fstest.MapFile{Data: []byte("# Keys\n")},
		}},
		Root: "content", Locales: []string{"en"}, DefaultLocale: "en",
	}).WithSources(nil)
	require.NoError(t, err)
	controller, ok := NewHelpCenterController(HelpCenterControllerConfig{
		BasePath: "/help", ContentService: content, Renderer: markdown.NewRenderer(),
		Searcher: &stubSearcher{results: []kb.SearchResult{searchHit("en/modules/keys.md", "Keys", "file <mark>keys</mark>")}},
	}).(*HelpCenterController)
	require.True(t, ok)

	hook := logrustest.NewGlobal()
	defer hook.Reset()
	rec := get(t, controller, person(t), language.English, "/help/search?q=keys", false)
	require.Equal(t, http.StatusOK, rec.Code)

	var warned bool
	for _, e := range hook.AllEntries() {
		if e.Message == "help center search ran without the documentation sources" {
			warned = true
			require.Error(t, e.Data["error"].(error))
		}
	}
	require.True(t, warned, "the tree error is logged: %v", hook.AllEntries())
}
