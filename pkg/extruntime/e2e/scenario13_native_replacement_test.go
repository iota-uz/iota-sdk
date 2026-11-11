package e2e_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage"
)

const (
	cmdSaveClient   = "crm.commands.saveClient"
	opUpsertClient  = "crm.operations.upsertClient"
	guardClientName = "e2e.clientguard.noBlockedNames"
)

// createClientForm submits the CRM "new client" form as the browser does.
func (e *env) createClientForm(a *actor, first, last, phone string) resp {
	e.t.Helper()
	return e.postForm(a, "/crm/clients", url.Values{"FirstName": {first}, "LastName": {last}, "Phone": {phone}})
}

// clientName reads a client's names from the CRM table.
func (e *env) clientName(phone string) (first, last string, found bool) {
	e.t.Helper()
	err := e.pool.QueryRow(context.Background(),
		`SELECT first_name, last_name FROM clients WHERE tenant_id = $1 AND regexp_replace(phone_number, '\D', '', 'g') = $2`, e.tenant, digits(phone)).Scan(&first, &last)
	if err != nil {
		return "", "", false
	}
	return first, last, true
}

func digits(phone string) string { return strings.TrimPrefix(phone, "+") }

func (e *env) clientIDByPhone(phone string) string {
	e.t.Helper()
	var id int64
	require.NoError(e.t, e.pool.QueryRow(context.Background(),
		`SELECT id FROM clients WHERE tenant_id = $1 AND regexp_replace(phone_number, '\D', '', 'g') = $2`, e.tenant, digits(phone)).Scan(&id))
	return itoa(uint(id))
}

// lastSaveBinding is the package recorded on the newest successful
// invocation of the client form command ("" for the Go default).
func (e *env) lastSaveBinding() string {
	e.t.Helper()
	return e.scalarString(`SELECT binding_package FROM extruntime.invocations WHERE contract = $1 AND outcome = 'ok'
		ORDER BY started_at DESC LIMIT 1`, cmdSaveClient)
}

// Scenario 13: a published native command (the CRM client form command with
// a Go default) is replaced by a package that writes through the published
// native operation; an independent guard of another package on that
// operation applies to the Go default and to the replacement; the CRM HTTP
// forms go through whichever implementation is bound; reverting the binding
// restores the Go default; a package cannot write CRM clients generically.
func TestScenario13NativeCommandReplacement(t *testing.T) {
	e := newEnv(t)
	admin := e.admin

	// The Go default serves the form, through the host: recorded, guarded.
	e.deploy(admin, "e2e.clientguard", "1.0.0", loadTree(t, "clientguard"))
	r := e.createClientForm(admin, "Ann", "Lee", "+998901112201")
	require.Less(t, r.Status, 400, r.String())
	first, last, ok := e.clientName("+998901112201")
	require.True(t, ok)
	require.Equal(t, []string{"Ann", "Lee"}, []string{first, last})
	require.Empty(t, e.lastSaveBinding(), "the Go default ran (no package binding)")
	require.Equal(t, 1, e.scalarInt(`SELECT count(*) FROM extruntime.invocations c JOIN extruntime.invocations o ON o.parent_invocation_id = c.id
		WHERE c.contract = $1 AND o.contract = $2 AND o.outcome = 'ok'`, cmdSaveClient, opUpsertClient), "the Go default wrote through the published operation")
	require.Equal(t, 1, e.scalarInt(`SELECT count(*) FROM extruntime.invocations WHERE contract = $1 AND outcome = 'ok'`, guardClientName),
		"the package guard ran on the Go default's write")

	r = e.createClientForm(admin, "Blocked", "Lee", "+998901112202")
	require.Equal(t, http.StatusUnprocessableEntity, r.Status, r.String())
	_, _, ok = e.clientName("+998901112202")
	require.False(t, ok, "the guard refused the Go default's write")

	// A replacement is an explicit binding of the native command.
	e.deploy(admin, "e2e.vip", "1.0.0", loadTree(t, "vip"))
	r = e.createClientForm(admin, "Bob", "Stone", "+998901112203")
	require.Less(t, r.Status, 400, r.String())
	_, last, _ = e.clientName("+998901112203")
	require.Equal(t, "Stone", last, "activating the package replaces nothing by itself")

	e.bind(admin, manage.SlotCommand, cmdSaveClient, "e2e.vip", "1.0.0", "saveClient", http.StatusCreated)
	r = e.createClientForm(admin, "Cara", "Stone", "+998901112204")
	require.Less(t, r.Status, 400, r.String())
	_, last, _ = e.clientName("+998901112204")
	require.Equal(t, "STONE", last, "the form went through the replacement")
	require.Equal(t, "e2e.vip", e.lastSaveBinding(), "the invocation records the replacement")

	// The edit forms dispatch the same command.
	id := e.clientIDByPhone("+998901112201")
	r = e.postForm(admin, "/crm/clients/"+id+"/edit/personal", url.Values{"FirstName": {"Ann"}, "LastName": {"Mixed"}, "Phone": {"+998901112201"}})
	require.Less(t, r.Status, 400, r.String())
	_, last, _ = e.clientName("+998901112201")
	require.Equal(t, "MIXED", last, "the update form went through the replacement")
	r = e.postForm(admin, "/crm/clients/"+id+"/edit/notes", url.Values{"Comments": {"prefers email"}})
	require.Less(t, r.Status, 400, r.String())
	require.Equal(t, "prefers email", e.scalarString(`SELECT comments FROM clients WHERE id = $1`, id))

	// The guard of the other package holds for the replacement.
	r = e.createClientForm(admin, "Blocked", "Stone", "+998901112205")
	require.Equal(t, http.StatusUnprocessableEntity, r.Status, r.String())
	_, _, ok = e.clientName("+998901112205")
	require.False(t, ok)

	// A package cannot write CRM clients generically: refused by the host.
	sneak := e.rpc(admin, "command", "e2e.vip.sneak", map[string]any{"client": map[string]string{"entity": "crm.client", "key": id}})
	require.False(t, sneak.OK)
	require.Equal(t, "denied", sneak.Error.Kind, "%+v", sneak.Error)
	require.Equal(t, "runtime.notWritable", sneak.Error.Code)
	first, _, _ = e.clientName("+998901112201")
	require.Equal(t, "Ann", first, "the client row is untouched")

	// Reverting the binding restores the Go default.
	e.unbind(admin, manage.SlotCommand, cmdSaveClient)
	r = e.createClientForm(admin, "Dan", "Stone", "+998901112206")
	require.Less(t, r.Status, 400, r.String())
	_, last, _ = e.clientName("+998901112206")
	require.Equal(t, "Stone", last)
	require.Empty(t, e.lastSaveBinding())
	r = e.createClientForm(admin, "Blocked", "Stone", "+998901112207")
	require.Equal(t, http.StatusUnprocessableEntity, r.Status, "the guard is independent of the binding")
}
