// Package events implements the extension runtime's event side: the
// transactional outbox (spi.Outbox and the Go API for native modules), the
// native event catalog, the after-commit dispatcher that delivers events to
// active subscriptions with per-row receipts, and cron schedules.
package events

import (
	"fmt"
	"regexp"
	"sort"
	"sync"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

var versionedEventID = regexp.MustCompile(`^[a-z][a-z0-9]*(\.[a-z][a-z0-9]*)*(\.[a-z][a-zA-Z0-9_]*)+\.v[0-9]+$`)

// NativeEvent declares a business event published by Go modules with a
// versioned payload type, in the same manifest form as package events, so it
// appears in the one catalog that kits, MCP and subscription authoring read.
type NativeEvent struct {
	// ID is the versioned event ID, e.g. "core.user.created.v1".
	ID string
	// Subject is the entity resource ID the event is about, if any.
	Subject string
	// Payload is the exact wire type of the payload; required.
	Payload manifest.Type
	// Description documents when the event is published.
	Description string
}

// Resource renders the declaration as a catalog resource.
func (n NativeEvent) Resource() manifest.Resource {
	payload := n.Payload
	r := manifest.Resource{
		ID:          n.ID,
		Kind:        manifest.ResEvent,
		Description: n.Description,
		Subject:     n.Subject,
		Payload:     &payload,
	}
	r.ContractHash = manifest.ContractHashOf(r)
	return r
}

// NativeCatalog is the registry of native (Go-published) events.
type NativeCatalog struct {
	mu     sync.RWMutex
	events map[string]manifest.Resource
}

func NewNativeCatalog() *NativeCatalog {
	return &NativeCatalog{events: map[string]manifest.Resource{}}
}

// Default is the process-wide catalog native modules register into at init
// time and Publish validates against.
var Default = NewNativeCatalog()

// Register adds a native event. The ID must end in a version (".v1"); the same
// ID cannot be registered with a different contract.
func (c *NativeCatalog) Register(ev NativeEvent) error {
	const op = serrors.Op("extruntime.events.NativeCatalog.Register")
	if !versionedEventID.MatchString(ev.ID) {
		return serrors.Wrap(op, fmt.Errorf("native event %q must be a dotted ID ending in a version such as .v1", ev.ID))
	}
	res := ev.Resource()
	c.mu.Lock()
	defer c.mu.Unlock()
	if prev, ok := c.events[ev.ID]; ok {
		if prev.ContractHash != res.ContractHash {
			return serrors.Wrap(op, fmt.Errorf("native event %q is already registered with a different contract; publish a new version instead", ev.ID))
		}
		return nil
	}
	c.events[ev.ID] = res
	return nil
}

// MustRegister registers into the catalog and panics on an invalid
// declaration. For package-level declarations of native modules.
func (c *NativeCatalog) MustRegister(ev NativeEvent) NativeEvent {
	if err := c.Register(ev); err != nil {
		panic(err)
	}
	return ev
}

// RegisterNative registers into Default.
func RegisterNative(ev NativeEvent) error { return Default.Register(ev) }

// MustRegisterNative registers into Default and panics when invalid.
func MustRegisterNative(ev NativeEvent) NativeEvent { return Default.MustRegister(ev) }

// Lookup returns the registered native event.
func (c *NativeCatalog) Lookup(id string) (manifest.Resource, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	r, ok := c.events[id]
	return r, ok
}

// Resources lists every native event in ID order, ready to merge into the
// catalog next to package events.
func (c *NativeCatalog) Resources() []manifest.Resource {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]manifest.Resource, 0, len(c.events))
	for _, r := range c.events {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Resolved lists the events as catalog entries marked native.
func (c *NativeCatalog) Resolved() []spi.ResolvedResource {
	rs := c.Resources()
	out := make([]spi.ResolvedResource, len(rs))
	for i, r := range rs {
		out[i] = spi.ResolvedResource{Resource: r, Native: true}
	}
	return out
}
