// Package businessevent defines the persisted record of a business action and the actor who performed it.
package businessevent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/fielddiff"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

var (
	ErrNotFound = errors.New("business event not found")
	ErrConflict = serrors.NewConflict("business event already recorded with different content").WithReason("event_conflict")
)

type Channel string

const (
	ChannelUI     Channel = "ui"
	ChannelMCP    Channel = "mcp"
	ChannelChat   Channel = "chat"
	ChannelImport Channel = "import"
	ChannelJob    Channel = "job"
	ChannelAPI    Channel = "api"
)

func (c Channel) Valid() bool {
	switch c {
	case ChannelUI, ChannelMCP, ChannelChat, ChannelImport, ChannelJob, ChannelAPI:
		return true
	}
	return false
}

type Outcome string

const (
	OutcomeSucceeded Outcome = "succeeded"
	OutcomeFailed    Outcome = "failed"
	OutcomeDenied    Outcome = "denied"
)

func (o Outcome) Valid() bool {
	switch o {
	case OutcomeSucceeded, OutcomeFailed, OutcomeDenied:
		return true
	}
	return false
}

type Category string

const (
	CategoryBusiness Category = "business"
	CategorySecurity Category = "security"
	// CategoryLegacy marks records migrated from the request-level action log.
	CategoryLegacy Category = "legacy"
)

func (c Category) Valid() bool {
	switch c {
	case CategoryBusiness, CategorySecurity, CategoryLegacy:
		return true
	}
	return false
}

type Subject struct {
	Resource       string `json:"resource"`
	ID             string `json:"id"`
	RevisionBefore string `json:"revision_before,omitempty"`
	RevisionAfter  string `json:"revision_after,omitempty"`
}

type Change struct {
	Resource string           `json:"resource"`
	ID       string           `json:"id"`
	Fields   []fielddiff.Diff `json:"fields"`
}

// Legacy keeps a migrated record with exactly the detail it had; nothing missing is reconstructed.
type Legacy struct {
	Source     string          `json:"source"`
	SourceID   string          `json:"source_id"`
	UserID     string          `json:"user_id,omitempty"`
	Incomplete bool            `json:"incomplete"`
	Payload    json.RawMessage `json:"payload"`
}

// Event is a business action as stored. Legacy events carry Legacy and may lack
// occurred_at, actor, channel, operation and outcome.
type Event struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	Type          string
	SchemaVersion int
	Category      Category
	OccurredAt    time.Time
	RecordedAt    time.Time
	Actor         *ActorSnapshot
	Initiator     *ActorSnapshot
	Channel       Channel
	OperationID   string
	RequestID     string
	RunID         string
	ClientID      string
	Outcome       Outcome
	Subjects      []Subject
	Changes       []Change
	Legacy        *Legacy
}

// Fingerprint identifies what happened, leaving out when and through which request it was delivered,
// so a redelivery of the same event matches the stored one.
func (e Event) Fingerprint() (string, error) {
	data, err := json.Marshal(struct {
		Type          string         `json:"event_type"`
		SchemaVersion int            `json:"schema_version"`
		Category      Category       `json:"category"`
		Actor         *ActorSnapshot `json:"actor_snapshot"`
		Initiator     *ActorSnapshot `json:"initiator_snapshot"`
		Channel       Channel        `json:"channel"`
		OperationID   string         `json:"operation_id"`
		Outcome       Outcome        `json:"outcome"`
		Subjects      []Subject      `json:"subjects"`
		Changes       []Change       `json:"changes"`
	}{e.Type, e.SchemaVersion, e.Category, e.Actor, e.Initiator, e.Channel, e.OperationID, e.Outcome, e.Subjects, e.Changes})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// Position is a place in the timeline, ordered by time and then event ID, both descending.
type Position struct {
	Time time.Time
	ID   uuid.UUID
}

// FindParams narrows the timeline; zero values do not filter. An event is visible when it
// concerns at least one of Readable resources or none at all.
type FindParams struct {
	Resource   string
	SubjectID  string
	ActorID    string
	EventTypes []string
	Channels   []Channel
	// WithoutChannel includes legacy events, which have no channel.
	WithoutChannel bool
	Outcomes       []Outcome
	From           time.Time
	To             time.Time
	Readable       []string
	After          *Position
	Limit          int
}

// Position places the event in the timeline. Legacy events without occurred_at use the time they were recorded.
func (e Event) Position() Position {
	if e.OccurredAt.IsZero() {
		return Position{Time: e.RecordedAt, ID: e.ID}
	}
	return Position{Time: e.OccurredAt, ID: e.ID}
}

type Repository interface {
	// Append stores the event in the transaction carried by ctx. Storing an event that is
	// already present with the same fingerprint is a no-op; a different one is ErrConflict.
	Append(ctx context.Context, e Event) error
	Get(ctx context.Context, id uuid.UUID) (Event, error)
	ListByOperation(ctx context.Context, operationID string) ([]Event, error)
	// Find returns the tenant's events newest first.
	Find(ctx context.Context, params FindParams) ([]Event, error)
}
