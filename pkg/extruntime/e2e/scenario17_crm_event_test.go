package e2e_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type seenRow struct {
	ClientID  string `json:"clientId"`
	FirstName string `json:"firstName"`
	Changed   string `json:"changed"`
}

func (e *env) seenRows(a *actor) []seenRow {
	e.t.Helper()
	var rows []seenRow
	e.mustRPC(a, "query", "e2e.clientwatch.seenRows", map[string]any{}, &rows)
	return rows
}

// Scenario 17: the CRM module publishes crm.client.updated.v1 through the
// transactional outbox in the upsertClient transaction (declared in the
// application's native set, so it is in the kit as crm.Updated). A package
// subscription receives the update made through the CRM HTTP edit form after
// the commit; a save that rolls back (a package command that writes through
// crm.operations.upsertClient and then fails) leaves no event and delivers
// nothing.
func TestScenario17CRMClientEventsThroughTheOutbox(t *testing.T) {
	e := newEnv(t)
	admin := e.admin
	const ns = "e2e.clientwatch"
	e.deploy(admin, ns, "1.0.0", loadTree(t, "clientwatch"))
	e.assignOwner(admin, ns+".onClientUpdated", admin)

	// A created client publishes crm.client.created.v1, which this package
	// does not subscribe to.
	r := e.createClientForm(admin, "Eve", "Stone", "+998901114401")
	require.Less(t, r.Status, 400, r.String())
	id := e.clientIDByPhone("+998901114401")
	require.Equal(t, 1, e.scalarInt(`SELECT count(*) FROM extruntime.outbox_events WHERE event = 'crm.client.created.v1' AND subject_key = $1`, id))

	// ---- the HTTP edit form: the event commits with the client and is delivered after the commit
	r = e.postForm(admin, "/crm/clients/"+id+"/edit/personal", url.Values{"FirstName": {"Evelyn"}, "LastName": {"Stone"}, "Phone": {"+998901114401"}})
	require.Less(t, r.Status, 400, r.String())
	var payload json.RawMessage
	require.NoError(t, e.pool.QueryRow(context.Background(), `SELECT payload FROM extruntime.outbox_events
		WHERE event = 'crm.client.updated.v1' AND subject_entity = 'crm.client' AND subject_key = $1 AND tenant_id = $2`, id, e.tenant).Scan(&payload))
	require.JSONEq(t, `{"clientId":"`+id+`","firstName":"Evelyn","lastName":"Stone","phone":"998901114401","changed":["firstName"]}`, string(payload))
	e.eventually("the subscription sees the committed update", 30*time.Second, func() bool { return len(e.seenRows(admin)) == 1 })
	require.Equal(t, []seenRow{{ClientID: id, FirstName: "Evelyn", Changed: "firstName"}}, e.seenRows(admin))
	require.Equal(t, int64(admin.id), int64(e.scalarInt(`SELECT initiator_user_id FROM extruntime.outbox_events WHERE event = 'crm.client.updated.v1' AND subject_key = $1`, id)),
		"the form user is the initiator")

	// ---- a rolled back save: the operation wrote and enqueued, the transaction failed
	res := e.rpc(admin, "command", ns+".renameThenFail", map[string]any{"client": map[string]string{"entity": "crm.client", "key": id}, "firstName": "Ghost"})
	require.False(t, res.OK)
	require.Equal(t, http.StatusUnprocessableEntity, res.Status, "%+v", res.Error)
	require.Equal(t, "e2e.clientwatch.abandoned", res.Error.Issues[0].Code)
	first, _, _ := e.clientName("+998901114401")
	require.Equal(t, "Evelyn", first, "the client write rolled back")
	require.Equal(t, 1, e.scalarInt(`SELECT count(*) FROM extruntime.outbox_events WHERE event = 'crm.client.updated.v1' AND subject_key = $1`, id),
		"the event rolled back with the write")
	// Give the dispatcher (polling every 100ms) time to (not) act.
	time.Sleep(2 * time.Second)
	require.Len(t, e.seenRows(admin), 1, "nothing was delivered for the rolled back save")
	require.Equal(t, 1, e.scalarInt(`SELECT count(*) FROM extruntime.subscription_receipts WHERE subscription = $1 AND status = 'done'`, ns+".onClientUpdated"))
}
