package core

import (
	"context"

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
// fallback outbox dispatcher of a composition without the extension core.
//
// The bridge is contributed like wiring.ContributeConsumers (written out so
// the core module does not import the extension runtime). The fallback
// dispatcher is private to core: it is neither provided in the container nor
// started when a component declares that it serves the outbox
// (events.DispatcherOwner: pkg/extruntime/wiring, or the extension runtime
// composed directly), which then delivers the contributed consumers with its
// own settings. A process therefore runs one dispatcher.
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

	bc := builder.Context()
	if c.options.DisableNotificationWorker || c.options.DisableOutboxDispatcher ||
		(!bc.HasCapability(composition.CapabilityAPI) && !bc.HasCapability(composition.CapabilityWorker)) {
		return
	}
	composition.ContributeHooks(builder, func(container *composition.Container) ([]composition.Hook, error) {
		owners, err := composition.Contributions[events.DispatcherOwner](container)
		if err != nil {
			return nil, err
		}
		if len(owners) > 0 {
			return nil, nil // another component serves the outbox
		}
		d, err := fallbackDispatcher(container)
		if err != nil {
			return nil, err
		}
		return []composition.Hook{{Name: "core.outbox-dispatch", Start: func(ctx context.Context) (composition.StopFn, error) {
			d.Start(composition.LifecycleContext(ctx))
			return func(context.Context) error { d.Stop(); return nil }, nil
		}}}, nil
	})
}

// fallbackDispatcher delivers the contributed Go consumers of the outbox with
// the default settings. Subjects come from the container's host.SubjectLoader
// when one is provided, else from the core users.
func fallbackDispatcher(container *composition.Container) (*events.Dispatcher, error) {
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
	return events.NewDispatcher(events.Config{DB: db, Subjects: subjects, Consumers: contributedConsumers(container)})
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
