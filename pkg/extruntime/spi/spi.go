// Package spi holds the Go interfaces shared between extension runtime
// slices (host, data, workflow, notify, ui). Implementations live in their
// own packages; this package has no logic.
package spi

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
)

// Subject is the identity an invocation acts as: the initiating user, or the
// explicit owner-user of a trigger without a user. Never a service principal.
type Subject struct {
	TenantID uuid.UUID   `json:"tenantId"`
	UserID   int64       `json:"userId"`
	Kind     SubjectKind `json:"kind"`
	// Permissions are RBAC permission names (permission.Name(), e.g. "User.Read").
	Permissions []string `json:"permissions"`
	Locale      string   `json:"locale,omitempty"`
	// Credential identifies how the actor authenticated (session, mcp
	// personal credential id, endpoint) for audit; never an identity source.
	Credential string `json:"credential,omitempty"`
}

type SubjectKind string

const (
	SubjectInitiator SubjectKind = "initiator"
	SubjectOwner     SubjectKind = "owner"
)

// Caller is the package version whose code issues a data call. Field keys are
// resolved through that version's schema map, so a workflow pinned to v1 keeps
// working after v2 is activated (or activation is blocked as incompatible).
type Caller struct {
	// Package is the package namespace (the same identifier as
	// ResolvedResource.Package).
	Package string `json:"package"`
	Version string `json:"version"`
}

// Scope is who acts and which code acts.
type Scope struct {
	Subject Subject
	Caller  Caller
}

func (s Subject) Has(permission string) bool {
	if permission == "" {
		return true
	}
	for _, p := range s.Permissions {
		if p == permission {
			return true
		}
	}
	return false
}

type Order struct {
	Field string `json:"field"`
	Desc  bool   `json:"desc,omitempty"`
}

// FindQuery is the typed query of tx.find/q.find. Access predicates are
// applied by the store before Limit/Offset/count.
type FindQuery struct {
	Where   *manifest.Filter `json:"where,omitempty"`
	OrderBy []Order          `json:"orderBy,omitempty"`
	Limit   int              `json:"limit,omitempty"`
	Offset  int              `json:"offset,omitempty"`
}

// DataStore executes typed data calls inside a Go-owned transaction. Records
// are JSON objects in exact wire form with "key" (string) and declared fields.
// Get on an absent record (or overlay row) returns JSON null, not an error.
type DataStore interface {
	Get(ctx context.Context, tx pgx.Tx, sc Scope, entity, key string) (json.RawMessage, error)
	Find(ctx context.Context, tx pgx.Tx, sc Scope, entity string, q FindQuery) (json.RawMessage, error)
	Count(ctx context.Context, tx pgx.Tx, sc Scope, entity string, where *manifest.Filter) (int64, error)
	Insert(ctx context.Context, tx pgx.Tx, sc Scope, entity string, values json.RawMessage) (json.RawMessage, error)
	Update(ctx context.Context, tx pgx.Tx, sc Scope, entity, key string, patch json.RawMessage) (json.RawMessage, error)
	Delete(ctx context.Context, tx pgx.Tx, sc Scope, entity, key string) error
	// PutOverlay upserts the 1:1 overlay row of baseKey.
	PutOverlay(ctx context.Context, tx pgx.Tx, sc Scope, overlay, baseKey string, values json.RawMessage) (json.RawMessage, error)
}

// Producer identifies what produced an intent (event, notice, start) so
// receipts, dedupe and history link back to it.
type Producer struct {
	InvocationID string `json:"invocationId,omitempty"`
	EventID      string `json:"eventId,omitempty"`
	InstanceID   string `json:"instanceId,omitempty"`
	Occurrence   string `json:"occurrence,omitempty"` // "<name>#<n>"
}

// ObjectRef is the wire Ref<Entity>.
type ObjectRef struct {
	Entity string `json:"entity"`
	Key    string `json:"key"`
}

type EventIntent struct {
	Event    string          `json:"event"`
	Subject  *ObjectRef      `json:"subject,omitempty"`
	Payload  json.RawMessage `json:"payload"`
	Producer Producer        `json:"producer"`
}

// Outbox enqueues events in the caller's tx (atomic with data).
type Outbox interface {
	Emit(ctx context.Context, tx pgx.Tx, sc Scope, e EventIntent) (eventID string, err error)
}

type StartIntent struct {
	Workflow string          `json:"workflow"`
	Input    json.RawMessage `json:"input"`
	// Key dedupes the start (tenant-scoped). Required outside subscriptions
	// and workflows, where the host derives it.
	Key         string   `json:"key"`
	Predecessor string   `json:"predecessor,omitempty"`
	Producer    Producer `json:"producer"`
}

// Starter writes a workflow start intent in the caller's tx.
type Starter interface {
	Start(ctx context.Context, tx pgx.Tx, sc Scope, s StartIntent) (instanceID string, err error)
}

type RecipientKind string

const (
	RecipientUser    RecipientKind = "user"
	RecipientRecord  RecipientKind = "record"
	RecipientAddress RecipientKind = "address"
)

type Recipient struct {
	Kind    RecipientKind     `json:"kind"`
	UserID  int64             `json:"userId,omitempty"`
	Record  *ObjectRef        `json:"record,omitempty"`
	Address map[string]string `json:"address,omitempty"` // channel -> address
}

type NoticeIntent struct {
	Notice     string          `json:"notice"`
	Version    string          `json:"version"` // package version pinning templates
	Subject    *ObjectRef      `json:"subject,omitempty"`
	Payload    json.RawMessage `json:"payload"`
	Recipients []Recipient     `json:"recipients"`
	Channels   []string        `json:"channels"`
	Producer   Producer        `json:"producer"`
}

// Notifier enqueues an explicit notice send in the caller's tx; rendering
// and delivery happen after commit in the shared notification service.
type Notifier interface {
	Enqueue(ctx context.Context, tx pgx.Tx, sc Scope, n NoticeIntent) (noticeID string, err error)
}

// WriteChange is what an entity write rule sees inside the writing tx.
type WriteChange struct {
	Entity   string          `json:"entity"`
	Key      string          `json:"key"`
	Op       string          `json:"op"` // insert|update|delete
	Existing json.RawMessage `json:"existing"`
	Proposed json.RawMessage `json:"proposed"`
	Changed  []string        `json:"changed"`
}

// WriteGuard runs entity write rules (package guards with phase "write") in
// the same tx before the row is written. A non-nil error aborts the write.
type WriteGuard interface {
	CheckWrite(ctx context.Context, tx pgx.Tx, sc Scope, change WriteChange) error
}

// ResolvedResource is a resource found in the active catalog together with
// the package version that declares it. Native resources have Native=true and
// an empty Version.
type ResolvedResource struct {
	Resource manifest.Resource
	Package  string
	Version  string
	Native   bool
}

// Catalog resolves resource IDs across native (Go-registered) definitions and
// activated package versions.
type Catalog interface {
	Resolve(ctx context.Context, id string) (ResolvedResource, bool, error)
	Entities(ctx context.Context) ([]ResolvedResource, error)
	// Guards lists active guards of target in a phase across all activated
	// packages (additive, independent of the command binding), in stable ID order.
	Guards(ctx context.Context, target string, phase manifest.GuardPhase) ([]ResolvedGuard, error)
}

type ResolvedGuard struct {
	Guard   manifest.Guard
	Package string
	Version string
}
