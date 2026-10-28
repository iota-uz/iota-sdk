package changes

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/fielddiff"
	"github.com/iota-uz/iota-sdk/pkg/mcp"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const (
	defaultPlanTTL           = 30 * time.Minute
	defaultApprovalTTL       = 30 * time.Minute
	defaultMaxChanges        = 200
	defaultMaxEffectAttempts = 5
	pendingRetryAfter        = 2 * time.Second
)

type Config struct {
	Store      Store
	Authorizer mcp.Authorizer
	Kinds      []Kind
	Effects    map[string]EffectHandler
	// ApprovalURL points to the trusted page that shows a plan's diff to a
	// human; the link itself is not an approval.
	ApprovalURL       func(planID string) string
	PlanTTL           time.Duration
	ApprovalTTL       time.Duration
	MaxChanges        int
	MaxEffectAttempts int
	// ToolPolicy guards the tools themselves; every change is additionally
	// checked against its Kind policy.
	ToolPolicy mcp.Policy
	Now        func() time.Time
}

type Service struct {
	cfg   Config
	kinds map[string]Kind
}

func kindKey(entity, op string) string { return entity + "." + op }

func NewService(cfg Config) (*Service, error) {
	const op serrors.Op = "changes.NewService"
	if cfg.Store == nil || cfg.Authorizer == nil {
		return nil, serrors.New(serrors.Invalid, "store and authorizer are required").WithOp(op)
	}
	if cfg.ApprovalURL == nil {
		return nil, serrors.New(serrors.Invalid, "approval URL builder is required").WithOp(op)
	}
	if cfg.PlanTTL == 0 {
		cfg.PlanTTL = defaultPlanTTL
	}
	if cfg.ApprovalTTL == 0 {
		cfg.ApprovalTTL = defaultApprovalTTL
	}
	if cfg.MaxChanges == 0 {
		cfg.MaxChanges = defaultMaxChanges
	}
	if cfg.MaxEffectAttempts == 0 {
		cfg.MaxEffectAttempts = defaultMaxEffectAttempts
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	s := &Service{cfg: cfg, kinds: make(map[string]Kind, len(cfg.Kinds))}
	for _, k := range cfg.Kinds {
		key := kindKey(k.Entity(), k.Op())
		if _, dup := s.kinds[key]; dup {
			return nil, serrors.New(serrors.AlreadyExists, "change kind "+key+" is already registered").WithOp(op)
		}
		s.kinds[key] = k
	}
	return s, nil
}

func principal(ctx context.Context) (mcp.Principal, error) {
	p, ok := mcp.PrincipalFrom(ctx)
	if !ok {
		return mcp.Principal{}, mcp.Fail(mcp.CodeForbidden, mcp.RecoveryReauthenticate, "the caller is not authenticated")
	}
	return p, nil
}

func (s *Service) kind(c Change) (Kind, bool) {
	k, ok := s.kinds[kindKey(c.Entity, c.Op)]
	return k, ok
}

// Prepare validates the changes and stores an immutable plan. Nothing in the
// business data is written; an invalid batch yields no applicable plan.
func (s *Service) Prepare(ctx context.Context, changes []Change, reason string) (Prepared, error) {
	const op serrors.Op = "changes.Service.Prepare"
	p, err := principal(ctx)
	if err != nil {
		return Prepared{}, err
	}
	if len(changes) == 0 {
		return Prepared{}, mcp.Fail(mcp.CodeInvalidInput, mcp.RecoveryFixInput, "the plan has no changes",
			mcp.Issue{Path: "/changes", Code: "minItems", Message: "at least one change is required"})
	}
	if len(changes) > s.cfg.MaxChanges {
		return Prepared{}, mcp.Fail(mcp.CodeLimitExceeded, mcp.RecoveryFixInput,
			fmt.Sprintf("a plan holds at most %d changes", s.cfg.MaxChanges))
	}

	var errs []IndexedIssue
	var warnings []mcp.Issue
	items := make([]PlanItem, 0, len(changes))
	approval := false
	for i, c := range changes {
		at := fmt.Sprintf("/changes/%d", i)
		k, ok := s.kind(c)
		if !ok {
			errs = append(errs, IndexedIssue{Index: i, Path: at, Code: "unknown_kind",
				Message: "no change kind " + kindKey(c.Entity, c.Op)})
			continue
		}
		if k.Existing() && (c.ID == "" || c.ExpectedRevision == "") {
			errs = append(errs, IndexedIssue{Index: i, Path: at + "/expected_revision", Code: "required",
				Message: "changes to existing objects need id and expected_revision"})
			continue
		}
		authCtx, err := s.cfg.Authorizer.Authorize(ctx, p, k.Policy())
		if err != nil {
			return Prepared{}, err
		}
		planned, err := k.Plan(authCtx, c)
		if err != nil {
			if code := serrors.CodeOf(err); code == serrors.NotFound || code == serrors.Invalid {
				f := mcp.FailureFrom(err)
				errs = append(errs, IndexedIssue{Index: i, Path: at, Code: string(f.Code), Message: f.Message})
				continue
			}
			return Prepared{}, serrors.Wrap(op, err)
		}
		for _, iss := range planned.Errors {
			errs = append(errs, IndexedIssue{Index: i, Path: at + iss.Path, Code: iss.Code, Message: iss.Message})
		}
		if k.Existing() && planned.Revision != c.ExpectedRevision {
			errs = append(errs, IndexedIssue{Index: i, Path: at + "/expected_revision", Code: "revision_mismatch",
				Message: "current revision is " + planned.Revision + "; re-read the object and prepare again"})
		}
		for _, w := range planned.Warnings {
			warnings = append(warnings, mcp.Issue{Path: at + w.Path, Code: w.Code, Message: w.Message})
		}
		diff := planned.Diff
		if diff == nil {
			diff = []fielddiff.Diff{}
		}
		items = append(items, PlanItem{Index: i, Op: c.Op, Entity: c.Entity, ID: c.ID, Ref: c.Ref,
			ExpectedRevision: c.ExpectedRevision, Diff: diff})
		approval = approval || planned.NeedsApproval
	}
	if len(errs) > 0 {
		return Prepared{Status: StatusInvalid, Errors: errs, Warnings: warnings}, nil
	}

	plan := Plan{
		ID:               uuid.NewString(),
		TenantID:         p.TenantID.String(),
		Subject:          p.Subject(),
		ClientID:         p.ClientID,
		Reason:           strings.TrimSpace(reason),
		Changes:          changes,
		Items:            items,
		Warnings:         warnings,
		ApprovalRequired: approval,
		ExpiresAt:        s.cfg.Now().Add(s.cfg.PlanTTL).UTC().Truncate(time.Second),
	}
	if plan.Hash, err = hashPlan(plan); err != nil {
		return Prepared{}, serrors.Wrap(op, err)
	}
	if err := s.cfg.Store.SavePlan(ctx, plan); err != nil {
		return Prepared{}, serrors.Wrap(op, err)
	}
	out := Prepared{Status: StatusReady, PlanID: plan.ID, PlanHash: plan.Hash, ExpiresAt: plan.ExpiresAt,
		Items: items, Warnings: warnings, Approval: ApprovalState{Required: approval}}
	if approval {
		out.Approval.URL = s.cfg.ApprovalURL(plan.ID)
	}
	return out, nil
}

// hashPlan binds the whole effect, the base revisions, the subject and the
// expiry, so any difference produces another hash.
func hashPlan(p Plan) (string, error) {
	raw, err := json.Marshal(struct {
		TenantID  string     `json:"tenant_id"`
		Subject   string     `json:"subject"`
		ClientID  string     `json:"client_id"`
		Changes   []Change   `json:"changes"`
		Items     []PlanItem `json:"items"`
		ExpiresAt string     `json:"expires_at"`
	}{p.TenantID, p.Subject, p.ClientID, p.Changes, p.Items, p.ExpiresAt.UTC().Format(time.RFC3339)})
	if err != nil {
		return "", err
	}
	canonical, err := canonicalJSON(raw)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}

func canonicalJSON(raw []byte) ([]byte, error) {
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return json.Marshal(v)
}

type ApplyInput struct {
	PlanID         string `json:"plan_id"`
	PlanHash       string `json:"plan_hash"`
	IdempotencyKey string `json:"idempotency_key"`
	ApprovalRef    string `json:"approval_ref,omitempty"`
}

// Apply executes a prepared plan at most once per idempotency key. A pending
// result means the outcome is not settled yet and must be checked again with
// the same key, never re-prepared blindly.
func (s *Service) Apply(ctx context.Context, in ApplyInput) (Operation, error) {
	const op serrors.Op = "changes.Service.Apply"
	p, err := principal(ctx)
	if err != nil {
		return Operation{}, err
	}
	tenant, subject := p.TenantID.String(), p.Subject()
	key := strings.TrimSpace(in.IdempotencyKey)
	if key == "" {
		return Operation{}, mcp.Fail(mcp.CodeInvalidInput, mcp.RecoveryFixInput, "idempotency_key is required")
	}

	existing, err := s.cfg.Store.OperationByKey(ctx, tenant, subject, key)
	switch {
	case err == nil:
		if existing.PlanID != in.PlanID || existing.PlanHash != in.PlanHash {
			return Operation{}, keyReused(existing.ID)
		}
		if existing.Status != OperationPending || existing.Receipt != nil {
			return s.withRetry(existing), nil
		}
	case serrors.CodeOf(err) != serrors.NotFound:
		return Operation{}, serrors.Wrap(op, err)
	}

	plan, err := s.cfg.Store.Plan(ctx, tenant, in.PlanID)
	if serrors.CodeOf(err) == serrors.NotFound || (err == nil && plan.Subject != subject) {
		return Operation{}, mcp.Fail(mcp.CodeNotFound, mcp.RecoveryReadThenPrepare, "unknown plan "+in.PlanID)
	}
	if err != nil {
		return Operation{}, serrors.Wrap(op, err)
	}
	if plan.Hash != in.PlanHash {
		return Operation{}, mcp.Fail(mcp.CodeConflict, mcp.RecoveryReadThenPrepare,
			"plan_hash does not match the prepared plan")
	}
	if existing.ID == "" {
		if !s.cfg.Now().Before(plan.ExpiresAt) {
			return Operation{}, mcp.Fail(mcp.CodePlanExpired, mcp.RecoveryReadThenPrepare,
				"the plan expired; read the data again and prepare a new plan")
		}
		if plan.ApprovalRequired {
			if err := s.checkApproval(ctx, plan, subject, in.ApprovalRef); err != nil {
				return Operation{}, err
			}
		}
		existing, err = s.cfg.Store.ClaimOperation(ctx, Operation{ID: uuid.NewString(), TenantID: tenant,
			Subject: subject, IdempotencyKey: key, PlanID: plan.ID, PlanHash: plan.Hash})
		if err != nil {
			return Operation{}, serrors.Wrap(op, err)
		}
		if existing.PlanID != plan.ID || existing.PlanHash != plan.Hash {
			return Operation{}, keyReused(existing.ID)
		}
	}
	return s.execute(ctx, p, plan, existing)
}

func keyReused(operationID string) *mcp.Failure {
	return mcp.Fail(mcp.CodeConflict, mcp.RecoveryFixInput,
		"the idempotency key was already used for another plan").WithOperation(operationID)
}

func (s *Service) checkApproval(ctx context.Context, plan Plan, subject, ref string) error {
	now := s.cfg.Now()
	var a Approval
	var err error
	if ref != "" {
		sum := sha256.Sum256([]byte(ref))
		a, err = s.cfg.Store.Approval(ctx, plan.TenantID, hex.EncodeToString(sum[:]))
	} else {
		a, err = s.cfg.Store.PlanApproval(ctx, plan.TenantID, plan.ID, plan.Hash, subject, now)
	}
	if err != nil && serrors.CodeOf(err) != serrors.NotFound {
		return err
	}
	if err != nil || a.PlanID != plan.ID || a.PlanHash != plan.Hash || a.Subject != subject || !now.Before(a.ExpiresAt) {
		return mcp.Fail(mcp.CodeApprovalRequired, mcp.RecoveryOpenApproval,
			"a person must review and approve this plan at "+s.cfg.ApprovalURL(plan.ID))
	}
	return nil
}

var errTransient = errors.New("transient apply failure")

func (s *Service) execute(ctx context.Context, p mcp.Principal, plan Plan, operation Operation) (Operation, error) {
	const op serrors.Op = "changes.Service.execute"
	var settled *Operation
	err := s.cfg.Store.InTx(ctx, func(txCtx context.Context) error {
		locked, err := s.cfg.Store.LockOperation(txCtx, operation.ID)
		if err != nil {
			return err
		}
		if locked.Status != OperationPending || locked.Receipt != nil {
			settled = &locked
			return nil
		}
		receipt := Receipt{OperationID: operation.ID, PlanID: plan.ID, PlanHash: plan.Hash, Items: []ReceiptItem{}}
		var effects []Effect
		for i, c := range plan.Changes {
			applied, err := s.applyOne(txCtx, p, i, c)
			if err != nil {
				return err
			}
			receipt.Items = append(receipt.Items, ReceiptItem{Index: i, Entity: c.Entity, ID: applied.ID,
				Revision: applied.Revision, Ref: c.Ref})
			effects = append(effects, applied.Effects...)
		}
		receipt.CommittedAt = s.cfg.Now().UTC().Format(time.RFC3339)
		if err := s.cfg.Store.CompleteOperation(txCtx, operation.ID, receipt, effects); err != nil {
			return errors.Join(errTransient, err)
		}
		done := operation
		done.Receipt = &receipt
		done.Status, done.ExternalEffects = OperationSucceeded, EffectsNone
		if len(effects) > 0 {
			done.Status, done.ExternalEffects = OperationPending, EffectsPending
		}
		settled = &done
		return nil
	})
	if err == nil {
		return s.withRetry(*settled), nil
	}
	var f *mcp.Failure
	if errors.As(err, &f) && !errors.Is(err, errTransient) && !errors.Is(err, ErrOperationLocked) && isDefinite(f.Code) {
		f = f.WithOperation(operation.ID)
		if ferr := s.cfg.Store.FailOperation(ctx, operation.ID, nil, EffectsNone, f); ferr != nil {
			return Operation{}, serrors.Wrap(op, ferr)
		}
		operation.Status, operation.ExternalEffects, operation.Error = OperationFailed, EffectsNone, f
		return operation, nil
	}
	// The transaction either rolled back on a transient error or its commit
	// outcome is unknown; the operation stays pending and the same key resumes it.
	operation.Status, operation.ExternalEffects = OperationPending, EffectsNone
	return s.withRetry(operation), nil
}

func isDefinite(code mcp.Code) bool {
	switch code {
	case mcp.CodeInvalidInput, mcp.CodeNotFound, mcp.CodeForbidden, mcp.CodeConflict:
		return true
	case mcp.CodeApprovalRequired, mcp.CodePlanExpired, mcp.CodeLimitExceeded, mcp.CodeBusy, mcp.CodeDependencyUnavailable:
		return false
	default:
		return false
	}
}

// applyOne re-authorizes against current account state and re-checks the
// planned revision inside the transaction: no last-write-wins.
func (s *Service) applyOne(ctx context.Context, p mcp.Principal, i int, c Change) (Applied, error) {
	at := fmt.Sprintf("/changes/%d", i)
	k, ok := s.kind(c)
	if !ok {
		return Applied{}, mcp.Fail(mcp.CodeConflict, mcp.RecoveryReadThenPrepare,
			"change kind "+kindKey(c.Entity, c.Op)+" is no longer available")
	}
	authCtx, err := s.cfg.Authorizer.Authorize(ctx, p, k.Policy())
	if err != nil {
		return Applied{}, mcp.FailureFrom(err)
	}
	if k.Existing() {
		current, err := k.Revision(authCtx, c.ID)
		if err != nil {
			return Applied{}, mcp.FailureFrom(err)
		}
		if current != c.ExpectedRevision {
			return Applied{}, mcp.Fail(mcp.CodeConflict, mcp.RecoveryReadThenPrepare,
				"the object changed after the plan was prepared; re-read it and prepare again",
				mcp.Issue{Path: at + "/expected_revision", Code: "revision_mismatch", Message: "current revision is " + current})
		}
	}
	applied, err := k.Apply(authCtx, c)
	if err != nil {
		return Applied{}, mcp.FailureFrom(err)
	}
	return applied, nil
}

func (s *Service) withRetry(o Operation) Operation {
	if o.Status == OperationPending {
		o.RetryAfterMS = pendingRetryAfter.Milliseconds()
	}
	return o
}

// OperationGet looks an operation up by exactly one selector. Not finding one
// does not prove that nothing was written.
func (s *Service) OperationGet(ctx context.Context, operationID, idempotencyKey string) (Operation, error) {
	const op serrors.Op = "changes.Service.OperationGet"
	p, err := principal(ctx)
	if err != nil {
		return Operation{}, err
	}
	if (operationID == "") == (idempotencyKey == "") {
		return Operation{}, mcp.Fail(mcp.CodeInvalidInput, mcp.RecoveryFixInput,
			"pass exactly one of operation_id and idempotency_key")
	}
	var o Operation
	if operationID != "" {
		o, err = s.cfg.Store.Operation(ctx, p.TenantID.String(), p.Subject(), operationID)
	} else {
		o, err = s.cfg.Store.OperationByKey(ctx, p.TenantID.String(), p.Subject(), idempotencyKey)
	}
	if serrors.CodeOf(err) == serrors.NotFound {
		return Operation{}, mcp.Fail(mcp.CodeNotFound, mcp.RecoveryCheckOperation,
			"no operation is recorded for this selector; this alone does not prove that nothing was written")
	}
	if err != nil {
		return Operation{}, serrors.Wrap(op, err)
	}
	return s.withRetry(o), nil
}

// ReviewPlan returns a stored plan for the trusted approval page.
func (s *Service) ReviewPlan(ctx context.Context, planID string) (Plan, error) {
	p, err := principal(ctx)
	if err != nil {
		return Plan{}, err
	}
	return s.cfg.Store.Plan(ctx, p.TenantID.String(), planID)
}

type Grant struct {
	Ref       string
	ExpiresAt time.Time
}

// Approve is called by the trusted UI after the person saw the plan's diff;
// it must never be reachable from a model-facing tool. The approver must be
// allowed to perform every change of the plan.
func (s *Service) Approve(ctx context.Context, planID, planHash string) (Grant, error) {
	const op serrors.Op = "changes.Service.Approve"
	p, err := principal(ctx)
	if err != nil {
		return Grant{}, err
	}
	plan, err := s.cfg.Store.Plan(ctx, p.TenantID.String(), planID)
	if err != nil {
		return Grant{}, serrors.Wrap(op, err)
	}
	if plan.Hash != planHash {
		return Grant{}, mcp.Fail(mcp.CodeConflict, mcp.RecoveryReadThenPrepare, "the plan was replaced")
	}
	now := s.cfg.Now()
	if !now.Before(plan.ExpiresAt) {
		return Grant{}, mcp.Fail(mcp.CodePlanExpired, mcp.RecoveryReadThenPrepare, "the plan expired")
	}
	for _, c := range plan.Changes {
		k, ok := s.kind(c)
		if !ok {
			return Grant{}, mcp.Fail(mcp.CodeConflict, mcp.RecoveryReadThenPrepare, "the plan uses an unavailable change kind")
		}
		if _, err := s.cfg.Authorizer.Authorize(ctx, p, k.Policy()); err != nil {
			return Grant{}, err
		}
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return Grant{}, serrors.Wrap(op, err)
	}
	ref := base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256([]byte(ref))
	expires := now.Add(s.cfg.ApprovalTTL)
	if plan.ExpiresAt.Before(expires) {
		expires = plan.ExpiresAt
	}
	if err := s.cfg.Store.SaveApproval(ctx, Approval{RefHash: hex.EncodeToString(sum[:]), TenantID: plan.TenantID,
		PlanID: plan.ID, PlanHash: plan.Hash, Subject: plan.Subject, ApprovedBy: p.Subject(), ExpiresAt: expires}); err != nil {
		return Grant{}, serrors.Wrap(op, err)
	}
	return Grant{Ref: ref, ExpiresAt: expires}, nil
}

// DispatchEffects delivers committed external effects from the outbox. Run
// it periodically; it is safe to run on several instances at once.
func (s *Service) DispatchEffects(ctx context.Context, limit int) (int, error) {
	const op serrors.Op = "changes.Service.DispatchEffects"
	delivered := 0
	err := s.cfg.Store.InTx(ctx, func(txCtx context.Context) error {
		pending, err := s.cfg.Store.PendingEffects(txCtx, limit)
		if err != nil {
			return err
		}
		for _, e := range pending {
			attempts := e.Attempts + 1
			var deliverErr error
			if h, ok := s.cfg.Effects[e.Effect.Kind]; ok {
				tenantID, perr := uuid.Parse(e.TenantID)
				if perr != nil {
					return perr
				}
				deliverErr = h(composables.WithTenantID(ctx, tenantID), e.ID, e.Effect)
			} else {
				deliverErr = errors.New("no handler for effect " + e.Effect.Kind)
				attempts = s.cfg.MaxEffectAttempts
			}
			lastError := ""
			if deliverErr != nil {
				lastError = deliverErr.Error()
			} else {
				delivered++
			}
			final := deliverErr != nil && attempts >= s.cfg.MaxEffectAttempts
			if err := s.cfg.Store.FinishEffect(txCtx, e.ID, deliverErr == nil, attempts, lastError, final); err != nil {
				return err
			}
			if err := s.cfg.Store.SettleEffects(txCtx, e.OperationID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, serrors.Wrap(op, err)
	}
	return delivered, nil
}
