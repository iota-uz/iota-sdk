package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const (
	opManage = serrors.Op("extruntime.workflow.manage")
)

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
	// Native, when set, keeps only native (true) or package (false) instances.
	Native *bool
	// ParentID, when set, keeps only the children of that instance.
	ParentID *uuid.UUID
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
		  AND ($6::boolean IS NULL OR native = $6) AND ($7::uuid IS NULL OR parent_instance_id = $7)
		ORDER BY created_at DESC, id LIMIT $4 OFFSET $5`, f.TenantID, f.Status, f.Workflow, f.Limit, f.Offset, f.Native, f.ParentID)
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

	tasks, err := e.queryTasks(ctx, e.cfg.DB, tenant, `t.instance_id = $2 ORDER BY t.created_at, t.name, t.n`, id)
	if err != nil {
		return h, err
	}
	h.Tasks = tasks

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
	signals, permission, err := e.signalContract(ctx, inst)
	if err != nil {
		return err
	}
	typ, declared := signals[r.name]
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
		if !current.Has(permission) {
			return serrors.Wrap(opManage, fmt.Errorf("%w: permission %s is required for %s", errSignalDenied, permission, inst.Workflow))
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

// signalContract is what a signal to the instance is checked against: the
// signals and the permission of its workflow contract, from Config.Natives for
// a native instance and from the catalog for a package one.
func (e *Engine) signalContract(ctx context.Context, inst Instance) (map[string]manifest.Type, string, error) {
	if inst.Native {
		nc, ok, err := e.lookupNative(inst.Workflow)
		if err != nil {
			return nil, "", serrors.Wrap(opManage, err)
		}
		if !ok {
			return nil, "", serrors.Wrap(opManage, fmt.Errorf("%w: workflow %s has no Go implementation in this process", ErrInvalid, inst.Workflow))
		}
		return nc.Signals, nc.Permission, nil
	}
	cat := e.catalog()
	if cat == nil {
		return nil, "", serrors.Wrap(opManage, fmt.Errorf("%w: workflow %s is a package workflow and the package runtime is not attached", ErrInvalid, inst.Workflow))
	}
	res, ok, err := cat.Resolve(ctx, inst.Workflow)
	if err != nil {
		return nil, "", serrors.Wrap(opManage, err)
	}
	if !ok {
		return nil, "", serrors.Wrap(opManage, fmt.Errorf("%w: workflow %s is not in the catalog", ErrInvalid, inst.Workflow))
	}
	return res.Resource.Signals, res.Resource.Permission, nil
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
			path, pathErr := json.Marshal(pe.Path)
			params, paramsErr := json.Marshal(map[string]string{"message": pe.Message})
			if pathErr != nil || paramsErr != nil {
				// Strings always encode; without the structured detail the message still says it.
				return &pool.WireError{Kind: pool.ErrKindValidation, Code: "runtime.invalidValue", Message: err.Error()}
			}
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

// Terminate stops an instance: no further steps, open tasks and timers are
// cancelled, later signals are rejected, an in-flight run is aborted. Effects
// already committed stay (no compensation) and children keep running (see
// TerminateTree).
func (e *Engine) Terminate(ctx context.Context, tenant, id uuid.UUID) error {
	err := retryConflicts(ctx, func() error {
		return e.inTx(ctx, func(tx pgx.Tx) error {
			inst, err := e.lockInstance(ctx, tx, tenant, id)
			if err != nil {
				return err
			}
			if !live(inst.Status) {
				return serrors.Wrap(opManage, fmt.Errorf("%w: instance %s is already finished", ErrNotAccepting, id))
			}
			return e.terminateLocked(ctx, tx, inst)
		})
	})
	if err != nil {
		return err
	}
	e.cancelLocal(id.String())
	return nil
}

// retryConflicts reruns fn (a whole transaction) when PostgreSQL aborted it
// for a deadlock or a serialization failure. A termination locks the
// instance row before its StatusObserver touches the consumer's rows, while a
// run locks the consumer's rows before its fence: when both meet, PostgreSQL
// aborts one, and a termination must not fail for that.
func retryConflicts(ctx context.Context, fn func() error) error {
	var err error
	for attempt := range 4 {
		if err = fn(); err == nil || !isConflict(err) || ctx.Err() != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return err
		case <-time.After(time.Duration(attempt+1) * 20 * time.Millisecond):
		}
	}
	return err
}

func isConflict(err error) bool {
	var pe *pgconn.PgError
	return errors.As(err, &pe) && (pe.Code == "40P01" || pe.Code == "40001")
}

// TerminateTree terminates an instance and every unfinished descendant (by
// parent) in one transaction. A descendant's running step finishes, but its
// further host calls are rejected. The root must be live; finished
// descendants are left as they are.
func (e *Engine) TerminateTree(ctx context.Context, tenant, id uuid.UUID) error {
	var stopped []uuid.UUID
	err := retryConflicts(ctx, func() error {
		stopped = stopped[:0]
		return e.inTx(ctx, func(tx pgx.Tx) error { return e.terminateTreeTx(ctx, tx, tenant, id, &stopped) })
	})
	if err != nil {
		return err
	}
	// The root's in-flight run is aborted as by Terminate; a descendant's
	// running step finishes, and its further host calls are rejected.
	e.cancelLocal(id.String())
	for _, s := range stopped[1:] {
		e.staleLocal(s.String())
	}
	return nil
}

func (e *Engine) terminateTreeTx(ctx context.Context, tx pgx.Tx, tenant, id uuid.UUID, stopped *[]uuid.UUID) error {
	root, err := e.lockInstance(ctx, tx, tenant, id)
	if err != nil {
		return err
	}
	if !live(root.Status) {
		return serrors.Wrap(opManage, fmt.Errorf("%w: instance %s is already finished", ErrNotAccepting, id))
	}
	// Level by level: a level's rows are locked before their children are
	// read in a new statement. A child insert holds a key-share lock on its
	// parent row (the parent_instance_id foreign key), so it either
	// committed before the parent lock was granted and is seen by the next
	// level's read, or waits for this transaction and then finds its parent
	// terminated (a run's fence fails, its insert rolls back).
	var descendants []Instance
	frontier := []uuid.UUID{root.ID}
	for len(frontier) > 0 {
		level, err := e.lockChildren(ctx, tx, tenant, frontier)
		if err != nil {
			return err
		}
		frontier = frontier[:0]
		for _, c := range level {
			frontier = append(frontier, c.ID)
			if live(c.Status) {
				descendants = append(descendants, c)
			}
		}
	}
	for _, inst := range append([]Instance{root}, descendants...) {
		if err := e.terminateLocked(ctx, tx, inst); err != nil {
			return err
		}
		*stopped = append(*stopped, inst.ID)
	}
	return nil
}

// lockChildren locks and returns the children of the given instances, in id
// order, whatever their status (a finished child may have live children).
func (e *Engine) lockChildren(ctx context.Context, tx pgx.Tx, tenant uuid.UUID, parents []uuid.UUID) ([]Instance, error) {
	rows, err := tx.Query(ctx, `
		SELECT `+instanceCols+` FROM extruntime.workflow_instances
		WHERE tenant_id = $1 AND parent_instance_id = ANY($2)
		ORDER BY id FOR UPDATE`, tenant, parents)
	if err != nil {
		return nil, serrors.FromDB(opManage, err)
	}
	defer rows.Close()
	var out []Instance
	for rows.Next() {
		c, err := scanInstance(rows)
		if err != nil {
			return nil, serrors.FromDB(opManage, err)
		}
		out = append(out, c)
	}
	return out, serrors.FromDB(opManage, rows.Err())
}

func live(status string) bool {
	return status == StatusRunning || status == StatusWaiting || status == StatusBlocked
}

// lockInstance reads the instance row under FOR UPDATE.
func (e *Engine) lockInstance(ctx context.Context, tx pgx.Tx, tenant, id uuid.UUID) (Instance, error) {
	inst, err := scanInstance(tx.QueryRow(ctx, `SELECT `+instanceCols+` FROM extruntime.workflow_instances WHERE id = $1 AND tenant_id = $2 FOR UPDATE`, id, tenant))
	if errors.Is(err, pgx.ErrNoRows) {
		return Instance{}, serrors.Wrap(opManage, fmt.Errorf("%w: instance %s", ErrNotFound, id))
	}
	return inst, serrors.FromDB(opManage, err)
}

// terminateLocked terminates a live instance whose row tx has locked, with
// its open tasks and waits, and tells the contract's StatusObserver.
func (e *Engine) terminateLocked(ctx context.Context, tx pgx.Tx, inst Instance) error {
	updated, err := scanInstance(tx.QueryRow(ctx, `
		UPDATE extruntime.workflow_instances
		SET status = 'terminated', wake_at = NULL, lease_owner = NULL, lease_until = NULL, finished_at = now(), updated_at = now()
		WHERE id = $1 AND tenant_id = $2
		RETURNING `+instanceCols, inst.ID, inst.TenantID))
	if err != nil {
		return serrors.FromDB(opManage, err)
	}
	if err := e.cancelOpen(ctx, tx, inst.TenantID, inst.ID); err != nil {
		return err
	}
	if err := e.afterTransition(ctx, tx, updated, inst.Status, StatusTerminated); err != nil {
		return serrors.Wrap(opManage, err)
	}
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
	if old.Native {
		if version != "" {
			return uuid.Nil, serrors.Wrap(opManage, fmt.Errorf("%w: a native workflow has no package versions", ErrInvalid))
		}
		nc, ok, lerr := e.lookupNative(old.Workflow)
		if lerr != nil {
			return uuid.Nil, serrors.Wrap(opManage, lerr)
		}
		if !ok {
			return uuid.Nil, serrors.Wrap(opManage, fmt.Errorf("%w: workflow %s has no Go implementation in this process", ErrInvalid, old.Workflow))
		}
		if nc.Input != nil {
			if verr := manifest.ValidateValue(*nc.Input, nonNull(old.Input)); verr != nil {
				return uuid.Nil, serrors.Wrap(opManage, fmt.Errorf("%w: input no longer matches the workflow contract: %v", ErrInvalid, verr))
			}
		}
		impl = Implementation{Native: true}
	} else {
		deps := e.deps()
		if deps.impl == nil || deps.catalog == nil {
			return uuid.Nil, serrors.Wrap(opManage, fmt.Errorf("%w: the package runtime is not attached", ErrInvalid))
		}
		if version == "" {
			impl, err = deps.impl.Active(ctx, old.Workflow)
		} else {
			impl, err = deps.impl.At(ctx, old.Workflow, old.Package, version)
		}
		if err != nil {
			return uuid.Nil, serrors.Wrap(opManage, fmt.Errorf("%w: %v", ErrInvalid, err))
		}
		if res, ok, rerr := deps.catalog.Resolve(ctx, old.Workflow); rerr == nil && ok && res.Resource.Input != nil {
			if verr := manifest.ValidateValue(*res.Resource.Input, old.Input); verr != nil {
				return uuid.Nil, serrors.Wrap(opManage, fmt.Errorf("%w: input no longer matches the workflow contract: %v", ErrInvalid, verr))
			}
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
	return e.resolve(ctx, tenant, id, name, n, result, nil)
}

// ResolveIf is Resolve and Resume of the occurrence an operator observed, in one transaction: its
// updated_at must still be observedAt under the instance's lock, else nothing changes and the error is
// ErrConflict (a newer attempt took its place). within, when set, writes the caller's own state in the
// same transaction, before the instance is resumed.
func (e *Engine) ResolveIf(ctx context.Context, tenant, id uuid.UUID, name string, n int, observedAt time.Time, result json.RawMessage, within func(ctx context.Context, tx pgx.Tx) error) error {
	return e.resolve(ctx, tenant, id, name, n, result, &opIf{observedAt: observedAt, within: within})
}

// opIf binds a management operation to the occurrence an operator observed, and resumes the instance
// with it.
type opIf struct {
	observedAt time.Time
	within     func(ctx context.Context, tx pgx.Tx) error
}

func (e *Engine) resolve(ctx context.Context, tenant, id uuid.UUID, name string, n int, result json.RawMessage, cond *opIf) error {
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
	if err := sameOccurrence(ctx, tx, tenant, id, name, n, cond); err != nil {
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
	if err := e.resumeIf(ctx, tx, tenant, id, cond); err != nil {
		return err
	}
	return serrors.FromDB(opManage, tx.Commit(ctx))
}

// Retry forgets an unresolved occurrence so the next run executes it again as
// new. For an ambiguous command this accepts the risk of a duplicate effect.
// Call Resume afterwards.
func (e *Engine) Retry(ctx context.Context, tenant, id uuid.UUID, name string, n int) error {
	return e.retry(ctx, tenant, id, name, n, nil)
}

// RetryIf is Retry and Resume of the occurrence an operator observed, in one transaction (as ResolveIf).
func (e *Engine) RetryIf(ctx context.Context, tenant, id uuid.UUID, name string, n int, observedAt time.Time, within func(ctx context.Context, tx pgx.Tx) error) error {
	return e.retry(ctx, tenant, id, name, n, &opIf{observedAt: observedAt, within: within})
}

func (e *Engine) retry(ctx context.Context, tenant, id uuid.UUID, name string, n int, cond *opIf) error {
	tx, err := e.cfg.DB.Begin(ctx)
	if err != nil {
		return serrors.FromDB(opManage, err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if err := e.lockBlocked(ctx, tx, tenant, id); err != nil {
		return err
	}
	if err := sameOccurrence(ctx, tx, tenant, id, name, n, cond); err != nil {
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
	if err := e.resumeIf(ctx, tx, tenant, id, cond); err != nil {
		return err
	}
	return serrors.FromDB(opManage, tx.Commit(ctx))
}

// sameOccurrence checks, under the instance's lock, that the occurrence is still the one observed.
func sameOccurrence(ctx context.Context, tx pgx.Tx, tenant, id uuid.UUID, name string, n int, cond *opIf) error {
	if cond == nil {
		return nil
	}
	observedAt := &cond.observedAt
	var at time.Time
	err := tx.QueryRow(ctx, `SELECT updated_at FROM extruntime.workflow_occurrences WHERE instance_id = $1 AND tenant_id = $2 AND name = $3 AND n = $4`,
		id, tenant, name, n).Scan(&at)
	if errors.Is(err, pgx.ErrNoRows) {
		return serrors.Wrap(opManage, fmt.Errorf("%w: occurrence %s#%d is gone", ErrConflict, name, n))
	}
	if err != nil {
		return serrors.FromDB(opManage, err)
	}
	if !at.Equal(*observedAt) {
		return serrors.Wrap(opManage, fmt.Errorf("%w: occurrence %s#%d changed since it was observed", ErrConflict, name, n))
	}
	return nil
}

// resumeIf runs the caller's writes and resumes the instance in the operation's transaction (opIf).
func (e *Engine) resumeIf(ctx context.Context, tx pgx.Tx, tenant, id uuid.UUID, cond *opIf) error {
	if cond == nil {
		return nil
	}
	if cond.within != nil {
		if err := cond.within(ctx, tx); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `
		UPDATE extruntime.workflow_instances
		SET status = 'running', blocked_reason = '', blocked_detail = '', failures = 0, lease_owner = NULL, lease_until = NULL, wake_at = NULL, updated_at = now()
		WHERE id = $1 AND tenant_id = $2 AND status = 'blocked'`, id, tenant); err != nil {
		return serrors.FromDB(opManage, err)
	}
	return notify(ctx, tx)
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
