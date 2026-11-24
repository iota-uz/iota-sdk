package changes_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/mcp"
	"github.com/iota-uz/iota-sdk/pkg/mcp/changes"
)

type auditLog struct {
	mu      sync.Mutex
	entries []mcp.AuditEntry
}

func (a *auditLog) Record(_ context.Context, e mcp.AuditEntry) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.entries = append(a.entries, e)
}

func (a *auditLog) all() []mcp.AuditEntry {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]mcp.AuditEntry(nil), a.entries...)
}

// A receipt's results are shown only while the caller still passes the kind's policy; the status,
// ids and revisions stay readable.
func TestService_ReceiptResultsNeedTheKindPolicyNow(t *testing.T) {
	w := newWorld(t)
	_, engine := w.service(t)
	plan, f := call[preparedView](t, engine, w.ctx, changes.ToolPrepare, map[string]any{"changes": []any{createChange("hi")}})
	require.Nil(t, f)
	apply := map[string]any{"plan_id": plan.PlanID, "plan_hash": plan.PlanHash, "idempotency_key": "r-1"}
	op, f := call[operationView](t, engine, w.ctx, changes.ToolApply, apply)
	require.Nil(t, f)
	require.NotNil(t, op.Receipt)
	assert.JSONEq(t, `{"secret":"result"}`, string(op.Receipt.Items[0].Result))

	w.allowed.Store(false)
	again, f := call[operationView](t, engine, w.ctx, changes.ToolApply, apply)
	require.Nil(t, f)
	require.NotNil(t, again.Receipt)
	assert.Equal(t, op.Receipt.Items[0].ID, again.Receipt.Items[0].ID)
	assert.Empty(t, again.Receipt.Items[0].Result, "a retry after the permission was revoked shows no result")
	got, f := call[operationView](t, engine, w.ctx, changes.ToolOperationGet, map[string]any{"operation_id": op.OperationID})
	require.Nil(t, f)
	require.NotNil(t, got.Receipt)
	assert.Empty(t, got.Receipt.Items[0].Result)
	assert.Equal(t, op.Receipt.Items[0].Revision, got.Receipt.Items[0].Revision)
}

// The audit event of an applied change is stored with its receipt: when the process stops before
// delivering it, a later dispatch delivers it, once.
func TestService_ChangeAuditSurvivesAStopBeforeDelivery(t *testing.T) {
	w := newWorld(t)
	// No auditor and no MCP call around Apply: the event is committed and not delivered.
	svc, _ := w.service(t)
	prepared, err := svc.Prepare(mcp.WithPrincipal(w.ctx, mcp.Principal{TenantID: w.tenant, UserID: 42, ClientID: "planner", Scopes: mcp.AllScopes()}), []changes.Change{{Op: "create", Entity: "note", Fields: []byte(`{"body":"a"}`)}}, "")
	require.NoError(t, err)
	applier := mcp.WithPrincipal(w.ctx, mcp.Principal{TenantID: w.tenant, UserID: 42, ClientID: "applier", Scopes: mcp.AllScopes()})
	op, err := svc.Apply(applier, changes.ApplyInput{PlanID: prepared.PlanID, PlanHash: prepared.PlanHash, IdempotencyKey: "a-1"})
	require.NoError(t, err)
	require.NotNil(t, op.Receipt)

	log := &auditLog{}
	restarted, _ := w.service(t, func(c *changes.Config) { c.Auditor = log })
	_, err = restarted.DispatchEffects(context.Background(), 10)
	require.NoError(t, err)
	entries := log.all()
	require.Len(t, entries, 1)
	assert.Equal(t, "changes:note.create", entries[0].Tool)
	assert.Equal(t, op.ID, entries[0].OperationID)
	require.NotNil(t, entries[0].Item)
	assert.Equal(t, 0, *entries[0].Item)
	assert.Equal(t, w.tenant.String(), entries[0].TenantID)
	assert.Equal(t, "user:42", entries[0].Subject)
	assert.Equal(t, "applier", entries[0].ClientID, "the client that applied the plan, not the one that prepared it")

	_, err = restarted.DispatchEffects(context.Background(), 10)
	require.NoError(t, err)
	assert.Len(t, log.all(), 1, "a delivered event is not sent again")
}

// refusingLog does not store the entries of one operation while refuse is set.
type refusingLog struct {
	auditLog
	refuse string
}

func (l *refusingLog) RecordConfirmed(ctx context.Context, e mcp.AuditEntry) error {
	if e.OperationID == l.refuse {
		return assert.AnError
	}
	l.Record(ctx, e)
	return nil
}

func (w *world) applyNote(t *testing.T, svc *changes.Service, ctx context.Context, key string) changes.Operation {
	t.Helper()
	prepared, err := svc.Prepare(ctx, []changes.Change{{Op: "create", Entity: "note", Fields: []byte(`{"body":"a"}`)}}, "")
	require.NoError(t, err)
	op, err := svc.Apply(ctx, changes.ApplyInput{PlanID: prepared.PlanID, PlanHash: prepared.PlanHash, IdempotencyKey: key})
	require.NoError(t, err)
	require.NotNil(t, op.Receipt)
	return op
}

// An entry the auditor does not confirm is put off with a backoff and the queue goes on: the next
// entries are delivered. Once due it is delivered with the request of its apply, not of the delivery.
func TestService_UnconfirmedAuditIsRetriedWithoutBlockingTheQueue(t *testing.T) {
	w := newWorld(t)
	svc, _ := w.service(t)
	stuck := w.applyNote(t, svc, mcp.WithRequestID(w.ctx, "req-apply"), "q-1")
	other := w.applyNote(t, svc, w.ctx, "q-2")

	log := &refusingLog{refuse: stuck.ID}
	restarted, _ := w.service(t, func(c *changes.Config) { c.Auditor = log })
	dispatch := mcp.WithRequestID(context.Background(), "req-dispatch")
	_, err := restarted.DispatchEffects(dispatch, 10)
	require.NoError(t, err)
	entries := log.all()
	require.Len(t, entries, 1, "the entry after the refused one is delivered")
	assert.Equal(t, other.ID, entries[0].OperationID)
	var attempts int
	require.NoError(t, w.pool.QueryRow(context.Background(),
		`SELECT attempts FROM mcp.operation_audit WHERE operation_id = $1`, stuck.ID).Scan(&attempts))
	assert.Equal(t, 1, attempts)

	// Not due yet: nothing is sent again.
	_, err = restarted.DispatchEffects(dispatch, 10)
	require.NoError(t, err)
	assert.Len(t, log.all(), 1)

	// Due, and the auditor confirms now.
	log.refuse = ""
	_, err = w.pool.Exec(context.Background(), `UPDATE mcp.operation_audit SET next_attempt_at = now() - interval '1 second' WHERE operation_id = $1`, stuck.ID)
	require.NoError(t, err)
	_, err = restarted.DispatchEffects(dispatch, 10)
	require.NoError(t, err)
	entries = log.all()
	require.Len(t, entries, 2)
	assert.Equal(t, stuck.ID, entries[1].OperationID)
	assert.Equal(t, "req-apply", entries[1].RequestID)
}
