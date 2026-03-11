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
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/commands/common"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/dbctl/credentials"
	"github.com/iota-uz/iota-sdk/pkg/dbctl/ops"
	"github.com/iota-uz/iota-sdk/pkg/dbctl/persistence"
	"github.com/iota-uz/iota-sdk/pkg/dbctl/policy"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RunOptions struct {
	Operation     string
	Mode          ops.ExecutionMode
	Yes           bool
	ApproveTicket string
	JSONOutput    bool
	PolicyPath    string
	Actor         string
	Out           io.Writer
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
	if strings.TrimSpace(opts.Operation) == "" {
		return nil, fmt.Errorf("operation is required")
	}
	spec, err := ops.Get(opts.Operation)
	if err != nil {
		return nil, err
	}
	cfg, payload, err := policy.Load(opts.PolicyPath)
	if err != nil {
		return nil, err
	}
	target := resolveTarget()
	decision := policy.Evaluate(cfg, target, spec.Kind == ops.OperationKindDestructive)
	if !decision.Allowed {
		return nil, fmt.Errorf("policy denied operation: %s", strings.Join(decision.Reasons, "; "))
	}
	if decision.RequireYes && !opts.Yes {
		return nil, fmt.Errorf("policy requires --yes confirmation")
	}
	if decision.RequireTicket && strings.TrimSpace(opts.ApproveTicket) == "" {
		return nil, fmt.Errorf("policy requires --approve-ticket")
	}

	pool, err := getControlDatabasePool(ctx, opts.Operation)
	if err != nil {
		return nil, fmt.Errorf("open database pool: %w", err)
	}
	defer pool.Close()

	execCtx := &ops.ExecutionContext{
		Run: &ops.RunContext{
			Operation:      opts.Operation,
			ExecutionMode:  opts.Mode,
			Target:         target,
			PolicyDecision: decision,
			ApproveTicket:  opts.ApproveTicket,
			Yes:            opts.Yes,
		},
		Pool:       pool,
		Policy:     cfg,
		PolicyHash: policy.HashPolicy(payload),
		PolicyPath: opts.PolicyPath,
	}
	for _, cond := range spec.Preconditions {
		if cond.Check == nil {
			continue
		}
		if err := cond.Check(ctx, execCtx); err != nil {
			return nil, fmt.Errorf("precondition %s failed: %w", cond.ID, err)
		}
	}
	return &PlanResult{RunContext: *execCtx.Run, Spec: spec, PolicyHash: execCtx.PolicyHash}, nil
}

func Apply(ctx context.Context, opts RunOptions) error {
	if opts.Out == nil {
		opts.Out = os.Stdout
	}
	plan, err := Plan(ctx, opts)
	if err != nil {
		return err
	}
	Emit(opts.Out, opts.JSONOutput, Event{Type: "preflight", Operation: plan.Spec.Name, Message: "plan completed"})

	pool, err := getControlDatabasePool(ctx, opts.Operation)
	if err != nil {
		return fmt.Errorf("open database pool: %w", err)
	}
	defer pool.Close()

	cfg, payload, err := policy.Load(opts.PolicyPath)
	if err != nil {
		return err
	}
	repo := persistence.New(pool)
	if err := repo.EnsureTables(ctx); err != nil {
		return err
	}

	runID := uuid.NewString()
	actor := strings.TrimSpace(opts.Actor)
	if actor == "" {
		actor = os.Getenv("USER")
		if actor == "" {
			actor = "unknown"
		}
	}
	targetFP := fingerprint(plan.RunContext.Target)
	run := persistence.RunRecord{
		ID:                runID,
		Operation:         plan.Spec.Name,
		Mode:              string(opts.Mode),
		StartedAt:         time.Now().UTC(),
		Actor:             actor,
		Status:            "running",
		TargetFingerprint: targetFP,
		PolicyHash:        policy.HashPolicy(payload),
	}
	if err := repo.InsertRun(ctx, run); err != nil {
		return err
	}

	lock, locked, lockErr := acquireRunLock(ctx, pool, plan.Spec.Name, targetFP)
	if lockErr != nil {
		return lockErr
	}
	if !locked {
		err := fmt.Errorf("failed to acquire run lock for operation")
		errText := err.Error()
		_ = repo.UpdateRunStatus(ctx, runID, "failed", &errText)
		return err
	}
	defer releaseRunLock(ctx, lock)

	execCtx := &ops.ExecutionContext{
		Run: &ops.RunContext{
			Operation:      plan.Spec.Name,
			ExecutionMode:  opts.Mode,
			Target:         plan.RunContext.Target,
			PolicyDecision: plan.RunContext.PolicyDecision,
			ApproveTicket:  opts.ApproveTicket,
			Yes:            opts.Yes,
			RunID:          runID,
		},
		Pool:       pool,
		Policy:     cfg,
		PolicyHash: plan.PolicyHash,
		JSONOutput: opts.JSONOutput,
		PolicyPath: opts.PolicyPath,
	}

	for _, step := range plan.Spec.Steps {
		Emit(opts.Out, opts.JSONOutput, Event{Type: "step_start", Operation: plan.Spec.Name, StepID: step.ID, Message: step.Description})
		startedAt := time.Now().UTC()
		_ = repo.UpsertStep(ctx, persistence.StepRecord{RunID: runID, StepID: step.ID, Status: "running", StartedAt: startedAt})

		stepErr := runStep(ctx, execCtx, step)
		if stepErr != nil {
			errText := stepErr.Error()
			finished := time.Now().UTC()
			_ = repo.UpsertStep(ctx, persistence.StepRecord{RunID: runID, StepID: step.ID, Status: "failed", StartedAt: startedAt, FinishedAt: &finished, Error: &errText})
			_ = repo.UpdateRunStatus(ctx, runID, "failed", &errText)
			Emit(opts.Out, opts.JSONOutput, Event{Type: "step_end", Operation: plan.Spec.Name, StepID: step.ID, Status: "failed", Message: stepErr.Error()})
			return stepErr
		}
		finished := time.Now().UTC()
		_ = repo.UpsertStep(ctx, persistence.StepRecord{RunID: runID, StepID: step.ID, Status: "succeeded", StartedAt: startedAt, FinishedAt: &finished})
		Emit(opts.Out, opts.JSONOutput, Event{Type: "step_end", Operation: plan.Spec.Name, StepID: step.ID, Status: "succeeded", Message: "step completed"})
	}

	for _, cond := range plan.Spec.Postconditions {
		if cond.Check == nil {
			continue
		}
		if err := cond.Check(ctx, execCtx); err != nil {
			errText := err.Error()
			_ = repo.UpdateRunStatus(ctx, runID, "failed", &errText)
			return fmt.Errorf("postcondition %s failed: %w", cond.ID, err)
		}
	}

	if strings.HasPrefix(plan.Spec.Name, "seed.") {
		if err := createBootstrapArtifact(ctx, repo, execCtx); err != nil {
			Emit(opts.Out, opts.JSONOutput, Event{Type: "summary", Operation: plan.Spec.Name, Status: "warning", Message: "failed to create bootstrap artifact"})
		}
	}

	_ = repo.UpdateRunStatus(ctx, runID, "succeeded", nil)
	Emit(opts.Out, opts.JSONOutput, Event{Type: "summary", Operation: plan.Spec.Name, Status: "succeeded", Message: "run completed"})
	return nil
}

func createBootstrapArtifact(ctx context.Context, repo *persistence.Repository, execCtx *ops.ExecutionContext) error {
	includeSecret := execCtx.Run.PolicyDecision.CredentialEmission != "token_only"
	artifact, err := credentials.NewBootstrapArtifact(execCtx.Run.Operation, time.Duration(execCtx.Policy.Credentials.TokenTTLSecond)*time.Second, includeSecret)
	if err != nil {
		return err
	}
	payload, err := artifact.JSON()
	if err != nil {
		return err
	}
	return repo.InsertArtifact(ctx, execCtx.Run.RunID, "credential_bootstrap", payload)
}

func runStep(ctx context.Context, execCtx *ops.ExecutionContext, step ops.StepSpec) error {
	if step.Handler == nil {
		return nil
	}
	switch step.TxMode {
	case ops.TxModeNone:
		return step.Handler(ctx, execCtx)
	case ops.TxModeOwnTx, ops.TxModeSingleTx:
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

func resolveTarget() policy.Target {
	conf := configuration.Use()
	return policy.Target{
		Environment: strings.TrimSpace(conf.GoAppEnvironment),
		Host:        strings.TrimSpace(conf.Database.Host),
		Port:        strings.TrimSpace(conf.Database.Port),
		Name:        strings.TrimSpace(conf.Database.Name),
		User:        strings.TrimSpace(conf.Database.User),
	}
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
