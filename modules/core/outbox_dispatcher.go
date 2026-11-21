package core

import (
	"context"
	"sync/atomic"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	corePerms "github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
)

// buildOutboxDelivery registers the outbox → notification bridge and the
// Go-only outbox dispatcher of a composition without the extension runtime.
//
// The bridge is contributed like wiring.ContributeConsumers (written out so
// the core module does not import the extension runtime). The dispatcher is a
// default: the wiring component provides the one shared dispatcher of the
// process (with the runtime's package subscriptions) and replaces it. Core
// starts the dispatcher only when it is its own, so a process runs exactly one.
func (c *component) buildOutboxDelivery(builder *composition.Builder) {
	composition.Contribute(builder, func(container *composition.Container) ([]events.ConsumerSource, error) {
		catalog, err := composition.Resolve[*notifications.Catalog](container)
		if err != nil {
			return nil, err
		}
		dispatch, err := composition.Resolve[*services.NotificationDispatchService](container)
		if err != nil {
			return nil, err
		}
		return []events.ConsumerSource{events.StaticConsumers(services.NewNotificationOutboxBridge(catalog, dispatch))}, nil
	})

	own := &atomic.Pointer[events.Dispatcher]{}
	composition.ProvideDefault[*events.Dispatcher](builder, func(container *composition.Container) (*events.Dispatcher, error) {
		db, err := composition.Resolve[*pgxpool.Pool](container)
		if err != nil {
			return nil, err
		}
		subjects, err := composition.Resolve[host.SubjectLoader](container)
		if composition.IsNotProvided(err) {
			users, uerr := composition.Resolve[user.Repository](container)
			if uerr != nil {
				return nil, uerr
			}
			subjects, err = &host.UserSubjects{Users: users, Universe: corePerms.Permissions, Pool: db}, nil
		}
		if err != nil {
			return nil, err
		}
		d, err := events.NewDispatcher(events.Config{DB: db, Subjects: subjects, Consumers: contributedConsumers(container)})
		if err != nil {
			return nil, err
		}
		own.Store(d)
		return d, nil
	})

	bc := builder.Context()
	if c.options.DisableNotificationWorker || c.options.DisableOutboxDispatcher ||
		(!bc.HasCapability(composition.CapabilityAPI) && !bc.HasCapability(composition.CapabilityWorker)) {
		return
	}
	composition.ContributeHooks(builder, func(container *composition.Container) ([]composition.Hook, error) {
		d, err := composition.Resolve[*events.Dispatcher](container)
		if err != nil {
			return nil, err
		}
		if d != own.Load() {
			return nil, nil // the wiring component owns and starts the dispatcher
		}
		return []composition.Hook{{Name: "core.outbox-dispatch", Start: func(ctx context.Context) (composition.StopFn, error) {
			d.Start(composition.LifecycleContext(ctx))
			return func(context.Context) error { d.Stop(); return nil }, nil
		}}}, nil
	})
}

// contributedConsumers reads the contributed consumer sources on every pass.
func contributedConsumers(container *composition.Container) events.ConsumerSource {
	return events.ConsumerSourceFunc(func(ctx context.Context) ([]events.Consumer, error) {
		srcs, err := composition.Contributions[events.ConsumerSource](container)
		if err != nil {
			return nil, err
		}
		return events.ComposeConsumers(srcs...).Consumers(ctx)
	})
}
