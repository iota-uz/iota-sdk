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
	TenantID    uuid.UUID `json:"tenantId"`
	UserID      int64     `json:"userId"`
	Permissions []string  `json:"permissions"`
	Locale      string    `json:"locale,omitempty"`
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
	Get(ctx context.Context, tx pgx.Tx, s Subject, entity, key string) (json.RawMessage, error)
	Find(ctx context.Context, tx pgx.Tx, s Subject, entity string, q FindQuery) (json.RawMessage, error)
	Count(ctx context.Context, tx pgx.Tx, s Subject, entity string, where *manifest.Filter) (int64, error)
	Insert(ctx context.Context, tx pgx.Tx, s Subject, entity string, values json.RawMessage) (json.RawMessage, error)
	Update(ctx context.Context, tx pgx.Tx, s Subject, entity, key string, patch json.RawMessage) (json.RawMessage, error)
	Delete(ctx context.Context, tx pgx.Tx, s Subject, entity, key string) error
	// PutOverlay upserts the 1:1 overlay row of baseKey.
	PutOverlay(ctx context.Context, tx pgx.Tx, s Subject, overlay, baseKey string, values json.RawMessage) (json.RawMessage, error)
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
	CheckWrite(ctx context.Context, tx pgx.Tx, s Subject, change WriteChange) error
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
}
