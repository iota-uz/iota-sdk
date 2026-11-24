package notifications

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/notification"
	"github.com/jackc/pgx/v5"
)

type GroupOption struct {
	ID   uuid.UUID
	Name string
}
type RoleOption struct {
	ID   uint
	Name string
}
type AudienceRepository interface {
	Groups(context.Context) ([]GroupOption, error)
	Roles(context.Context) ([]RoleOption, error)
	Resolve(context.Context, []uuid.UUID, []uint) ([]uint, error)
}

// Audience names who receives a notice. Users, groups, roles and participants
// expand to ERP users; records and addresses are reached on external channels.
type Audience struct {
	Users  []uint
	Groups []uuid.UUID
	Roles  []uint
	// Participants are RecipientDefinition keys of the Source event's
	// definition, for example "created_user".
	Participants []string
	// Records are business objects reached through their entity's
	// ContactResolver.
	Records []ObjectRef
	// Addresses are direct channel addresses, one map per recipient.
	Addresses []map[Channel]string
	// Addressees are ready user, record or address recipients, for example an
	// address recipient with a language override. Groups, roles and
	// participants expand only through the fields above; an Addressee of
	// those kinds is Invalid.
	Addressees []Addressee
}

// IsZero reports whether the audience names nobody.
func (a Audience) IsZero() bool {
	return len(a.Users)+len(a.Groups)+len(a.Roles)+len(a.Participants)+len(a.Records)+len(a.Addresses)+len(a.Addressees) == 0
}

// direct lists the audience members that need no resolution, in the order
// users, ready addressees, records, addresses.
func (a Audience) direct() []Addressee {
	out := make([]Addressee, 0, len(a.Users)+len(a.Addressees)+len(a.Records)+len(a.Addresses))
	for _, id := range a.Users {
		if id != 0 {
			out = append(out, UserAddressee(id))
		}
	}
	out = append(out, a.Addressees...)
	for _, r := range a.Records {
		ref := r
		out = append(out, Addressee{Kind: RecipientRecord, Record: &ref})
	}
	for _, addresses := range a.Addresses {
		out = append(out, AddressAddressee(addresses))
	}
	return out
}

// AudienceRequest is one send to an audience by an external sender: a graph
// notify node, a workflow or a module.
type AudienceRequest struct {
	// Event is the notice: Key is a registered definition and Subject is
	// checked for access per user recipient at delivery.
	Event    Event
	Audience Audience
	// Source is the domain event whose definition resolves Participants. It is
	// required when Participants is set.
	Source *Event
	// Channels defaults to in-app.
	Channels []Channel
	// Level overrides the definition's default level when set.
	Level notification.Level
	// Producer identifies the run, node or occurrence behind the send and
	// derives a DedupeKey when Event has none.
	Producer Producer
}

// Skip reasons of recipients dropped while an audience is resolved.
const (
	SkipInactive           = "inactive"
	SkipNotFound           = "not_found"
	SkipNoContactResolver  = "no_contact_resolver"
	SkipUnknownParticipant = "unknown_participant"
)

// SkippedRecipient is a recipient dropped while an audience was resolved.
type SkippedRecipient struct {
	Kind RecipientKind
	// Ref identifies the dropped recipient: a user, group or role ID, a
	// participant key or a record reference.
	Ref    string
	Reason string
}

// ResolvedAudience is an audience expanded to its unique recipients.
type ResolvedAudience struct {
	// UserIDs are the users after groups, roles and participants are expanded,
	// without duplicates.
	UserIDs []uint
	// Addressees are the unique record and address recipients.
	Addressees []Addressee
	Skipped    []SkippedRecipient
}

// Total is the number of resolved recipients.
func (r ResolvedAudience) Total() int { return len(r.UserIDs) + len(r.Addressees) }

// AudienceReceipt reports what a send accepted.
type AudienceReceipt struct {
	// NoticeID identifies the notice. A repeated send with the same dedupe
	// identity returns the notice stored by the first.
	NoticeID uuid.UUID
	// Recipients is the number of recipients of the durable job.
	Recipients int
	// Skipped are the audience members the resolution that queued this send dropped (inactive users,
	// unknown groups, ...). It is empty for a replay, which returns the first receipt without resolving.
	Skipped []SkippedRecipient
}

// AudienceSender sends a notice to an audience. External senders depend on
// this interface.
type AudienceSender interface {
	// SendTx queues the send in the caller's transaction: the notice, its
	// recipients and the dispatch job commit or roll back with it.
	SendTx(ctx context.Context, tx pgx.Tx, req AudienceRequest) (AudienceReceipt, error)
	// Resolve expands the audience without writing anything, for dry runs
	// and previews.
	Resolve(ctx context.Context, req AudienceRequest) (ResolvedAudience, error)
}
