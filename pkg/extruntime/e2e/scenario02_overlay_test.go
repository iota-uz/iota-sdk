package e2e_test

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"

	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	crmServices "github.com/iota-uz/iota-sdk/modules/crm/services"
)

const (
	colTier     = "e2e.clientextra.tier"
	colScore    = "e2e.clientextra.score"
	colInternal = "e2e.clientextra.internal"
)

// newClient creates a CRM client through the CRM service and returns its key.
func (e *env) newClient(by *actor, first string) string {
	e.t.Helper()
	c, err := client.New(first, client.WithLastName("Doe"), client.WithTenantID(e.tenant))
	require.NoError(e.t, err)
	svc := resolve[*crmServices.ClientService](e.t, e.scope.Container)
	require.NoError(e.t, svc.Create(e.userContext(by), c))
	var id int64
	require.NoError(e.t, e.pool.QueryRow(context.Background(),
		`SELECT id FROM clients WHERE tenant_id = $1 AND first_name = $2`, e.tenant, first).Scan(&id))
	return itoa(uint(id))
}

// clientsTable fetches the rows of the clients list (the HTMX fragment the
// filter and sort controls refresh).
func (e *env) clientsTable(a *actor, q url.Values) string {
	e.t.Helper()
	r := e.do(a, http.MethodGet, "/crm/clients?"+q.Encode(), nil, "HX-Request", "true", "HX-Target", "clients-table-body")
	require.Equal(e.t, http.StatusOK, r.Status, r.String())
	return r.String()
}

// order returns the first names in the order they appear in the body.
func order(body string, names ...string) []string {
	type pos struct {
		name string
		at   int
	}
	var found []pos
	for _, n := range names {
		if i := strings.Index(body, n+" Doe"); i >= 0 {
			found = append(found, pos{n, i})
		}
	}
	for i := range found {
		for j := i + 1; j < len(found); j++ {
			if found[j].at < found[i].at {
				found[i], found[j] = found[j], found[i]
			}
		}
	}
	out := make([]string, len(found))
	for i, f := range found {
		out[i] = f.name
	}
	return out
}

// exportClients posts the list export and returns the file the application
// stored for the download.
func (e *env) exportClients(a *actor, format string, q url.Values) []byte {
	e.t.Helper()
	q.Set("format", format)
	r := e.do(a, http.MethodPost, "/crm/clients/export?"+q.Encode(), nil)
	require.Equal(e.t, http.StatusSeeOther, r.Status, r.String())
	// The redirect names the stored download (content-addressed, so equal
	// exports share a file); the file lives under the application's uploads.
	loc := r.Header.Get("Location")
	require.NotEmpty(e.t, loc)
	b, err := os.ReadFile(strings.TrimPrefix(loc, "/"))
	require.NoError(e.t, err, loc)
	return b
}

func sheet(t *testing.T, content []byte) [][]string {
	t.Helper()
	f, err := excelize.OpenReader(bytes.NewReader(content))
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	rows, err := f.GetRows(f.GetSheetName(0))
	require.NoError(t, err)
	return rows
}

// Scenario 2: an overlay of crm.client contributed by an extension package
// appears in the native clients list (filter and sort) and in the CSV and
// Excel export; without the package the list and export are the standard
// ones, and a field the viewer may not read is neither shown nor exported.
func TestScenario02OverlayColumnsInClientListAndExport(t *testing.T) {
	e := newEnv(t)
	const ns = "e2e.clientextra"
	admin := e.admin
	viewer := e.newActor("viewer", "Client.Read")
	manager := e.newActor("manager", "Client.Read", permViewIntern)
	names := []string{"Ann", "Bob", "Cid"}
	newestFirst := []string{"Cid", "Bob", "Ann"} // the list orders by creation, newest first
	key := map[string]string{}
	for _, n := range names {
		key[n] = e.newClient(admin, n)
	}

	// Without the package: the standard list, no extension controls, the
	// standard three export columns; extension parameters change nothing.
	body := e.get(viewer, "/crm/clients").String()
	require.Contains(t, body, "Ann Doe")
	require.NotContains(t, body, "Loyalty tier")
	require.NotContains(t, body, `name="ext.sort"`)
	stale := url.Values{"ext." + colTier: {"gold"}}
	require.Equal(t, newestFirst, order(e.clientsTable(viewer, stale), names...), "an unknown column filter is ignored")
	csv := strings.TrimPrefix(string(e.exportClients(viewer, "csv", url.Values{})), "\ufeff")
	require.Len(t, strings.Split(strings.TrimSpace(strings.Split(csv, "\n")[0]), ","), 3, csv)

	// The package is authored, built and activated; the overlay table appears.
	act := e.deploy(admin, ns, "1.0.0", loadTree(t, "clientextra"))
	require.NotEmpty(t, act.SchemaSteps)

	// Absent overlay rows read as the declared default / empty.
	body = e.get(manager, "/crm/clients").String()
	require.Contains(t, body, "Loyalty tier")
	require.Contains(t, body, `name="ext.`+colTier+`"`, "filter control of the contributed column")
	require.Contains(t, body, `name="ext.sort"`)
	require.Contains(t, body, "standard", "a client without an overlay row reads the declared default")
	require.NotContains(t, cells(body, colTier), "gold")

	// The command fills the overlay for two clients.
	var out struct {
		Tier  string `json:"tier"`
		Score *int   `json:"score"`
	}
	e.mustRPC(admin, "command", "e2e.clientextra.setProfile",
		map[string]any{"client": map[string]string{"entity": "crm.client", "key": key["Ann"]}, "tier": "gold", "score": 30, "internalNote": "board-member"}, &out)
	require.Equal(t, "gold", out.Tier)
	require.NotNil(t, out.Score)
	e.mustRPC(admin, "command", "e2e.clientextra.setProfile",
		map[string]any{"client": map[string]string{"entity": "crm.client", "key": key["Cid"]}, "tier": "silver", "score": 10}, &out)
	// A later update changes only what it names.
	e.mustRPC(admin, "command", "e2e.clientextra.setProfile",
		map[string]any{"client": map[string]string{"entity": "crm.client", "key": key["Ann"]}, "score": 31}, &out)
	require.Equal(t, "gold", out.Tier)
	require.Equal(t, http.StatusForbidden, e.rpc(viewer, "command", "e2e.clientextra.setProfile",
		map[string]any{"client": map[string]string{"entity": "crm.client", "key": key["Bob"]}, "tier": "gold"}).Status)

	// List: values, filter and sort.
	body = e.get(manager, "/crm/clients").String()
	require.ElementsMatch(t, []string{"gold", "silver", "standard"}, cells(body, colTier))
	require.Contains(t, body, "board-member", "the manager may read the internal note")
	require.Equal(t, []string{"Cid"}, order(e.clientsTable(manager, url.Values{"ext." + colTier: {"silver"}}), names...))
	require.Equal(t, []string{"Ann", "Cid", "Bob"}, order(e.clientsTable(manager, url.Values{"ext.sort": {colScore}, "ext.dir": {"desc"}}), names...),
		"descending by score; a client without a score sorts last")
	asc := order(e.clientsTable(manager, url.Values{"ext.sort": {colScore}, "ext.dir": {"asc"}}), names...)
	require.Less(t, indexOf(asc, "Cid"), indexOf(asc, "Ann"), "ascending by score: %v", asc)
	require.Equal(t, []string{"Ann"}, order(e.clientsTable(manager, url.Values{"extfrom." + colScore: {"11"}}), names...), "range filter")
	require.Equal(t, []string{"Ann"}, order(e.clientsTable(manager, url.Values{"ext." + colInternal: {"board-member"}}), names...))

	// The viewer, without the internal-note permission, sees the other columns
	// and neither the column, nor its values, nor can filter by it.
	body = e.get(viewer, "/crm/clients").String()
	require.Contains(t, body, "Loyalty tier")
	require.NotContains(t, body, "Internal note")
	require.NotContains(t, body, `name="ext.`+colInternal+`"`)
	require.NotContains(t, body, "board-member")
	require.Equal(t, newestFirst, order(e.clientsTable(viewer, url.Values{"ext." + colInternal: {"board-member"}}), names...), "a hidden column is not a filter")

	// CSV export carries the contributed exportable columns and the list filters.
	csv = strings.TrimPrefix(string(e.exportClients(manager, "csv", url.Values{})), "\ufeff")
	lines := strings.Split(strings.TrimSpace(csv), "\n")
	require.Len(t, lines, 4, csv)
	for _, h := range []string{"Loyalty tier", "Score", "Internal note"} {
		require.Contains(t, lines[0], h)
	}
	require.Contains(t, csv, "gold")
	require.Contains(t, csv, "board-member")
	require.Contains(t, csv, "standard", "defaults are exported for clients without an overlay row")
	filtered := string(e.exportClients(manager, "csv", url.Values{"ext." + colTier: {"gold"}}))
	require.Contains(t, filtered, "Ann")
	require.NotContains(t, filtered, "Cid")
	require.NotContains(t, filtered, "Bob")

	// ... but not the hidden field, for a viewer without the permission.
	vcsv := string(e.exportClients(viewer, "csv", url.Values{}))
	require.Contains(t, vcsv, "Loyalty tier")
	require.NotContains(t, vcsv, "Internal note")
	require.NotContains(t, vcsv, "board-member")

	// Excel export.
	rows := sheet(t, e.exportClients(manager, "excel", url.Values{}))
	require.Len(t, rows, 4)
	require.Contains(t, rows[0], "Loyalty tier")
	var ann []string
	for _, r := range rows {
		if len(r) > 0 && strings.HasPrefix(r[0], "Ann") {
			ann = r
		}
	}
	require.NotNil(t, ann)
	require.Contains(t, ann, "gold")
	require.Contains(t, ann, "31")
	vrows := sheet(t, e.exportClients(viewer, "excel", url.Values{}))
	require.NotContains(t, vrows[0], "Internal note")
	for _, r := range vrows {
		require.NotContains(t, r, "board-member")
	}

	// Deactivating the package returns the standard list and export.
	e.mgmt(admin, http.MethodPost, "/packages/"+ns+"/deactivate", nil, http.StatusNoContent, nil)
	body = e.get(manager, "/crm/clients").String()
	require.NotContains(t, body, "Loyalty tier")
	csv = strings.TrimPrefix(string(e.exportClients(manager, "csv", url.Values{})), "\ufeff")
	require.Len(t, strings.Split(strings.TrimSpace(strings.Split(csv, "\n")[0]), ","), 3)
}

var cellRe = map[string]*regexp.Regexp{}

// cells returns the rendered values of a contributed column, one per row.
func cells(body, col string) []string {
	re, ok := cellRe[col]
	if !ok {
		re = regexp.MustCompile(`data-ext-cell="` + regexp.QuoteMeta(col) + `"[^>]*>\s*([^<]*?)\s*<`)
		cellRe[col] = re
	}
	matches := re.FindAllStringSubmatch(body, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		out = append(out, m[1])
	}
	return out
}

func indexOf(list []string, v string) int {
	for i, x := range list {
		if x == v {
			return i
		}
	}
	return -1
}
