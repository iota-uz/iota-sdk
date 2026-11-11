package notifications

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
)

// RecipientKind says who a notice is addressed to. A recipient is a different
// identity from the notice subject and from the channel target (address or
// device) it is eventually reached on.
type RecipientKind string

const (
	// RecipientUser is an ERP user: inbox plus channels through the user's
	// addresses and registered push devices.
	RecipientUser RecipientKind = "user"
	// RecipientRecord is a typed business object (for example a customer)
	// expanded to contacts by the ContactResolver registered for its entity.
	// It needs no ERP account.
	RecipientRecord RecipientKind = "record"
	// RecipientAddress is a direct typed channel address (an email, a phone
	// number or a push token). It needs neither an account nor a record.
	RecipientAddress RecipientKind = "address"
)

// Valid reports whether k is a known recipient kind.
func (k RecipientKind) Valid() bool {
	switch k {
	case RecipientUser, RecipientRecord, RecipientAddress:
		return true
	}
	return false
}

// ObjectRef is a typed reference to a business object: an entity ID and the
// record key within it.
type ObjectRef struct {
	Entity string `json:"entity"`
	Key    string `json:"key"`
}

const (
	maxRefPartLength = 200
	maxAddressLength = 320
)

// IsZero reports whether the reference is unset.
func (r ObjectRef) IsZero() bool { return r.Entity == "" && r.Key == "" }

// Validate rejects half-set, oversized or control-character references.
func (r ObjectRef) Validate() error {
	for _, part := range []string{r.Entity, r.Key} {
		if strings.TrimSpace(part) == "" || utf8.RuneCountInString(part) > maxRefPartLength || strings.ContainsFunc(part, isControl) {
			return errors.New("invalid object reference")
		}
	}
	return nil
}

func isControl(r rune) bool { return r < 0x20 || r == 0x7f }

// IsZero reports whether the event has no subject.
func (s SubjectReference) IsZero() bool { return s.Type == "" && s.ID == "" }

// Ref converts the envelope subject into a typed object reference.
func (s SubjectReference) Ref() ObjectRef { return ObjectRef{Entity: s.Type, Key: s.ID} }

// Subject converts the reference into an envelope subject.
func (r ObjectRef) Subject() SubjectReference { return SubjectReference{Type: r.Entity, ID: r.Key} }

// Producer identifies what produced a notice so receipts, dedupe and history
// link back to it: a runtime invocation, a domain event, or a workflow
// instance with the step occurrence ("<name>#<n>").
type Producer struct {
	InvocationID string `json:"invocation_id,omitempty"`
	EventID      string `json:"event_id,omitempty"`
	InstanceID   string `json:"instance_id,omitempty"`
	Occurrence   string `json:"occurrence,omitempty"`
}

// IsZero reports whether no producer identity is set.
func (p Producer) IsZero() bool { return p == Producer{} }

// Identity is the stable producer identity used to derive a dedupe key: the
// workflow occurrence when present, else the invocation, else the event.
func (p Producer) Identity() string {
	switch {
	case p.InstanceID != "":
		return "instance:" + p.InstanceID + "#" + p.Occurrence
	case p.InvocationID != "":
		return "invocation:" + p.InvocationID
	case p.EventID != "":
		return "event:" + p.EventID
	}
	return ""
}

// Validate bounds every producer field.
func (p Producer) Validate() error {
	for _, v := range []string{p.InvocationID, p.EventID, p.InstanceID, p.Occurrence} {
		if utf8.RuneCountInString(v) > maxRefPartLength || strings.ContainsFunc(v, isControl) {
			return errors.New("invalid notification producer")
		}
	}
	return nil
}

// Addressee is one recipient of a notice.
type Addressee struct {
	Kind RecipientKind `json:"kind"`
	// UserID is set for user recipients.
	UserID uint `json:"user_id,omitempty"`
	// Record is set for record recipients.
	Record *ObjectRef `json:"record,omitempty"`
	// Addresses maps a channel to a direct address for address recipients.
	Addresses map[Channel]string `json:"addresses,omitempty"`
	// Language optionally overrides the content language of address recipients.
	Language string `json:"language,omitempty"`
}

// UserAddressee addresses an ERP user.
func UserAddressee(id uint) Addressee { return Addressee{Kind: RecipientUser, UserID: id} }

// RecordAddressee addresses a business record through its ContactResolver.
func RecordAddressee(entity, key string) Addressee {
	return Addressee{Kind: RecipientRecord, Record: &ObjectRef{Entity: entity, Key: key}}
}

// AddressAddressee addresses direct channel addresses.
func AddressAddressee(addresses map[Channel]string) Addressee {
	return Addressee{Kind: RecipientAddress, Addresses: addresses}
}

// Validate checks that exactly the fields of the recipient's kind are set.
func (a Addressee) Validate() error {
	switch a.Kind {
	case RecipientUser:
		if a.UserID == 0 || a.Record != nil || len(a.Addresses) > 0 {
			return errors.New("invalid user recipient")
		}
	case RecipientRecord:
		if a.UserID != 0 || a.Record == nil || len(a.Addresses) > 0 {
			return errors.New("invalid record recipient")
		}
		if err := a.Record.Validate(); err != nil {
			return err
		}
	case RecipientAddress:
		if a.UserID != 0 || a.Record != nil || len(a.Addresses) == 0 {
			return errors.New("invalid address recipient")
		}
		for channel, address := range a.Addresses {
			if !channel.External() || strings.TrimSpace(address) == "" || utf8.RuneCountInString(address) > maxAddressLength || strings.ContainsFunc(address, isControl) {
				return errors.New("invalid recipient address")
			}
		}
	default:
		return errors.New("invalid recipient kind")
	}
	return nil
}

// Key is the recipient identity within one notice.
func (a Addressee) Key() string {
	switch a.Kind {
	case RecipientUser:
		return fmt.Sprintf("user:%d", a.UserID)
	case RecipientRecord:
		if a.Record != nil {
			return "record:" + a.Record.Entity + "/" + a.Record.Key
		}
	case RecipientAddress:
		return "address:" + a.fingerprint()[:16]
	}
	return string(a.Kind)
}

func (a Addressee) fingerprint() string {
	channels := make([]string, 0, len(a.Addresses))
	for channel, address := range a.Addresses {
		channels = append(channels, string(channel)+"="+address)
	}
	sort.Strings(channels)
	ref := ""
	if a.Record != nil {
		ref = a.Record.Entity + "/" + a.Record.Key
	}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%s|%d|%s|%s", a.Kind, a.UserID, ref, strings.Join(channels, ",")))))
}

// DeriveDedupeKey builds the stable identity of an explicit send from its
// producer, so a replayed producer step cannot enqueue the same notice twice
// while a different recipient set, subject or payload from the same producer
// still produces its own notice. It returns "" when the producer is unknown.
func DeriveDedupeKey(producer Producer, event Event, recipients []Addressee, channels []Channel) string {
	identity := producer.Identity()
	if identity == "" {
		return ""
	}
	keys := make([]string, 0, len(recipients))
	for _, r := range recipients {
		keys = append(keys, r.fingerprint())
	}
	sort.Strings(keys)
	names := make([]string, 0, len(channels))
	for _, c := range channels {
		names = append(names, string(c))
	}
	sort.Strings(names)
	// json.Marshal sorts map keys, so equal payloads hash equally.
	data := ""
	if len(event.Data) > 0 {
		if encoded, err := json.Marshal(event.Data); err == nil {
			data = string(encoded)
		}
	}
	sum := sha256.Sum256([]byte(strings.Join([]string{event.Key, event.SnapshotRef, event.Subject.Type, event.Subject.ID, strings.Join(keys, ","), strings.Join(names, ","), data, string(event.Payload)}, "|")))
	key := fmt.Sprintf("producer:%s:%x", identity, sum[:12])
	if len(key) > 190 {
		key = fmt.Sprintf("producer:sha256:%x", sha256.Sum256([]byte(key)))
	}
	return key
}

// Target is one concrete way to reach a recipient on a channel: an email
// address, a phone number or a push device. DeviceID identifies the device
// (not the token) so one person can have several; it is empty otherwise.
type Target struct {
	Channel  Channel
	Address  string
	DeviceID string
}

// Key identifies the target within a recipient and channel.
func (t Target) Key() string {
	if t.DeviceID != "" {
		return "device:" + t.DeviceID
	}
	return "address:" + t.Address
}

// Contact is what a ContactResolver knows about a business record: the
// language to write in and every target the record can be reached on.
type Contact struct {
	Language string
	Targets  []Target
}

// ErrContactNotFound is returned by a ContactResolver when the record does not
// exist or has no contact data. The delivery is recorded as no_address.
var ErrContactNotFound = errors.New("notification contact not found")

// ContactResolver expands a business record into channel targets. It runs in
// the notification worker after the producer's transaction committed, so it may
// read the database or call other services. It is never given a transaction.
type ContactResolver interface {
	ResolveContact(ctx context.Context, tenantID uuid.UUID, ref ObjectRef) (Contact, error)
}

// ContactResolverFunc adapts a function to ContactResolver.
type ContactResolverFunc func(context.Context, uuid.UUID, ObjectRef) (Contact, error)

// ResolveContact implements ContactResolver.
func (f ContactResolverFunc) ResolveContact(ctx context.Context, tenantID uuid.UUID, ref ObjectRef) (Contact, error) {
	return f(ctx, tenantID, ref)
}

// ContactResolvers registers one ContactResolver per entity ID. Registration is
// safe at runtime, so an extension can publish a resolver for its own entity.
type ContactResolvers struct {
	mu       sync.RWMutex
	resolver map[string]ContactResolver
}

// NewContactResolvers returns an empty registry.
func NewContactResolvers() *ContactResolvers {
	return &ContactResolvers{resolver: map[string]ContactResolver{}}
}

// Register adds the resolver of an entity; a second registration fails instead
// of silently replacing the first.
func (r *ContactResolvers) Register(entity string, resolver ContactResolver) error {
	if strings.TrimSpace(entity) == "" || resolver == nil {
		return errors.New("invalid contact resolver")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.resolver[entity]; exists {
		return fmt.Errorf("contact resolver already registered: %s", entity)
	}
	r.resolver[entity] = resolver
	return nil
}

// Lookup returns the resolver registered for an entity.
func (r *ContactResolvers) Lookup(entity string) (ContactResolver, bool) {
	if r == nil {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	resolver, ok := r.resolver[entity]
	return resolver, ok
}

// Entities lists the entities that have a resolver.
func (r *ContactResolvers) Entities() []string {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]string, 0, len(r.resolver))
	for entity := range r.resolver {
		result = append(result, entity)
	}
	slices.Sort(result)
	return result
}

// SubjectAccess answers whether a user may see the notice subject. It is
// checked for user recipients only; external contacts are governed by
// DeliveryPolicy. A denial is recorded as a visible skipped delivery.
type SubjectAccess interface {
	CanAccess(ctx context.Context, tenantID uuid.UUID, recipient user.User, subject ObjectRef) (bool, error)
}

// SubjectAccessFunc adapts a function to SubjectAccess.
type SubjectAccessFunc func(context.Context, uuid.UUID, user.User, ObjectRef) (bool, error)

// CanAccess implements SubjectAccess.
func (f SubjectAccessFunc) CanAccess(ctx context.Context, tenantID uuid.UUID, recipient user.User, subject ObjectRef) (bool, error) {
	return f(ctx, tenantID, recipient, subject)
}

// PolicyRequest describes one external delivery about to be created.
type PolicyRequest struct {
	TenantID uuid.UUID
	// EventKey and EventVersion are the notice key ("acme.renewal.v1") and
	// its version (1).
	EventKey     string
	EventVersion int
	SnapshotRef  string
	Subject      ObjectRef
	Recipient    Addressee
	Target       Target
}

// PolicyDecision is the verdict of a DeliveryPolicy. Reason is a short machine
// code ([a-z0-9_.-]) stored with the skipped delivery; it must not carry
// personal data.
type PolicyDecision struct {
	Allow  bool
	Reason string
}

// DeliveryPolicy is the business policy (consent, opt-out, quiet contact
// rules) applied to record and address recipients. The SDK defines no consent
// schema: the domain package that owns the data implements this interface.
// The default allows everything.
type DeliveryPolicy interface {
	Evaluate(ctx context.Context, request PolicyRequest) (PolicyDecision, error)
}

// DeliveryPolicyFunc adapts a function to DeliveryPolicy.
type DeliveryPolicyFunc func(context.Context, PolicyRequest) (PolicyDecision, error)

// Evaluate implements DeliveryPolicy.
func (f DeliveryPolicyFunc) Evaluate(ctx context.Context, request PolicyRequest) (PolicyDecision, error) {
	return f(ctx, request)
}

// DeliveryPolicies composes the business delivery policies of a deployment: the
// policy configured in Go (ModuleOptions.NotificationDeliveryPolicy) and the
// named policies that runtime sources (for example extension packages)
// register. It is itself a DeliveryPolicy and is what the notification
// platform consults.
//
// Composition is deterministic: the Go-configured policy is evaluated first,
// then the named policies in lexical order of their names. The first denial
// wins and its reason is the one stored; later policies are not consulted. A
// policy that fails (returns an error) makes the whole evaluation fail, so the
// delivery is retried instead of being sent without the policy's verdict. With
// no policy at all, everything is allowed.
type DeliveryPolicies struct {
	mu    sync.RWMutex
	base  DeliveryPolicy
	named map[string]DeliveryPolicy
}

// NewDeliveryPolicies returns a composition whose first member is base (nil
// for none).
func NewDeliveryPolicies(base DeliveryPolicy) *DeliveryPolicies {
	return &DeliveryPolicies{base: base, named: map[string]DeliveryPolicy{}}
}

// Set installs a named policy, replacing the one registered under the same
// name (a restarted runtime re-installs its own); a nil policy removes it.
func (p *DeliveryPolicies) Set(name string, policy DeliveryPolicy) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("invalid delivery policy")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if policy == nil {
		delete(p.named, name)
		return nil
	}
	p.named[name] = policy
	return nil
}

// Names lists the registered policy names in evaluation order.
func (p *DeliveryPolicies) Names() []string {
	if p == nil {
		return nil
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	names := make([]string, 0, len(p.named))
	for name := range p.named {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// Evaluate implements DeliveryPolicy.
func (p *DeliveryPolicies) Evaluate(ctx context.Context, request PolicyRequest) (PolicyDecision, error) {
	if p == nil {
		return PolicyDecision{Allow: true}, nil
	}
	p.mu.RLock()
	members := make([]DeliveryPolicy, 0, len(p.named)+1)
	if p.base != nil {
		members = append(members, p.base)
	}
	names := make([]string, 0, len(p.named))
	for name := range p.named {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		members = append(members, p.named[name])
	}
	p.mu.RUnlock()
	for _, member := range members {
		decision, err := member.Evaluate(ctx, request)
		if err != nil {
			return PolicyDecision{}, err
		}
		if !decision.Allow {
			return decision, nil
		}
	}
	return PolicyDecision{Allow: true}, nil
}

// AllowAllPolicy is the default DeliveryPolicy.
type AllowAllPolicy struct{}

// Evaluate allows every delivery.
func (AllowAllPolicy) Evaluate(context.Context, PolicyRequest) (PolicyDecision, error) {
	return PolicyDecision{Allow: true}, nil
}
