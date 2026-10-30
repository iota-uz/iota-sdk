package changes_test

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/fielddiff"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/mcp"
	"github.com/iota-uz/iota-sdk/pkg/mcp/changes"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

func TestMain(m *testing.M) {
	if err := os.Chdir("../../.."); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func requirePostgres(t *testing.T) {
	t.Helper()
	host, port := os.Getenv("DB_HOST"), os.Getenv("DB_PORT")
	if host == "" {
		host = "localhost"
	}
	if port == "" {
		port = "5432"
	}
	conn, err := (&net.Dialer{Timeout: 500 * time.Millisecond}).Dial("tcp", net.JoinHostPort(host, port))
	if err != nil {
		t.Skipf("postgres not available: %v", err)
	}
	_ = conn.Close()
}

// notes is a minimal application domain: rows with an integer revision,
// written only through its kinds.
type notes struct {
	pool      *pgxpool.Pool
	failApply atomic.Bool
	approval  bool
}

func (n *notes) exec(ctx context.Context, sql string, args ...any) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, sql, args...)
	return err
}

func (n *notes) revision(ctx context.Context, id string) (string, error) {
	var rev int
	if err := n.pool.QueryRow(ctx, `SELECT revision FROM test_notes WHERE id = $1`, id).Scan(&rev); err != nil {
		return "", serrors.New(serrors.NotFound, "no such note")
	}
	return strconv.Itoa(rev), nil
}

type noteFields struct {
	Body *string `json:"body,omitempty"`
}

func bodySchema() *jsonschema.Schema {
	return &jsonschema.Schema{Type: "object", AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
		Properties: map[string]*jsonschema.Schema{"body": {Type: "string", MinLength: jsonschema.Ptr(1)}}}
}

type createNote struct{ *notes }

func (createNote) Entity() string                                   { return "note" }
func (createNote) Op() string                                       { return "create" }
func (createNote) Description() string                              { return "Create a note." }
func (createNote) Fields() *jsonschema.Schema                       { return bodySchema() }
func (createNote) Policy() mcp.Policy                               { return mcp.Policy{Permissions: []string{"Note.Write"}} }
func (createNote) Existing() bool                                   { return false }
func (createNote) Revision(context.Context, string) (string, error) { return "", nil }

func (k createNote) Plan(_ context.Context, c changes.Change) (changes.Planned, error) {
	var f noteFields
	if err := json.Unmarshal(c.Fields, &f); err != nil {
		return changes.Planned{}, err
	}
	if f.Body == nil {
		return changes.Planned{Errors: []mcp.Issue{{Path: "/fields/body", Code: "required", Message: "body is required"}}}, nil
	}
	raw, err := json.Marshal(*f.Body)
	if err != nil {
		return changes.Planned{}, err
	}
	return changes.Planned{
		Diff:          []fielddiff.Diff{{Field: "body", Before: fielddiff.Absent(), After: fielddiff.Present(raw)}},
		NeedsApproval: k.approval,
	}, nil
}

func (k createNote) Apply(ctx context.Context, c changes.Change) (changes.Applied, error) {
	if k.failApply.Load() {
		return changes.Applied{}, serrors.New(serrors.Unavailable, "database went away")
	}
	var f noteFields
	if err := json.Unmarshal(c.Fields, &f); err != nil {
		return changes.Applied{}, err
	}
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return changes.Applied{}, err
	}
	id := uuid.NewString()
	if err := k.exec(ctx, `INSERT INTO test_notes (id, tenant_id, body, revision) VALUES ($1, $2, $3, 1)`, id, tenant, *f.Body); err != nil {
		return changes.Applied{}, err
	}
	payload, err := json.Marshal(map[string]string{"note": id})
	if err != nil {
		return changes.Applied{}, err
	}
	return changes.Applied{ID: id, Revision: "1", Effects: []changes.Effect{{Kind: "notify", Payload: payload}}}, nil
}

type patchNote struct{ *notes }

func (patchNote) Entity() string             { return "note" }
func (patchNote) Op() string                 { return "patch" }
func (patchNote) Description() string        { return "Change listed fields of a note." }
func (patchNote) Fields() *jsonschema.Schema { return bodySchema() }
func (patchNote) Policy() mcp.Policy         { return mcp.Policy{Permissions: []string{"Note.Write"}} }
func (patchNote) Existing() bool             { return true }

func (k patchNote) Revision(ctx context.Context, id string) (string, error) {
	return k.revision(ctx, id)
}

func (k patchNote) Plan(ctx context.Context, c changes.Change) (changes.Planned, error) {
	rev, err := k.revision(ctx, c.ID)
	if err != nil {
		return changes.Planned{}, err
	}
	var f noteFields
	_ = json.Unmarshal(c.Fields, &f)
	var diff []fielddiff.Diff
	if f.Body != nil {
		raw, err := json.Marshal(*f.Body)
		if err != nil {
			return changes.Planned{}, err
		}
		diff = append(diff, fielddiff.Diff{Field: "body", Before: fielddiff.Redacted(), After: fielddiff.Present(raw)})
	}
	return changes.Planned{Diff: diff, Revision: rev}, nil
}

func (k patchNote) Apply(ctx context.Context, c changes.Change) (changes.Applied, error) {
	var f noteFields
	_ = json.Unmarshal(c.Fields, &f)
	if f.Body != nil {
		if err := k.exec(ctx, `UPDATE test_notes SET body = $2, revision = revision + 1 WHERE id = $1`, c.ID, *f.Body); err != nil {
			return changes.Applied{}, err
		}
	}
	tx, _ := composables.UseTx(ctx)
	var rev int
	if err := tx.QueryRow(ctx, `SELECT revision FROM test_notes WHERE id = $1`, c.ID).Scan(&rev); err != nil {
		return changes.Applied{}, err
	}
	return changes.Applied{ID: c.ID, Revision: strconv.Itoa(rev)}, nil
}

type world struct {
	pool    *pgxpool.Pool
	notes   *notes
	allowed *atomic.Bool
	now     time.Time
	ctx     context.Context
	tenant  uuid.UUID
}

func newWorld(t *testing.T) *world {
	t.Helper()
	requirePostgres(t)
	env := itf.Setup(t)
	pool := env.Pool
	ctx := context.Background()
	tenant := uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO tenants (id, name) VALUES ($1, $2)`, tenant, "mcp-"+tenant.String())
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS test_notes (id uuid PRIMARY KEY, tenant_id uuid NOT NULL, body text NOT NULL, revision int NOT NULL)`)
	require.NoError(t, err)
	allowed := &atomic.Bool{}
	allowed.Store(true)
	return &world{
		pool: pool, notes: &notes{pool: pool}, allowed: allowed, now: time.Now(), tenant: tenant,
		ctx: mcp.WithPrincipal(ctx, mcp.Principal{TenantID: tenant, UserID: 42, Scopes: mcp.AllScopes()}),
	}
}

// service builds a fresh instance on the same database, as after a restart.
func (w *world) service(t *testing.T, opts ...func(*changes.Config)) (*changes.Service, *mcp.Engine) {
	t.Helper()
	authz := mcp.AuthorizerFunc(func(ctx context.Context, p mcp.Principal, policy mcp.Policy) (context.Context, error) {
		if len(policy.Permissions) > 0 && !w.allowed.Load() {
			return nil, mcp.Fail(mcp.CodeForbidden, mcp.RecoveryRequestAccess, "missing permission")
		}
		return composables.WithTenantID(ctx, p.TenantID), nil
	})
	cfg := changes.Config{
		Store:       changes.NewPostgresStore(w.pool),
		Authorizer:  authz,
		Kinds:       []changes.Kind{createNote{w.notes}, patchNote{w.notes}},
		ApprovalURL: func(id string) string { return "https://erp.test/mcp/plans/" + id },
		Effects: map[string]changes.EffectHandler{
			"notify": func(context.Context, string, changes.Effect) error { return nil },
		},
		Now: func() time.Time { return w.now },
	}
	for _, o := range opts {
		o(&cfg)
	}
	svc, err := changes.NewService(cfg)
	require.NoError(t, err)
	reg := mcp.NewRegistry()
	require.NoError(t, reg.Register(svc.Tools()...))
	engine, err := mcp.NewEngine(reg, authz)
	require.NoError(t, err)
	return svc, engine
}

func call[T any](t *testing.T, engine *mcp.Engine, ctx context.Context, tool string, input any) (T, *mcp.Failure) {
	t.Helper()
	raw, err := json.Marshal(input)
	require.NoError(t, err)
	res := engine.Call(ctx, tool, raw)
	var out T
	if !res.OK {
		return out, res.Error
	}
	require.NoError(t, json.Unmarshal(res.Data, &out))
	return out, nil
}

type preparedView struct {
	Status   string                 `json:"status"`
	PlanID   string                 `json:"plan_id"`
	PlanHash string                 `json:"plan_hash"`
	Items    []changes.PlanItem     `json:"items"`
	Errors   []changes.IndexedIssue `json:"errors"`
	Approval changes.ApprovalState  `json:"approval"`
}

type operationView struct {
	OperationID     string           `json:"operation_id"`
	Status          string           `json:"status"`
	Receipt         *changes.Receipt `json:"receipt"`
	ExternalEffects string           `json:"external_effects"`
	Error           *mcp.Failure     `json:"error"`
	RetryAfterMS    int64            `json:"retry_after_ms"`
}

func createChange(body string) map[string]any {
	return map[string]any{"op": "create", "entity": "note", "fields": map[string]any{"body": body}}
}

func (w *world) count(t *testing.T) int {
	t.Helper()
	var n int
	require.NoError(t, w.pool.QueryRow(context.Background(), `SELECT count(*) FROM test_notes WHERE tenant_id = $1`, w.tenant).Scan(&n))
	return n
}

func TestService_ApplyIsIdempotentAcrossRestart(t *testing.T) {
	w := newWorld(t)
	_, engine := w.service(t)
	plan, f := call[preparedView](t, engine, w.ctx, changes.ToolPrepare, map[string]any{"changes": []any{createChange("hello")}})
	require.Nil(t, f)
	require.Equal(t, changes.StatusReady, plan.Status)
	assert.Equal(t, 0, w.count(t), "prepare must not write business data")
	require.Len(t, plan.Items, 1)
	assert.Equal(t, fielddiff.StateAbsent, plan.Items[0].Diff[0].Before.State)

	apply := map[string]any{"plan_id": plan.PlanID, "plan_hash": plan.PlanHash, "idempotency_key": "k-1"}
	first, f := call[operationView](t, engine, w.ctx, changes.ToolApply, apply)
	require.Nil(t, f)
	assert.Equal(t, changes.OperationPending, first.Status, "committed but its effect is not delivered yet")
	assert.Equal(t, changes.EffectsPending, first.ExternalEffects)
	require.NotNil(t, first.Receipt)

	restarted, engine2 := w.service(t)
	byKey, f := call[operationView](t, engine2, w.ctx, changes.ToolOperationGet, map[string]any{"idempotency_key": "k-1"})
	require.Nil(t, f)
	assert.Equal(t, first.OperationID, byKey.OperationID)
	assert.Equal(t, first.Receipt, byKey.Receipt)

	again, f := call[operationView](t, engine2, w.ctx, changes.ToolApply, apply)
	require.Nil(t, f)
	assert.Equal(t, first.OperationID, again.OperationID)
	assert.Equal(t, 1, w.count(t), "repeating the key must not write twice")

	delivered, err := restarted.DispatchEffects(context.Background(), 10)
	require.NoError(t, err)
	assert.Equal(t, 1, delivered)
	settled, f := call[operationView](t, engine2, w.ctx, changes.ToolOperationGet, map[string]any{"operation_id": first.OperationID})
	require.Nil(t, f)
	assert.Equal(t, changes.OperationSucceeded, settled.Status)
	assert.Equal(t, changes.EffectsSucceeded, settled.ExternalEffects)

	other, f := call[preparedView](t, engine2, w.ctx, changes.ToolPrepare, map[string]any{"changes": []any{createChange("other")}})
	require.Nil(t, f)
	_, f = call[operationView](t, engine2, w.ctx, changes.ToolApply,
		map[string]any{"plan_id": other.PlanID, "plan_hash": other.PlanHash, "idempotency_key": "k-1"})
	require.NotNil(t, f)
	assert.Equal(t, mcp.CodeConflict, f.Code)
	assert.Equal(t, first.OperationID, f.OperationID)
}

func TestService_UnknownOutcomeStaysPendingAndResumes(t *testing.T) {
	w := newWorld(t)
	_, engine := w.service(t)
	plan, f := call[preparedView](t, engine, w.ctx, changes.ToolPrepare, map[string]any{"changes": []any{createChange("x")}})
	require.Nil(t, f)
	apply := map[string]any{"plan_id": plan.PlanID, "plan_hash": plan.PlanHash, "idempotency_key": "k-crash"}

	w.notes.failApply.Store(true)
	interrupted, f := call[operationView](t, engine, w.ctx, changes.ToolApply, apply)
	require.Nil(t, f)
	assert.Equal(t, changes.OperationPending, interrupted.Status)
	assert.Nil(t, interrupted.Receipt)
	assert.Positive(t, interrupted.RetryAfterMS)

	w.notes.failApply.Store(false)
	_, engine2 := w.service(t)
	seen, f := call[operationView](t, engine2, w.ctx, changes.ToolOperationGet, map[string]any{"idempotency_key": "k-crash"})
	require.Nil(t, f)
	assert.Equal(t, changes.OperationPending, seen.Status, "an unsettled outcome is never reported as nothing done")

	resumed, f := call[operationView](t, engine2, w.ctx, changes.ToolApply, apply)
	require.Nil(t, f)
	assert.Equal(t, interrupted.OperationID, resumed.OperationID)
	require.NotNil(t, resumed.Receipt)
	assert.Equal(t, 1, w.count(t))

	_, f = call[operationView](t, engine2, w.ctx, changes.ToolOperationGet, map[string]any{"idempotency_key": "never-used"})
	require.NotNil(t, f)
	assert.Equal(t, mcp.CodeNotFound, f.Code)
	assert.Equal(t, mcp.RecoveryCheckOperation, f.Recovery)
}

func TestService_StalePlanIsNotApplied(t *testing.T) {
	cases := []struct {
		name     string
		mutate   func(t *testing.T, w *world, noteID string)
		code     mcp.Code
		recovery mcp.Recovery
	}{
		{
			name: "data changed after prepare",
			mutate: func(t *testing.T, w *world, id string) {
				t.Helper()
				_, err := w.pool.Exec(context.Background(), `UPDATE test_notes SET body = 'theirs', revision = revision + 1 WHERE id = $1`, id)
				require.NoError(t, err)
			},
			code: mcp.CodeConflict, recovery: mcp.RecoveryReadThenPrepare,
		},
		{
			name:   "permission revoked after approval",
			mutate: func(t *testing.T, w *world, _ string) { t.Helper(); w.allowed.Store(false) },
			code:   mcp.CodeForbidden, recovery: mcp.RecoveryRequestAccess,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t)
			w.notes.approval = true
			svc, engine := w.service(t)
			noteID := uuid.NewString()
			_, err := w.pool.Exec(context.Background(), `INSERT INTO test_notes (id, tenant_id, body, revision) VALUES ($1, $2, 'mine', 1)`, noteID, w.tenant)
			require.NoError(t, err)

			plan, f := call[preparedView](t, engine, w.ctx, changes.ToolPrepare, map[string]any{"changes": []any{
				map[string]any{"op": "patch", "entity": "note", "id": noteID, "expected_revision": "1", "fields": map[string]any{"body": "new"}},
			}})
			require.Nil(t, f)
			require.Equal(t, changes.StatusReady, plan.Status)
			assert.Equal(t, fielddiff.StateRedacted, plan.Items[0].Diff[0].Before.State)
			grant, err := svc.Approve(w.ctx, plan.PlanID, plan.PlanHash)
			require.NoError(t, err)

			tc.mutate(t, w, noteID)
			op, f := call[operationView](t, engine, w.ctx, changes.ToolApply, map[string]any{
				"plan_id": plan.PlanID, "plan_hash": plan.PlanHash, "idempotency_key": "k-stale", "approval_ref": grant.Ref,
			})
			require.Nil(t, f)
			assert.Equal(t, changes.OperationFailed, op.Status)
			assert.Nil(t, op.Receipt)
			require.NotNil(t, op.Error)
			assert.Equal(t, tc.code, op.Error.Code)
			assert.Equal(t, tc.recovery, op.Error.Recovery)

			var body string
			require.NoError(t, w.pool.QueryRow(context.Background(), `SELECT body FROM test_notes WHERE id = $1`, noteID).Scan(&body))
			assert.NotEqual(t, "new", body, "no last-write-wins")
		})
	}
}

func TestService_ApprovalComesOnlyFromTrustedChannel(t *testing.T) {
	w := newWorld(t)
	w.notes.approval = true
	svc, engine := w.service(t)
	plan, f := call[preparedView](t, engine, w.ctx, changes.ToolPrepare, map[string]any{"changes": []any{createChange("needs a human")}})
	require.Nil(t, f)
	require.True(t, plan.Approval.Required)
	assert.Equal(t, "https://erp.test/mcp/plans/"+plan.PlanID, plan.Approval.URL)

	apply := func(key, ref string) (operationView, *mcp.Failure) {
		in := map[string]any{"plan_id": plan.PlanID, "plan_hash": plan.PlanHash, "idempotency_key": key}
		if ref != "" {
			in["approval_ref"] = ref
		}
		return call[operationView](t, engine, w.ctx, changes.ToolApply, in)
	}
	_, f = apply("k-a", "")
	require.NotNil(t, f)
	assert.Equal(t, mcp.CodeApprovalRequired, f.Code)
	assert.Equal(t, mcp.RecoveryOpenApproval, f.Recovery)
	_, f = apply("k-a", "confirmed: true")
	require.NotNil(t, f)
	assert.Equal(t, mcp.CodeApprovalRequired, f.Code)

	_, f = call[operationView](t, engine, w.ctx, changes.ToolApply, map[string]any{
		"plan_id": plan.PlanID, "plan_hash": plan.PlanHash, "idempotency_key": "k-a", "confirmed": true,
	})
	require.NotNil(t, f)
	assert.Equal(t, mcp.CodeInvalidInput, f.Code, "unknown arguments are rejected")

	replaced, f := call[preparedView](t, engine, w.ctx, changes.ToolPrepare, map[string]any{"changes": []any{createChange("needs a human")}})
	require.Nil(t, f)
	grant, err := svc.Approve(w.ctx, replaced.PlanID, replaced.PlanHash)
	require.NoError(t, err)
	_, f = apply("k-a", grant.Ref)
	require.NotNil(t, f, "an approval of another plan does not carry over")
	assert.Equal(t, mcp.CodeApprovalRequired, f.Code)

	grant, err = svc.Approve(w.ctx, plan.PlanID, plan.PlanHash)
	require.NoError(t, err)
	op, f := apply("k-a", grant.Ref)
	require.Nil(t, f)
	require.NotNil(t, op.Receipt)
	assert.Equal(t, 1, w.count(t))
}

func TestService_PrepareRejectsInvalidBatches(t *testing.T) {
	cases := []struct {
		name    string
		input   map[string]any
		code    mcp.Code
		invalid string
	}{
		{name: "empty batch", input: map[string]any{"changes": []any{}}, code: mcp.CodeInvalidInput},
		{name: "unknown kind", input: map[string]any{"changes": []any{map[string]any{"op": "delete", "entity": "note"}}}, code: mcp.CodeInvalidInput},
		{name: "patch without revision", input: map[string]any{"changes": []any{
			map[string]any{"op": "patch", "entity": "note", "id": uuid.NewString(), "fields": map[string]any{}},
		}}, code: mcp.CodeInvalidInput},
		{name: "unknown field", input: map[string]any{"changes": []any{
			map[string]any{"op": "create", "entity": "note", "fields": map[string]any{"body": "x", "owner": 1}},
		}}, code: mcp.CodeInvalidInput},
		{name: "domain rejects item", input: map[string]any{"changes": []any{
			map[string]any{"op": "create", "entity": "note", "fields": map[string]any{}},
		}}, invalid: "/changes/0/fields/body"},
		{name: "stale revision at prepare", input: map[string]any{"changes": []any{
			map[string]any{"op": "patch", "entity": "note", "id": "00000000-0000-0000-0000-000000000001", "expected_revision": "1", "fields": map[string]any{}},
		}}, invalid: "/changes/0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t)
			_, engine := w.service(t)
			out, f := call[preparedView](t, engine, w.ctx, changes.ToolPrepare, tc.input)
			if tc.code != "" {
				require.NotNil(t, f)
				assert.Equal(t, tc.code, f.Code)
				assert.NotEmpty(t, f.Issues)
				return
			}
			require.Nil(t, f)
			assert.Equal(t, changes.StatusInvalid, out.Status)
			assert.Empty(t, out.PlanID, "an invalid batch yields no applicable plan")
			require.NotEmpty(t, out.Errors)
			assert.Equal(t, tc.invalid, out.Errors[0].Path)
		})
	}
}

func TestService_ExpiredPlan(t *testing.T) {
	w := newWorld(t)
	_, engine := w.service(t)
	plan, f := call[preparedView](t, engine, w.ctx, changes.ToolPrepare, map[string]any{"changes": []any{createChange("late")}})
	require.Nil(t, f)
	w.now = w.now.Add(time.Hour)
	_, f = call[operationView](t, engine, w.ctx, changes.ToolApply,
		map[string]any{"plan_id": plan.PlanID, "plan_hash": plan.PlanHash, "idempotency_key": "k-late"})
	require.NotNil(t, f)
	assert.Equal(t, mcp.CodePlanExpired, f.Code)
	assert.Equal(t, 0, w.count(t))
}

func TestService_LostEffectFailsOperationButKeepsReceipt(t *testing.T) {
	w := newWorld(t)
	svc, engine := w.service(t, func(c *changes.Config) {
		c.MaxEffectAttempts = 1
		c.Effects = map[string]changes.EffectHandler{
			"notify": func(context.Context, string, changes.Effect) error { return errors.New("smtp down") },
		}
	})
	plan, f := call[preparedView](t, engine, w.ctx, changes.ToolPrepare, map[string]any{"changes": []any{createChange("y")}})
	require.Nil(t, f)
	op, f := call[operationView](t, engine, w.ctx, changes.ToolApply,
		map[string]any{"plan_id": plan.PlanID, "plan_hash": plan.PlanHash, "idempotency_key": "k-fx"})
	require.Nil(t, f)

	_, err := svc.DispatchEffects(context.Background(), 10)
	require.NoError(t, err)
	failed, f := call[operationView](t, engine, w.ctx, changes.ToolOperationGet, map[string]any{"operation_id": op.OperationID})
	require.Nil(t, f)
	assert.Equal(t, changes.OperationFailed, failed.Status)
	assert.Equal(t, changes.EffectsFailed, failed.ExternalEffects)
	require.NotNil(t, failed.Receipt, "a failed effect does not undo the commit")
	assert.Equal(t, mcp.RecoveryNone, failed.Error.Recovery)
}

func TestService_OperationGetNeedsExactlyOneSelector(t *testing.T) {
	w := newWorld(t)
	_, engine := w.service(t)
	for _, input := range []map[string]any{{}, {"operation_id": "a", "idempotency_key": "b"}} {
		_, f := call[operationView](t, engine, w.ctx, changes.ToolOperationGet, input)
		require.NotNil(t, f)
		assert.Equal(t, mcp.CodeInvalidInput, f.Code)
	}
}
