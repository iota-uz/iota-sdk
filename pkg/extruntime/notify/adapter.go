package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/notification"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/catalog"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Runner executes exports in Node workers; *host.Host implements it.
type Runner interface {
	Run(ctx context.Context, req host.RunRequest) (host.Result, error)
}

// Artifacts resolves a published package version to its artifact directory;
// manage.Service implements it (and host.ArtifactResolver).
type Artifacts interface {
	ResolveArtifact(ctx context.Context, pkg, version string) (pool.Artifact, []string, error)
}

// Owners returns the owner-user of a trigger in a tenant.
type Owners interface {
	OwnerOf(ctx context.Context, tenantID uuid.UUID, triggerID string) (userID int64, ok bool, err error)
}

// SubjectResolver loads the current rights of a user: contact resolvers run
// as their trigger owner with those rights, never as a service principal.
type SubjectResolver interface {
	ResolveSubject(ctx context.Context, tenantID uuid.UUID, userID int64) (spi.Subject, error)
}

// Published lists the package@version references of every published version
// that has notices or templates, oldest first. A restarted process registers
// them as retained snapshots so notices queued before the restart (and
// notices of workflow instances pinned to older versions) keep resolving.
type Published interface {
	PublishedRefs(ctx context.Context) ([]string, error)
}

// Config wires the Adapter.
type Config struct {
	Catalog   *catalog.Catalog
	Notices   *notifications.Catalog
	Renderers *notifications.ContentRenderers
	Contacts  *notifications.ContactResolvers
	// Policies is the delivery policy composition of the notification
	// platform; the adapter installs the policies of active packages on it.
	Policies  *notifications.DeliveryPolicies
	Runner    Runner
	Artifacts Artifacts
	Manifests Manifests
	Owners    Owners
	Subjects  SubjectResolver
	Published Published
	Logger    *slog.Logger
}

// Adapter keeps the notification catalog, the content renderers and the
// contact resolver registry in step with the activated packages.
type Adapter struct {
	cfg Config
	log *slog.Logger

	mu         sync.Mutex
	registered map[string]bool   // package@version snapshots known to the catalog
	current    map[string]string // package -> version whose source is active
	renderers  map[string]bool   // key@ref renderers registered
	contacts   map[string]bool   // entity resolvers registered
	retained   bool
}

// NewAdapter validates the wiring.
func NewAdapter(cfg Config) (*Adapter, error) {
	switch {
	case cfg.Catalog == nil, cfg.Notices == nil, cfg.Renderers == nil, cfg.Runner == nil, cfg.Artifacts == nil, cfg.Manifests == nil:
		return nil, serrors.New(serrors.Invalid, "extruntime notify: Catalog, Notices, Renderers, Runner, Artifacts and Manifests are required")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	a := &Adapter{cfg: cfg, log: cfg.Logger, registered: map[string]bool{}, current: map[string]string{},
		renderers: map[string]bool{}, contacts: map[string]bool{}}
	if cfg.Policies != nil {
		if err := cfg.Policies.Set(PolicyName, &deliveryPolicy{a: a}); err != nil {
			return nil, serrors.Wrap("extruntime.notify.NewAdapter", err)
		}
	}
	return a, nil
}

// definitionSource implements notifications.DefinitionSource for one package
// version.
type definitionSource struct {
	name string
	defs []notifications.Definition
}

func (s *definitionSource) Name() string { return s.name }
func (s *definitionSource) Definitions(context.Context) ([]notifications.Definition, error) {
	return s.defs, nil
}

// noticeOf describes a notice a package version provides.
type noticeOf struct {
	resource manifest.Resource
	template *manifest.Template
}

// notices lists what a package version contributes: every notice it renders
// (a template for it) and every notice it declares.
func (a *Adapter) notices(ctx context.Context, m manifest.Manifest) ([]noticeOf, error) {
	byID := map[string]*noticeOf{}
	var order []string
	for _, r := range m.Resources {
		if r.Kind == manifest.ResNotice {
			byID[r.ID] = &noticeOf{resource: r}
			order = append(order, r.ID)
		}
	}
	for i := range m.Templates {
		t := m.Templates[i]
		n, ok := byID[t.Notice]
		if !ok {
			res, found, err := a.cfg.Catalog.Resolve(ctx, t.Notice)
			if err != nil {
				return nil, err
			}
			if !found || res.Resource.Kind != manifest.ResNotice {
				return nil, serrors.New(serrors.FailedPrecondition, "template of unknown notice "+t.Notice)
			}
			n = &noticeOf{resource: res.Resource}
			byID[t.Notice] = n
			order = append(order, t.Notice)
		}
		n.template = &t
	}
	out := make([]noticeOf, 0, len(order))
	for _, id := range order {
		out = append(out, *byID[id])
	}
	return out, nil
}

func label(r manifest.Resource) map[string]string {
	out := map[string]string{}
	for k, v := range r.Label {
		out[k] = v
	}
	if out["en"] == "" {
		out["en"] = r.ID
	}
	return out
}

func (a *Adapter) definitions(ctx context.Context, ns, version string, m manifest.Manifest) ([]notifications.Definition, error) {
	ns2, err := a.notices(ctx, m)
	if err != nil {
		return nil, err
	}
	source := SourceName(ns)
	ref := SnapshotRef(ns, version)
	defs := make([]notifications.Definition, 0, len(ns2))
	seen := map[string]string{}
	for _, n := range ns2 {
		key := EventKey(n.resource.ID)
		if other, dup := seen[key]; dup {
			return nil, serrors.New(serrors.FailedPrecondition, fmt.Sprintf("notices %s and %s map to the same catalog key %s", other, n.resource.ID, key))
		}
		seen[key] = n.resource.ID
		payload := n.resource.Payload
		desc := map[string]string{}
		if n.resource.Description != "" {
			desc["en"] = n.resource.Description
		}
		d := notifications.Definition{
			Module: ns, Key: key, Name: label(n.resource), Description: desc,
			DefaultLevel: notification.LevelInfo, Source: source, SnapshotRef: ref,
		}
		if payload != nil {
			p := *payload
			d.ValidatePayload = func(e notifications.Event) error {
				if len(e.Payload) == 0 {
					return fmt.Errorf("missing notification payload")
				}
				return manifest.ValidateValue(p, e.Payload)
			}
		}
		defs = append(defs, d)
	}
	return defs, nil
}

// register makes package@version known: its definitions (activate = visible
// in the active catalog), its renderers and its contact resolvers.
func (a *Adapter) register(ctx context.Context, ns, version string, activate bool) error {
	ref := SnapshotRef(ns, version)
	m, err := a.cfg.Manifests.ManifestOf(ctx, ns, version)
	if err != nil {
		return err
	}
	defs, err := a.definitions(ctx, ns, version, m)
	if err != nil {
		return err
	}
	if !a.registered[ref] {
		// Notices another source already publishes stay with that source.
		var fresh []notifications.Definition
		for _, d := range defs {
			if _, taken := a.cfg.Notices.Get(d.Key); taken && activate {
				a.log.Warn("extruntime notify: notice key already published, skipping", "key", d.Key, "package", ns)
				continue
			}
			fresh = append(fresh, d)
		}
		if len(fresh) > 0 {
			if err := a.cfg.Notices.RegisterSource(ctx, &definitionSource{name: SourceName(ns), defs: fresh}); err != nil {
				return serrors.Wrap("extruntime.notify.Adapter.register", err)
			}
		}
		a.registered[ref] = true
		if !activate && a.current[ns] == "" {
			// A retained snapshot only serves queued notices.
			a.cfg.Notices.Deactivate(SourceName(ns))
		}
	} else if activate {
		a.log.Debug("extruntime notify: snapshot was published before, queued notices keep resolving it", "ref", ref)
	}
	for _, t := range m.Templates {
		key := EventKey(t.Notice)
		id := key + "@" + ref
		if a.renderers[id] {
			continue
		}
		if err := a.cfg.Renderers.Register(key, ref, &renderer{a: a, pkg: ns, version: version, notice: t.Notice, template: t}); err != nil {
			return serrors.Wrap("extruntime.notify.Adapter.register", err)
		}
		a.renderers[id] = true
	}
	if activate {
		if err := a.registerContacts(ctx, m); err != nil {
			return err
		}
	}
	return nil
}

func (a *Adapter) registerContacts(ctx context.Context, m manifest.Manifest) error {
	if a.cfg.Contacts == nil {
		return nil
	}
	for _, i := range m.Implementations {
		if i.Kind != manifest.ImplContactResolver {
			continue
		}
		res, found, err := a.cfg.Catalog.Resolve(ctx, i.Implements)
		if err != nil {
			return err
		}
		if !found || res.Resource.Kind != manifest.ResContactResolver || res.Resource.Subject == "" {
			continue
		}
		entity := res.Resource.Subject
		if a.contacts[entity] {
			continue
		}
		if err := a.cfg.Contacts.Register(entity, &contactResolver{a: a, entity: entity}); err != nil {
			a.log.Warn("extruntime notify: contact resolver not registered", "entity", entity, "error", err)
			continue
		}
		a.contacts[entity] = true
	}
	return nil
}

// Ensure makes package@version resolvable by the notification catalog.
func (a *Adapter) Ensure(ctx context.Context, pkg, version string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.registered[SnapshotRef(pkg, version)] {
		return nil
	}
	active := a.current[pkg] == version
	return a.register(ctx, pkg, version, active)
}

// Sync reconciles the notification catalog with the activated packages:
// new versions are published under their package@version snapshot, removed
// or replaced ones are deactivated (their snapshots and renderers stay for
// notices already queued).
func (a *Adapter) Sync(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	active, err := a.cfg.Catalog.Active(ctx)
	if err != nil {
		return err
	}
	if !a.retained && a.cfg.Published != nil {
		refs, err := a.cfg.Published.PublishedRefs(ctx)
		if err != nil {
			return err
		}
		live := map[string]bool{}
		for _, av := range active {
			live[SnapshotRef(av.Namespace, av.Version)] = true
		}
		for _, ref := range refs {
			pkg, ver, ok := SplitRef(ref)
			if !ok || a.registered[ref] || live[ref] {
				continue
			}
			if err := a.register(ctx, pkg, ver, false); err != nil {
				a.log.Warn("extruntime notify: cannot retain snapshot", "ref", ref, "error", err)
			}
		}
		a.retained = true
	}
	now := map[string]string{}
	var errs []error
	for _, av := range active {
		now[av.Namespace] = av.Version
		if a.current[av.Namespace] == av.Version {
			continue
		}
		if _, had := a.current[av.Namespace]; had {
			a.cfg.Notices.Deactivate(SourceName(av.Namespace))
		}
		if err := a.register(ctx, av.Namespace, av.Version, true); err != nil {
			errs = append(errs, fmt.Errorf("%s@%s: %w", av.Namespace, av.Version, err))
			continue
		}
		a.current[av.Namespace] = av.Version
	}
	for pkg := range a.current {
		if _, still := now[pkg]; !still {
			a.cfg.Notices.Deactivate(SourceName(pkg))
			delete(a.current, pkg)
		}
	}
	return errors.Join(errs...)
}

// Run follows activations until ctx is done: it syncs once, then after every
// catalog change.
func (a *Adapter) Run(ctx context.Context) {
	kick := make(chan struct{}, 1)
	a.cfg.Catalog.OnChange(func() {
		select {
		case kick <- struct{}{}:
		default:
		}
	})
	for {
		var retry <-chan time.Time
		if err := a.Sync(ctx); err != nil && ctx.Err() == nil {
			a.log.Error("extruntime notify: sync failed", "error", err)
			retry = time.After(5 * time.Second)
		}
		select {
		case <-ctx.Done():
			return
		case <-kick:
		case <-retry:
		}
	}
}

// ---- rendering ----

type renderer struct {
	a        *Adapter
	pkg      string
	version  string
	notice   string
	template manifest.Template
}

func permanent(err error) error {
	return fmt.Errorf("%w: %w", notifications.ErrPermanent, err)
}

type rendered struct {
	Title     string            `json:"title"`
	Body      string            `json:"body"`
	ActionURL string            `json:"actionUrl"`
	Subject   string            `json:"subject"`
	HTML      string            `json:"html"`
	Text      string            `json:"text"`
	Data      map[string]string `json:"data"`
}

func triggerFor(p notifications.Producer, ref string) host.Trigger {
	switch {
	case p.InstanceID != "":
		return host.Trigger{Kind: host.TriggerWorkflow, Ref: ref}
	case p.EventID != "":
		return host.Trigger{Kind: host.TriggerSubscription, Ref: ref}
	}
	return host.Trigger{Kind: host.TriggerUI, Ref: ref}
}

// Render runs the package's template for one channel in the Node pool. It is
// called by the notification worker after the producer committed and never
// inside a write transaction.
func (r *renderer) Render(ctx context.Context, req notifications.RenderRequest) (notifications.ChannelContent, error) {
	const op = serrors.Op("extruntime.notify.renderer.Render")
	channel := templateChannel(req.Channel)
	supported := false
	for _, c := range r.template.Channels {
		if c == channel {
			supported = true
		}
	}
	if !supported {
		return notifications.ChannelContent{}, permanent(fmt.Errorf("%s has no %s template", r.notice, channel))
	}
	art, secrets, err := r.a.cfg.Artifacts.ResolveArtifact(ctx, r.pkg, r.version)
	if err != nil {
		return notifications.ChannelContent{}, serrors.Wrap(op, err)
	}
	var subject any
	if !req.Event.Subject.IsZero() {
		subject = spi.ObjectRef{Entity: req.Event.Subject.Type, Key: req.Event.Subject.ID}
	}
	var payload any
	if len(req.Event.Payload) > 0 {
		payload = req.Event.Payload
	}
	input, err := json.Marshal(map[string]any{"channel": channel, "payload": payload, "subject": subject, "locale": req.Language})
	if err != nil {
		return notifications.ChannelContent{}, serrors.Wrap(op, err)
	}
	actor := int64(req.Event.ActorUserID)
	res, err := r.a.cfg.Runner.Run(ctx, host.RunRequest{
		Export: host.Export{
			Kind: pool.KindTemplate, Resource: r.notice,
			Binding: host.Binding{Contract: r.notice, Package: r.pkg, Version: r.version, Export: r.template.Export, Artifact: art, Secrets: secrets},
		},
		Input:   input,
		Subject: spi.Subject{TenantID: req.Event.TenantID, UserID: actor, Kind: spi.SubjectInitiator, Locale: req.Language},
		Trigger: triggerFor(req.Event.Producer, "notice:"+req.Event.Key),
	})
	if err != nil {
		// A template that throws or is missing fails the same way again.
		if _, isWire := pool.AsWireError(err); isWire {
			return notifications.ChannelContent{}, permanent(err)
		}
		return notifications.ChannelContent{}, serrors.Wrap(op, err)
	}
	return decodeContent(req.Channel, res.Output)
}

func decodeContent(channel notifications.Channel, raw json.RawMessage) (notifications.ChannelContent, error) {
	var out rendered
	trimmed := strings.TrimSpace(string(raw))
	if strings.HasPrefix(trimmed, `"`) {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return notifications.ChannelContent{}, permanent(err)
		}
		out.Text, out.Body = s, s
	} else if err := json.Unmarshal(raw, &out); err != nil {
		return notifications.ChannelContent{}, permanent(fmt.Errorf("template result is not an object or string: %w", err))
	}
	content := notifications.ChannelContent{Title: out.Title, Body: out.Body, ActionURL: out.ActionURL, Subject: out.Subject, HTML: out.HTML, Text: out.Text, Data: out.Data}
	switch channel {
	case notifications.ChannelSMS:
		if content.Text == "" {
			content.Text = content.Body
		}
		if content.Text == "" {
			return notifications.ChannelContent{}, permanent(errors.New("sms template returned no text"))
		}
	case notifications.ChannelEmail:
		if content.Subject == "" || (content.HTML == "" && content.Text == "") {
			return notifications.ChannelContent{}, permanent(errors.New("email template needs a subject and html or text"))
		}
	case notifications.ChannelInApp, notifications.ChannelPush:
		if content.Title == "" && content.Body == "" {
			return notifications.ChannelContent{}, permanent(errors.New("template returned no title or body"))
		}
	}
	return content, nil
}

// ---- contact resolvers ----

type contactResolver struct {
	a      *Adapter
	entity string
}

type resolverImpl struct {
	resolverID string
	pkg        string
	version    string
	export     string
}

// find returns the active implementation of the entity's contact resolver.
func (c *contactResolver) find(ctx context.Context) (resolverImpl, bool, error) {
	active, err := c.a.cfg.Catalog.Active(ctx)
	if err != nil {
		return resolverImpl{}, false, err
	}
	for _, av := range active {
		for _, i := range av.Manifest.Implementations {
			if i.Kind != manifest.ImplContactResolver {
				continue
			}
			res, found, err := c.a.cfg.Catalog.Resolve(ctx, i.Implements)
			if err != nil {
				return resolverImpl{}, false, err
			}
			if found && res.Resource.Kind == manifest.ResContactResolver && res.Resource.Subject == c.entity {
				return resolverImpl{resolverID: i.Implements, pkg: av.Namespace, version: av.Version, export: i.Export}, true, nil
			}
		}
	}
	return resolverImpl{}, false, nil
}

// ResolveContact implements notifications.ContactResolver by running the
// package's resolver as the tenant's owner of that resolver.
func (c *contactResolver) ResolveContact(ctx context.Context, tenantID uuid.UUID, ref notifications.ObjectRef) (notifications.Contact, error) {
	const op = serrors.Op("extruntime.notify.contactResolver.ResolveContact")
	impl, ok, err := c.find(ctx)
	if err != nil {
		return notifications.Contact{}, serrors.Wrap(op, err)
	}
	if !ok {
		return notifications.Contact{}, notifications.ErrContactNotFound
	}
	if c.a.cfg.Owners == nil || c.a.cfg.Subjects == nil {
		return notifications.Contact{}, serrors.New(serrors.FailedPrecondition, "contact resolvers need trigger owners").WithOp(op)
	}
	owner, has, err := c.a.cfg.Owners.OwnerOf(ctx, tenantID, impl.resolverID)
	if err != nil {
		return notifications.Contact{}, serrors.Wrap(op, err)
	}
	if !has {
		return notifications.Contact{}, serrors.New(serrors.FailedPrecondition, "no owner is assigned to contact resolver "+impl.resolverID).WithOp(op)
	}
	subject, err := c.a.cfg.Subjects.ResolveSubject(ctx, tenantID, owner)
	if err != nil {
		return notifications.Contact{}, serrors.Wrap(op, err)
	}
	subject.Kind = spi.SubjectOwner
	art, secrets, err := c.a.cfg.Artifacts.ResolveArtifact(ctx, impl.pkg, impl.version)
	if err != nil {
		return notifications.Contact{}, serrors.Wrap(op, err)
	}
	input, err := json.Marshal(spi.ObjectRef{Entity: ref.Entity, Key: ref.Key})
	if err != nil {
		return notifications.Contact{}, serrors.Wrap(op, err)
	}
	res, err := c.a.cfg.Runner.Run(ctx, host.RunRequest{
		Export: host.Export{
			Kind: pool.KindQuery, Resource: impl.resolverID,
			Binding: host.Binding{Contract: impl.resolverID, Package: impl.pkg, Version: impl.version, Export: impl.export, Artifact: art, Secrets: secrets},
		},
		Input: input, Subject: subject,
		Trigger: host.Trigger{Kind: host.TriggerSubscription, Ref: "contact:" + impl.resolverID},
	})
	if err != nil {
		return notifications.Contact{}, serrors.Wrap(op, err)
	}
	var targets map[string][]string
	if err := json.Unmarshal(res.Output, &targets); err != nil {
		return notifications.Contact{}, serrors.New(serrors.Invalid, "contact resolver result is not a target map").WithOp(op).WithCause(err)
	}
	contact := notifications.Contact{}
	for _, name := range []string{"sms", "email", "push"} {
		ch, _ := coreChannel(name)
		for _, addr := range targets[name] {
			if strings.TrimSpace(addr) != "" {
				contact.Targets = append(contact.Targets, notifications.Target{Channel: ch, Address: addr})
			}
		}
	}
	if len(contact.Targets) == 0 {
		return notifications.Contact{}, notifications.ErrContactNotFound
	}
	return contact, nil
}
