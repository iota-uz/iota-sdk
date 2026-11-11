package e2e_test

import (
	"encoding/json"
	"html"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/ui"
	"github.com/iota-uz/iota-sdk/pkg/shellext"
)

type cellElement struct {
	Contribution shellext.Module `json:"contribution"`
	Value        any             `json:"value"`
	Row          map[string]any  `json:"row"`
	Env          ui.IslandEnv    `json:"env"`
}

// cellElements parses the cell mount elements of a list page (or its HTMX fragment), in row order.
func cellElements(t *testing.T, body string) []cellElement {
	t.Helper()
	matches := cellElementRe.FindAllStringSubmatch(body, -1)
	out := make([]cellElement, 0, len(matches))
	for _, m := range matches {
		var el cellElement
		require.NoError(t, json.Unmarshal([]byte(html.UnescapeString(m[2])), &el), m[2])
		require.Equal(t, el.Contribution.ID, html.UnescapeString(m[1]), "the element names its renderer")
		out = append(out, el)
	}
	return out
}

// Scenario 16: a package cell renderer for a column another package adds to
// the native clients list. The list stays server-rendered (the standard cell
// is the fallback inside the mount element); the renderer is an island the
// browser mounts; filters, sorting and exports keep using the raw value.
func TestScenario16CellRendererIsAnIslandAndExportsStayRaw(t *testing.T) {
	e := newEnv(t)
	admin := e.admin
	manager := e.newActor("manager", "Client.Read", permViewIntern)
	viewer := e.newActor("viewer", "Client.Read")
	ann := e.newClient(admin, "Ann")
	bob := e.newClient(admin, "Bob")

	e.deploy(admin, "e2e.clientextra", "1.0.0", loadTree(t, "clientextra"))
	var out struct {
		Tier string `json:"tier"`
	}
	e.mustRPC(admin, "command", "e2e.clientextra.setProfile",
		map[string]any{"client": map[string]string{"entity": "crm.client", "key": ann}, "tier": "gold", "score": 30, "internalNote": "board-member"}, &out)

	// The column without a renderer: the standard cells, no island, no runtime host.
	body := e.get(manager, "/crm/clients").String()
	require.ElementsMatch(t, []string{"gold", "standard"}, cells(body, colTier))
	require.NotContains(t, body, ui.CellAttr)
	require.False(t, hostScriptRe.MatchString(body))

	e.deploy(admin, "e2e.tiercell", "1.0.0", loadTree(t, "tiercell"))

	// With the renderer: every cell of the column is an island whose fallback is the standard cell, so the page
	// reads the same without JavaScript. The island carries the raw value and the row's readable values.
	body = e.get(manager, "/crm/clients").String()
	require.ElementsMatch(t, []string{"gold", "standard"}, cells(body, colTier), "the fallback is the standard cell")
	islands := cellElements(t, body)
	require.Len(t, islands, 2)
	require.Len(t, hostScriptRe.FindAllString(body, -1), 1, "the runtime host is written once for all cells")
	byValue := map[any]cellElement{}
	for _, el := range islands {
		byValue[el.Value] = el
		require.Equal(t, "e2e.tiercell.tierCell", el.Contribution.ID)
		require.Equal(t, "tierCell", el.Contribution.Export)
		require.Regexp(t, `^/_ext/assets/[0-9a-f]+/client/`, el.Contribution.Module)
		require.Equal(t, "/_ext/rpc", el.Env.RPC)
	}
	gold := byValue["gold"]
	require.Equal(t, map[string]any{"entity": "crm.client", "key": ann}, gold.Row["ref"])
	require.Equal(t, "gold", gold.Row[colTier])
	require.EqualValues(t, 30, gold.Row[colScore], "an int column arrives as a number")
	require.Equal(t, "board-member", gold.Row[colInternal], "the manager may read the internal note")
	require.Equal(t, map[string]any{"entity": "crm.client", "key": bob}, byValue["standard"].Row["ref"])
	require.Nil(t, byValue["standard"].Row[colScore], "an absent value is null")
	js := e.get(manager, gold.Contribution.Module)
	require.Equal(t, http.StatusOK, js.Status)

	// The island carries only what the viewer may read: no hidden column in the row.
	vbody := e.get(viewer, "/crm/clients").String()
	for _, el := range cellElements(t, vbody) {
		require.NotContains(t, el.Row, colInternal)
	}
	require.NotContains(t, vbody, "board-member")

	// The HTMX fragments that filter and sort carry islands too, and the filter still works on the raw value.
	frag := e.clientsTable(manager, url.Values{"ext." + colTier: {"gold"}})
	require.Len(t, cellElements(t, frag), 1)
	require.Equal(t, []string{"Ann"}, order(frag, "Ann", "Bob"))

	// Exports never use the renderer: the raw value, nothing of the island.
	csv := string(e.exportClients(manager, "csv", url.Values{}))
	require.Contains(t, csv, "gold")
	require.NotContains(t, csv, "GOLD")
	require.NotContains(t, csv, ui.CellAttr)
	rows := sheet(t, e.exportClients(manager, "excel", url.Values{}))
	var annRow []string
	for _, r := range rows {
		if len(r) > 0 && strings.HasPrefix(r[0], "Ann") {
			annRow = r
		}
	}
	require.NotNil(t, annRow)
	require.Contains(t, annRow, "gold")

	// Deactivating the renderer's package returns the standard list; the column stays.
	e.mgmt(admin, http.MethodPost, "/packages/e2e.tiercell/deactivate", nil, http.StatusNoContent, nil)
	require.Eventually(t, func() bool {
		body = e.get(manager, "/crm/clients").String()
		return !strings.Contains(body, ui.CellAttr)
	}, 3*time.Second, 20*time.Millisecond, "the renderer vanishes with the package")
	require.ElementsMatch(t, []string{"gold", "standard"}, cells(body, colTier))
}
