package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const (
	opManage = serrors.Op("extruntime.workflow.manage")
)

// Task is one human_tasks row.
type Task struct {
	ID             uuid.UUID
	InstanceID     uuid.UUID
	Name           string
	N              int
	SubjectEntity  string
	SubjectKey     string
	AssigneeUserID int64
	DecisionSchema json.RawMessage
	Status         string
	Decision       json.RawMessage
	DecidedBy      *int64
	DueAt          *time.Time
	CreatedAt      time.Time
	DecidedAt      *time.Time
}

// Signal is one workflow_signals row.
type Signal struct {
	ID           uuid.UUID
	Name         string
	Payload      json.RawMessage
	SenderUserID *int64
	Key          string
	ConsumedName string
	ConsumedN    *int
	CreatedAt    time.Time
	ConsumedAt   *time.Time
}

// InvocationInfo is the summary of an invocation caused by an instance.
type InvocationInfo struct {
	ID               uuid.UUID
	ParentID         *uuid.UUID
	Contract         string
	BindingPackage   string
	BindingVersion   string
	BindingExport    string
	Outcome          string
	CommittedTxCount int
	StartedAt        time.Time
	FinishedAt       *time.Time
}

// History is everything the management UI and MCP show about an instance.
type History struct {
	Instance    Instance
	Occurrences []Occurrence
	Tasks       []Task
	Signals     []Signal
	Invocations []InvocationInfo
}

// ListFilter narrows List. TenantID is required.
type ListFilter struct {
	TenantID uuid.UUID
	Status   string
	Workflow string
	Limit    int
	Offset   int
}

// List returns the tenant's instances, newest first.
func (e *Engine) List(ctx context.Context, f ListFilter) ([]Instance, error) {
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 100
	}
	rows, err := e.cfg.DB.Query(ctx, `
		SELECT `+instanceCols+` FROM extruntime.workflow_instances
		WHERE tenant_id = $1 AND ($2 = '' OR status = $2) AND ($3 = '' OR workflow = $3)
		ORDER BY created_at DESC, id LIMIT $4 OFFSET $5`, f.TenantID, f.Status, f.Workflow, f.Limit, f.Offset)
	if err != nil {
		return nil, serrors.FromDB(opManage, err)
	}
	defer rows.Close()
	var out []Instance
	for rows.Next() {
		i, err := scanInstance(rows)
		if err != nil {
			return nil, serrors.FromDB(opManage, err)
		}
		out = append(out, i)
	}
	return out, serrors.FromDB(opManage, rows.Err())
}

func (e *Engine) instance(ctx context.Context, q queryer, tenant, id uuid.UUID) (Instance, error) {
	i, err := scanInstance(q.QueryRow(ctx, `SELECT `+instanceCols+` FROM extruntime.workflow_instances WHERE id = $1 AND tenant_id = $2`, id, tenant))
	if errors.Is(err, pgx.ErrNoRows) {
		return Instance{}, serrors.Wrap(opManage, fmt.Errorf("%w: instance %s", ErrNotFound, id))
	}
	return i, serrors.FromDB(opManage, err)
}

// Get returns the full history of an instance: occurrences, tasks, signals
// and the invocations it caused.
func (e *Engine) Get(ctx context.Context, tenant, id uuid.UUID) (History, error) {
	inst, err := e.instance(ctx, e.cfg.DB, tenant, id)
	if err != nil {
		return History{}, err
	}
	h := History{Instance: inst}

	rows, err := e.cfg.DB.Query(ctx, `SELECT `+occurrenceCols+` FROM extruntime.workflow_occurrences WHERE instance_id = $1 AND tenant_id = $2 ORDER BY created_at, name, n`, id, tenant)
	if err != nil {
		return h, serrors.FromDB(opManage, err)
	}
	for rows.Next() {
		o, err := scanOccurrence(rows)
		if err != nil {
			rows.Close()
			return h, serrors.FromDB(opManage, err)
		}
		h.Occurrences = append(h.Occurrences, o)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return h, serrors.FromDB(opManage, err)
	}

	trows, err := e.cfg.DB.Query(ctx, `
		SELECT id, instance_id, name, n, subject_entity, subject_key, assignee_user_id, decision_schema, status, decision, decided_by, due_at, created_at, decided_at
		FROM extruntime.human_tasks WHERE instance_id = $1 AND tenant_id = $2 ORDER BY created_at, name, n`, id, tenant)
	if err != nil {
		return h, serrors.FromDB(opManage, err)
	}
	for trows.Next() {
		var t Task
		if err := trows.Scan(&t.ID, &t.InstanceID, &t.Name, &t.N, &t.SubjectEntity, &t.SubjectKey, &t.AssigneeUserID, &t.DecisionSchema, &t.Status, &t.Decision, &t.DecidedBy, &t.DueAt, &t.CreatedAt, &t.DecidedAt); err != nil {
			trows.Close()
			return h, serrors.FromDB(opManage, err)
		}
		h.Tasks = append(h.Tasks, t)
	}
	trows.Close()
	if err := trows.Err(); err != nil {
		return h, serrors.FromDB(opManage, err)
	}

	srows, err := e.cfg.DB.Query(ctx, `
		SELECT id, name, payload, sender_user_id, COALESCE(signal_key, ''), COALESCE(consumed_name, ''), consumed_n, created_at, consumed_at
		FROM extruntime.workflow_signals WHERE instance_id = $1 AND tenant_id = $2 ORDER BY created_at, id`, id, tenant)
	if err != nil {
		return h, serrors.FromDB(opManage, err)
	}
	for srows.Next() {
		var s Signal
		if err := srows.Scan(&s.ID, &s.Name, &s.Payload, &s.SenderUserID, &s.Key, &s.ConsumedName, &s.ConsumedN, &s.CreatedAt, &s.ConsumedAt); err != nil {
			srows.Close()
			return h, serrors.FromDB(opManage, err)
		}
		h.Signals = append(h.Signals, s)
	}
	srows.Close()
	if err := srows.Err(); err != nil {
		return h, serrors.FromDB(opManage, err)
	}

	irows, err := e.cfg.DB.Query(ctx, `
		SELECT id, parent_invocation_id, contract, binding_package, binding_version, binding_export, outcome, committed_tx_count, started_at, finished_at
		FROM extruntime.invocations
		WHERE tenant_id = $1 AND trigger_kind = 'workflow' AND trigger_ref = $2 ORDER BY started_at, id`, tenant, id.String())
	if err != nil {
		return h, serrors.FromDB(opManage, err)
	}
	defer irows.Close()
	for irows.Next() {
		var i InvocationInfo
		if err := irows.Scan(&i.ID, &i.ParentID, &i.Contract, &i.BindingPackage, &i.BindingVersion, &i.BindingExport, &i.Outcome, &i.CommittedTxCount, &i.StartedAt, &i.FinishedAt); err != nil {
			return h, serrors.FromDB(opManage, err)
		}
		h.Invocations = append(h.Invocations, i)
	}
	return h, serrors.FromDB(opManage, irows.Err())
}

func notify(ctx context.Context, tx queryer) error {
	_, err := tx.Exec(ctx, `SELECT pg_notify($1, '')`, NotifyChannel)
	return serrors.FromDB(opManage, err)
}

// Signal delivers a named signal to an instance. The payload is validated
// against the workflow's declared signal type. A repeated signal with the same
// key is a no-op. A terminated, completed or failed instance rejects it.
func (e *Engine) Signal(ctx context.Context, tenant uuid.UUID, sender int64, instance uuid.UUID, name string, payload json.RawMessage, key string) error {
	tx, err := e.cfg.DB.Begin(ctx)
	if err != nil {
		return serrors.FromDB(opManage, err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if err := e.deliverSignal(ctx, tx, signalReq{tenant: tenant, sender: sender, instance: instance, name: name, payload: payload, key: key}); err != nil {
		return err
	}
	return serrors.FromDB(opManage, tx.Commit(ctx))
}

// Typed errors of tx.signal / invoke.signal (host.Signaler).
const (
	CodeInstanceNotFound     = "workflow.instanceNotFound"
	CodeInstanceNotAccepting = "workflow.instanceNotAccepting"
	CodeInstanceMismatch     = "workflow.instanceMismatch"
	CodeUnknownSignal        = "workflow.unknownSignal"
	CodeSignalDenied         = "workflow.signalDenied"
)

var errSignalDenied = errors.New("extruntime workflow: subject may not signal this workflow")

// SignalInTx implements host.Signaler: the signal row is written in the
// caller's transaction, atomically with the caller's data. The instance must
// belong to the subject's tenant and run the given workflow, the payload must
// match the workflow contract's signal schema, and the subject's CURRENT
// rights must include the workflow's permission. Failures are typed wire
// errors and leave the transaction usable.
func (e *Engine) SignalInTx(ctx context.Context, tx pgx.Tx, sc spi.Scope, in host.SignalIntent) error {
	id, err := uuid.Parse(in.Instance)
	if err != nil {
		return signalWire(fmt.Errorf("%w: instance %q is not an instance id", ErrNotFound, in.Instance))
	}
	subject := sc.Subject
	return signalWire(e.deliverSignal(ctx, tx, signalReq{
		tenant: subject.TenantID, sender: subject.UserID, instance: id, workflow: in.Workflow, name: in.Name, payload: in.Payload, key: in.Key, actor: &subject,
	}))
}

type signalReq struct {
	tenant   uuid.UUID
	sender   int64
	instance uuid.UUID
	// workflow, when set, must be the workflow the instance runs.
	workflow string
	name     string
	payload  json.RawMessage
	key      string
	// actor, when set, is checked against the workflow's permission with the
	// rights it holds now.
	actor *spi.Subject
}

type unknownSignalError struct{ workflow, name string }

func (u *unknownSignalError) Error() string {
	return fmt.Sprintf("workflow %s declares no signal %q", u.workflow, u.name)
}

type mismatchError struct{ got, want string }

func (m *mismatchError) Error() string {
	return fmt.Sprintf("instance runs workflow %s, not %s", m.want, m.got)
}

// deliverSignal validates and writes one signal in q's transaction.
func (e *Engine) deliverSignal(ctx context.Context, q queryer, r signalReq) error {
	inst, err := e.instance(ctx, q, r.tenant, r.instance)
	if err != nil {
		return err
	}
	if r.workflow != "" && r.workflow != inst.Workflow {
		return serrors.Wrap(opManage, fmt.Errorf("%w: %w", ErrInvalid, &mismatchError{got: r.workflow, want: inst.Workflow}))
	}
	res, ok, err := e.cfg.Catalog.Resolve(ctx, inst.Workflow)
	if err != nil {
		return serrors.Wrap(opManage, err)
	}
	if !ok {
		return serrors.Wrap(opManage, fmt.Errorf("%w: workflow %s is not in the catalog", ErrInvalid, inst.Workflow))
	}
	typ, declared := res.Resource.Signals[r.name]
	if !declared {
		return serrors.Wrap(opManage, fmt.Errorf("%w: %w", ErrInvalid, &unknownSignalError{workflow: inst.Workflow, name: r.name}))
	}
	payload := r.payload
	if len(payload) == 0 {
		payload = json.RawMessage(`null`)
	}
	if verr := manifest.ValidateValue(typ, payload); verr != nil {
		return serrors.Wrap(opManage, fmt.Errorf("%w: signal %q payload: %w", ErrInvalid, r.name, verr))
	}
	if r.actor != nil {
		kind := r.actor.Kind
		if kind == "" {
			kind = spi.SubjectInitiator
		}
		current, err := e.cfg.Subjects.LoadSubject(ctx, r.tenant, r.actor.UserID, kind)
		if err != nil {
			return serrors.Wrap(opManage, fmt.Errorf("%w: %w", errSignalDenied, err))
		}
		if !current.Has(res.Resource.Permission) {
			return serrors.Wrap(opManage, fmt.Errorf("%w: permission %s is required for %s", errSignalDenied, res.Resource.Permission, inst.Workflow))
		}
	}
	// Locking the instance row first serializes with a run that is suspending.
	var status string
	err = q.QueryRow(ctx, `
		UPDATE extruntime.workflow_instances
		SET wake_at = CASE WHEN status = 'waiting' THEN now() ELSE wake_at END, updated_at = now()
		WHERE id = $1 AND tenant_id = $2 AND status IN ('running', 'waiting', 'blocked')
		RETURNING status`, r.instance, r.tenant).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return serrors.Wrap(opManage, fmt.Errorf("%w: instance %s is %s", ErrNotAccepting, r.instance, inst.Status))
	}
	if err != nil {
		return serrors.FromDB(opManage, err)
	}
	var sig any
	if r.key != "" {
		sig = r.key
	}
	var sender64 any
	if r.sender > 0 {
		sender64 = r.sender
	}
	if _, err := q.Exec(ctx, `
		INSERT INTO extruntime.workflow_signals (id, tenant_id, instance_id, name, payload, sender_user_id, signal_key)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (tenant_id, instance_id, signal_key) WHERE signal_key IS NOT NULL DO NOTHING`,
		uuid.New(), r.tenant, r.instance, r.name, []byte(payload), sender64, sig); err != nil {
		return serrors.FromDB(opManage, err)
	}
	return notify(ctx, q)
}

// signalWire maps the engine's sentinel errors to the typed wire errors the
// worker (and so the package author) sees.
func signalWire(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := pool.AsWireError(err); ok {
		return err
	}
	switch {
	case errors.Is(err, ErrNotFound):
		return &pool.WireError{Kind: pool.ErrKindInternal, Code: CodeInstanceNotFound, Message: err.Error()}
	case errors.Is(err, ErrNotAccepting):
		return &pool.WireError{Kind: pool.ErrKindInternal, Code: CodeInstanceNotAccepting, Message: err.Error()}
	case errors.Is(err, errSignalDenied):
		return &pool.WireError{Kind: pool.ErrKindDenied, Code: CodeSignalDenied, Message: err.Error()}
	case errors.Is(err, ErrInvalid):
		var mm *mismatchError
		if errors.As(err, &mm) {
			return &pool.WireError{Kind: pool.ErrKindInternal, Code: CodeInstanceMismatch, Message: err.Error()}
		}
		var pe *manifest.PathError
		if errors.As(err, &pe) {
			path, _ := json.Marshal(pe.Path)
			params, _ := json.Marshal(map[string]string{"message": pe.Message})
			return &pool.WireError{Kind: pool.ErrKindValidation, Code: "runtime.invalidValue", Message: err.Error(),
				Issues: []pool.Issue{{Code: "runtime.invalidValue", Phase: "input", Path: path, Params: params}}}
		}
		var us *unknownSignalError
		if errors.As(err, &us) {
			return &pool.WireError{Kind: pool.ErrKindInternal, Code: CodeUnknownSignal, Message: err.Error()}
		}
		return &pool.WireError{Kind: pool.ErrKindValidation, Code: "runtime.invalidValue", Message: err.Error()}
	}
	return err
}

// DecideTask records the decision of an open human task. The actor must be
// the assignee (or hold Config.TaskAdminPermission), judged by their CURRENT
// rights; the decision must match the task's decision schema. A decision and
// the task's timeout race on a compare-and-set of the task status, so exactly
// one wins; a late decision gets ErrTaskClosed.
func (e *Engine) DecideTask(ctx context.Context, actor spi.Subject, taskID uuid.UUID, decision json.RawMessage) error {
	var (
		instanceID uuid.UUID
		assignee   int64
		schema     []byte
		status     string
	)
	err := e.cfg.DB.QueryRow(ctx, `
		SELECT instance_id, assignee_user_id, decision_schema, status FROM extruntime.human_tasks WHERE id = $1 AND tenant_id = $2`,
		taskID, actor.TenantID).Scan(&instanceID, &assignee, &schema, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return serrors.Wrap(opManage, fmt.Errorf("%w: task %s", ErrNotFound, taskID))
	}
	if err != nil {
		return serrors.FromDB(opManage, err)
	}
	if status != "open" {
		return serrors.Wrap(opManage, fmt.Errorf("%w: task %s is %s", ErrTaskClosed, taskID, status))
	}
	current, err := e.cfg.Subjects.LoadSubject(ctx, actor.TenantID, actor.UserID, spi.SubjectInitiator)
	if err != nil {
		return serrors.Wrap(opManage, fmt.Errorf("%w: %v", ErrForbidden, err))
	}
	if actor.UserID != assignee && !(e.cfg.TaskAdminPermission != "" && current.Has(e.cfg.TaskAdminPermission)) {
		return serrors.Wrap(opManage, fmt.Errorf("%w: user %d is not the assignee of task %s", ErrForbidden, actor.UserID, taskID))
	}
	if len(schema) > 0 {
		var typ manifest.Type
		if err := json.Unmarshal(schema, &typ); err == nil {
			if verr := manifest.ValidateValue(typ, decision); verr != nil {
				return serrors.Wrap(opManage, fmt.Errorf("%w: decision: %v", ErrInvalid, verr))
			}
		}
	}
	tx, err := e.cfg.DB.Begin(ctx)
	if err != nil {
		return serrors.FromDB(opManage, err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	// The instance row is locked first, as every run write does (fence), and
	// the wake is decided under that lock: a run that is suspending commits its
	// wait before this proceeds, so the instance is `waiting` here and gets
	// woken; a run that has not suspended yet sees the decision when it does.
	// Updating only `status = 'waiting'` rows without the lock would lose a
	// decision committed between the run's wake computation and its commit.
	if _, err := tx.Exec(ctx, `
		UPDATE extruntime.workflow_instances
		SET wake_at = CASE WHEN status = 'waiting' THEN now() ELSE wake_at END, updated_at = now()
		WHERE id = $1 AND tenant_id = $2`, instanceID, actor.TenantID); err != nil {
		return serrors.FromDB(opManage, err)
	}
	tag, err := tx.Exec(ctx, `
		UPDATE extruntime.human_tasks SET status = 'decided', decision = $3, decided_by = $4, decided_at = now()
		WHERE id = $1 AND tenant_id = $2 AND status = 'open'`, taskID, actor.TenantID, []byte(decision), actor.UserID)
	if err != nil {
		return serrors.FromDB(opManage, err)
	}
	if tag.RowsAffected() == 0 {
		return serrors.Wrap(opManage, fmt.Errorf("%w: task %s was decided or expired first", ErrTaskClosed, taskID))
	}
	if err := notify(ctx, tx); err != nil {
		return err
	}
	return serrors.FromDB(opManage, tx.Commit(ctx))
}

// Terminate stops an instance: no further steps, open tasks and timers are
// cancelled, later signals are rejected, an in-flight run is aborted. Effects
// already committed stay (no compensation) and children keep running.
func (e *Engine) Terminate(ctx context.Context, tenant, id uuid.UUID) error {
	tx, err := e.cfg.DB.Begin(ctx)
	if err != nil {
		return serrors.FromDB(opManage, err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	tag, err := tx.Exec(ctx, `
		UPDATE extruntime.workflow_instances
		SET status = 'terminated', wake_at = NULL, lease_owner = NULL, lease_until = NULL, finished_at = now(), updated_at = now()
		WHERE id = $1 AND tenant_id = $2 AND status IN ('running', 'waiting', 'blocked')`, id, tenant)
	if err != nil {
		return serrors.FromDB(opManage, err)
	}
	if tag.RowsAffected() == 0 {
		if _, gerr := e.instance(ctx, tx, tenant, id); gerr != nil {
			return gerr
		}
		return serrors.Wrap(opManage, fmt.Errorf("%w: instance %s is already finished", ErrNotAccepting, id))
	}
	if _, err := tx.Exec(ctx, `UPDATE extruntime.human_tasks SET status = 'cancelled' WHERE instance_id = $1 AND tenant_id = $2 AND status = 'open'`, id, tenant); err != nil {
		return serrors.FromDB(opManage, err)
	}
	if _, err := tx.Exec(ctx, `UPDATE extruntime.workflow_occurrences SET status = 'cancelled', wake_at = NULL, updated_at = now() WHERE instance_id = $1 AND tenant_id = $2 AND status = 'waiting'`, id, tenant); err != nil {
		return serrors.FromDB(opManage, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return serrors.FromDB(opManage, err)
	}
	e.cancelLocal(id.String())
	return nil
}

// StartV2 starts a new instance of the same workflow and input on the given
// version of the pinned package (the current version when version is empty).
// The new instance links to the old one as predecessor and has no start key.
// The old instance must have finished, been terminated or be blocked; live
// instances are never migrated.
func (e *Engine) StartV2(ctx context.Context, actor spi.Subject, predecessor uuid.UUID, version string) (uuid.UUID, error) {
	old, err := e.instance(ctx, e.cfg.DB, actor.TenantID, predecessor)
	if err != nil {
		return uuid.Nil, err
	}
	if old.Status == StatusRunning || old.Status == StatusWaiting {
		return uuid.Nil, serrors.Wrap(opManage, fmt.Errorf("%w: terminate instance %s before starting a new version", ErrNotAccepting, predecessor))
	}
	var impl Implementation
	if version == "" {
		impl, err = e.cfg.Impl.Active(ctx, old.Workflow)
	} else {
		impl, err = e.cfg.Impl.At(ctx, old.Workflow, old.Package, version)
	}
	if err != nil {
		return uuid.Nil, serrors.Wrap(opManage, fmt.Errorf("%w: %v", ErrInvalid, err))
	}
	if res, ok, rerr := e.cfg.Catalog.Resolve(ctx, old.Workflow); rerr == nil && ok && res.Resource.Input != nil {
		if verr := manifest.ValidateValue(*res.Resource.Input, old.Input); verr != nil {
			return uuid.Nil, serrors.Wrap(opManage, fmt.Errorf("%w: input no longer matches the workflow contract: %v", ErrInvalid, verr))
		}
	}
	tx, err := e.cfg.DB.Begin(ctx)
	if err != nil {
		return uuid.Nil, serrors.FromDB(opManage, err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	id, _, err := e.insertInstance(ctx, tx, startSpec{
		workflow: old.Workflow, input: old.Input, impl: impl,
		subject:     spi.Subject{TenantID: old.TenantID, UserID: old.SubjectUserID, Kind: old.SubjectKind},
		predecessor: &predecessor,
	})
	if err != nil {
		return uuid.Nil, err
	}
	return id, serrors.FromDB(opManage, tx.Commit(ctx))
}

// lockBlocked locks the instance row in tx and requires it to be blocked. The
// status is read under the lock, so a Resume (and the run it starts) that
// commits first is seen here, and the occurrence write that follows can never
// touch the journal of a running instance.
func (e *Engine) lockBlocked(ctx context.Context, tx pgx.Tx, tenant, id uuid.UUID) error {
	var status string
	err := tx.QueryRow(ctx, `SELECT status FROM extruntime.workflow_instances WHERE id = $1 AND tenant_id = $2 FOR UPDATE`, id, tenant).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return serrors.Wrap(opManage, fmt.Errorf("%w: instance %s", ErrNotFound, id))
	}
	if err != nil {
		return serrors.FromDB(opManage, err)
	}
	if status != StatusBlocked {
		return serrors.Wrap(opManage, fmt.Errorf("%w: instance %s is %s, not blocked", ErrNotAccepting, id, status))
	}
	return nil
}

// Resolve completes a blocked or failed call/step occurrence with a result
// chosen by an operator (for example the outcome of an ambiguous command found
// by inspecting the data). Call Resume afterwards.
func (e *Engine) Resolve(ctx context.Context, tenant, id uuid.UUID, name string, n int, result json.RawMessage) error {
	if !json.Valid(result) {
		return serrors.Wrap(opManage, fmt.Errorf("%w: result is not valid JSON", ErrInvalid))
	}
	tx, err := e.cfg.DB.Begin(ctx)
	if err != nil {
		return serrors.FromDB(opManage, err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if err := e.lockBlocked(ctx, tx, tenant, id); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `
		UPDATE extruntime.workflow_occurrences
		SET status = 'completed', result = $5, error = NULL, meta = COALESCE(meta, '{}'::jsonb) || '{"resolved":true}'::jsonb, updated_at = now()
		WHERE instance_id = $1 AND tenant_id = $2 AND name = $3 AND n = $4 AND status IN ('started', 'failed') AND kind IN ('call', 'step')`,
		id, tenant, name, n, []byte(result))
	if err != nil {
		return serrors.FromDB(opManage, err)
	}
	if tag.RowsAffected() == 0 {
		return serrors.Wrap(opManage, fmt.Errorf("%w: occurrence %s#%d is not an unresolved call or step", ErrNotFound, name, n))
	}
	return serrors.FromDB(opManage, tx.Commit(ctx))
}

// Retry forgets an unresolved occurrence so the next run executes it again as
// new. For an ambiguous command this accepts the risk of a duplicate effect.
// Call Resume afterwards.
func (e *Engine) Retry(ctx context.Context, tenant, id uuid.UUID, name string, n int) error {
	tx, err := e.cfg.DB.Begin(ctx)
	if err != nil {
		return serrors.FromDB(opManage, err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if err := e.lockBlocked(ctx, tx, tenant, id); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `
		DELETE FROM extruntime.workflow_occurrences
		WHERE instance_id = $1 AND tenant_id = $2 AND name = $3 AND n = $4 AND status IN ('started', 'failed', 'waiting')`,
		id, tenant, name, n)
	if err != nil {
		return serrors.FromDB(opManage, err)
	}
	if tag.RowsAffected() == 0 {
		return serrors.Wrap(opManage, fmt.Errorf("%w: occurrence %s#%d is not retryable", ErrNotFound, name, n))
	}
	return serrors.FromDB(opManage, tx.Commit(ctx))
}

// Resume makes a blocked instance runnable again: after the cause was removed
// (rights restored, binding or artifact fixed) or the occurrence resolved.
func (e *Engine) Resume(ctx context.Context, tenant, id uuid.UUID) error {
	tx, err := e.cfg.DB.Begin(ctx)
	if err != nil {
		return serrors.FromDB(opManage, err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	tag, err := tx.Exec(ctx, `
		UPDATE extruntime.workflow_instances
		SET status = 'running', blocked_reason = '', blocked_detail = '', failures = 0, lease_owner = NULL, lease_until = NULL, wake_at = NULL, updated_at = now()
		WHERE id = $1 AND tenant_id = $2 AND status = 'blocked'`, id, tenant)
	if err != nil {
		return serrors.FromDB(opManage, err)
	}
	if tag.RowsAffected() == 0 {
		if _, gerr := e.instance(ctx, tx, tenant, id); gerr != nil {
			return gerr
		}
		return serrors.Wrap(opManage, fmt.Errorf("%w: instance %s is not blocked", ErrNotAccepting, id))
	}
	if err := notify(ctx, tx); err != nil {
		return err
	}
	return serrors.FromDB(opManage, tx.Commit(ctx))
}
