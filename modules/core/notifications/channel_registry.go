package notifications

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/composition"
)

// AddressSource says where the worker finds a user's address on a channel.
type AddressSource string

const (
	// AddressResolverEmail asks the AddressResolver with ChannelEmail.
	AddressResolverEmail AddressSource = "email"
	// AddressResolverPhone asks the AddressResolver with ChannelSMS.
	AddressResolverPhone AddressSource = "phone"
	// AddressPushDevices reads the push device registry.
	AddressPushDevices AddressSource = "push"
	// AddressLinked reads the user's link (ChannelLinkRepository).
	AddressLinked AddressSource = "linked"
	// AddressNone needs no address (in-app).
	AddressNone AddressSource = "none"
)

// ChannelSpec describes one channel of the registry.
type ChannelSpec struct {
	Channel Channel
	// Label names the channel per language (en, ru, uz, uz-Cyrl, ...).
	Label map[string]string
	// Order is the position of the channel in pickers.
	Order   int
	Address AddressSource
	// OptIn channels are off for a user until the user turns them on.
	OptIn bool
	// ConnectorKind is the kind of the connector whose connection the channel
	// sends through ("" for the built-in channels). A tenant without such a
	// connection and without an enabled platform credential cannot use it.
	ConnectorKind string
	// Summary is what the delivery's title and body columns keep of the
	// rendered content; nil keeps its title and body.
	Summary func(ChannelContent) (title, body string)
	// LinkURL is where a user confirms a link for an AddressLinked channel: the
	// one-use token through the connection. Required for AddressLinked.
	LinkURL func(ctx context.Context, tenantID uuid.UUID, connectionID, token string) (string, error)
}

// LabelFor names the channel in a language, falling back to the base
// language, English, and the channel name.
func (s ChannelSpec) LabelFor(language string) string {
	if v := s.Label[language]; v != "" {
		return v
	}
	if base, _, ok := strings.Cut(language, "-"); ok {
		if v := s.Label[base]; v != "" {
			return v
		}
	}
	if v := s.Label["en"]; v != "" {
		return v
	}
	return string(s.Channel)
}

// Validate checks the spec before it is registered.
func (s ChannelSpec) Validate() error {
	switch {
	case !s.Channel.Valid():
		return fmt.Errorf("notification channel %q: the name must match %s", s.Channel, channelPattern)
	case s.Address == "":
		return fmt.Errorf("notification channel %q: no address source", s.Channel)
	case s.Address == AddressLinked && s.LinkURL == nil:
		return fmt.Errorf("notification channel %q: a linked channel needs LinkURL", s.Channel)
	case s.Channel == ChannelInApp && s.Address != AddressNone:
		return fmt.Errorf("notification channel %q is the in-app channel", s.Channel)
	}
	return nil
}

// summary is the title and body a delivery of the channel keeps.
func (s ChannelSpec) summary(content ChannelContent) (string, string) {
	if s.Summary != nil {
		return s.Summary(content)
	}
	return content.Summary(s.Channel)
}

// ChannelRegistry is the open set of channels of an application. Built-in
// channels are entries like any other.
type ChannelRegistry interface {
	// Register adds a channel and its adapter; a channel twice is an error.
	Register(ChannelSpec, ChannelAdapter) error
	// Lookup returns a registered channel; the adapter is nil when none is
	// installed (in-app, or a built-in provider the application did not configure).
	Lookup(Channel) (ChannelSpec, ChannelAdapter, bool)
	// All lists the registered channels by Order.
	All() []ChannelSpec
	// ForTenant lists the channels the tenant can use now: in-app, the
	// channels with an installed adapter and, for a connector-backed channel, a
	// connection or an enabled platform credential.
	ForTenant(ctx context.Context, tenantID uuid.UUID) ([]ChannelSpec, error)
}

// ChannelConnections picks the connection a connector-backed channel sends
// through for a tenant: the one the tenant chose, else the platform credential
// enabled for it. ok is false when there is neither.
type ChannelConnections interface {
	Connection(ctx context.Context, tenantID uuid.UUID, spec ChannelSpec) (connectionID string, ok bool, err error)
}

// SummaryFor is the title and body a delivery of the channel keeps.
func SummaryFor(registry ChannelRegistry, channel Channel, content ChannelContent) (string, string) {
	if registry != nil {
		if spec, _, ok := registry.Lookup(channel); ok {
			return spec.summary(content)
		}
	}
	return content.Summary(channel)
}

// ChannelContribution is a channel a component adds to the registry.
type ChannelContribution struct {
	Spec    ChannelSpec
	Adapter ChannelAdapter
}

// ContributeChannel adds a channel to the application's registry. A channel
// registered twice fails the composition.
func ContributeChannel(b *composition.Builder, spec ChannelSpec, adapter ChannelAdapter) {
	composition.Contribute(b, func(*composition.Container) ([]ChannelContribution, error) {
		return []ChannelContribution{{Spec: spec, Adapter: adapter}}, nil
	})
}

// ContributeChannels adds channels built from the container (an adapter that
// needs other providers).
func ContributeChannels(b *composition.Builder, factory func(*composition.Container) ([]ChannelContribution, error)) {
	composition.Contribute(b, factory)
}

// BuiltinChannelSpecs are the specs of the in-app, email, SMS and push channels.
func BuiltinChannelSpecs() []ChannelSpec {
	return []ChannelSpec{
		{Channel: ChannelInApp, Order: 0, Address: AddressNone, Label: map[string]string{
			"en": "In the app", "ru": "В приложении", "uz": "Ilovada", "uz-Cyrl": "Иловада", "pt-BR": "No aplicativo", "zh": "应用内",
		}, Summary: func(c ChannelContent) (string, string) { return c.Title, c.Body }},
		{Channel: ChannelEmail, Order: 10, Address: AddressResolverEmail, Label: map[string]string{
			"en": "Email", "ru": "Эл. почта", "uz": "E-pochta", "uz-Cyrl": "Э-почта", "pt-BR": "E-mail", "zh": "电子邮件",
		}, Summary: func(c ChannelContent) (string, string) {
			return firstNonEmpty(c.Subject, c.Title), firstNonEmpty(c.Text, c.Body)
		}},
		{Channel: ChannelSMS, Order: 20, Address: AddressResolverPhone, Label: map[string]string{
			"en": "SMS", "ru": "СМС", "uz": "SMS", "uz-Cyrl": "SMS", "pt-BR": "SMS", "zh": "短信",
		}, Summary: func(c ChannelContent) (string, string) { return "", firstNonEmpty(c.Text, c.Body) }},
		{Channel: ChannelPush, Order: 30, Address: AddressPushDevices, Label: map[string]string{
			"en": "Push", "ru": "Push-уведомление", "uz": "Push-bildirishnoma", "uz-Cyrl": "Push-билдиришнома", "pt-BR": "Notificação push", "zh": "推送",
		}, Summary: func(c ChannelContent) (string, string) { return c.Title, c.Body }},
	}
}

type registryEntry struct {
	spec    ChannelSpec
	adapter ChannelAdapter
}

// Registry is the ChannelRegistry of the core module. Contributed channels
// are loaded once, on first use (or by Load at start).
type Registry struct {
	mu          sync.RWMutex
	entries     map[Channel]registryEntry
	connections ChannelConnections
	source      func() ([]ChannelContribution, error)
	once        sync.Once
	loadErr     error
}

var _ ChannelRegistry = (*Registry)(nil)

// NewChannelRegistry is a registry with the built-in channels and the given
// provider adapters of built-in channels.
func NewChannelRegistry(adapters ...ChannelAdapter) (*Registry, error) {
	r := &Registry{entries: map[Channel]registryEntry{}}
	for _, spec := range BuiltinChannelSpecs() {
		r.entries[spec.Channel] = registryEntry{spec: spec}
	}
	for _, a := range adapters {
		if a == nil {
			continue
		}
		if err := r.Register(ChannelSpec{Channel: a.Channel()}, a); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// WithConnections sets how connector-backed channels find a tenant's connection.
func (r *Registry) WithConnections(c ChannelConnections) *Registry {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.connections = c
	return r
}

// WithSource sets where contributed channels come from; they are registered
// once, on first use or by Load.
func (r *Registry) WithSource(source func() ([]ChannelContribution, error)) *Registry {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.source = source
	return r
}

// Load registers the contributed channels now and reports a conflict.
func (r *Registry) Load() error {
	r.once.Do(func() {
		r.mu.RLock()
		source := r.source
		r.mu.RUnlock()
		if source == nil {
			return
		}
		list, err := source()
		if err != nil {
			r.loadErr = err
			return
		}
		var errs []error
		for _, c := range list {
			if err := r.Register(c.Spec, c.Adapter); err != nil {
				errs = append(errs, err)
			}
		}
		r.loadErr = errors.Join(errs...)
	})
	return r.loadErr
}

// Register adds a channel. A built-in channel without a provider takes the
// adapter (its spec stays the built-in one); any other repeat is an error.
func (r *Registry) Register(spec ChannelSpec, adapter ChannelAdapter) error {
	if adapter != nil && spec.Channel == "" {
		spec.Channel = adapter.Channel()
	}
	if adapter != nil && adapter.Channel() != spec.Channel {
		return fmt.Errorf("notification channel %q: the adapter is for %q", spec.Channel, adapter.Channel())
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.entries[spec.Channel]; ok {
		if existing.spec.Channel.Builtin() && existing.adapter == nil && adapter != nil && spec.Channel != ChannelInApp {
			existing.adapter = adapter
			r.entries[spec.Channel] = existing
			return nil
		}
		return fmt.Errorf("notification channel %q is registered twice", spec.Channel)
	}
	if err := spec.Validate(); err != nil {
		return err
	}
	if adapter == nil {
		return fmt.Errorf("notification channel %q: no adapter", spec.Channel)
	}
	r.entries[spec.Channel] = registryEntry{spec: spec, adapter: adapter}
	return nil
}

func (r *Registry) Lookup(c Channel) (ChannelSpec, ChannelAdapter, bool) {
	_ = r.Load()
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.entries[c]
	return e.spec, e.adapter, ok
}

func (r *Registry) All() []ChannelSpec {
	_ = r.Load()
	r.mu.RLock()
	out := make([]ChannelSpec, 0, len(r.entries))
	for _, e := range r.entries {
		out = append(out, e.spec)
	}
	r.mu.RUnlock()
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Order != out[j].Order {
			return out[i].Order < out[j].Order
		}
		return out[i].Channel < out[j].Channel
	})
	return out
}

// Installed lists the channels with an adapter (and in-app), by Order,
// whatever the tenant.
func (r *Registry) Installed() []ChannelSpec {
	var out []ChannelSpec
	for _, spec := range r.All() {
		if _, adapter, _ := r.Lookup(spec.Channel); adapter != nil || spec.Channel == ChannelInApp {
			out = append(out, spec)
		}
	}
	return out
}

func (r *Registry) ForTenant(ctx context.Context, tenantID uuid.UUID) ([]ChannelSpec, error) {
	if err := r.Load(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	connections := r.connections
	r.mu.RUnlock()
	installed := r.Installed()
	out := make([]ChannelSpec, 0, len(installed))
	for _, spec := range installed {
		if spec.ConnectorKind != "" {
			if connections == nil {
				continue
			}
			_, ok, err := connections.Connection(ctx, tenantID, spec)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
		}
		out = append(out, spec)
	}
	return out, nil
}

// OptInChannels lists the registered opt-in channels.
func (r *Registry) OptInChannels() []Channel {
	var out []Channel
	for _, spec := range r.All() {
		if spec.OptIn {
			out = append(out, spec.Channel)
		}
	}
	return out
}

// Connection is the connection a connector-backed channel sends through for
// the tenant now (see ChannelConnections).
func (r *Registry) Connection(ctx context.Context, tenantID uuid.UUID, channel Channel) (string, bool, error) {
	spec, _, ok := r.Lookup(channel)
	if !ok || spec.ConnectorKind == "" {
		return "", false, nil
	}
	r.mu.RLock()
	connections := r.connections
	r.mu.RUnlock()
	if connections == nil {
		return "", false, nil
	}
	return connections.Connection(ctx, tenantID, spec)
}
