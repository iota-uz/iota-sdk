package runtime_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRequeueEndpointNeedsOperateAndReportsHowManyWereRequeued(t *testing.T) {
	e := newAPIEnv(t)
	const sub = "acme.sub.onThing"
	ctx := t.Context()
	event := uuid.NewString()
	_, err := e.db.Exec(ctx, `INSERT INTO extruntime.outbox_events (id, tenant_id, event, payload) VALUES ($1, $2, 'core.events.UserCreated', '{}')`, event, e.tenant)
	require.NoError(t, err)
	_, err = e.db.Exec(ctx, `INSERT INTO extruntime.subscription_receipts (subscription, event_id, tenant_id, status, attempts, next_at, last_error)
		VALUES ($1, $2, $3, 'dead', 5, now() + interval '1 hour', 'poison')`, sub, event, e.tenant)
	require.NoError(t, err)

	const path = "/subscriptions/" + sub + "/requeue"
	e.json(2, http.MethodPost, path, nil, http.StatusForbidden, nil)
	e.json(1, http.MethodPost, path, map[string]string{"event": "nope"}, http.StatusBadRequest, nil)

	var out struct{ Requeued int64 }
	e.json(1, http.MethodPost, path, map[string]string{"event": event}, http.StatusOK, &out)
	require.EqualValues(t, 1, out.Requeued)
	var status string
	var attempts int
	require.NoError(t, e.db.QueryRow(ctx, `SELECT status, attempts FROM extruntime.subscription_receipts WHERE subscription = $1`, sub).Scan(&status, &attempts))
	require.Equal(t, "failed", status)
	require.Zero(t, attempts)

	e.json(1, http.MethodPost, path, nil, http.StatusOK, &out)
	require.Zero(t, out.Requeued, "nothing dead or blocked is left")
	var audited int
	require.NoError(t, e.db.QueryRow(ctx, `SELECT count(*) FROM extruntime.audit_log WHERE action = 'events.requeue'`).Scan(&audited))
	require.Equal(t, 4, audited, "the denied, the invalid and the two successful requests are audited")
}
