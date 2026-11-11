package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const (
	opStart   = serrors.Op("extruntime.workflow.Start")
	opPrepare = serrors.Op("extruntime.workflow.prepareStart")
)

// startSpec describes a new instance row.
type startSpec struct {
	workflow    string
	input       json.RawMessage
	impl        Implementation
	subject     spi.Subject
	key         string // tenant-scoped dedupe key; empty = no key (start-v2)
	predecessor *uuid.UUID
	parent      *uuid.UUID
	invocation  string
}

// Start implements spi.Starter: it writes a start intent in the caller's
// transaction. The instance pins the package version that is active now, and
// the tenant-scoped start key makes a repeated start (a redelivered
// subscription, a replayed workflow transaction) return the same instance.
func (e *Engine) Start(ctx context.Context, tx pgx.Tx, sc spi.Scope, in spi.StartIntent) (string, error) {
	if in.Key == "" {
		return "", &pool.WireError{Kind: pool.ErrKindInternal, Code: "runtime.startKeyRequired", Message: "a workflow start needs a dedupe key"}
	}
	impl, err := e.prepareStart(ctx, in.Workflow, in.Input)
	if err != nil {
		return "", err
	}
	spec := startSpec{workflow: in.Workflow, input: in.Input, impl: impl, subject: sc.Subject, key: in.Key, invocation: in.Producer.InvocationID}
	if in.Predecessor != "" {
		pid, err := uuid.Parse(in.Predecessor)
		if err != nil {
			return "", serrors.Wrap(opStart, fmt.Errorf("%w: predecessor %q", ErrInvalid, in.Predecessor))
		}
		spec.predecessor = &pid
	}
	if in.Producer.InstanceID != "" {
		pid, err := uuid.Parse(in.Producer.InstanceID)
		if err == nil {
			spec.parent = &pid
		}
	}
	id, _, err := e.insertInstance(ctx, tx, spec)
	if err != nil {
		return "", serrors.Wrap(opStart, err)
	}
	return id.String(), nil
}

// prepareStart resolves the workflow contract and the implementation to pin
// and validates the input, before any lock is taken.
func (e *Engine) prepareStart(ctx context.Context, workflow string, input json.RawMessage) (Implementation, error) {
	res, ok, err := e.cfg.Catalog.Resolve(ctx, workflow)
	if err != nil {
		return Implementation{}, serrors.Wrap(opPrepare, err)
	}
	if !ok || res.Resource.Kind != manifest.ResWorkflow {
		return Implementation{}, &pool.WireError{Kind: pool.ErrKindInternal, Code: "workflow.unknownWorkflow", Message: workflow + " is not a workflow contract"}
	}
	if res.Resource.Input != nil {
		if verr := manifest.ValidateValue(*res.Resource.Input, input); verr != nil {
			return Implementation{}, &pool.WireError{Kind: pool.ErrKindValidation, Code: "runtime.invalidValue", Message: verr.Error()}
		}
	}
	impl, err := e.cfg.Impl.Active(ctx, workflow)
	if err != nil {
		return Implementation{}, &pool.WireError{Kind: pool.ErrKindInternal, Code: "workflow.noImplementation", Message: fmt.Sprintf("%s has no active implementation: %v", workflow, err)}
	}
	return impl, nil
}

// insertInstance inserts the instance, or returns the existing one with the
// same start key. created reports whether this call inserted it.
func (e *Engine) insertInstance(ctx context.Context, tx queryer, s startSpec) (uuid.UUID, bool, error) {
	id := uuid.New()
	input := s.input
	if len(input) == 0 {
		input = json.RawMessage(`null`)
	}
	var key any
	if s.key != "" {
		key = s.key
	}
	var inv any
	if s.invocation != "" {
		inv = s.invocation
	}
	kind := s.subject.Kind
	if kind != spi.SubjectOwner {
		kind = spi.SubjectInitiator
	}
	var got uuid.UUID
	err := tx.QueryRow(ctx, `
		INSERT INTO extruntime.workflow_instances
			(id, tenant_id, workflow, package, version, artifact_hash, export, input, subject_user_id, subject_kind,
			 predecessor_id, parent_instance_id, start_invocation_id, start_key)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13::uuid, $14)
		ON CONFLICT (tenant_id, start_key) WHERE start_key IS NOT NULL DO NOTHING
		RETURNING id`,
		id, s.subject.TenantID, s.workflow, s.impl.Package, s.impl.Version, s.impl.Artifact.Hash, s.impl.Export, []byte(input),
		s.subject.UserID, string(kind), s.predecessor, s.parent, inv, key).Scan(&got)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `SELECT id FROM extruntime.workflow_instances WHERE tenant_id = $1 AND start_key = $2`, s.subject.TenantID, s.key).Scan(&got)
		if err != nil {
			return uuid.Nil, false, serrors.FromDB(opStart, err)
		}
		return got, false, nil
	}
	if err != nil {
		return uuid.Nil, false, serrors.FromDB(opStart, err)
	}
	if _, err := tx.Exec(ctx, `SELECT pg_notify($1, '')`, NotifyChannel); err != nil {
		return uuid.Nil, false, serrors.FromDB(opStart, err)
	}
	return got, true, nil
}
