package notify

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/catalog"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Dispatcher is the explicit-send entry point of the core notification
// service; *services.NotificationDispatchService implements it.
type Dispatcher interface {
	EnqueueAddressedTx(ctx context.Context, tx pgx.Tx, req services.AddressedRequest) (services.AddressedReceipt, error)
}

// Actives lists the activated package versions; *catalog.Catalog does.
type Actives interface {
	Active(ctx context.Context) ([]catalog.ActiveVersion, error)
}

// Ensurer makes sure the definition snapshot of package@version is known to
// the notification catalog before a notice pinned to it is enqueued.
type Ensurer interface {
	Ensure(ctx context.Context, pkg, version string) error
}

// Manifests loads the manifest of any published version; manage.Service
// implements it. Versions are immutable, so a pinned version that is no
// longer active keeps resolving.
type Manifests interface {
	ManifestOf(ctx context.Context, pkg, version string) (manifest.Manifest, error)
}

// Notifier implements spi.Notifier over the core notification service.
type Notifier struct {
	dispatch  Dispatcher
	actives   Actives
	ensure    Ensurer
	manifests Manifests
}

var _ spi.Notifier = (*Notifier)(nil)

// NewNotifier builds the adapter. ensure may be nil when the definition
// source is synchronized elsewhere.
func NewNotifier(dispatch Dispatcher, actives Actives, manifests Manifests, ensure Ensurer) *Notifier {
	return &Notifier{dispatch: dispatch, actives: actives, manifests: manifests, ensure: ensure}
}

// templateOwner finds the package version whose templates render the notice:
// the caller's pinned version when it has a template for it, otherwise the
// active package that provides one.
func (n *Notifier) templateOwner(ctx context.Context, sc spi.Scope, in spi.NoticeIntent) (pkg, version string, err error) {
	active, err := n.actives.Active(ctx)
	if err != nil {
		return "", "", err
	}
	hasTemplate := func(m manifest.Manifest) bool {
		for _, t := range m.Templates {
			if t.Notice == in.Notice {
				return true
			}
		}
		return false
	}
	callerPkg, callerVer := sc.Caller.Package, in.Version
	if callerVer == "" {
		callerVer = sc.Caller.Version
	}
	if callerPkg != "" && callerVer != "" {
		for _, av := range active {
			if av.Namespace == callerPkg && av.Version == callerVer && hasTemplate(av.Manifest) {
				return callerPkg, callerVer, nil
			}
		}
		// The caller may be a pinned version that is no longer active: its
		// templates are still the ones to use.
		if mf, ok := n.pinnedManifest(ctx, callerPkg, callerVer); ok && hasTemplate(mf) {
			return callerPkg, callerVer, nil
		}
	}
	for _, av := range active {
		if hasTemplate(av.Manifest) {
			return av.Namespace, av.Version, nil
		}
	}
	return "", "", serrors.New(serrors.NotFound, "no active package provides templates for notice "+in.Notice)
}

func (n *Notifier) pinnedManifest(ctx context.Context, pkg, version string) (manifest.Manifest, bool) {
	if n.manifests == nil {
		return manifest.Manifest{}, false
	}
	mf, err := n.manifests.ManifestOf(ctx, pkg, version)
	return mf, err == nil
}

func addressee(r spi.Recipient) (notifications.Addressee, error) {
	switch r.Kind {
	case spi.RecipientUser:
		if r.UserID <= 0 {
			return notifications.Addressee{}, serrors.New(serrors.Invalid, "invalid user recipient")
		}
		return notifications.UserAddressee(uint(r.UserID)), nil
	case spi.RecipientRecord:
		if r.Record == nil {
			return notifications.Addressee{}, serrors.New(serrors.Invalid, "invalid record recipient")
		}
		return notifications.RecordAddressee(r.Record.Entity, r.Record.Key), nil
	case spi.RecipientAddress:
		addrs := make(map[notifications.Channel]string, len(r.Address))
		for name, v := range r.Address {
			c, err := coreChannel(name)
			if err != nil {
				return notifications.Addressee{}, err
			}
			addrs[c] = v
		}
		return notifications.AddressAddressee(addrs), nil
	}
	return notifications.Addressee{}, serrors.New(serrors.Invalid, "invalid recipient kind "+string(r.Kind))
}

// Enqueue queues an explicit notice in the caller's transaction: the notice,
// its recipients and the dispatch job commit with the business data and a
// rollback removes them. The notice is pinned to the template owner's
// package version (Event.SnapshotRef = package@version), so rendering after
// a later activation still uses the templates the producer saw.
func (n *Notifier) Enqueue(ctx context.Context, tx pgx.Tx, sc spi.Scope, in spi.NoticeIntent) (string, error) {
	const op = serrors.Op("extruntime.notify.Notifier.Enqueue")
	pkg, version, err := n.templateOwner(ctx, sc, in)
	if err != nil {
		return "", serrors.Wrap(op, err)
	}
	if n.ensure != nil {
		if err := n.ensure.Ensure(ctx, pkg, version); err != nil {
			return "", serrors.Wrap(op, err)
		}
	}
	recipients := make([]notifications.Addressee, 0, len(in.Recipients))
	for _, r := range in.Recipients {
		a, err := addressee(r)
		if err != nil {
			return "", serrors.Wrap(op, err)
		}
		recipients = append(recipients, a)
	}
	channels := make([]notifications.Channel, 0, len(in.Channels))
	for _, name := range in.Channels {
		c, err := coreChannel(name)
		if err != nil {
			return "", serrors.Wrap(op, err)
		}
		channels = append(channels, c)
	}
	event := notifications.Event{
		Key:         EventKey(in.Notice),
		ID:          uuid.NewString(),
		TenantID:    sc.Subject.TenantID,
		SnapshotRef: SnapshotRef(pkg, version),
		Payload:     in.Payload,
		Producer: notifications.Producer{
			InvocationID: in.Producer.InvocationID, EventID: in.Producer.EventID,
			InstanceID: in.Producer.InstanceID, Occurrence: in.Producer.Occurrence,
		},
	}
	if sc.Subject.UserID > 0 {
		event.ActorUserID = uint(sc.Subject.UserID)
	}
	if in.Subject != nil {
		event.Subject = notifications.SubjectReference{Type: in.Subject.Entity, ID: in.Subject.Key}
	}
	ctx = composables.WithTenantID(ctx, sc.Subject.TenantID)
	receipt, err := n.dispatch.EnqueueAddressedTx(ctx, tx, services.AddressedRequest{
		Event: event, Recipients: recipients, Channels: channels, Producer: event.Producer,
	})
	if err != nil {
		return "", serrors.Wrap(op, fmt.Errorf("enqueue notice %s: %w", in.Notice, err))
	}
	return receipt.NoticeID.String(), nil
}

// SetEnsurer connects the definition adapter after construction (the adapter
// needs the host, the host needs this notifier).
func (n *Notifier) SetEnsurer(e Ensurer) { n.ensure = e }
