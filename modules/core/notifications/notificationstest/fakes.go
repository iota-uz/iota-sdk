// Package notificationstest provides synthetic notification providers for
// tests: channel adapters that record messages and can fail on demand, a
// ContactResolver backed by a map, and recording policies. Nothing here talks
// to a network and nothing is registered in production compositions.
package notificationstest

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/pkg/constants"
)

// Adapter is a fake email, SMS or push provider. Every call is recorded; the
// outcome of a call is chosen per target address, so one target can fail while
// its sibling succeeds.
type Adapter struct {
	channel notifications.Channel
	mu      sync.Mutex
	sent    []notifications.Message
	calls   []notifications.Message
	byAddr  map[string]error
	all     error
	inTx    bool
}

// NewAdapter returns a fake provider for a channel.
func NewAdapter(channel notifications.Channel) *Adapter {
	return &Adapter{channel: channel, byAddr: map[string]error{}}
}

// NewEmail returns a fake email provider.
func NewEmail() *Adapter { return NewAdapter(notifications.ChannelEmail) }

// NewSMS returns a fake SMS provider.
func NewSMS() *Adapter { return NewAdapter(notifications.ChannelSMS) }

// NewPush returns a fake push provider.
func NewPush() *Adapter { return NewAdapter(notifications.ChannelPush) }

// Channel implements notifications.ChannelAdapter.
func (a *Adapter) Channel() notifications.Channel { return a.channel }

// Send records the call and returns the programmed outcome for the address.
func (a *Adapter) Send(ctx context.Context, m notifications.Message) (notifications.Receipt, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if ctx.Value(constants.TxKey) != nil {
		a.inTx = true
	}
	a.calls = append(a.calls, m)
	if err, ok := a.byAddr[m.Address]; ok && err != nil {
		return notifications.Receipt{}, err
	}
	if a.all != nil {
		return notifications.Receipt{}, a.all
	}
	a.sent = append(a.sent, m)
	return notifications.Receipt{ProviderMessageID: fmt.Sprintf("%s-%s-%d", a.channel, m.DeliveryID, m.Attempt)}, nil
}

// FailAddress makes sends to one address fail with err; nil clears it. Wrap
// notifications.ErrPermanent or notifications.ErrAmbiguous to choose the kind.
func (a *Adapter) FailAddress(address string, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err == nil {
		delete(a.byAddr, address)
		return
	}
	a.byAddr[address] = err
}

// FailAll makes every send fail with err; nil clears it.
func (a *Adapter) FailAll(err error) { a.mu.Lock(); a.all = err; a.mu.Unlock() }

// Sent returns the messages the provider accepted.
func (a *Adapter) Sent() []notifications.Message {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]notifications.Message(nil), a.sent...)
}

// Calls returns every send attempt, including failed ones.
func (a *Adapter) Calls() []notifications.Message {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]notifications.Message(nil), a.calls...)
}

// SentTo returns the accepted messages addressed to one target.
func (a *Adapter) SentTo(address string) []notifications.Message {
	var out []notifications.Message
	for _, m := range a.Sent() {
		if m.Address == address {
			out = append(out, m)
		}
	}
	return out
}

// CalledInTransaction reports whether any send ran inside a database transaction.
func (a *Adapter) CalledInTransaction() bool { a.mu.Lock(); defer a.mu.Unlock(); return a.inTx }

// Contacts is a ContactResolver backed by a map of records.
type Contacts struct {
	mu       sync.Mutex
	records  map[notifications.ObjectRef]notifications.Contact
	calls    int
	inTx     bool
	failWith error
}

// NewContacts returns an empty fake contact book.
func NewContacts() *Contacts {
	return &Contacts{records: map[notifications.ObjectRef]notifications.Contact{}}
}

// Add defines the contact of a record.
func (c *Contacts) Add(ref notifications.ObjectRef, contact notifications.Contact) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.records[ref] = contact
}

// FailWith makes every resolution fail with err; nil clears it.
func (c *Contacts) FailWith(err error) { c.mu.Lock(); c.failWith = err; c.mu.Unlock() }

// Calls is the number of resolutions.
func (c *Contacts) Calls() int { c.mu.Lock(); defer c.mu.Unlock(); return c.calls }

// CalledInTransaction reports whether a resolution ran inside a database transaction.
func (c *Contacts) CalledInTransaction() bool { c.mu.Lock(); defer c.mu.Unlock(); return c.inTx }

// ResolveContact implements notifications.ContactResolver.
func (c *Contacts) ResolveContact(ctx context.Context, _ uuid.UUID, ref notifications.ObjectRef) (notifications.Contact, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	if ctx.Value(constants.TxKey) != nil {
		c.inTx = true
	}
	if c.failWith != nil {
		return notifications.Contact{}, c.failWith
	}
	contact, ok := c.records[ref]
	if !ok {
		return notifications.Contact{}, notifications.ErrContactNotFound
	}
	return contact, nil
}

// Access is a SubjectAccess that denies chosen users.
type Access struct {
	mu     sync.Mutex
	denied map[uint]bool
	checks []notifications.ObjectRef
}

// NewAccess returns an Access that allows everyone.
func NewAccess() *Access { return &Access{denied: map[uint]bool{}} }

// Deny blocks one user from every subject.
func (a *Access) Deny(userID uint) { a.mu.Lock(); a.denied[userID] = true; a.mu.Unlock() }

// Checks lists the subjects that were checked.
func (a *Access) Checks() []notifications.ObjectRef {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]notifications.ObjectRef(nil), a.checks...)
}

// CanAccess implements notifications.SubjectAccess.
func (a *Access) CanAccess(_ context.Context, _ uuid.UUID, u user.User, subject notifications.ObjectRef) (bool, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.checks = append(a.checks, subject)
	return !a.denied[u.ID()], nil
}

// Policy is a DeliveryPolicy that denies chosen addresses and records requests.
type Policy struct {
	mu       sync.Mutex
	denied   map[string]string
	requests []notifications.PolicyRequest
}

// NewPolicy returns a Policy that allows everything.
func NewPolicy() *Policy { return &Policy{denied: map[string]string{}} }

// Deny blocks an address with a machine-readable reason.
func (p *Policy) Deny(address, reason string) { p.mu.Lock(); p.denied[address] = reason; p.mu.Unlock() }

// Requests lists the evaluated deliveries.
func (p *Policy) Requests() []notifications.PolicyRequest {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]notifications.PolicyRequest(nil), p.requests...)
}

// Evaluate implements notifications.DeliveryPolicy.
func (p *Policy) Evaluate(_ context.Context, r notifications.PolicyRequest) (notifications.PolicyDecision, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.requests = append(p.requests, r)
	if reason, ok := p.denied[r.Target.Address]; ok {
		return notifications.PolicyDecision{Allow: false, Reason: reason}, nil
	}
	return notifications.PolicyDecision{Allow: true}, nil
}
