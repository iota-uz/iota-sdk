package e2e_test

import (
	"encoding/json"
	"html"
	"net/http"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/ui"
	"github.com/iota-uz/iota-sdk/pkg/shellext"
)

const (
	permSidebarIntern = "e2e.sidebar.intern"
	permBoardManage   = "e2e.board.manage"
	slotClientSidebar = "crm.slots.clientSidebar"
	slotBoardWidgets  = "e2e.board.widgets"
)

var (
	slotElementRe = regexp.MustCompile(`<div ` + ui.SlotAttr + `="([^"]*)" ` + ui.SlotConfigAttr + `="([^"]*)" hx-disable></div>`)
	cellElementRe = regexp.MustCompile(`<span ` + ui.CellAttr + `="([^"]*)" ` + ui.CellConfigAttr + `="([^"]*)" hx-disable>`)
	clientCtxRe   = regexp.MustCompile(`<script id="iota-client-context" type="application/json">(.*?)</script>`)
	hostScriptRe  = regexp.MustCompile(`<script type="module" src="/assets/runtime-host.js"`)
)

type slotElement struct {
	Props         map[string]any    `json:"props"`
	Contributions []shellext.Module `json:"contributions"`
	Env           ui.IslandEnv      `json:"env"`
}

// slotElements parses the slot mount elements of a server-rendered page by slot ID.
func slotElements(t *testing.T, body string) map[string]slotElement {
	t.Helper()
	out := map[string]slotElement{}
	for _, m := range slotElementRe.FindAllStringSubmatch(body, -1) {
		var el slotElement
		require.NoError(t, json.Unmarshal([]byte(html.UnescapeString(m[2])), &el), m[2])
		out[html.UnescapeString(m[1])] = el
	}
	return out
}

// pageData reads the PageData of a runtime page from its client context.
func pageData(t *testing.T, body string) ui.PageData {
	t.Helper()
	m := clientCtxRe.FindStringSubmatch(body)
	require.NotNil(t, m, "the page carries a client context")
	var ctx struct {
		Initial ui.PageData `json:"initial"`
	}
	require.NoError(t, json.Unmarshal([]byte(m[1]), &ctx))
	return ctx.Initial
}

func moduleIDs(mods []shellext.Module) []string {
	ids := make([]string, 0, len(mods))
	for _, c := range mods {
		ids = append(ids, c.ID)
	}
	return ids
}

// Scenario 15: packages contribute components to slots. The CRM declares a
// native slot on its client view (page and drawer, Templ/HTMX); another
// package declares a slot on a Solid page of its own. Contributions are
// additive (no binding), ordered, each filtered by its own permission on the
// server, and appear and vanish with activation, without a restart.
func TestScenario15SlotContributionsOnLegacyAndRuntimePages(t *testing.T) {
	e := newEnv(t)
	admin := e.admin
	viewer := e.newActor("viewer", "Client.Read")
	staff := e.newActor("staff", "Client.Read", permSidebarIntern, permBoardManage)
	id := e.newClient(admin, "Ann")
	clientPage := "/crm/clients/" + id

	// No package: the pages render, the slots are empty and nothing loads the runtime.
	body := e.get(viewer, clientPage).String()
	require.Contains(t, body, "Ann Doe")
	require.Contains(t, body, `data-testid="crm-client-sidebar"`)
	require.Empty(t, slotElements(t, body))
	require.False(t, hostScriptRe.MatchString(body), "no contribution, no runtime host")

	// The board declares a slot of its own; the sidebar package contributes to the CRM's and to the board's.
	e.deploy(admin, "e2e.board", "1.0.0", loadTree(t, "board"))
	e.deploy(admin, "e2e.sidebar", "1.0.0", loadTree(t, "sidebar"))

	// Legacy full page: the mount element carries the typed props and the contributions this viewer gets,
	// in order (early -5, profile 10); the intern note needs a permission the viewer lacks.
	body = e.get(viewer, clientPage).String()
	els := slotElements(t, body)
	require.Len(t, els, 1, "one slot on the page")
	sidebar := els[slotClientSidebar]
	require.Equal(t, map[string]any{"clientId": float64(mustAtoi(t, id))}, sidebar.Props)
	require.Equal(t, []string{"e2e.sidebar.early", "e2e.sidebar.profile"}, moduleIDs(sidebar.Contributions))
	require.NotContains(t, body, "e2e.sidebar.internNote", "a contribution the viewer may not have is absent from the HTML")
	require.Equal(t, "/_ext/rpc", sidebar.Env.RPC)
	require.Len(t, hostScriptRe.FindAllString(body, -1), 1, "the runtime host is written once")
	module := sidebar.Contributions[0].Module
	require.Regexp(t, `^/_ext/assets/[0-9a-f]+/client/`, module)
	js := e.get(viewer, module)
	require.Equal(t, http.StatusOK, js.Status)
	require.Contains(t, js.Header.Get("Content-Type"), "javascript")

	// The member with the permission gets the third contribution, after the others.
	body = e.get(staff, clientPage).String()
	require.Equal(t, []string{"e2e.sidebar.early", "e2e.sidebar.profile", "e2e.sidebar.internNote"},
		moduleIDs(slotElements(t, body)[slotClientSidebar].Contributions))

	// The drawer (an HTMX fragment) carries the same slot, and its own copy of the host so it is self-contained.
	r := e.do(staff, http.MethodGet, "/crm/clients?tab=profile&view="+id, nil, "HX-Request", "true")
	require.Equal(t, http.StatusOK, r.Status)
	require.Equal(t, []string{"e2e.sidebar.early", "e2e.sidebar.profile", "e2e.sidebar.internNote"},
		moduleIDs(slotElements(t, r.String())[slotClientSidebar].Contributions))
	require.Regexp(t, hostScriptRe, r.String())

	// A Solid page of a package: the contributions of its own slot arrive with the page data. The board's
	// own widgets and the sidebar package's, in order (10, 20) for everybody; the manager widget (30) only
	// for the member with the board permission.
	pd := pageData(t, e.get(viewer, "/board").String())
	require.Equal(t, "e2e.board.board", pd.Page.ID)
	require.Equal(t, []string{"e2e.board.clock", "e2e.sidebar.boardNote"}, moduleIDs(pd.Slots[slotBoardWidgets]))
	// A Solid page (or the shell renderer) may mount any slot, so the page data lists every slot the viewer gets
	// something in, and nothing the viewer may not have.
	require.Equal(t, []string{"e2e.sidebar.early", "e2e.sidebar.profile"}, moduleIDs(pd.Slots[slotClientSidebar]))
	pd = pageData(t, e.get(staff, "/board").String())
	require.Equal(t, []string{"e2e.board.clock", "e2e.sidebar.boardNote", "e2e.board.admin"}, moduleIDs(pd.Slots[slotBoardWidgets]))

	// Deactivating the contributing package takes its components away, the board's own remain.
	e.mgmt(admin, http.MethodPost, "/packages/e2e.sidebar/deactivate", nil, http.StatusNoContent, nil)
	require.Eventually(t, func() bool { return len(slotElements(t, e.get(staff, clientPage).String())) == 0 },
		3*time.Second, 20*time.Millisecond, "the contributions vanish with the package")
	pd = pageData(t, e.get(staff, "/board").String())
	require.Equal(t, []string{"e2e.board.clock", "e2e.board.admin"}, moduleIDs(pd.Slots[slotBoardWidgets]))
}

func mustAtoi(t *testing.T, s string) int {
	t.Helper()
	var n int
	require.NoError(t, json.Unmarshal([]byte(s), &n))
	return n
}
