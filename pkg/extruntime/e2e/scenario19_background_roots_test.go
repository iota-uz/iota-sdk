package e2e_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

// historyRow is one CRM business-history record of a client.
type historyRow struct {
	Channel, ActorKind, ActorID, Initiator, Operation, Run string
}

func (e *env) clientHistory(eventType, clientID string) []historyRow {
	e.t.Helper()
	rows, err := e.pool.Query(context.Background(), `
		SELECT channel, actor_kind, actor_id, COALESCE(initiator_snapshot->>'user_id', ''), operation_id, COALESCE(run_id, '')
		FROM action_logs WHERE tenant_id = $1 AND event_type = $2 AND $3 = ANY(subject_refs) ORDER BY created_at, id`,
		e.tenant, eventType, "crm.client:"+clientID)
	require.NoError(e.t, err)
	defer rows.Close()
	var out []historyRow
	for rows.Next() {
		var r historyRow
		require.NoError(e.t, rows.Scan(&r.Channel, &r.ActorKind, &r.ActorID, &r.Initiator, &r.Operation, &r.Run))
		out = append(out, r)
	}
	require.NoError(e.t, rows.Err())
	return out
}

// Scenario 19: the Go defaults of native contracts work from every root, not
// only from an HTTP request. crm.operations.upsertClient writes business
// history and publishes events and needs the user the work is done for, so
// the host derives that context from the invocation's subject and trigger:
// a package subscription (acting as the event initiator) and a public
// endpoint (acting as its trigger owner) both call it through tx.call. The
// history says the extension runtime did the work for that user; the client
// event names the same user as its initiator; the module's eventbus event
// (an after-commit callback) still fires after the subscription's own
// transaction commits.
func TestScenario19NativeDefaultsFromBackgroundRoots(t *testing.T) {
	e := newEnv(t)
	admin := e.admin
	const ns = "e2e.clientsync"
	gateway := e.newActor("lead-gateway-owner", "Client.Read", "Client.Create")

	updated := make(chan *client.UpdatedEvent, 4)
	bus := resolve[eventbus.EventBus](t, e.scope.Container)
	unsubscribe := bus.Subscribe(func(ev *client.UpdatedEvent) { updated <- ev })
	defer unsubscribe()

	e.deploy(admin, ns, "1.0.0", loadTree(t, "clientsync"))
	e.assignOwner(admin, ns+".onClientCreated", admin)
	e.assignOwner(admin, ns+".ingest", gateway)

	// ---- a subscription (initiator subject) calls upsertClient
	r := e.createClientForm(admin, "Fay", "Park", "+998901119901")
	require.Less(t, r.Status, 400, r.String())
	id := e.clientIDByPhone("+998901119901")
	e.eventually("the subscription completed the client", 30*time.Second, func() bool {
		return e.scalarString(`SELECT COALESCE(comments, '') FROM clients WHERE id = $1`, id) == "synced by subscription"
	})
	require.Equal(t, 1, e.scalarInt(`SELECT count(*) FROM extruntime.subscription_receipts WHERE subscription = $1 AND status = 'done'`, ns+".onClientCreated"))
	hist := e.clientHistory("crm.client.updated", id)
	require.Len(t, hist, 1)
	require.Equal(t, "job", hist[0].Channel)
	require.Equal(t, []string{"service", "extruntime:subscription", itoa(admin.id)},
		[]string{hist[0].ActorKind, hist[0].ActorID, hist[0].Initiator}, "the runtime acted for the event initiator")
	require.NotEmpty(t, hist[0].Operation)
	require.NotEmpty(t, hist[0].Run)
	require.Equal(t, int64(admin.id), int64(e.scalarInt(
		`SELECT initiator_user_id FROM extruntime.outbox_events WHERE event = 'crm.client.updated.v1' AND subject_key = $1`, id)),
		"the client event names the user the subscription acted as")
	select {
	case ev := <-updated:
		require.Equal(t, id, itoa(ev.Result.ID()), "the eventbus event ran after the subscription's commit")
	case <-time.After(10 * time.Second):
		t.Fatal("the after-commit eventbus event of upsertClient was not published for the subscription's write")
	}

	// ---- a public endpoint (owner subject) calls upsertClient
	resp := e.do(nil, http.MethodPost, "/_ext/endpoints/"+ns+"/ingest",
		strings.NewReader(`{"firstName":"Gus","lastName":"Hale","phone":"+998901119902"}`), "Content-Type", "application/json")
	require.Equal(t, http.StatusCreated, resp.Status, resp.String())
	leadID := e.clientIDByPhone("+998901119902")
	hist = e.clientHistory("crm.client.created", leadID)
	require.Len(t, hist, 1)
	require.Equal(t, "api", hist[0].Channel)
	require.Equal(t, []string{"service", "extruntime:endpoint", itoa(gateway.id)},
		[]string{hist[0].ActorKind, hist[0].ActorID, hist[0].Initiator}, "the runtime acted for the endpoint's owner")
	require.Equal(t, int64(gateway.id), int64(e.scalarInt(
		`SELECT initiator_user_id FROM extruntime.outbox_events WHERE event = 'crm.client.created.v1' AND subject_key = $1`, leadID)),
		"the owner is the initiator of the event")
}
