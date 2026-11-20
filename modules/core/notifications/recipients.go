package notifications

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

// Recipient is a person who can be picked individually on a notification rule.
type Recipient struct {
	ID   uint
	Name string
	Hint string
}

// RecipientQuery selects one page of the recipient directory.
type RecipientQuery struct {
	Search string
	Limit  int
	Offset int
}

// RecipientDirectory defines who can be picked individually on a rule. It never
// returns a person who cannot receive notifications, so an ID that GetByIDs
// omits is unavailable.
type RecipientDirectory interface {
	Search(ctx context.Context, query RecipientQuery) ([]Recipient, error)
	GetByIDs(ctx context.Context, ids []uint) ([]Recipient, error)
}

// Kinds that only appear in SkippedRecipient: audience members that expand
// to users and are never recipients of a delivery themselves.
const (
	RecipientGroup       RecipientKind = "group"
	RecipientRole        RecipientKind = "role"
	RecipientParticipant RecipientKind = "participant"
)

// ErrInvalidAudience marks an audience the caller must fix: an invalid record
// or address, participants without a source, or a source event that is not
// registered or belongs to another tenant.
var ErrInvalidAudience = errors.New("invalid notification audience")

// AudienceUsers loads users so the resolver can drop missing, foreign, blocked
// and inactive ones.
type AudienceUsers interface {
	GetByIDs(context.Context, []uint) ([]user.User, error)
}

// AudienceResolver expands an audience to unique recipients in the tenant of
// the context. It is shared by event rules and audience sends, so both resolve
// groups, roles and participants the same way. It only reads: membership is
// fixed at resolution time, while eligibility, permissions and subject access
// are rechecked per user at delivery.
type AudienceResolver struct {
	catalog  *Catalog
	groups   AudienceRepository
	users    AudienceUsers
	contacts *ContactResolvers
}

// NewAudienceResolver builds a resolver. users may be nil to keep named and
// participant users unchecked until delivery, as event rules do (they filter
// named users through their directory);
// contacts may be nil when no records are resolved.
func NewAudienceResolver(catalog *Catalog, groups AudienceRepository, users AudienceUsers, contacts *ContactResolvers) *AudienceResolver {
	return &AudienceResolver{catalog: catalog, groups: groups, users: users, contacts: contacts}
}

// Resolve expands the audience. Participants are resolved from the definition
// of source; a source must be registered and belong to the context's tenant. Unknown groups, roles and participants, unavailable users and
// records without a contact resolver are reported in Skipped; malformed input
// returns an error wrapping ErrInvalidAudience. User IDs keep the order users,
// groups and roles, participants; repository errors are returned unchanged.
func (r *AudienceResolver) Resolve(ctx context.Context, audience Audience, source *Event) (ResolvedAudience, error) {
	invalid := func(message string) (ResolvedAudience, error) {
		return ResolvedAudience{}, fmt.Errorf("%w: %s", ErrInvalidAudience, message)
	}
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return ResolvedAudience{}, err
	}
	source, err = r.CheckSource(ctx, audience, source)
	if err != nil {
		return ResolvedAudience{}, err
	}
	var definition Definition
	if source != nil {
		definition, _ = r.catalog.For(*source)
	}
	result := ResolvedAudience{}
	skip := func(kind RecipientKind, ref, reason string) {
		result.Skipped = append(result.Skipped, SkippedRecipient{Kind: kind, Ref: ref, Reason: reason})
	}
	seenUsers := map[uint]bool{}
	addUsers := func(ids []uint) {
		for _, id := range ids {
			if id != 0 && !seenUsers[id] {
				seenUsers[id] = true
				result.UserIDs = append(result.UserIDs, id)
			}
		}
	}
	seenAddressees := map[string]bool{}
	var named []uint
	for _, a := range audience.direct() {
		if err := a.Validate(); err != nil {
			return invalid(err.Error())
		}
		if seenAddressees[a.Key()] {
			continue
		}
		seenAddressees[a.Key()] = true
		switch a.Kind {
		case RecipientUser:
			named = append(named, a.UserID)
		case RecipientRecord:
			if _, ok := r.contacts.Lookup(a.Record.Entity); !ok {
				skip(RecipientRecord, a.Record.Entity+"/"+a.Record.Key, SkipNoContactResolver)
				continue
			}
			result.Addressees = append(result.Addressees, a)
		case RecipientAddress:
			result.Addressees = append(result.Addressees, a)
		case RecipientGroup, RecipientRole, RecipientParticipant:
			// Validate rejects these kinds above.
		}
	}
	available, err := r.availableUsers(ctx, tenant, named, skip)
	if err != nil {
		return ResolvedAudience{}, err
	}
	addUsers(available)
	groups, roles, err := r.existing(ctx, audience.Groups, audience.Roles, skip)
	if err != nil {
		return ResolvedAudience{}, err
	}
	if len(groups)+len(roles) > 0 {
		members, err := r.groups.Resolve(ctx, groups, roles)
		if err != nil {
			return ResolvedAudience{}, err
		}
		addUsers(members)
	}
	seenKeys := map[string]bool{}
	for _, key := range audience.Participants {
		if seenKeys[key] {
			continue
		}
		seenKeys[key] = true
		found := false
		for _, recipient := range definition.RecipientKeys {
			if recipient.Key != key {
				continue
			}
			found = true
			ids, err := recipient.Resolve(ctx, *source)
			if err != nil {
				return ResolvedAudience{}, err
			}
			// A participant callback may name anyone: drop missing, foreign
			// and unavailable users like named ones.
			ids, err = r.availableUsers(ctx, tenant, slices.DeleteFunc(slices.Clone(ids), func(id uint) bool { return id == 0 || seenUsers[id] }), skip)
			if err != nil {
				return ResolvedAudience{}, err
			}
			addUsers(ids)
			break
		}
		if !found {
			skip(RecipientParticipant, key, SkipUnknownParticipant)
		}
	}
	return result, nil
}

// CheckSource validates the source event of an audience without resolving
// anyone: participants need a source, and a source must be registered and
// belong to the context's tenant. It returns the normalized source.
func (r *AudienceResolver) CheckSource(ctx context.Context, audience Audience, source *Event) (*Event, error) {
	if len(audience.Participants) > 0 && source == nil {
		return nil, fmt.Errorf("%w: participants need a source event", ErrInvalidAudience)
	}
	if source == nil {
		return nil, nil //nolint:nilnil // No source is valid for an audience without participants.
	}
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, err
	}
	normalized, err := r.catalog.Normalize(*source)
	if err != nil {
		return nil, fmt.Errorf("%w: unknown source event", ErrInvalidAudience)
	}
	if normalized.TenantID != tenant {
		return nil, fmt.Errorf("%w: source event belongs to another tenant", ErrInvalidAudience)
	}
	return &normalized, nil
}

// availableUsers keeps the users that exist in the tenant and can receive
// notifications, in request order.
func (r *AudienceResolver) availableUsers(ctx context.Context, tenant uuid.UUID, ids []uint, skip func(RecipientKind, string, string)) ([]uint, error) {
	if r.users == nil || len(ids) == 0 {
		return ids, nil
	}
	users, err := r.users.GetByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	found := make(map[uint]user.User, len(users))
	for _, u := range users {
		found[u.ID()] = u
	}
	out := make([]uint, 0, len(ids))
	for _, id := range ids {
		u, ok := found[id]
		switch {
		case !ok || u.TenantID() != tenant:
			skip(RecipientUser, strconv.FormatUint(uint64(id), 10), SkipNotFound)
		case u.IsBlocked() || u.Type() != user.TypeUser || u.Status() != user.StatusActive:
			skip(RecipientUser, strconv.FormatUint(uint64(id), 10), SkipInactive)
		default:
			out = append(out, id)
		}
	}
	return out, nil
}

// existing keeps the groups and roles of the tenant, without duplicates.
func (r *AudienceResolver) existing(ctx context.Context, groups []uuid.UUID, roles []uint, skip func(RecipientKind, string, string)) ([]uuid.UUID, []uint, error) {
	var keptGroups []uuid.UUID
	if len(groups) > 0 {
		options, err := r.groups.Groups(ctx)
		if err != nil {
			return nil, nil, err
		}
		known := make(map[uuid.UUID]bool, len(options))
		for _, g := range options {
			known[g.ID] = true
		}
		seen := map[uuid.UUID]bool{}
		for _, id := range groups {
			if seen[id] {
				continue
			}
			seen[id] = true
			if !known[id] {
				skip(RecipientGroup, id.String(), SkipNotFound)
				continue
			}
			keptGroups = append(keptGroups, id)
		}
	}
	var keptRoles []uint
	if len(roles) > 0 {
		options, err := r.groups.Roles(ctx)
		if err != nil {
			return nil, nil, err
		}
		known := make(map[uint]bool, len(options))
		for _, role := range options {
			known[role.ID] = true
		}
		seen := map[uint]bool{}
		for _, id := range roles {
			if seen[id] {
				continue
			}
			seen[id] = true
			if !known[id] {
				skip(RecipientRole, strconv.FormatUint(uint64(id), 10), SkipNotFound)
				continue
			}
			keptRoles = append(keptRoles, id)
		}
	}
	return keptGroups, keptRoles, nil
}
