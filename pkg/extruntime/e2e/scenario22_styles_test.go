package e2e_test

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var linkTagRe = regexp.MustCompile(`<link rel="stylesheet" href="([^"]*)" data-iota-ext-css/?>`)

// Scenario 22: a package styles its client without inline styles. It builds with the real build worker: the UI kit is
// shared with the host (not bundled), Tailwind utilities are compiled with the SDK design tokens for the classes the
// sources use, and a scoped CSS module is emitted; all of it is part of the immutable version artifact. The page links the
// version's stylesheets once, before the mount element; the slot contributions carry them for the browser host; the asset
// route serves them like any chunk, and they go away with the activation.
func TestScenario22PackageStylesheetsAreLinkedAndServedWithTheVersion(t *testing.T) {
	e := newEnv(t)
	admin := e.admin
	viewer := e.newActor("viewer", "Client.Read")
	id := e.newClient(admin, "Ann")

	e.deploy(admin, "e2e.styled", "1.0.0", loadTree(t, "styled"))

	// The page: both stylesheets of the version are linked in the HTML, once, utilities first, before the mount.
	body := e.get(viewer, "/styled/dashboard").String()
	tags := linkTagRe.FindAllStringSubmatch(body, -1)
	require.Len(t, tags, 2, "the version's two stylesheets, each once")
	utilities, scoped := tags[0][1], tags[1][1]
	require.Regexp(t, `^/_ext/assets/[0-9a-f]+/client/tailwind\.css$`, utilities)
	require.Regexp(t, `^/_ext/assets/[0-9a-f]+/client/index-[A-Z0-9]+\.css$`, scoped)
	mount := strings.Index(body, `id="iota-client-route-mount"`)
	require.Positive(t, mount)
	require.Less(t, strings.Index(body, utilities), mount, "linked before the page's mount element")
	pd := pageData(t, body)
	require.Equal(t, []string{utilities, scoped}, pd.Page.Styles, "the browser host gets the same list to ensure before the first render")

	// The asset route serves them like the chunks: private, immutable, text/css.
	css := e.get(viewer, utilities)
	require.Equal(t, http.StatusOK, css.Status)
	require.Contains(t, css.Header.Get("Content-Type"), "text/css")
	require.Equal(t, "private, max-age=31536000, immutable", css.Header.Get("Cache-Control"))
	require.Contains(t, css.String(), "oklch(var(--primary-600)", "bg-primary-600 resolved on the SDK tokens")
	require.Contains(t, css.String(), "padding:calc(var(--spacing) * 4)")
	require.NotContains(t, css.String(), "bg-fuchsia", "only the classes the sources use")
	scopedCSS := e.get(viewer, scoped).String()
	require.Regexp(t, `\.Dashboard_[0-9a-f]{8}_panel\b`, scopedCSS)
	require.Regexp(t, `\.Badge_[0-9a-f]{8}_badge\b`, scopedCSS)

	// The client manifest says the page needs the host's UI kit, and the page chunk reads it from the host.
	manifestURL := strings.Replace(utilities, "tailwind.css", "manifest.json", 1)
	var cm struct {
		Shared []string `json:"shared"`
		CSS    []string `json:"css"`
	}
	require.NoError(t, json.Unmarshal(e.get(viewer, manifestURL).Body, &cm))
	require.Contains(t, cm.Shared, "@iota-uz/sdk/solid-ui")
	require.Equal(t, []string{"client/tailwind.css", strings.TrimPrefix(scoped, utilities[:strings.Index(utilities, "client/")])}, cm.CSS)
	require.NotContains(t, e.get(viewer, pd.Page.Module).String(), "btn-loading-indicator", "the kit is the host's, not bundled in the entry chunk")

	// A slot contribution of the same version (a Templ/HTMX page): the island config carries the version's
	// stylesheets; the HTML carries no link of its own, because the browser host links them once per document.
	page := e.get(viewer, "/crm/clients/"+id).String()
	el := slotElements(t, page)[slotClientSidebar]
	require.Equal(t, []string{"e2e.styled.clientBadge"}, moduleIDs(el.Contributions))
	require.Equal(t, []string{utilities, scoped}, el.Contributions[0].Styles)
	require.Empty(t, linkTagRe.FindAllString(page, -1))

	// Deactivating the version takes the page, its stylesheets and the contribution away.
	e.mgmt(admin, http.MethodPost, "/packages/e2e.styled/deactivate", nil, http.StatusNoContent, nil)
	require.Eventually(t, func() bool { return e.get(viewer, "/styled/dashboard").Status == http.StatusNotFound },
		3*time.Second, 20*time.Millisecond)
	require.Equal(t, http.StatusNotFound, e.get(viewer, utilities).Status)
}
