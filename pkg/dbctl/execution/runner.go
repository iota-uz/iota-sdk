package execution

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/commands/common"
	"github.com/iota-uz/iota-sdk/pkg/commands/e2e"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/dbctl/ops"
	"github.com/iota-uz/iota-sdk/pkg/dbctl/policy"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RunOptions struct {
	Operation  string
	Mode       ops.ExecutionMode
	Yes        bool
	DryRun     bool
	JSONOutput bool
	PolicyPath string
	Actor      string
	Out        io.Writer
}

type PlanResult struct {
	RunContext ops.RunContext
	Spec       ops.OperationSpec
	PolicyHash string
}

type runLock struct {
	conn *pgxpool.Conn
	key  int64
}

func Plan(ctx context.Context, opts RunOptions) (*PlanResult, error) {
	const op serrors.Op = "dbctl.execution.Plan"
	if strings.TrimSpace(opts.Operation) == "" {
		return nil, serrors.E(op, serrors.Invalid, "operation is required")
	}
	spec, err := ops.Get(opts.Operation)
	if err != nil {
		return nil, err
	}
	cfg, payload, err := policy.Load(opts.PolicyPath)
	if err != nil {
		return nil, err
	}
	target := resolveTarget(opts.Operation)
	decision := policy.Evaluate(cfg, target, spec.Kind == ops.OperationKindDestructive)
	if !decision.Allowed {
		return nil, serrors.E(op, serrors.PermissionDenied, "policy denied operation: "+strings.Join(decision.Reasons, "; "))
	}
	if spec.Kind == ops.OperationKindDestructive && !opts.Yes {
		return nil, serrors.E(op, serrors.Invalid, "destructive operations require --yes confirmation")
	}

	pool, err := getControlDatabasePool(ctx, opts.Operation)
	if err != nil {
		return nil, serrors.E(op, err, "open database pool")
	}
	defer pool.Close()

	execCtx := &ops.ExecutionContext{
		Run: &ops.RunContext{
			Operation:      opts.Operation,
			ExecutionMode:  opts.Mode,
			Target:         target,
			PolicyDecision: decision,
			Yes:            opts.Yes,
		},
		Pool:       pool,
		PolicyPath: opts.PolicyPath,
	}
	for _, cond := range spec.Preconditions {
		if cond.Check == nil {
			continue
		}
		if err := cond.Check(ctx, execCtx); err != nil {
			return nil, serrors.E(op, err, "precondition "+cond.ID+" failed")
		}
	}
	return &PlanResult{RunContext: *execCtx.Run, Spec: spec, PolicyHash: policy.HashPolicy(payload)}, nil
}

func Apply(ctx context.Context, opts RunOptions) error {
	const op serrors.Op = "dbctl.execution.Apply"
	if opts.Out == nil {
		opts.Out = os.Stdout
	}
	plan, err := Plan(ctx, opts)
	if err != nil {
		return err
	}
	Emit(opts.Out, opts.JSONOutput, Event{Type: "preflight", Operation: plan.Spec.Name, Message: "plan completed"})
	Emit(opts.Out, opts.JSONOutput, Event{
		Type:      "safety_banner",
		Operation: plan.Spec.Name,
		Message:   fmt.Sprintf("target env=%s host=%s:%s user=%s db=%s steps=%d", plan.RunContext.Target.Environment, plan.RunContext.Target.Host, plan.RunContext.Target.Port, plan.RunContext.Target.User, plan.RunContext.Target.Name, len(plan.Spec.Steps)),
		Payload: map[string]any{
			"target":          plan.RunContext.Target,
			"planned_actions": stepSummaries(plan.Spec),
			"confirmed":       opts.Yes,
			"dry_run":         opts.DryRun,
		},
	})
	if opts.DryRun {
		Emit(opts.Out, opts.JSONOutput, Event{
			Type:      "end_summary",
			Operation: plan.Spec.Name,
			Status:    "dry-run",
			Message:   "dry-run preview completed",
			Payload: map[string]any{
				"target":          plan.RunContext.Target,
				"planned_actions": stepSummaries(plan.Spec),
				"final_status":    "dry-run",
			},
		})
		return nil
	}

	pool, err := getControlDatabasePool(ctx, opts.Operation)
	if err != nil {
		return serrors.E(op, err, "open database pool")
	}
	defer pool.Close()

	targetFP := fingerprint(plan.RunContext.Target)
	lock, locked, lockErr := acquireRunLock(ctx, pool, plan.Spec.Name, targetFP)
	if lockErr != nil {
		return lockErr
	}
	if !locked {
		return serrors.E(op, "failed to acquire run lock for operation")
	}
	defer releaseRunLock(ctx, lock)

	execCtx := &ops.ExecutionContext{
		Run: &ops.RunContext{
			Operation:      plan.Spec.Name,
			ExecutionMode:  opts.Mode,
			Target:         plan.RunContext.Target,
			PolicyDecision: plan.RunContext.PolicyDecision,
			Yes:            opts.Yes,
		},
		Pool:       pool,
		JSONOutput: opts.JSONOutput,
		PolicyPath: opts.PolicyPath,
	}

	for _, step := range plan.Spec.Steps {
		Emit(opts.Out, opts.JSONOutput, Event{Type: "step_start", Operation: plan.Spec.Name, StepID: step.ID, Message: step.Description})
		stepErr := runStep(ctx, execCtx, step)
		if stepErr != nil {
			Emit(opts.Out, opts.JSONOutput, Event{Type: "step_end", Operation: plan.Spec.Name, StepID: step.ID, Status: "failed", Message: stepErr.Error()})
			return stepErr
		}
		Emit(opts.Out, opts.JSONOutput, Event{Type: "step_end", Operation: plan.Spec.Name, StepID: step.ID, Status: "succeeded", Message: "step completed"})
	}

	for _, cond := range plan.Spec.Postconditions {
		if cond.Check == nil {
			continue
		}
		if err := cond.Check(ctx, execCtx); err != nil {
			return serrors.E(op, err, "postcondition "+cond.ID+" failed")
		}
	}

	Emit(opts.Out, opts.JSONOutput, Event{
		Type:      "end_summary",
		Operation: plan.Spec.Name,
		Status:    "succeeded",
		Message:   "run completed",
		Payload: map[string]any{
			"target":          plan.RunContext.Target,
			"planned_actions": stepSummaries(plan.Spec),
			"final_status":    "succeeded",
		},
	})
	Emit(opts.Out, opts.JSONOutput, Event{Type: "summary", Operation: plan.Spec.Name, Status: "succeeded", Message: "run completed"})
	return nil
}

func runStep(ctx context.Context, execCtx *ops.ExecutionContext, step ops.StepSpec) error {
	if step.Handler == nil {
		return nil
	}
	switch step.TxMode {
	case ops.TxModeNone:
		return step.Handler(ctx, execCtx)
	case ops.TxModeOwnTx:
		tx, err := execCtx.Pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer func() { _ = tx.Rollback(ctx) }()
		execCtx.Tx = tx
		err = step.Handler(ctx, execCtx)
		if err != nil {
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		execCtx.Tx = nil
		return nil
	default:
		return fmt.Errorf("unknown tx mode: %s", step.TxMode)
	}
}

func stepSummaries(spec ops.OperationSpec) []map[string]string {
	steps := make([]map[string]string, 0, len(spec.Steps))
	for _, step := range spec.Steps {
		steps = append(steps, map[string]string{
			"id":          step.ID,
			"description": step.Description,
			"tx_mode":     string(step.TxMode),
		})
	}
	return steps
}

func resolveTarget(operation string) policy.Target {
	conf := configuration.Use()
	databaseName := strings.TrimSpace(conf.Database.Name)
	if isE2EOperation(operation) {
		databaseName = e2e.E2EDBName
	}
	return policy.Target{
		Environment: strings.TrimSpace(conf.GoAppEnvironment),
		Host:        strings.TrimSpace(conf.Database.Host),
		Port:        strings.TrimSpace(conf.Database.Port),
		Name:        databaseName,
		User:        strings.TrimSpace(conf.Database.User),
	}
}

func isE2EOperation(operation string) bool {
	op := strings.TrimSpace(operation)
	return strings.HasPrefix(op, "db.e2e.") || op == "seed.e2e"
}

func fingerprint(target policy.Target) string {
	raw := strings.Join([]string{target.Environment, target.Host, target.Port, target.Name, target.User}, "|")
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func advisoryKey(parts ...string) int64 {
	h := fnv.New64a()
	for _, p := range parts {
		_, _ = h.Write([]byte(p))
	}
	return int64(h.Sum64())
}

func getControlDatabasePool(ctx context.Context, operation string) (*pgxpool.Pool, error) {
	return common.GetDatabasePool(ctx, controlDatabaseName(operation))
}

func controlDatabaseName(operation string) string {
	switch operation {
	case "db.e2e.create", "db.e2e.drop", "db.e2e.reset":
		return "postgres"
	default:
		return ""
	}
}

func acquireRunLock(ctx context.Context, pool *pgxpool.Pool, operation, targetFP string) (*runLock, bool, error) {
	if pool == nil {
		return nil, false, errors.New("nil pool for lock acquisition")
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("acquire advisory lock connection: %w", err)
	}
	key := advisoryKey(operation, targetFP)
	var locked bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", key).Scan(&locked); err != nil {
		conn.Release()
		return nil, false, fmt.Errorf("acquire advisory lock: %w", err)
	}
	if !locked {
		conn.Release()
		return nil, false, nil
	}
	return &runLock{conn: conn, key: key}, true, nil
}

func releaseRunLock(ctx context.Context, lock *runLock) {
	if lock == nil || lock.conn == nil {
		return
	}
	defer lock.conn.Release()
	var unlocked bool
	_ = lock.conn.QueryRow(ctx, "SELECT pg_advisory_unlock($1)", lock.key).Scan(&unlocked)
}
