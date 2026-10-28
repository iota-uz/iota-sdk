// Package changes implements guarded writes for AI callers: prepare_changes
// stores an immutable, hashed plan without touching business data,
// apply_changes executes exactly that plan once per idempotency key in one
// transaction, and operation_get recovers the outcome after a lost response.
package changes

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/iota-uz/iota-sdk/pkg/fielddiff"
	"github.com/iota-uz/iota-sdk/pkg/mcp"
)

// Change is one typed operation; the (entity, op) pair selects its Kind and
// Fields is validated against that Kind's schema.
type Change struct {
	Op               string          `json:"op"`
	Entity           string          `json:"entity"`
	ID               string          `json:"id,omitempty"`
	Ref              string          `json:"ref,omitempty"`
	ExpectedRevision string          `json:"expected_revision,omitempty"`
	Fields           json.RawMessage `json:"fields,omitempty"`
}

// Kind is an application-registered operation on one entity. Implementations
// call the existing domain services; they never write past them.
type Kind interface {
	Entity() string
	Op() string
	Description() string
	// Fields describes Change.Fields. Patch kinds keep every field optional:
	// an absent field is left untouched, clearing is an explicit value.
	Fields() *jsonschema.Schema
	Policy() mcp.Policy
	// Existing reports that the change addresses a stored object, which makes
	// id and expected_revision mandatory.
	Existing() bool
	// Plan checks the change against current data without writing.
	Plan(ctx context.Context, c Change) (Planned, error)
	// Revision returns the current revision of an existing object; it is
	// compared with the planned one inside the apply transaction.
	Revision(ctx context.Context, id string) (string, error)
	// Apply performs the change; ctx carries the plan transaction.
	Apply(ctx context.Context, c Change) (Applied, error)
}

type Planned struct {
	Diff []fielddiff.Diff
	// Revision is the revision Plan observed for an existing object.
	Revision      string
	Errors        []mcp.Issue
	Warnings      []mcp.Issue
	NeedsApproval bool
}

type Applied struct {
	ID       string
	Revision string
	// Effects are delivered after commit through the transactional outbox.
	Effects []Effect
}

type Effect struct {
	Kind    string
	Payload json.RawMessage
}

// EffectHandler delivers one external effect. It may run more than once for
// the same effect id and must deduplicate on it.
type EffectHandler func(ctx context.Context, effectID string, e Effect) error

type IndexedIssue struct {
	Index   int    `json:"index"`
	Path    string `json:"path"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type PlanItem struct {
	Index            int              `json:"index"`
	Op               string           `json:"op"`
	Entity           string           `json:"entity"`
	ID               string           `json:"id,omitempty"`
	Ref              string           `json:"ref,omitempty"`
	ExpectedRevision string           `json:"expected_revision,omitempty"`
	Diff             []fielddiff.Diff `json:"diff"`
}

type ApprovalState struct {
	Required bool   `json:"required"`
	URL      string `json:"url,omitempty"`
}

const (
	StatusInvalid = "invalid"
	StatusReady   = "ready"
)

type Prepared struct {
	Status    string
	Errors    []IndexedIssue
	Warnings  []mcp.Issue
	PlanID    string
	PlanHash  string
	ExpiresAt time.Time
	Items     []PlanItem
	Approval  ApprovalState
}

func (p Prepared) MarshalJSON() ([]byte, error) {
	warnings := p.Warnings
	if warnings == nil {
		warnings = []mcp.Issue{}
	}
	if p.Status == StatusInvalid {
		return json.Marshal(struct {
			Status   string         `json:"status"`
			Errors   []IndexedIssue `json:"errors"`
			Warnings []mcp.Issue    `json:"warnings"`
		}{p.Status, p.Errors, warnings})
	}
	return json.Marshal(struct {
		Status    string        `json:"status"`
		PlanID    string        `json:"plan_id"`
		PlanHash  string        `json:"plan_hash"`
		ExpiresAt string        `json:"expires_at"`
		Items     []PlanItem    `json:"items"`
		Warnings  []mcp.Issue   `json:"warnings"`
		Approval  ApprovalState `json:"approval"`
	}{p.Status, p.PlanID, p.PlanHash, p.ExpiresAt.UTC().Format(time.RFC3339), p.Items, warnings, p.Approval})
}

type ReceiptItem struct {
	Index    int    `json:"index"`
	Entity   string `json:"entity"`
	ID       string `json:"id"`
	Revision string `json:"revision"`
	Ref      string `json:"ref,omitempty"`
}

type Receipt struct {
	OperationID string        `json:"operation_id"`
	PlanID      string        `json:"plan_id"`
	PlanHash    string        `json:"plan_hash"`
	CommittedAt string        `json:"committed_at"`
	Items       []ReceiptItem `json:"items"`
}

const (
	OperationPending   = "pending"
	OperationSucceeded = "succeeded"
	OperationFailed    = "failed"

	EffectsNone      = "none"
	EffectsPending   = "pending"
	EffectsSucceeded = "succeeded"
	EffectsFailed    = "failed"
)

// Operation is the durable record of one apply attempt. A receipt means the
// database commit happened, whatever the external effects did afterwards.
type Operation struct {
	ID              string
	TenantID        string
	Subject         string
	IdempotencyKey  string
	PlanID          string
	PlanHash        string
	Status          string
	ExternalEffects string
	Receipt         *Receipt
	Error           *mcp.Failure
	RetryAfterMS    int64
}

func (o Operation) MarshalJSON() ([]byte, error) {
	switch o.Status {
	case OperationPending:
		return json.Marshal(struct {
			OperationID     string   `json:"operation_id"`
			Status          string   `json:"status"`
			Receipt         *Receipt `json:"receipt"`
			ExternalEffects string   `json:"external_effects"`
			RetryAfterMS    int64    `json:"retry_after_ms"`
		}{o.ID, o.Status, o.Receipt, o.ExternalEffects, o.RetryAfterMS})
	case OperationSucceeded:
		return json.Marshal(struct {
			OperationID     string   `json:"operation_id"`
			Status          string   `json:"status"`
			Receipt         *Receipt `json:"receipt"`
			ExternalEffects string   `json:"external_effects"`
		}{o.ID, o.Status, o.Receipt, o.ExternalEffects})
	default:
		return json.Marshal(struct {
			OperationID     string       `json:"operation_id"`
			Status          string       `json:"status"`
			Receipt         *Receipt     `json:"receipt"`
			ExternalEffects string       `json:"external_effects"`
			Error           *mcp.Failure `json:"error"`
		}{o.ID, o.Status, o.Receipt, o.ExternalEffects, o.Error})
	}
}

// Plan is the stored, immutable result of prepare_changes.
type Plan struct {
	ID               string
	TenantID         string
	Subject          string
	ClientID         string
	Hash             string
	Reason           string
	Changes          []Change
	Items            []PlanItem
	Warnings         []mcp.Issue
	ApprovalRequired bool
	ExpiresAt        time.Time
}

// Approval is issued by a trusted UI after a human saw the plan's diff.
type Approval struct {
	RefHash    string
	TenantID   string
	PlanID     string
	PlanHash   string
	Subject    string
	ApprovedBy string
	ExpiresAt  time.Time
}

type PendingEffect struct {
	ID          string
	OperationID string
	TenantID    string
	Attempts    int
	Effect      Effect
}
